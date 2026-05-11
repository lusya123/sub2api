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
//   - Public channels are group-level display rows configured by admins. The
//     status API never exposes account names, emails, IPs, or credentials.
//
// Model metadata can be supplied by the public status config; known Claude
// defaults are kept in modelCatalog as a fallback for bootstrap and tests.
package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/channelhealthsample"
	"github.com/Wei-Shaw/sub2api/ent/channelmonitor"
	"github.com/Wei-Shaw/sub2api/ent/channelmonitorhistory"
	"github.com/Wei-Shaw/sub2api/ent/group"

	"golang.org/x/sync/singleflight"
)

const (
	// statusWindowMinutes is the rolling window used for heartbeat aggregation.
	statusWindowMinutes = 90
	// statusLoadWindow is the "is this account active right now" window used
	// for Group.LoadPct. 5 minutes matches the prober cadence so newly-warm
	// channels count as loaded.
	statusLoadWindow = 5 * time.Minute
	// statusCacheTTL is how long we cache ListModels / GetModelDetail results
	// in process memory. 30s is short enough that a failing model clears fast
	// and long enough that a DoS-level burst of anonymous polls collapses to
	// one DB query per 30s regardless of QPS.
	statusCacheTTL = 30 * time.Second
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
		ReleaseDate:   "2026-04-16",
		PromptCaching: true,
		Note:          "Max分组暂不支持1M上下文",
		Pricing:       StatusPricing{InputPerMTok: 5, OutputPerMTok: 25, CacheWrite: 6.25, CacheRead: 0.5},
	},
	"claude-sonnet-4-6": {
		Provider:      "ANTHROPIC",
		ReleaseDate:   "2026-02-17",
		PromptCaching: true,
		Note:          "仅AWS-Q支持1M上下文",
		Pricing:       StatusPricing{InputPerMTok: 3, OutputPerMTok: 15, CacheWrite: 3.75, CacheRead: 0.3},
	},
	"claude-opus-4-6": {
		Provider:      "ANTHROPIC",
		ReleaseDate:   "2026-02-05",
		PromptCaching: true,
		Note:          "仅AWS-Q支持1M上下文",
		Pricing:       StatusPricing{InputPerMTok: 5, OutputPerMTok: 25, CacheWrite: 6.25, CacheRead: 0.5},
	},
	"claude-sonnet-4-5-20250929": {
		Provider:      "ANTHROPIC",
		ReleaseDate:   "2025-09-29",
		PromptCaching: true,
		Pricing:       StatusPricing{InputPerMTok: 3, OutputPerMTok: 15, CacheWrite: 3.75, CacheRead: 0.3},
	},
	"claude-haiku-4-5-20251001": {
		Provider:      "ANTHROPIC",
		ReleaseDate:   "2025-10-15",
		PromptCaching: true,
		Pricing:       StatusPricing{InputPerMTok: 1, OutputPerMTok: 5, CacheWrite: 1.25, CacheRead: 0.1},
	},
	"glm-5": {
		Provider:      "Z.AI",
		PromptCaching: true,
		Pricing:       StatusPricing{InputPerMTok: 1, OutputPerMTok: 3.2, CacheWrite: 0, CacheRead: 0.2},
	},
	"minimax-m2.5": {
		Provider:      "MINIMAX",
		PromptCaching: true,
		Pricing:       StatusPricing{InputPerMTok: 0.3, OutputPerMTok: 1.2, CacheWrite: 0.375, CacheRead: 0.03},
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
	case strings.HasPrefix(lower, "glm") || strings.HasPrefix(lower, "chatglm"):
		provider = "Z.AI"
	case strings.HasPrefix(lower, "minimax") || strings.HasPrefix(lower, "abab"):
		provider = "MINIMAX"
	}
	return modelMetadata{Provider: provider, PromptCaching: false}
}

// listModelsCache holds the cached ListModels result with a monotonic expiry.
// Access is guarded by StatusPageService.listMu.
type listModelsCache struct {
	data     []StatusModel
	modelSet map[string]struct{} // for Fix 1 fast-path: known model names
	expireAt time.Time
}

// modelDetailCacheEntry is a per-model cached detail. map access is guarded by
// StatusPageService.detailMu; the entry itself is immutable once published.
type modelDetailCacheEntry struct {
	data     *StatusModel
	expireAt time.Time
}

// StatusPageService aggregates channel_health_samples into the public status
// page shape. All reads go through *dbent.Client — same DI style as the
// recorder/prober upstream of it.
//
// Caching: ListModels / GetModelDetail / isKnownModel share two in-process
// caches with a 30s TTL. singleflight collapses concurrent misses onto a
// single DB round-trip so a burst of anonymous /status hits never fans out.
// Errors are NOT cached — next call retries immediately.
type StatusPageService struct {
	entClient   *dbent.Client
	settingRepo SettingRepository
	fixedConfig *PublicStatusConfig
	// nowFn is overridable for tests.
	nowFn func() time.Time

	// listMu guards listCache. RWMutex because the steady-state is "many
	// readers hit a fresh cache; one writer every 30s refreshes it".
	listMu    sync.RWMutex
	listCache *listModelsCache

	// detailMu guards detailCache. Per-model keys.
	detailMu    sync.RWMutex
	detailCache map[string]*modelDetailCacheEntry

	// sf collapses concurrent cache misses. Keyed by "list" or "detail:<name>".
	sf singleflight.Group
}

// NewStatusPageService wires the service.
func NewStatusPageService(entClient *dbent.Client) *StatusPageService {
	return &StatusPageService{
		entClient:   entClient,
		nowFn:       func() time.Time { return time.Now().UTC() },
		detailCache: make(map[string]*modelDetailCacheEntry),
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
//
// Cached for statusCacheTTL. singleflight collapses concurrent misses.
func (s *StatusPageService) ListModels(ctx context.Context) ([]StatusModel, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("status_page_service: entClient is nil")
	}

	// Fast path: fresh cache.
	if c := s.cachedList(); c != nil {
		return c.data, nil
	}

	// Miss: collapse concurrent callers onto one DB round-trip.
	v, err, _ := s.sf.Do("list", func() (any, error) {
		// Re-check under the writer lock path: another goroutine may have
		// filled the cache while we were queued behind singleflight.
		if c := s.cachedList(); c != nil {
			return c, nil
		}
		fresh, err := s.loadListFromDB(ctx)
		if err != nil {
			return nil, err
		}
		s.listMu.Lock()
		s.listCache = fresh
		s.listMu.Unlock()
		return fresh, nil
	})
	if err != nil {
		return nil, err
	}
	cache, ok := v.(*listModelsCache)
	if !ok || cache == nil {
		return nil, fmt.Errorf("status page list cache returned unexpected type %T", v)
	}
	return cache.data, nil
}

// cachedList returns the current cache entry if it's still fresh, else nil.
func (s *StatusPageService) cachedList() *listModelsCache {
	s.listMu.RLock()
	defer s.listMu.RUnlock()
	if s.listCache == nil {
		return nil
	}
	if s.nowFn().After(s.listCache.expireAt) {
		return nil
	}
	return s.listCache
}

func (s *StatusPageService) clearCaches() {
	if s == nil {
		return
	}
	s.listMu.Lock()
	s.listCache = nil
	s.listMu.Unlock()
	s.detailMu.Lock()
	s.detailCache = make(map[string]*modelDetailCacheEntry)
	s.detailMu.Unlock()
}

// ClearCaches invalidates public status list/detail caches after monitor or
// status-page configuration changes.
func (s *StatusPageService) ClearCaches() {
	s.clearCaches()
}

// loadListFromDB does the actual Group.Query + catalog join. Callers are
// responsible for caching the result.
func (s *StatusPageService) loadListFromDB(ctx context.Context) (*listModelsCache, error) {
	if monitorCache, err := s.loadListFromMonitorConfigs(ctx); err != nil {
		return nil, err
	} else if monitorCache != nil && len(monitorCache.data) > 0 {
		return monitorCache, nil
	}

	cfg, err := s.loadPublicStatusConfig(ctx)
	if err != nil {
		return nil, err
	}
	models := enabledPublicStatusModels(cfg)
	groupConfigByID := enabledPublicStatusGroups(cfg)
	if len(models) == 0 || len(groupConfigByID) == 0 {
		return &listModelsCache{
			data:     []StatusModel{},
			modelSet: map[string]struct{}{},
			expireAt: s.nowFn().Add(statusCacheTTL),
		}, nil
	}

	groupIDs := make([]int64, 0, len(groupConfigByID))
	for id := range groupConfigByID {
		groupIDs = append(groupIDs, id)
	}
	groups, err := s.entClient.Group.Query().
		Where(group.DeletedAtIsNil(), group.IDIn(groupIDs...), group.StatusEQ(StatusActive)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list groups: %w", err)
	}
	schedulableCounts, err := s.countSchedulableAccountsByGroup(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: count schedulable accounts: %w", err)
	}
	modelSet := map[string]struct{}{}
	modelByName := map[string]PublicStatusModelConfig{}
	for _, m := range models {
		for _, g := range groups {
			gc := groupConfigByID[g.ID]
			if schedulableCounts[g.ID] == 0 {
				continue
			}
			if !publicStatusGroupConfigSupportsModel(gc, m.Name) {
				continue
			}
			if !groupSupportsStatusModel(g, m.Name) {
				continue
			}
			modelSet[m.Name] = struct{}{}
			modelByName[m.Name] = m
			break
		}
	}
	names := make([]string, 0, len(modelSet))
	for n := range modelSet {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]StatusModel, 0, len(names))
	for _, n := range names {
		md := metadataFromConfig(modelByName[n])
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
	return &listModelsCache{
		data:     out,
		modelSet: modelSet,
		expireAt: s.nowFn().Add(statusCacheTTL),
	}, nil
}

// ErrStatusModelUnknown is returned by GetModelDetail when the requested model
// is not present in the public status configuration. The handler layer
// translates this sentinel into a 404 response so hostile traffic with junk
// model names doesn't pay the full 4-query aggregation cost.
var ErrStatusModelUnknown = errors.New("status_page_service: unknown model")

// isKnownModel reports whether `name` appears in the public status model list.
// Reuses the ListModels cache so a GetModelDetail
// call on a junk name costs one cache lookup, not a full DB scan.
func (s *StatusPageService) isKnownModel(ctx context.Context, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	// Ensure the list cache is primed (populates modelSet). On cache hit this
	// is a ~nanosecond RWMutex read — no DB traffic for repeated junk names.
	if _, err := s.ListModels(ctx); err != nil {
		return false, err
	}
	s.listMu.RLock()
	defer s.listMu.RUnlock()
	if s.listCache == nil {
		return false, nil
	}
	_, ok := s.listCache.modelSet[name]
	return ok, nil
}

// GetModelDetail returns the fully-populated status payload for one model.
// The 90-minute window is fixed and not configurable via the public API
// (callers must not get to control cost). The returned *StatusModel is
// ready to be JSON-marshalled for the /api/public/status/model/:name endpoint.
//
// Unknown models (not configured for public status) return
// ErrStatusModelUnknown without hitting the samples/accounts/groups tables —
// this is the DoS fast-path that lets the handler 404 hostile traffic cheaply.
//
// Cached for statusCacheTTL per-model. singleflight collapses concurrent
// misses on the same model name.
func (s *StatusPageService) GetModelDetail(ctx context.Context, modelName string) (*StatusModel, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("status_page_service: entClient is nil")
	}
	if modelName == "" {
		return nil, errors.New("status_page_service: model name required")
	}

	// Unknown-model fast-path: piggyback on the ListModels cache so a flood
	// of GetModelDetail("bogus-<n>") calls costs no DB traffic at all.
	known, err := s.isKnownModel(ctx, modelName)
	if err != nil {
		return nil, err
	}
	if !known {
		return nil, ErrStatusModelUnknown
	}

	// Fresh cache hit?
	if d := s.cachedDetail(modelName); d != nil {
		return d, nil
	}

	// Miss: collapse duplicate callers.
	v, err, _ := s.sf.Do("detail:"+modelName, func() (any, error) {
		if d := s.cachedDetail(modelName); d != nil {
			return d, nil
		}
		fresh, err := s.loadDetailFromDB(ctx, modelName)
		if err != nil {
			return nil, err
		}
		s.detailMu.Lock()
		s.detailCache[modelName] = &modelDetailCacheEntry{
			data:     fresh,
			expireAt: s.nowFn().Add(statusCacheTTL),
		}
		s.detailMu.Unlock()
		return fresh, nil
	})
	if err != nil {
		return nil, err
	}
	model, ok := v.(*StatusModel)
	if !ok || model == nil {
		return nil, fmt.Errorf("status page detail cache returned unexpected type %T", v)
	}
	return model, nil
}

// cachedDetail returns a fresh cached *StatusModel for modelName, else nil.
func (s *StatusPageService) cachedDetail(modelName string) *StatusModel {
	s.detailMu.RLock()
	defer s.detailMu.RUnlock()
	entry, ok := s.detailCache[modelName]
	if !ok || entry == nil {
		return nil
	}
	if s.nowFn().After(entry.expireAt) {
		return nil
	}
	return entry.data
}

// loadDetailFromDB does the full 4-query aggregation. Callers cache the result.
func (s *StatusPageService) loadDetailFromDB(ctx context.Context, modelName string) (*StatusModel, error) {
	if model, found, err := s.loadDetailFromMonitorConfigs(ctx, modelName); err != nil {
		return nil, err
	} else if found {
		return model, nil
	}

	cfg, err := s.loadPublicStatusConfig(ctx)
	if err != nil {
		return nil, err
	}
	modelConfigByName := map[string]PublicStatusModelConfig{}
	for _, m := range enabledPublicStatusModels(cfg) {
		modelConfigByName[m.Name] = m
	}
	modelConfig := modelConfigByName[modelName]
	md := metadataFromConfig(modelConfig)
	now := s.nowFn().UTC()
	windowStart := floorToMinute(now.Add(-time.Duration(statusWindowMinutes) * time.Minute))

	groupConfigByID := enabledPublicStatusGroups(cfg)
	groupIDs := make([]int64, 0, len(groupConfigByID))
	for id := range groupConfigByID {
		groupIDs = append(groupIDs, id)
	}

	// 1) Find which configured groups support this model. Only non-deleted,
	// active groups are in scope.
	var groups []*dbent.Group
	if len(groupIDs) > 0 {
		groups, err = s.entClient.Group.Query().
			Where(group.DeletedAtIsNil(), group.IDIn(groupIDs...), group.StatusEQ(StatusActive)).
			All(ctx)
	} else {
		groups = []*dbent.Group{}
	}
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list groups: %w", err)
	}
	schedulableCounts, err := s.countSchedulableAccountsByGroup(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: count schedulable accounts: %w", err)
	}
	// ordered iteration for stable output
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].SortOrder != groups[j].SortOrder {
			return groups[i].SortOrder < groups[j].SortOrder
		}
		return groups[i].ID < groups[j].ID
	})

	type scopedGroup struct {
		id           int64
		name         string
		aggregateKey string
		sortOrder    int
		rank         int
		probeLines   []PublicStatusProbeLineConfig
	}
	scoped := make([]scopedGroup, 0, len(groups))
	scopedGroupIDs := make([]int64, 0, len(groups))
	for _, g := range groups {
		if schedulableCounts[g.ID] == 0 {
			continue
		}
		if !groupSupportsStatusModel(g, modelName) {
			continue
		}
		gc := groupConfigByID[g.ID]
		if !publicStatusGroupConfigSupportsModel(gc, modelName) {
			continue
		}
		displayName := strings.TrimSpace(gc.DisplayName)
		aggregateKey := strings.TrimSpace(gc.AggregateKey)
		if displayName == "" {
			displayName, _ = suggestedPublicStatusGroup(g)
		}
		if displayName == "" {
			displayName = g.Name
		}
		scoped = append(scoped, scopedGroup{
			id:           g.ID,
			name:         displayName,
			aggregateKey: aggregateKey,
			sortOrder:    gc.SortOrder,
			rank:         publicStatusGroupRank(aggregateKey, displayName),
			probeLines:   publicStatusGroupProbeLines(gc, displayName),
		})
		scopedGroupIDs = append(scopedGroupIDs, g.ID)
	}
	sort.SliceStable(scoped, func(i, j int) bool {
		if scoped[i].sortOrder != scoped[j].sortOrder {
			if scoped[i].sortOrder == 0 {
				return false
			}
			if scoped[j].sortOrder == 0 {
				return true
			}
			return scoped[i].sortOrder < scoped[j].sortOrder
		}
		if scoped[i].rank != scoped[j].rank {
			return scoped[i].rank < scoped[j].rank
		}
		if scoped[i].name != scoped[j].name {
			return scoped[i].name < scoped[j].name
		}
		return scoped[i].id < scoped[j].id
	})

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
	var samples []*dbent.ChannelHealthSample
	if len(scopedGroupIDs) > 0 {
		samples, err = s.entClient.ChannelHealthSample.Query().
			Where(
				channelhealthsample.ModelEQ(modelName),
				channelhealthsample.GroupIDIn(scopedGroupIDs...),
				channelhealthsample.BucketTsGTE(windowStart),
			).
			All(ctx)
	} else {
		samples = []*dbent.ChannelHealthSample{}
	}
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list samples: %w", err)
	}

	// 3) Global heartbeats (all channels collapsed into one bar).
	result.Heartbeats = buildHeartbeats(samples, now)
	result.AvailabilityPct = availabilityFromBeats(result.Heartbeats)

	if len(scoped) == 0 {
		// No configured group — still return the model shell so the frontend can
		// show pricing / metadata even during a config gap.
		return result, nil
	}

	// 4) Per-configured-group heartbeats. The public channel is the configured
	// group or aggregate group (for example all monthly-card backend groups
	// collapsed into one "月卡" channel). User requests are made by group, so
	// account-level rows are intentionally not exposed as public channels.
	samplesByGroup := map[int64][]*dbent.ChannelHealthSample{}
	for _, s := range samples {
		samplesByGroup[s.GroupID] = append(samplesByGroup[s.GroupID], s)
	}

	// Compute LoadPct per group using samples inside the last 5 minutes.
	loadCutoff := now.Add(-statusLoadWindow)
	activeGroups := map[int64]struct{}{}
	for _, s := range samples {
		if s.BucketTs.Before(loadCutoff) {
			continue
		}
		activeGroups[s.GroupID] = struct{}{}
	}

	type aggregate struct {
		name     string
		groupIDs []int64
		samples  []*dbent.ChannelHealthSample
		lines    []PublicStatusProbeLineConfig
		lineSeen map[string]struct{}
	}
	aggregates := map[string]*aggregate{}
	order := []string{}
	for _, sg := range scoped {
		aggKey := sg.aggregateKey
		if aggKey == "" {
			aggKey = fmt.Sprintf("group:%d", sg.id)
		}
		agg := aggregates[aggKey]
		if agg == nil {
			agg = &aggregate{name: sg.name, lineSeen: map[string]struct{}{}}
			aggregates[aggKey] = agg
			order = append(order, aggKey)
		}
		agg.groupIDs = append(agg.groupIDs, sg.id)
		agg.samples = append(agg.samples, samplesByGroup[sg.id]...)
		for _, line := range sg.probeLines {
			key := publicStatusProbeLineDisplayName(line)
			if key == "" {
				continue
			}
			seenKey := strings.ToLower(key)
			if _, ok := agg.lineSeen[seenKey]; ok {
				continue
			}
			agg.lineSeen[seenKey] = struct{}{}
			agg.lines = append(agg.lines, line)
		}
	}

	overallBeats := make([][]StatusBeat, 0, len(order))
	availabilityTotal := 0.0
	availabilityCount := 0
	for _, key := range order {
		agg := aggregates[key]
		beats := buildHeartbeats(agg.samples, now)
		overallBeats = append(overallBeats, beats)
		availability := availabilityFromBeats(beats)
		availabilityTotal += availability
		availabilityCount++
		active := 0
		for _, gid := range agg.groupIDs {
			if _, ok := activeGroups[gid]; ok {
				active++
			}
		}
		loadPct := 0.0
		if len(agg.groupIDs) > 0 {
			loadPct = float64(active) / float64(len(agg.groupIDs)) * 100.0
		}
		channels := make([]StatusChannel, 0, len(agg.lines))
		for _, line := range agg.lines {
			channels = append(channels, StatusChannel{
				Name:            publicStatusProbeLineDisplayName(line),
				AvailabilityPct: availability,
				Heartbeats:      beats,
			})
		}
		if len(channels) == 0 {
			channels = append(channels, StatusChannel{
				Name:            agg.name,
				AvailabilityPct: availability,
				Heartbeats:      beats,
			})
		}
		result.Groups = append(result.Groups, StatusGroup{
			Name:     agg.name,
			LoadPct:  loadPct,
			Channels: channels,
		})
	}
	if availabilityCount > 0 {
		result.AvailabilityPct = availabilityTotal / float64(availabilityCount)
		result.Heartbeats = mergeOverallBeats(overallBeats)
	}

	return result, nil
}

func (s *StatusPageService) loadListFromMonitorConfigs(ctx context.Context) (*listModelsCache, error) {
	monitors, err := s.listEnabledChannelMonitors(ctx)
	if err != nil {
		return nil, err
	}
	if len(monitors) == 0 {
		return nil, nil
	}

	type modelInfo struct {
		name     string
		provider string
	}
	byModel := map[string]*modelInfo{}
	for _, m := range monitors {
		for _, model := range monitorModels(m) {
			key := strings.ToLower(model)
			info := byModel[key]
			if info == nil {
				info = &modelInfo{name: model, provider: providerFromMonitor(m)}
				byModel[key] = info
			}
			if info.provider == "" {
				info.provider = providerFromMonitor(m)
			}
		}
	}

	models := make([]StatusModel, 0, len(byModel))
	modelSet := map[string]struct{}{}
	for _, info := range byModel {
		md := lookupMetadata(info.name)
		if info.provider != "" {
			md.Provider = info.provider
		}
		models = append(models, StatusModel{
			Name:          info.name,
			Provider:      md.Provider,
			ReleaseDate:   md.ReleaseDate,
			PromptCaching: md.PromptCaching,
			Note:          md.Note,
			Pricing:       md.Pricing,
		})
		modelSet[info.name] = struct{}{}
	}
	sort.SliceStable(models, func(i, j int) bool {
		ri, rj := publicStatusModelRank(models[i].Name), publicStatusModelRank(models[j].Name)
		if ri != rj {
			return ri < rj
		}
		return models[i].Name < models[j].Name
	})

	return &listModelsCache{
		data:     models,
		modelSet: modelSet,
		expireAt: s.nowFn().Add(statusCacheTTL),
	}, nil
}

func (s *StatusPageService) loadDetailFromMonitorConfigs(ctx context.Context, modelName string) (*StatusModel, bool, error) {
	monitors, err := s.listEnabledChannelMonitors(ctx)
	if err != nil {
		return nil, false, err
	}
	scoped := make([]*dbent.ChannelMonitor, 0, len(monitors))
	for _, m := range monitors {
		if monitorSupportsModel(m, modelName) {
			scoped = append(scoped, m)
		}
	}
	if len(scoped) == 0 {
		return nil, false, nil
	}

	md := lookupMetadata(modelName)
	if _, known := modelCatalog[modelName]; !known && providerFromMonitor(scoped[0]) != "" {
		md.Provider = providerFromMonitor(scoped[0])
	}
	result := &StatusModel{
		Name:          modelName,
		Provider:      md.Provider,
		ReleaseDate:   md.ReleaseDate,
		PromptCaching: md.PromptCaching,
		Note:          md.Note,
		Pricing:       md.Pricing,
		Heartbeats:    []StatusBeat{},
		Groups:        []StatusGroup{},
	}

	type monitorChannel struct {
		group string
		item  StatusChannel
		ok    bool
		beats []StatusBeat
	}
	channels := make([]monitorChannel, 0, len(scoped))
	for _, m := range scoped {
		history, err := s.monitorHistoryForModel(ctx, m.ID, modelName)
		if err != nil {
			return nil, false, err
		}
		beats := monitorHistoryToBeats(history)
		availability := availabilityFromBeats(beats)
		groupName := strings.TrimSpace(m.GroupName)
		if groupName == "" {
			groupName = providerDisplayName(providerFromMonitor(m))
		}
		channels = append(channels, monitorChannel{
			group: groupName,
			item: StatusChannel{
				Name:            m.Name,
				AvailabilityPct: availability,
				Heartbeats:      beats,
			},
			ok:    latestMonitorHistoryOK(history),
			beats: beats,
		})
	}

	groupOrder := []string{}
	grouped := map[string][]monitorChannel{}
	for _, ch := range channels {
		if _, ok := grouped[ch.group]; !ok {
			groupOrder = append(groupOrder, ch.group)
		}
		grouped[ch.group] = append(grouped[ch.group], ch)
	}
	sort.Strings(groupOrder)

	overallBeats := make([][]StatusBeat, 0, len(channels))
	availabilityTotal := 0.0
	for _, key := range groupOrder {
		rows := grouped[key]
		groupChannels := make([]StatusChannel, 0, len(rows))
		active := 0
		for _, row := range rows {
			groupChannels = append(groupChannels, row.item)
			availabilityTotal += row.item.AvailabilityPct
			overallBeats = append(overallBeats, row.beats)
			if row.ok {
				active++
			}
		}
		loadPct := 0.0
		if len(rows) > 0 {
			loadPct = float64(active) * 100.0 / float64(len(rows))
		}
		result.Groups = append(result.Groups, StatusGroup{
			Name:     key,
			LoadPct:  loadPct,
			Channels: groupChannels,
		})
	}
	if len(channels) > 0 {
		result.AvailabilityPct = availabilityTotal / float64(len(channels))
		result.Heartbeats = mergeOverallBeats(overallBeats)
	}
	return result, true, nil
}

func (s *StatusPageService) listEnabledChannelMonitors(ctx context.Context) ([]*dbent.ChannelMonitor, error) {
	if s == nil || s.entClient == nil {
		return nil, nil
	}
	monitors, err := s.entClient.ChannelMonitor.Query().
		Where(channelmonitor.EnabledEQ(true)).
		Order(dbent.Asc(channelmonitor.FieldGroupName), dbent.Asc(channelmonitor.FieldName), dbent.Asc(channelmonitor.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list channel monitors: %w", err)
	}
	return monitors, nil
}

func (s *StatusPageService) monitorHistoryForModel(ctx context.Context, monitorID int64, modelName string) ([]*dbent.ChannelMonitorHistory, error) {
	rows, err := s.entClient.ChannelMonitorHistory.Query().
		Where(channelmonitorhistory.MonitorIDEQ(monitorID), channelmonitorhistory.ModelEQ(modelName)).
		Order(dbent.Desc(channelmonitorhistory.FieldCheckedAt)).
		Limit(statusWindowMinutes).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("status_page_service: list monitor history: %w", err)
	}
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
	return rows, nil
}

func monitorModels(m *dbent.ChannelMonitor) []string {
	if m == nil {
		return nil
	}
	models := make([]string, 0, 1+len(m.ExtraModels))
	if primary := strings.TrimSpace(m.PrimaryModel); primary != "" {
		models = append(models, primary)
	}
	for _, model := range m.ExtraModels {
		if model = strings.TrimSpace(model); model != "" {
			models = append(models, model)
		}
	}
	return normalizeStringSlice(models)
}

func monitorSupportsModel(m *dbent.ChannelMonitor, modelName string) bool {
	target := strings.TrimSpace(modelName)
	if target == "" {
		return false
	}
	for _, model := range monitorModels(m) {
		if model == target {
			return true
		}
	}
	return false
}

func monitorHistoryToBeats(history []*dbent.ChannelMonitorHistory) []StatusBeat {
	beats := make([]StatusBeat, 0, len(history))
	for _, h := range history {
		beats = append(beats, StatusBeat{
			Ts:     h.CheckedAt.UTC().Format(time.RFC3339),
			Status: monitorStatusToBeatStatus(string(h.Status)),
		})
	}
	return beats
}

func latestMonitorHistoryOK(history []*dbent.ChannelMonitorHistory) bool {
	if len(history) == 0 {
		return false
	}
	switch string(history[len(history)-1].Status) {
	case MonitorStatusOperational, MonitorStatusDegraded:
		return true
	default:
		return false
	}
}

func monitorStatusToBeatStatus(status string) string {
	switch status {
	case MonitorStatusOperational:
		return "ok"
	case MonitorStatusDegraded:
		return "degraded"
	case MonitorStatusFailed, MonitorStatusError:
		return "down"
	default:
		return "unknown"
	}
}

func providerFromMonitor(m *dbent.ChannelMonitor) string {
	if m == nil {
		return ""
	}
	switch strings.ToLower(string(m.Provider)) {
	case PlatformAnthropic:
		return "ANTHROPIC"
	case PlatformGemini:
		return "GOOGLE"
	case PlatformOpenAI:
		return "OPENAI"
	default:
		return strings.ToUpper(string(m.Provider))
	}
}

func providerDisplayName(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "默认渠道"
	}
	return provider
}

func mergeOverallBeats(series [][]StatusBeat) []StatusBeat {
	if len(series) == 0 {
		return []StatusBeat{}
	}
	statusByTs := make(map[string]string)
	for _, beats := range series {
		for _, beat := range beats {
			if strings.TrimSpace(beat.Ts) == "" {
				continue
			}
			if existing, ok := statusByTs[beat.Ts]; ok {
				statusByTs[beat.Ts] = mergeStatus(existing, beat.Status)
			} else {
				statusByTs[beat.Ts] = beat.Status
			}
		}
	}
	timestamps := make([]string, 0, len(statusByTs))
	for ts := range statusByTs {
		timestamps = append(timestamps, ts)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		left, leftErr := time.Parse(time.RFC3339, timestamps[i])
		right, rightErr := time.Parse(time.RFC3339, timestamps[j])
		if leftErr == nil && rightErr == nil {
			return left.Before(right)
		}
		return timestamps[i] < timestamps[j]
	})
	if len(timestamps) > statusWindowMinutes {
		timestamps = timestamps[len(timestamps)-statusWindowMinutes:]
	}
	out := make([]StatusBeat, 0, len(timestamps))
	for _, ts := range timestamps {
		out = append(out, StatusBeat{Ts: ts, Status: statusByTs[ts]})
	}
	return out
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
