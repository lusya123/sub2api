// Package service — status page aggregation.
//
// StatusPageService turns the raw channel_health_samples table into the
// public-facing /api/public/status/* shape consumed by the kuma-mieru-styled
// heartbeat dashboard. It is read-only and does not talk to external systems.
//
// Aggregation semantics (kept in sync with the design doc):
//   - 90-minute rolling window at 1-minute granularity (exactly 90 buckets).
//   - Per bucket:
//     error_count>0 && success_count==0 -> "down"
//     rate_limited_count>0 || overloaded_count>0 -> "degraded"
//     success_count>0 -> "ok"
//     no sample                              -> "unknown"
//   - availability_pct = ok / (buckets that have samples), or 100.0 when the
//     whole window is empty (frontend renders the bar all-grey so the 100%
//     is visually honest).
//   - Channel names are masked — never leak account.name / email / IP. We
//     reach into extra.region first; otherwise fall back to "Channel #<id>".
//
// Model metadata (pricing, release date, prompt caching, note) is
// hard-coded for the four flagship Anthropic models the status page showcases
// today. TODO: move this to a first-class `model_catalog` table so ops can
// edit pricing without a redeploy.
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/ent/accountgroup"
	"github.com/Wei-Shaw/sub2api/ent/channelhealthsample"
	"github.com/Wei-Shaw/sub2api/ent/group"
)

const (
	// statusWindowMinutes is the rolling window used for heartbeat aggregation.
	statusWindowMinutes = 90
	// statusLoadWindow is the "is this account active right now" window used
	// for Group.LoadPct. 5 minutes matches the prober cadence so newly-warm
	// channels count as loaded.
	statusLoadWindow = 5 * time.Minute
)

// StatusBeat is one minute bucket's worth of heartbeat state.
type StatusBeat struct {
	Ts     string `json:"ts"`
	Status string `json:"status"` // ok | degraded | down | unknown
}

// StatusPricing mirrors the $/MTok grid shown on the model card.
type StatusPricing struct {
	InputPerMTok  float64 `json:"input_per_mtok"`
	OutputPerMTok float64 `json:"output_per_mtok"`
	CacheWrite    float64 `json:"cache_write"`
	CacheRead     float64 `json:"cache_read"`
}

// StatusChannel is one row in the group's channel list.
type StatusChannel struct {
	Name            string       `json:"name"`
	AvailabilityPct float64      `json:"availability_pct"`
	Heartbeats      []StatusBeat `json:"heartbeats"`
}

// StatusGroup is a group section inside a model card.
type StatusGroup struct {
	Name     string          `json:"name"`
	LoadPct  float64         `json:"load_pct"`
	Channels []StatusChannel `json:"channels"`
}

// StatusModel is the top-level shape returned for both list and detail views.
// ListModels leaves Heartbeats / Groups empty; GetModelDetail populates both.
type StatusModel struct {
	Name            string        `json:"name"`
	Provider        string        `json:"provider"`
	ReleaseDate     string        `json:"release_date,omitempty"`
	PromptCaching   bool          `json:"prompt_caching"`
	Note            string        `json:"note,omitempty"`
	Pricing         StatusPricing `json:"pricing"`
	AvailabilityPct float64       `json:"availability_pct"`
	Heartbeats      []StatusBeat  `json:"heartbeats"`
	Groups          []StatusGroup `json:"groups"`
}

// modelMetadata is the hard-coded catalogue. TODO: migrate to a table.
type modelMetadata struct {
	Provider      string
	ReleaseDate   string
	PromptCaching bool
	Note          string
	Pricing       StatusPricing
}

// modelCatalog covers the four models prioritised in the design mockup plus a
// handful of near-neighbours that show up in live model_routing keys. Unknown
// models resolve to a minimally-populated "ANTHROPIC" entry with zero pricing.
var modelCatalog = map[string]modelMetadata{
	"claude-opus-4-7": {
		Provider:      "ANTHROPIC",
		ReleaseDate:   "2026-01-15",
		PromptCaching: true,
		Note:          "Flagship reasoning model",
		Pricing:       StatusPricing{InputPerMTok: 15, OutputPerMTok: 75, CacheWrite: 18.75, CacheRead: 1.5},
	},
	"claude-sonnet-4-6": {
		Provider:      "ANTHROPIC",
		ReleaseDate:   "2025-09-29",
		PromptCaching: true,
		Note:          "Balanced model for everyday coding",
		Pricing:       StatusPricing{InputPerMTok: 3, OutputPerMTok: 15, CacheWrite: 3.75, CacheRead: 0.3},
	},
	"claude-opus-4-6": {
		Provider:      "ANTHROPIC",
		ReleaseDate:   "2025-08-05",
		PromptCaching: true,
		Note:          "Previous-generation flagship",
		Pricing:       StatusPricing{InputPerMTok: 5, OutputPerMTok: 25, CacheWrite: 6.25, CacheRead: 0.5},
	},
	"claude-haiku-4-5-20251001": {
		Provider:      "ANTHROPIC",
		ReleaseDate:   "2025-10-01",
		PromptCaching: true,
		Note:          "Fastest, cheapest tier",
		Pricing:       StatusPricing{InputPerMTok: 1, OutputPerMTok: 5, CacheWrite: 1.25, CacheRead: 0.1},
	},
}

// lookupMetadata returns the catalogue row for a model, or a default stub.
func lookupMetadata(name string) modelMetadata {
	if m, ok := modelCatalog[name]; ok {
		return m
	}
	// Best-effort provider guess from name prefix. Still zero-priced; the
	// frontend renders "--" for zero values per the design spec.
	provider := "ANTHROPIC"
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3"):
		provider = "OPENAI"
	case strings.HasPrefix(lower, "gemini-"):
		provider = "GOOGLE"
	}
	return modelMetadata{Provider: provider, PromptCaching: false}
}

// StatusPageService aggregates channel_health_samples into the public status
// page shape. All reads go through *dbent.Client — same DI style as the
// recorder/prober upstream of it.
type StatusPageService struct {
	entClient *dbent.Client
	// nowFn is overridable for tests.
	nowFn func() time.Time
}

// NewStatusPageService wires the service.
func NewStatusPageService(entClient *dbent.Client) *StatusPageService {
	return &StatusPageService{
		entClient: entClient,
		nowFn:     func() time.Time { return time.Now().UTC() },
	}
}

// WithNowFn overrides the clock for deterministic tests.
func (s *StatusPageService) WithNowFn(fn func() time.Time) *StatusPageService {
	if s == nil || fn == nil {
		return s
	}
	s.nowFn = fn
	return s
}

// ListModels returns every currently-routed model (distinct keys in any
// non-deleted group's model_routing, excluding wildcard patterns). Heartbeats
// and Groups are left empty — the frontend calls GetModelDetail for a
// specific card.
func (s *StatusPageService) ListModels(ctx context.Context) ([]StatusModel, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("status_page_service: entClient is nil")
	}
	groups, err := s.entClient.Group.Query().Where(group.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list groups: %w", err)
	}
	modelSet := map[string]struct{}{}
	for _, g := range groups {
		for k := range g.ModelRouting {
			if k == "" || strings.Contains(k, "*") {
				continue
			}
			modelSet[k] = struct{}{}
		}
	}
	names := make([]string, 0, len(modelSet))
	for n := range modelSet {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]StatusModel, 0, len(names))
	for _, n := range names {
		md := lookupMetadata(n)
		out = append(out, StatusModel{
			Name:          n,
			Provider:      md.Provider,
			ReleaseDate:   md.ReleaseDate,
			PromptCaching: md.PromptCaching,
			Note:          md.Note,
			Pricing:       md.Pricing,
			// availability is computed from the per-channel detail — for the
			// lightweight list we return 0 and let the frontend fetch detail.
			AvailabilityPct: 0,
		})
	}
	return out, nil
}

// GetModelDetail returns the fully-populated status payload for one model.
// The 90-minute window is fixed and not configurable via the public API
// (callers must not get to control cost). The returned *StatusModel is
// ready to be JSON-marshalled for the /api/public/status/model/:name endpoint.
func (s *StatusPageService) GetModelDetail(ctx context.Context, modelName string) (*StatusModel, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("status_page_service: entClient is nil")
	}
	if modelName == "" {
		return nil, errors.New("status_page_service: model name required")
	}

	md := lookupMetadata(modelName)
	now := s.nowFn().UTC()
	windowStart := floorToMinute(now.Add(-time.Duration(statusWindowMinutes) * time.Minute))

	// 1) Find which groups route this model. Only non-deleted groups are in
	// scope; wildcard routing keys are ignored (the prober ignores them too).
	groups, err := s.entClient.Group.Query().Where(group.DeletedAtIsNil()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list groups: %w", err)
	}
	// ordered iteration for stable output
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].ID < groups[j].ID
	})

	type scopedGroup struct {
		id   int64
		name string
	}
	scoped := make([]scopedGroup, 0, len(groups))
	groupIDs := make([]int64, 0, len(groups))
	for _, g := range groups {
		if _, ok := g.ModelRouting[modelName]; !ok {
			continue
		}
		scoped = append(scoped, scopedGroup{id: g.ID, name: g.Name})
		groupIDs = append(groupIDs, g.ID)
	}

	result := &StatusModel{
		Name:          modelName,
		Provider:      md.Provider,
		ReleaseDate:   md.ReleaseDate,
		PromptCaching: md.PromptCaching,
		Note:          md.Note,
		Pricing:       md.Pricing,
		Heartbeats:    make([]StatusBeat, 0, statusWindowMinutes),
		Groups:        []StatusGroup{},
	}

	// 2) Fetch all samples for this model inside the window. One query keeps
	// this cheap — the table is retained for only ~24h and heavily indexed on
	// (bucket_ts, account_id, group_id, model).
	samples, err := s.entClient.ChannelHealthSample.Query().
		Where(
			channelhealthsample.ModelEQ(modelName),
			channelhealthsample.BucketTsGTE(windowStart),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list samples: %w", err)
	}

	// 3) Global heartbeats (all channels collapsed into one bar).
	result.Heartbeats = buildHeartbeats(samples, now)
	result.AvailabilityPct = availabilityFromBeats(result.Heartbeats)

	if len(scoped) == 0 {
		// No routed group — still return the model shell so the frontend can
		// show pricing / metadata even during a config gap.
		return result, nil
	}

	// 4) Per-channel (account) heartbeats grouped by group.
	//
	// Collect the account IDs in scope via account_groups so we emit rows
	// even for channels with zero samples (they'll read as all-unknown).
	agRows, err := s.entClient.AccountGroup.Query().
		Where(accountgroup.GroupIDIn(groupIDs...)).
		Where(accountgroup.HasAccountWith(account.DeletedAtIsNil())).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list account_groups: %w", err)
	}
	accountIDSet := map[int64]struct{}{}
	channelsByGroup := map[int64][]int64{}
	for _, ag := range agRows {
		channelsByGroup[ag.GroupID] = append(channelsByGroup[ag.GroupID], ag.AccountID)
		accountIDSet[ag.AccountID] = struct{}{}
	}

	// Load accounts once for name masking.
	accountIDs := make([]int64, 0, len(accountIDSet))
	for id := range accountIDSet {
		accountIDs = append(accountIDs, id)
	}
	var accounts []*dbent.Account
	if len(accountIDs) > 0 {
		accounts, err = s.entClient.Account.Query().
			Where(account.IDIn(accountIDs...)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("status_page_service: list accounts: %w", err)
		}
	}
	accountByID := make(map[int64]*dbent.Account, len(accounts))
	for _, a := range accounts {
		accountByID[a.ID] = a
	}

	// Partition samples by (groupID, accountID) once; O(n) over samples.
	type key struct {
		gid, aid int64
	}
	samplesBy := map[key][]*dbent.ChannelHealthSample{}
	for _, s := range samples {
		k := key{gid: s.GroupID, aid: s.AccountID}
		samplesBy[k] = append(samplesBy[k], s)
	}

	// Compute LoadPct per group using samples inside the last 5 minutes.
	loadCutoff := now.Add(-statusLoadWindow)
	activeAccountsByGroup := map[int64]map[int64]struct{}{}
	for _, s := range samples {
		if s.BucketTs.Before(loadCutoff) {
			continue
		}
		if _, ok := activeAccountsByGroup[s.GroupID]; !ok {
			activeAccountsByGroup[s.GroupID] = map[int64]struct{}{}
		}
		activeAccountsByGroup[s.GroupID][s.AccountID] = struct{}{}
	}

	for _, sg := range scoped {
		ids := channelsByGroup[sg.id]
		// Stable ordering so UI diffs are small between polls.
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

		channels := make([]StatusChannel, 0, len(ids))
		for _, aid := range ids {
			ss := samplesBy[key{gid: sg.id, aid: aid}]
			beats := buildHeartbeats(ss, now)
			channels = append(channels, StatusChannel{
				Name:            maskChannelName(accountByID[aid], aid),
				AvailabilityPct: availabilityFromBeats(beats),
				Heartbeats:      beats,
			})
		}

		loadPct := 0.0
		if total := len(ids); total > 0 {
			active := len(activeAccountsByGroup[sg.id])
			loadPct = float64(active) / float64(total) * 100.0
		}

		result.Groups = append(result.Groups, StatusGroup{
			Name:     sg.name,
			LoadPct:  loadPct,
			Channels: channels,
		})
	}

	return result, nil
}

// buildHeartbeats projects a pile of samples onto the fixed 90-minute grid.
// Samples outside [now-90m, now) are ignored. Multiple samples landing in the
// same bucket (can happen if a bucket was counted from both passive and
// active sources concurrently) are merged via the usual precedence:
//
//	down > degraded > ok.
func buildHeartbeats(samples []*dbent.ChannelHealthSample, now time.Time) []StatusBeat {
	beats := make([]StatusBeat, statusWindowMinutes)
	base := floorToMinute(now.Add(-time.Duration(statusWindowMinutes-1) * time.Minute))
	for i := range beats {
		ts := base.Add(time.Duration(i) * time.Minute)
		beats[i] = StatusBeat{Ts: ts.UTC().Format(time.RFC3339), Status: "unknown"}
	}
	for _, s := range samples {
		bucket := floorToMinute(s.BucketTs)
		if bucket.Before(base) || !bucket.Before(base.Add(time.Duration(statusWindowMinutes)*time.Minute)) {
			continue
		}
		idx := int(bucket.Sub(base) / time.Minute)
		if idx < 0 || idx >= statusWindowMinutes {
			continue
		}
		status := bucketStatus(s)
		beats[idx].Status = mergeStatus(beats[idx].Status, status)
	}
	return beats
}

// bucketStatus applies the kuma-mieru-style threshold rules to one sample.
func bucketStatus(s *dbent.ChannelHealthSample) string {
	if s == nil {
		return "unknown"
	}
	if s.ErrorCount > 0 && s.SuccessCount == 0 {
		return "down"
	}
	if s.RateLimitedCount > 0 || s.OverloadedCount > 0 {
		// Still degraded even if success_count is also non-zero — the 429
		// signal is what the operator cares about.
		return "degraded"
	}
	if s.SuccessCount > 0 {
		return "ok"
	}
	return "unknown"
}

// mergeStatus collapses two bucket states into the worst of the two. Precedence
// (worst → best): down > degraded > ok > unknown.
func mergeStatus(a, b string) string {
	rank := func(s string) int {
		switch s {
		case "down":
			return 3
		case "degraded":
			return 2
		case "ok":
			return 1
		default:
			return 0
		}
	}
	if rank(a) >= rank(b) {
		return a
	}
	return b
}

// availabilityFromBeats computes the ok-ratio over buckets that actually have
// samples. All-unknown windows return 100.0 (frontend shows the bar grey and
// the 100% is honest — there is literally no evidence of failure).
func availabilityFromBeats(beats []StatusBeat) float64 {
	total := 0
	ok := 0
	for _, b := range beats {
		if b.Status == "unknown" {
			continue
		}
		total++
		if b.Status == "ok" {
			ok++
		}
	}
	if total == 0 {
		return 100.0
	}
	return float64(ok) / float64(total) * 100.0
}

// maskChannelName turns an account into a safe, public-facing name.
//
// Precedence:
//  1. extra.region      (operator-supplied region tag)
//  2. extra.location    (alternative tag used by legacy accounts)
//  3. "Channel #<id>"   (neutral fallback)
//
// Intentionally never echoes account.Name / notes / credentials — those
// frequently contain email addresses, internal host names, or IP:port tuples.
func maskChannelName(a *dbent.Account, id int64) string {
	if a != nil {
		if v, ok := a.Extra["region"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
		if v, ok := a.Extra["location"]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return fmt.Sprintf("Channel #%d", id)
}
