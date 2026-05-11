package service

import (
	"context"
	"fmt"
	"log/slog"
)

func (s *ModelMarketplaceMonitorService) ListUserView(ctx context.Context) ([]*ModelMarketplaceUserMonitorView, error) {
	monitors, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("list enabled model marketplace monitors: %w", err)
	}
	if len(monitors) == 0 {
		return []*ModelMarketplaceUserMonitorView{}, nil
	}

	ids, primaryByID, extrasByID := collectModelMarketplaceMonitorIndexes(monitors)
	summaries := s.BatchMonitorStatusSummary(ctx, ids, primaryByID, extrasByID)
	latestMap := s.batchModelMarketplaceLatest(ctx, ids)
	timelineMap := s.batchModelMarketplaceTimeline(ctx, ids, primaryByID)

	views := make([]*ModelMarketplaceUserMonitorView, 0, len(monitors))
	for _, m := range monitors {
		primaryLatest := pickModelMarketplaceLatest(latestMap[m.ID], m.PrimaryModel)
		view := buildModelMarketplaceUserView(m, summaries[m.ID], primaryLatest, timelineMap[m.ID])
		s.enrichModelMarketplaceUserViewPricing(view)
		views = append(views, view)
	}
	return views, nil
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
		ID:        m.ID,
		Name:      m.Name,
		Provider:  m.Provider,
		GroupName: m.GroupName,
		Models:    mergeModelMarketplaceDetails(m, latest, availMap),
	}
	s.enrichModelMarketplaceDetailPricing(detail)
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

func (s *ModelMarketplaceMonitorService) batchModelMarketplaceLatest(ctx context.Context, ids []int64) map[int64][]*ModelMarketplaceMonitorLatest {
	latestMap, err := s.repo.ListLatestForMonitorIDs(ctx, ids)
	if err != nil {
		slog.Warn("model_marketplace_monitor: user view batch latest failed", "error", err)
		return map[int64][]*ModelMarketplaceMonitorLatest{}
	}
	return latestMap
}

func (s *ModelMarketplaceMonitorService) batchModelMarketplaceTimeline(
	ctx context.Context,
	ids []int64,
	primaryByID map[int64]string,
) map[int64][]*ModelMarketplaceMonitorHistoryEntry {
	timelineMap, err := s.repo.ListRecentHistoryForMonitors(ctx, ids, primaryByID, modelMarketplaceTimelineMaxPoints)
	if err != nil {
		slog.Warn("model_marketplace_monitor: user view batch timeline failed", "error", err)
		return map[int64][]*ModelMarketplaceMonitorHistoryEntry{}
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
	timelineEntries []*ModelMarketplaceMonitorHistoryEntry,
) *ModelMarketplaceUserMonitorView {
	view := &ModelMarketplaceUserMonitorView{
		ID:               m.ID,
		Name:             m.Name,
		Provider:         m.Provider,
		GroupName:        m.GroupName,
		PrimaryModel:     m.PrimaryModel,
		PrimaryStatus:    summary.PrimaryStatus,
		PrimaryLatencyMs: summary.PrimaryLatencyMs,
		Availability7d:   summary.Availability7d,
		ExtraModels:      summary.ExtraModels,
		Timeline:         buildModelMarketplaceTimelinePoints(timelineEntries),
	}
	if primaryLatest != nil {
		view.PrimaryPingLatencyMs = primaryLatest.PingLatencyMs
	}
	return view
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
		d := ModelMarketplaceModelDetail{Model: model}
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

func (s *ModelMarketplaceMonitorService) modelMarketplaceDisplayPricing(model string) *ChannelModelPricing {
	if s == nil || s.pricingService == nil {
		return nil
	}
	return synthesizePricingFromLiteLLM(s.pricingService.GetModelPricing(model))
}

func (s *ModelMarketplaceMonitorService) enrichModelMarketplaceUserViewPricing(view *ModelMarketplaceUserMonitorView) {
	if view == nil {
		return
	}
	view.PrimaryPricing = s.modelMarketplaceDisplayPricing(view.PrimaryModel)
	for i := range view.ExtraModels {
		view.ExtraModels[i].Pricing = s.modelMarketplaceDisplayPricing(view.ExtraModels[i].Model)
	}
}

func (s *ModelMarketplaceMonitorService) enrichModelMarketplaceDetailPricing(detail *ModelMarketplaceUserMonitorDetail) {
	if detail == nil {
		return
	}
	for i := range detail.Models {
		detail.Models[i].Pricing = s.modelMarketplaceDisplayPricing(detail.Models[i].Model)
	}
}
