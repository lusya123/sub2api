package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const (
	modelMarketplaceUserViewCacheKey = "model_marketplace_user_view"
	modelMarketplaceUserViewCacheTTL = 5 * time.Second
)

func (s *ModelMarketplaceMonitorService) ListUserView(ctx context.Context) ([]*ModelMarketplaceUserMonitorView, error) {
	if views, ok := s.cachedUserView(); ok {
		return views, nil
	}

	loaded, err, _ := s.userViewSF.Do(modelMarketplaceUserViewCacheKey, func() (any, error) {
		if views, ok := s.cachedUserView(); ok {
			return views, nil
		}
		views, err := s.loadUserView(ctx)
		if err != nil {
			return nil, err
		}
		s.storeUserViewCache(views)
		return views, nil
	})
	if err != nil {
		return nil, err
	}
	views, _ := loaded.([]*ModelMarketplaceUserMonitorView)
	if views == nil {
		return []*ModelMarketplaceUserMonitorView{}, nil
	}
	return views, nil
}

func (s *ModelMarketplaceMonitorService) loadUserView(ctx context.Context) ([]*ModelMarketplaceUserMonitorView, error) {
	monitors, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled model marketplace monitors: %w", err)
	}
	if len(monitors) == 0 {
		return []*ModelMarketplaceUserMonitorView{}, nil
	}

	ids, primaryByID, extrasByID := collectModelMarketplaceMonitorIndexes(monitors)
	modelsByID := collectModelMarketplaceTimelineModels(primaryByID, extrasByID)
	summaries := s.BatchMonitorStatusSummary(ctx, ids, primaryByID, extrasByID)
	latestMap := s.batchModelMarketplaceLatest(ctx, ids)
	timelineMap := s.batchModelMarketplaceTimelines(ctx, modelsByID)

	views := make([]*ModelMarketplaceUserMonitorView, 0, len(monitors))
	for _, m := range monitors {
		primaryLatest := pickModelMarketplaceLatest(latestMap[m.ID], m.PrimaryModel)
		view := buildModelMarketplaceUserView(m, summaries[m.ID], primaryLatest, timelineMap[m.ID])
		s.enrichModelMarketplaceUserViewPricing(view, m.ModelCallConfigs)
		views = append(views, view)
	}
	return views, nil
}

func (s *ModelMarketplaceMonitorService) cachedUserView() ([]*ModelMarketplaceUserMonitorView, bool) {
	s.userViewMu.RLock()
	defer s.userViewMu.RUnlock()
	if s.userViewCache == nil || time.Now().After(s.userViewExpiresAt) {
		return nil, false
	}
	return s.userViewCache, true
}

func (s *ModelMarketplaceMonitorService) storeUserViewCache(views []*ModelMarketplaceUserMonitorView) {
	s.userViewMu.Lock()
	s.userViewCache = views
	s.userViewExpiresAt = time.Now().Add(modelMarketplaceUserViewCacheTTL)
	s.userViewMu.Unlock()
}

func (s *ModelMarketplaceMonitorService) invalidateUserViewCache() {
	s.userViewSF.Forget(modelMarketplaceUserViewCacheKey)
	s.userViewMu.Lock()
	s.userViewCache = nil
	s.userViewExpiresAt = time.Time{}
	s.userViewMu.Unlock()
}

func (s *ModelMarketplaceMonitorService) GetUserDetail(ctx context.Context, id int64) (*ModelMarketplaceUserMonitorDetail, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !m.Enabled {
		return nil, ErrModelMarketplaceMonitorNotFound
	}

	latest, err := s.repo.ListLatestPerModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list model marketplace latest per model: %w", err)
	}
	availMap, err := s.collectModelMarketplaceAvailabilityWindows(ctx, id)
	if err != nil {
		return nil, err
	}

	detail := &ModelMarketplaceUserMonitorDetail{
		ID:            m.ID,
		Name:          m.Name,
		Provider:      m.Provider,
		GroupName:     m.GroupName,
		EffectiveRate: normalizeModelMarketplaceMonitorEffectiveRateValue(m.EffectiveRate),
		Models:        mergeModelMarketplaceDetails(m, latest, availMap),
	}
	s.enrichModelMarketplaceDetailPricing(detail, m.ModelCallConfigs)
	return detail, nil
}

func collectModelMarketplaceMonitorIndexes(monitors []*ModelMarketplaceMonitor) ([]int64, map[int64]string, map[int64][]string) {
	ids := make([]int64, 0, len(monitors))
	primaryByID := make(map[int64]string, len(monitors))
	extrasByID := make(map[int64][]string, len(monitors))
	for _, m := range monitors {
		ids = append(ids, m.ID)
		primaryByID[m.ID] = m.PrimaryModel
		extrasByID[m.ID] = m.ExtraModels
	}
	return ids, primaryByID, extrasByID
}

func collectModelMarketplaceTimelineModels(primaryByID map[int64]string, extrasByID map[int64][]string) map[int64][]string {
	out := make(map[int64][]string, len(primaryByID))
	for id, primary := range primaryByID {
		models := make([]string, 0, 1+len(extrasByID[id]))
		if primary != "" {
			models = append(models, primary)
		}
		models = append(models, extrasByID[id]...)
		out[id] = models
	}
	return out
}

func (s *ModelMarketplaceMonitorService) batchModelMarketplaceLatest(ctx context.Context, ids []int64) map[int64][]*ModelMarketplaceMonitorLatest {
	latestMap, err := s.repo.ListLatestForMonitorIDs(ctx, ids)
	if err != nil {
		slog.Warn("model_marketplace_monitor: user view batch latest failed", "error", err)
		return map[int64][]*ModelMarketplaceMonitorLatest{}
	}
	return latestMap
}

func (s *ModelMarketplaceMonitorService) batchModelMarketplaceTimelines(
	ctx context.Context,
	modelsByID map[int64][]string,
) map[int64]map[string][]*ModelMarketplaceMonitorHistoryEntry {
	timelineMap, err := s.repo.ListRecentHistoryForMonitorModels(ctx, modelsByID, modelMarketplaceTimelineMaxPoints)
	if err != nil {
		slog.Warn("model_marketplace_monitor: user view batch timeline failed", "error", err)
		return map[int64]map[string][]*ModelMarketplaceMonitorHistoryEntry{}
	}
	return timelineMap
}

func pickModelMarketplaceLatest(rows []*ModelMarketplaceMonitorLatest, model string) *ModelMarketplaceMonitorLatest {
	if model == "" {
		return nil
	}
	for _, r := range rows {
		if r.Model == model {
			return r
		}
	}
	return nil
}

func (s *ModelMarketplaceMonitorService) collectModelMarketplaceAvailabilityWindows(
	ctx context.Context,
	monitorID int64,
) (map[int]map[string]*ModelMarketplaceMonitorAvailability, error) {
	out := make(map[int]map[string]*ModelMarketplaceMonitorAvailability, 3)
	windows := []int{
		modelMarketplaceAvailability7Days,
		modelMarketplaceAvailability15Days,
		modelMarketplaceAvailability30Days,
	}
	for _, w := range windows {
		rows, err := s.repo.ComputeAvailability(ctx, monitorID, w)
		if err != nil {
			return nil, fmt.Errorf("compute model marketplace availability %dd: %w", w, err)
		}
		out[w] = indexModelMarketplaceAvailabilityByModel(rows)
	}
	return out, nil
}

func buildModelMarketplaceUserView(
	m *ModelMarketplaceMonitor,
	summary ModelMarketplaceMonitorStatusSummary,
	primaryLatest *ModelMarketplaceMonitorLatest,
	timelineByModel map[string][]*ModelMarketplaceMonitorHistoryEntry,
) *ModelMarketplaceUserMonitorView {
	view := &ModelMarketplaceUserMonitorView{
		ID:                   m.ID,
		Name:                 m.Name,
		Provider:             m.Provider,
		GroupName:            m.GroupName,
		EffectiveRate:        normalizeModelMarketplaceMonitorEffectiveRateValue(m.EffectiveRate),
		PrimaryModel:         m.PrimaryModel,
		PrimaryDisplayNameZh: modelMarketplaceDisplayNameFor(m.ModelDisplayNames, m.PrimaryModel).Zh,
		PrimaryDisplayNameEn: modelMarketplaceDisplayNameFor(m.ModelDisplayNames, m.PrimaryModel).En,
		PrimaryCallModel:     modelMarketplaceCallModelFor(m.ModelCallConfigs, m.PrimaryModel),
		PrimaryRequestURL:    modelMarketplaceRequestURLFor(m.ModelCallConfigs, m.PrimaryModel),
		PrimaryStatus:        summary.PrimaryStatus,
		PrimaryLatencyMs:     summary.PrimaryLatencyMs,
		Availability7d:       summary.Availability7d,
		ExtraModels:          withModelMarketplaceExtraConfig(summary.ExtraModels, m.ModelDisplayNames, m.ModelCallConfigs, timelineByModel),
		Timeline:             buildModelMarketplaceTimelinePoints(timelineByModel[m.PrimaryModel]),
	}
	if primaryLatest != nil {
		view.PrimaryPingLatencyMs = primaryLatest.PingLatencyMs
	}
	return view
}

func normalizeModelMarketplaceMonitorEffectiveRateValue(rate float64) float64 {
	if rate <= 0 {
		return 1
	}
	return rate
}

func buildModelMarketplaceTimelinePoints(entries []*ModelMarketplaceMonitorHistoryEntry) []ModelMarketplaceUserTimelinePoint {
	out := make([]ModelMarketplaceUserTimelinePoint, 0, len(entries))
	for _, e := range entries {
		out = append(out, ModelMarketplaceUserTimelinePoint{
			Status:        e.Status,
			LatencyMs:     e.LatencyMs,
			PingLatencyMs: e.PingLatencyMs,
			CheckedAt:     e.CheckedAt,
		})
	}
	return out
}

func mergeModelMarketplaceDetails(
	m *ModelMarketplaceMonitor,
	latest []*ModelMarketplaceMonitorLatest,
	availMap map[int]map[string]*ModelMarketplaceMonitorAvailability,
) []ModelMarketplaceModelDetail {
	all := append([]string{m.PrimaryModel}, m.ExtraModels...)
	latestByModel := indexModelMarketplaceLatestByModel(latest)
	out := make([]ModelMarketplaceModelDetail, 0, len(all))
	for _, model := range all {
		names := modelMarketplaceDisplayNameFor(m.ModelDisplayNames, model)
		d := ModelMarketplaceModelDetail{
			Model:         model,
			DisplayNameZh: names.Zh,
			DisplayNameEn: names.En,
			CallModel:     modelMarketplaceCallModelFor(m.ModelCallConfigs, model),
			RequestURL:    modelMarketplaceRequestURLFor(m.ModelCallConfigs, model),
		}
		if l, ok := latestByModel[model]; ok {
			d.LatestStatus = l.Status
			d.LatestLatencyMs = l.LatencyMs
		}
		if a, ok := availMap[modelMarketplaceAvailability7Days][model]; ok {
			d.Availability7d = a.AvailabilityPct
			d.AvgLatency7dMs = a.AvgLatencyMs
		}
		if a, ok := availMap[modelMarketplaceAvailability15Days][model]; ok {
			d.Availability15d = a.AvailabilityPct
		}
		if a, ok := availMap[modelMarketplaceAvailability30Days][model]; ok {
			d.Availability30d = a.AvailabilityPct
		}
		out = append(out, d)
	}
	return out
}

func withModelMarketplaceExtraConfig(
	extras []ModelMarketplaceExtraModelStatus,
	names map[string]ModelMarketplaceModelDisplayName,
	callConfigs map[string]ModelMarketplaceModelCallConfig,
	timelineByModel map[string][]*ModelMarketplaceMonitorHistoryEntry,
) []ModelMarketplaceExtraModelStatus {
	out := make([]ModelMarketplaceExtraModelStatus, len(extras))
	copy(out, extras)
	for i := range out {
		display := modelMarketplaceDisplayNameFor(names, out[i].Model)
		out[i].DisplayNameZh = display.Zh
		out[i].DisplayNameEn = display.En
		out[i].CallModel = modelMarketplaceCallModelFor(callConfigs, out[i].Model)
		out[i].RequestURL = modelMarketplaceRequestURLFor(callConfigs, out[i].Model)
		out[i].Timeline = buildModelMarketplaceTimelinePoints(timelineByModel[out[i].Model])
	}
	return out
}

func modelMarketplaceDisplayNameFor(
	names map[string]ModelMarketplaceModelDisplayName,
	model string,
) ModelMarketplaceModelDisplayName {
	if names == nil {
		return ModelMarketplaceModelDisplayName{}
	}
	return names[model]
}

func modelMarketplaceCallModelFor(
	callConfigs map[string]ModelMarketplaceModelCallConfig,
	model string,
) string {
	if callConfigs == nil {
		return model
	}
	cfg := callConfigs[model]
	if cfg.Model == "" {
		return model
	}
	return cfg.Model
}

func modelMarketplaceRequestURLFor(
	callConfigs map[string]ModelMarketplaceModelCallConfig,
	model string,
) string {
	if callConfigs == nil {
		return ""
	}
	return callConfigs[model].RequestURL
}

func (s *ModelMarketplaceMonitorService) modelMarketplaceDisplayPricing(model string) *ChannelModelPricing {
	if s == nil || s.pricingService == nil {
		return nil
	}
	return synthesizePricingFromLiteLLM(s.pricingService.GetModelPricing(model))
}

func (s *ModelMarketplaceMonitorService) modelMarketplaceDisplayPricingForConfig(
	callModel string,
	configModel string,
	callConfigs map[string]ModelMarketplaceModelCallConfig,
) *ChannelModelPricing {
	if override := modelMarketplacePricingOverrideFor(callConfigs, configModel); override != nil {
		return override
	}
	return s.modelMarketplaceDisplayPricing(firstNonEmptyMarketplaceString(callModel, configModel))
}

func modelMarketplacePricingOverrideFor(
	callConfigs map[string]ModelMarketplaceModelCallConfig,
	model string,
) *ChannelModelPricing {
	if callConfigs == nil {
		return nil
	}
	override := callConfigs[model].Pricing
	if override == nil {
		return nil
	}
	const perMillion = 1_000_000
	p := &ChannelModelPricing{BillingMode: BillingModeToken}
	if override.InputPricePerMillion != nil {
		v := *override.InputPricePerMillion / perMillion
		p.InputPrice = &v
	}
	if override.OutputPricePerMillion != nil {
		v := *override.OutputPricePerMillion / perMillion
		p.OutputPrice = &v
	}
	if p.InputPrice == nil && p.OutputPrice == nil {
		return nil
	}
	return p
}

func (s *ModelMarketplaceMonitorService) enrichModelMarketplaceUserViewPricing(
	view *ModelMarketplaceUserMonitorView,
	callConfigs map[string]ModelMarketplaceModelCallConfig,
) {
	if view == nil {
		return
	}
	view.PrimaryPricing = s.modelMarketplaceDisplayPricingForConfig(view.PrimaryCallModel, view.PrimaryModel, callConfigs)
	for i := range view.ExtraModels {
		view.ExtraModels[i].Pricing = s.modelMarketplaceDisplayPricingForConfig(
			view.ExtraModels[i].CallModel,
			view.ExtraModels[i].Model,
			callConfigs,
		)
	}
}

func (s *ModelMarketplaceMonitorService) enrichModelMarketplaceDetailPricing(
	detail *ModelMarketplaceUserMonitorDetail,
	callConfigs map[string]ModelMarketplaceModelCallConfig,
) {
	if detail == nil {
		return
	}
	for i := range detail.Models {
		detail.Models[i].Pricing = s.modelMarketplaceDisplayPricingForConfig(
			detail.Models[i].CallModel,
			detail.Models[i].Model,
			callConfigs,
		)
	}
}

func firstNonEmptyMarketplaceString(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
