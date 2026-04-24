package service

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/ent/accountgroup"
	"github.com/Wei-Shaw/sub2api/ent/channelhealthsample"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// Defaults for the sparse active prober. The prober fills in (account, group,
// model) combos that saw no real traffic in the recent window so the public
// status page heartbeat bar stays populated.
const (
	// proberDefaultBudget is the max number of probes per tick.
	proberDefaultBudget = 200
	// proberDefaultColdFreshness is the "if we have a sample newer than this,
	// the combo is not cold" threshold.
	proberDefaultColdFreshness = 5 * time.Minute
	// proberTickBudget caps total tick wall-clock (leave 1 min buffer for 5-min cron).
	proberTickBudget = 4 * time.Minute
	// proberPerProbeTimeout bounds the single upstream call.
	proberPerProbeTimeout = 30 * time.Second
)

// channelProbeExecutor is the narrow dependency the prober actually needs.
// In production this is adapted from *AccountTestService (see
// accountTestProbeExecutor below); in tests a fake is injected via
// WithProbeExecutor.
type channelProbeExecutor interface {
	// ProbeOnce issues a minimal upstream call for (accountID, model) and
	// returns the HTTP status code equivalent + observed latency.
	ProbeOnce(ctx context.Context, accountID int64, model string) (statusCode int, latencyMs int, err error)
}

// accountTestProbeExecutor adapts *AccountTestService to channelProbeExecutor
// by invoking RunTestBackground and mapping the coarse "success/failed" result
// back into a status code that recorderOutcome can bucket.
type accountTestProbeExecutor struct {
	tester *AccountTestService
}

// ProbeOnce runs the service's background test path (no real http writer,
// cheap SSE buffer) and translates the result into a status code the recorder
// can map. RunTestBackground already reuses the same code paths as the manual
// test UI (which is what operators trust), so we lean on it instead of
// duplicating upstream call plumbing.
func (a *accountTestProbeExecutor) ProbeOnce(ctx context.Context, accountID int64, model string) (int, int, error) {
	if a == nil || a.tester == nil {
		return 0, 0, errors.New("channel_health_prober: tester is nil")
	}
	res, err := a.tester.RunTestBackground(ctx, accountID, model)
	if err != nil {
		return 0, 0, err
	}
	if res == nil {
		return 0, 0, errors.New("channel_health_prober: nil result")
	}
	latency := int(res.LatencyMs)
	if res.Status == "success" {
		return 200, latency, nil
	}
	// Coarse failure: map to a generic 500 so recorder counts it as error
	// (not rate-limited / overloaded). mapStatusToOutcome treats anything
	// outside {200..299, 429, 529} as OutcomeError.
	msg := res.ErrorMessage
	if msg == "" {
		msg = "probe failed"
	}
	// Preserve hints for 429 / 529 in error message — cheap string scan keeps
	// this resilient to future error-format tweaks without a full parser.
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, " 429") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "rate-limit"):
		return 429, latency, nil
	case strings.Contains(lower, " 529") || strings.Contains(lower, "overload"):
		return 529, latency, nil
	default:
		return 500, latency, nil
	}
}

// ChannelHealthProber is a sparse active prober. On each tick it enumerates
// cold (account × group × model) combos (no wildcard model patterns, no
// recent passive sample) and runs up to `budget` minimum-cost probes. Results
// are recorded via ChannelHealthRecorder with Source=SourceActiveProbe.
type ChannelHealthProber struct {
	entClient     *dbent.Client
	recorder      *ChannelHealthRecorder
	executor      channelProbeExecutor
	budget        int
	coldFreshness time.Duration
	// now abstracted for tests that need deterministic "recent" windows.
	nowFn func() time.Time
}

// NewChannelHealthProber wires the prober. A nil recorder or tester makes
// RunTick a no-op so the scheduled cron can always safely call it even in
// partially-configured dev environments.
func NewChannelHealthProber(entClient *dbent.Client, recorder *ChannelHealthRecorder, tester *AccountTestService) *ChannelHealthProber {
	p := &ChannelHealthProber{
		entClient:     entClient,
		recorder:      recorder,
		budget:        proberDefaultBudget,
		coldFreshness: proberDefaultColdFreshness,
		nowFn:         func() time.Time { return time.Now().UTC() },
	}
	if tester != nil {
		p.executor = &accountTestProbeExecutor{tester: tester}
	}
	return p
}

// WithProbeExecutor overrides the production executor. Tests use this to
// inject a fake that skips real HTTP calls.
func (p *ChannelHealthProber) WithProbeExecutor(exec channelProbeExecutor) *ChannelHealthProber {
	if p == nil {
		return p
	}
	p.executor = exec
	return p
}

// WithBudget overrides the per-tick probe budget. Primarily for tests.
func (p *ChannelHealthProber) WithBudget(n int) *ChannelHealthProber {
	if p == nil || n <= 0 {
		return p
	}
	p.budget = n
	return p
}

// WithColdFreshness overrides the "recent sample" window. Tests use small
// values (e.g. 2 minutes) so they can craft deterministic cold/warm combos.
func (p *ChannelHealthProber) WithColdFreshness(d time.Duration) *ChannelHealthProber {
	if p == nil || d <= 0 {
		return p
	}
	p.coldFreshness = d
	return p
}

// WithNowFn overrides the clock. Tests pin wall time to avoid flakes.
func (p *ChannelHealthProber) WithNowFn(fn func() time.Time) *ChannelHealthProber {
	if p == nil || fn == nil {
		return p
	}
	p.nowFn = fn
	return p
}

// candidate represents one (group, account, model) tuple the prober may fire.
// lastSampleTs is the newest bucket_ts we've ever recorded for this combo, or
// zero-value if none exist — zero-valued combos get probed first (NULLS FIRST).
type candidate struct {
	groupID      int64
	accountID    int64
	model        string
	lastSampleTs time.Time
	hasSample    bool
}

// RunTick is invoked every 5 minutes by the scheduled runner. It returns the
// number of probes actually fired so callers can log/monitor pressure.
//
// Steps:
//  1. Enumerate (ag.group_id, ag.account_id, model_key) from account_groups ×
//     groups.model_routing, filtering out soft-deleted groups/accounts, non-
//     schedulable accounts, and any model_key containing '*' (wildcard).
//  2. Exclude combos with a sample in the last `coldFreshness`.
//  3. Sort by lastSampleTs ASC NULLS FIRST; take top `budget`.
//  4. For each candidate, run ProbeOnce with a per-probe timeout and record
//     the outcome via the shared recorder under SourceActiveProbe.
//
// A tick-level deadline (proberTickBudget) guards against pathological slow
// upstreams exhausting the 5-min window.
func (p *ChannelHealthProber) RunTick(ctx context.Context) (int, error) {
	if p == nil {
		return 0, nil
	}
	// No-op in partially-wired environments so wire/main can always call us.
	if p.entClient == nil || p.recorder == nil || p.executor == nil {
		return 0, nil
	}

	tickCtx, cancel := context.WithTimeout(ctx, proberTickBudget)
	defer cancel()

	candidates, err := p.enumerateCandidates(tickCtx)
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	// ASC NULLS FIRST — probe the dark corners first.
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.hasSample != b.hasSample {
			return !a.hasSample // no-sample combos first
		}
		if !a.hasSample && !b.hasSample {
			// stable tie-breaker for determinism
			if a.accountID != b.accountID {
				return a.accountID < b.accountID
			}
			if a.groupID != b.groupID {
				return a.groupID < b.groupID
			}
			return a.model < b.model
		}
		return a.lastSampleTs.Before(b.lastSampleTs)
	})

	if len(candidates) > p.budget {
		candidates = candidates[:p.budget]
	}

	probed := 0
	for _, c := range candidates {
		if tickCtx.Err() != nil {
			break
		}
		probeCtx, probeCancel := context.WithTimeout(tickCtx, proberPerProbeTimeout)
		statusCode, latencyMs, probeErr := p.executor.ProbeOnce(probeCtx, c.accountID, c.model)
		probeCancel()

		// Never panic out of a tick; log and move on.
		if probeErr != nil {
			logger.LegacyPrintf("service.channel_health_prober",
				"[ChannelHealthProber] probe failed account=%d group=%d model=%s: %v",
				c.accountID, c.groupID, c.model, probeErr)
			// Even when the executor errors out, if we have a non-zero status
			// we still want to record it (e.g. upstream returned 429 but the
			// adapter surfaces it as err). If statusCode == 0 treat as error.
			if statusCode == 0 {
				statusCode = 500
			}
		}
		outcome := mapStatusToOutcome(statusCode)
		evt := ChannelHealthEvent{
			AccountID: c.accountID,
			GroupID:   c.groupID,
			Model:     c.model,
			Outcome:   outcome,
			LatencyMs: latencyMs,
			Source:    SourceActiveProbe,
			At:        p.nowFn(),
		}
		if err := p.recorder.Record(tickCtx, evt); err != nil {
			logger.LegacyPrintf("service.channel_health_prober",
				"[ChannelHealthProber] record failed account=%d group=%d model=%s: %v",
				c.accountID, c.groupID, c.model, err)
		}
		probed++
	}

	return probed, nil
}

// enumerateCandidates builds the cold-combo list.
//
// We deliberately do this in Go (multiple small ent queries + in-memory
// cross-product) instead of raw Postgres `jsonb_object_keys(...)` SQL for two
// reasons:
//  1. Tests run on SQLite — jsonb_object_keys is Postgres-only.
//  2. Sub2api has "hundreds of accounts × a few groups × a few non-wildcard
//     models"; the in-memory cross-product is tiny (worst-case low thousands
//     of combos) and trivially bounded.
//
// The logical SQL this mirrors (see commit body / task notes) is:
//
//	SELECT ag.group_id, ag.account_id, key
//	  FROM account_groups ag
//	  JOIN groups   g ON g.id = ag.group_id AND g.deleted_at IS NULL
//	  JOIN accounts a ON a.id = ag.account_id AND a.deleted_at IS NULL AND a.schedulable = true,
//	       jsonb_object_keys(g.model_routing) AS key
//	 WHERE position('*' IN key) = 0
//	   AND NOT EXISTS (
//	     SELECT 1 FROM channel_health_samples s
//	      WHERE s.account_id = ag.account_id
//	        AND s.group_id   = ag.group_id
//	        AND s.model      = key
//	        AND s.bucket_ts  > NOW() - :coldFreshness
//	   )
//	 ORDER BY <max(s.bucket_ts) per (account,group,model)> ASC NULLS FIRST
//	 LIMIT :budget;
func (p *ChannelHealthProber) enumerateCandidates(ctx context.Context) ([]candidate, error) {
	// 1) Groups (not deleted) with a populated model_routing map.
	groups, err := p.entClient.Group.Query().
		Where(group.DeletedAtIsNil()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	// group_id -> set of non-wildcard model keys.
	modelsByGroup := make(map[int64][]string, len(groups))
	for _, g := range groups {
		if len(g.ModelRouting) == 0 {
			continue
		}
		keys := make([]string, 0, len(g.ModelRouting))
		for k := range g.ModelRouting {
			// Wildcard patterns are not probable targets — we can only probe
			// a concrete model name. matchModelPattern uses trailing "*"
			// today but a literal containing "*" anywhere isn't safe either.
			if strings.Contains(k, "*") {
				continue
			}
			if k == "" {
				continue
			}
			keys = append(keys, k)
		}
		if len(keys) > 0 {
			modelsByGroup[g.ID] = keys
		}
	}
	if len(modelsByGroup) == 0 {
		return nil, nil
	}

	// 2) AccountGroup rows joined with schedulable, non-deleted accounts.
	// Collect group ids we actually need to join against.
	groupIDs := make([]int64, 0, len(modelsByGroup))
	for gid := range modelsByGroup {
		groupIDs = append(groupIDs, gid)
	}
	agRows, err := p.entClient.AccountGroup.Query().
		Where(accountgroup.GroupIDIn(groupIDs...)).
		Where(accountgroup.HasAccountWith(
			account.DeletedAtIsNil(),
			account.SchedulableEQ(true),
		)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(agRows) == 0 {
		return nil, nil
	}

	// 3) Build raw candidate list.
	cands := make([]candidate, 0, len(agRows)*2)
	combos := make(map[string]*candidate, len(agRows)*2)
	for _, ag := range agRows {
		models := modelsByGroup[ag.GroupID]
		for _, m := range models {
			c := candidate{groupID: ag.GroupID, accountID: ag.AccountID, model: m}
			cands = append(cands, c)
			combos[comboKey(ag.AccountID, ag.GroupID, m)] = &cands[len(cands)-1]
		}
	}
	if len(cands) == 0 {
		return nil, nil
	}

	// 4) Fetch samples inside coldFreshness — any match disqualifies the
	// combo. Same query also feeds lastSampleTs for ordering (any sample,
	// not just recent ones). We scope the query to (account_id, group_id,
	// model) triples we care about via the model-in list for cheapness.
	modelSet := map[string]struct{}{}
	accountSet := map[int64]struct{}{}
	groupSet := map[int64]struct{}{}
	for _, c := range cands {
		modelSet[c.model] = struct{}{}
		accountSet[c.accountID] = struct{}{}
		groupSet[c.groupID] = struct{}{}
	}
	models := make([]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, m)
	}
	accIDs := make([]int64, 0, len(accountSet))
	for a := range accountSet {
		accIDs = append(accIDs, a)
	}
	gIDs := make([]int64, 0, len(groupSet))
	for g := range groupSet {
		gIDs = append(gIDs, g)
	}
	now := p.nowFn()
	freshCutoff := now.Add(-p.coldFreshness)
	// Bound the scan by time too — without this, 24h × millions of samples
	// would be loaded every RunTick. 2x coldFreshness keeps enough buffer for
	// the "lastSampleTs for ordering" use case (a combo with a sample slightly
	// older than coldFreshness still needs its timestamp to sort NULLS-FIRST
	// candidates against it), while capping the read to a predictable window.
	scanCutoff := now.Add(-2 * p.coldFreshness)

	samples, err := p.entClient.ChannelHealthSample.Query().
		Where(
			channelhealthsample.AccountIDIn(accIDs...),
			channelhealthsample.GroupIDIn(gIDs...),
			channelhealthsample.ModelIn(models...),
			channelhealthsample.BucketTsGTE(scanCutoff),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// Mark combos: exclude if any sample is inside coldFreshness; otherwise
	// stash max(bucket_ts) for ordering.
	excluded := map[string]bool{}
	for _, s := range samples {
		key := comboKey(s.AccountID, s.GroupID, s.Model)
		c, ok := combos[key]
		if !ok {
			continue
		}
		if s.BucketTs.After(freshCutoff) {
			excluded[key] = true
			continue
		}
		if !c.hasSample || s.BucketTs.After(c.lastSampleTs) {
			c.lastSampleTs = s.BucketTs
			c.hasSample = true
		}
	}

	// 5) Collect survivors.
	out := make([]candidate, 0, len(cands))
	for i := range cands {
		key := comboKey(cands[i].accountID, cands[i].groupID, cands[i].model)
		if excluded[key] {
			continue
		}
		// re-read via the pointer we stashed so lastSampleTs is populated
		if c, ok := combos[key]; ok {
			out = append(out, *c)
		}
	}
	return out, nil
}

// comboKey builds the map key used to match samples back to candidates.
func comboKey(accountID, groupID int64, model string) string {
	var b strings.Builder
	b.Grow(len(model) + 24)
	b.WriteString(strconv.FormatInt(accountID, 10))
	b.WriteByte('|')
	b.WriteString(strconv.FormatInt(groupID, 10))
	b.WriteByte('|')
	b.WriteString(model)
	return b.String()
}

