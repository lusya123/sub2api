package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"golang.org/x/sync/errgroup"
)

var (
	ErrModelMarketplaceMonitorNotFound = infraerrors.NotFound(
		"MODEL_MARKETPLACE_MONITOR_NOT_FOUND", "model marketplace monitor not found",
	)
	ErrModelMarketplaceMonitorInvalidProvider = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_INVALID_PROVIDER", "provider must be one of openai/anthropic/gemini",
	)
	ErrModelMarketplaceMonitorInvalidInterval = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_INVALID_INTERVAL", "interval_seconds must be in [15, 3600]",
	)
	ErrModelMarketplaceMonitorInvalidEndpoint = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_INVALID_ENDPOINT", "endpoint must be a valid https URL",
	)
	ErrModelMarketplaceMonitorEndpointScheme = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_ENDPOINT_SCHEME", "endpoint must use https scheme",
	)
	ErrModelMarketplaceMonitorEndpointPath = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_ENDPOINT_PATH", "endpoint must be base origin only (no path/query/fragment)",
	)
	ErrModelMarketplaceMonitorEndpointPrivate = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_ENDPOINT_PRIVATE", "endpoint must be a public host",
	)
	ErrModelMarketplaceMonitorEndpointUnreachable = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_ENDPOINT_UNREACHABLE", "endpoint hostname could not be resolved",
	)
	ErrModelMarketplaceMonitorMissingAPIKey = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_MISSING_API_KEY", "api_key is required when creating a model marketplace monitor",
	)
	ErrModelMarketplaceMonitorMissingPrimaryModel = infraerrors.BadRequest(
		"MODEL_MARKETPLACE_MONITOR_MISSING_PRIMARY_MODEL", "primary_model is required",
	)
	ErrModelMarketplaceMonitorAPIKeyDecryptFailed = infraerrors.InternalServer(
		"MODEL_MARKETPLACE_MONITOR_KEY_DECRYPT_FAILED", "api key decryption failed; please re-edit the model marketplace monitor with a fresh key",
	)
)

type ModelMarketplaceMonitorScheduler interface {
	Schedule(m *ModelMarketplaceMonitor)
	Unschedule(id int64)
}

type ModelMarketplaceMonitorService struct {
	repo      ModelMarketplaceMonitorRepository
	encryptor SecretEncryptor
	scheduler ModelMarketplaceMonitorScheduler
}

func NewModelMarketplaceMonitorService(repo ModelMarketplaceMonitorRepository, encryptor SecretEncryptor) *ModelMarketplaceMonitorService {
	return &ModelMarketplaceMonitorService{repo: repo, encryptor: encryptor}
}

func (s *ModelMarketplaceMonitorService) List(ctx context.Context, params ModelMarketplaceMonitorListParams) ([]*ModelMarketplaceMonitor, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 200 {
		params.PageSize = 20
	}
	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list model marketplace monitors: %w", err)
	}
	for _, it := range items {
		s.decryptInPlace(it)
	}
	return items, total, nil
}

func (s *ModelMarketplaceMonitorService) Get(ctx context.Context, id int64) (*ModelMarketplaceMonitor, error) {
	m, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.decryptInPlace(m)
	return m, nil
}

func (s *ModelMarketplaceMonitorService) Create(ctx context.Context, p ModelMarketplaceMonitorCreateParams) (*ModelMarketplaceMonitor, error) {
	if err := validateModelMarketplaceCreateParams(p); err != nil {
		return nil, err
	}
	if err := validateModelMarketplaceBodyModeParams(p.BodyOverrideMode, p.BodyOverride); err != nil {
		return nil, err
	}
	if err := validateModelMarketplaceExtraHeaders(p.ExtraHeaders); err != nil {
		return nil, err
	}
	encrypted, err := s.encryptor.Encrypt(p.APIKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt api key: %w", err)
	}
	m := &ModelMarketplaceMonitor{
		Name:             strings.TrimSpace(p.Name),
		Provider:         p.Provider,
		Endpoint:         normalizeModelMarketplaceEndpoint(p.Endpoint),
		APIKey:           encrypted,
		PrimaryModel:     strings.TrimSpace(p.PrimaryModel),
		ExtraModels:      normalizeModelMarketplaceModels(p.ExtraModels),
		GroupName:        strings.TrimSpace(p.GroupName),
		Enabled:          p.Enabled,
		IntervalSeconds:  p.IntervalSeconds,
		CreatedBy:        p.CreatedBy,
		TemplateID:       p.TemplateID,
		ExtraHeaders:     emptyModelMarketplaceHeadersIfNil(p.ExtraHeaders),
		BodyOverrideMode: defaultModelMarketplaceBodyMode(p.BodyOverrideMode),
		BodyOverride:     p.BodyOverride,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("create model marketplace monitor: %w", err)
	}
	m.APIKey = strings.TrimSpace(p.APIKey)
	if s.scheduler != nil {
		s.scheduler.Schedule(m)
	}
	return m, nil
}

func validateModelMarketplaceCreateParams(p ModelMarketplaceMonitorCreateParams) error {
	if err := validateModelMarketplaceProvider(p.Provider); err != nil {
		return err
	}
	if err := validateModelMarketplaceInterval(p.IntervalSeconds); err != nil {
		return err
	}
	if err := validateModelMarketplaceEndpoint(p.Endpoint); err != nil {
		return err
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return ErrModelMarketplaceMonitorMissingAPIKey
	}
	if strings.TrimSpace(p.PrimaryModel) == "" {
		return ErrModelMarketplaceMonitorMissingPrimaryModel
	}
	return nil
}

func (s *ModelMarketplaceMonitorService) Update(ctx context.Context, id int64, p ModelMarketplaceMonitorUpdateParams) (*ModelMarketplaceMonitor, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := applyModelMarketplaceMonitorUpdate(existing, p); err != nil {
		return nil, err
	}
	newPlainAPIKey, apiKeyUpdated, err := s.applyAPIKeyUpdate(existing, p.APIKey)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, fmt.Errorf("update model marketplace monitor: %w", err)
	}
	if apiKeyUpdated {
		existing.APIKey = newPlainAPIKey
	} else {
		s.decryptInPlace(existing)
	}
	if s.scheduler != nil {
		s.scheduler.Schedule(existing)
	}
	return existing, nil
}

func (s *ModelMarketplaceMonitorService) applyAPIKeyUpdate(existing *ModelMarketplaceMonitor, raw *string) (plain string, updated bool, err error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "", false, nil
	}
	plain = strings.TrimSpace(*raw)
	encrypted, encErr := s.encryptor.Encrypt(plain)
	if encErr != nil {
		return "", false, fmt.Errorf("encrypt api key: %w", encErr)
	}
	existing.APIKey = encrypted
	return plain, true, nil
}

func (s *ModelMarketplaceMonitorService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete model marketplace monitor: %w", err)
	}
	if s.scheduler != nil {
		s.scheduler.Unschedule(id)
	}
	return nil
}

func (s *ModelMarketplaceMonitorService) ListHistory(ctx context.Context, id int64, model string, limit int) ([]*ModelMarketplaceMonitorHistoryEntry, error) {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = ModelMarketplaceMonitorHistoryDefaultLimit
	}
	if limit > ModelMarketplaceMonitorHistoryMaxLimit {
		limit = ModelMarketplaceMonitorHistoryMaxLimit
	}
	return s.repo.ListHistory(ctx, id, strings.TrimSpace(model), limit)
}

func (s *ModelMarketplaceMonitorService) RunCheck(ctx context.Context, id int64) ([]*ModelMarketplaceCheckResult, error) {
	m, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m.APIKeyDecryptFailed {
		return nil, ErrModelMarketplaceMonitorAPIKeyDecryptFailed
	}
	results := s.runChecksConcurrent(ctx, m)
	s.persistCheckResults(ctx, m, results)
	return results, nil
}

func (s *ModelMarketplaceMonitorService) runChecksConcurrent(ctx context.Context, m *ModelMarketplaceMonitor) []*ModelMarketplaceCheckResult {
	models := append([]string{m.PrimaryModel}, m.ExtraModels...)
	results := make([]*ModelMarketplaceCheckResult, len(models))
	pingMs := pingModelMarketplaceEndpointOrigin(ctx, m.Endpoint)
	opts := &ModelMarketplaceCheckOptions{
		ExtraHeaders:     m.ExtraHeaders,
		BodyOverrideMode: m.BodyOverrideMode,
		BodyOverride:     m.BodyOverride,
	}
	var eg errgroup.Group
	var mu sync.Mutex
	for i, model := range models {
		i, model := i, model
		eg.Go(func() error {
			r := runModelMarketplaceCheckForModel(ctx, m.Provider, m.Endpoint, m.APIKey, model, opts)
			r.PingLatencyMs = pingMs
			mu.Lock()
			results[i] = r
			mu.Unlock()
			return nil
		})
	}
	_ = eg.Wait()
	return results
}

func (s *ModelMarketplaceMonitorService) persistCheckResults(ctx context.Context, m *ModelMarketplaceMonitor, results []*ModelMarketplaceCheckResult) {
	rows := make([]*ModelMarketplaceMonitorHistoryRow, 0, len(results))
	for _, r := range results {
		rows = append(rows, &ModelMarketplaceMonitorHistoryRow{
			MonitorID:     m.ID,
			Model:         r.Model,
			Status:        r.Status,
			LatencyMs:     r.LatencyMs,
			PingLatencyMs: r.PingLatencyMs,
			Message:       r.Message,
			CheckedAt:     r.CheckedAt,
		})
	}
	if err := s.repo.InsertHistoryBatch(ctx, rows); err != nil {
		slog.Error("model_marketplace_monitor: insert history failed", "monitor_id", m.ID, "name", m.Name, "error", err)
	}
	if err := s.repo.MarkChecked(ctx, m.ID, time.Now()); err != nil {
		slog.Error("model_marketplace_monitor: mark checked failed", "monitor_id", m.ID, "error", err)
	}
}

func (s *ModelMarketplaceMonitorService) SetScheduler(sched ModelMarketplaceMonitorScheduler) {
	s.scheduler = sched
}

func (s *ModelMarketplaceMonitorService) ListEnabledMonitors(ctx context.Context) ([]*ModelMarketplaceMonitor, error) {
	all, err := s.repo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range all {
		s.decryptInPlace(m)
	}
	return all, nil
}

func (s *ModelMarketplaceMonitorService) DeleteHistoryBefore(ctx context.Context, before time.Time) (int64, error) {
	return s.repo.DeleteHistoryBefore(ctx, before)
}

func (s *ModelMarketplaceMonitorService) BatchMonitorStatusSummary(
	ctx context.Context,
	ids []int64,
	primaryByID map[int64]string,
	extrasByID map[int64][]string,
) map[int64]ModelMarketplaceMonitorStatusSummary {
	out := make(map[int64]ModelMarketplaceMonitorStatusSummary, len(ids))
	if len(ids) == 0 {
		return out
	}
	latestMap, err := s.repo.ListLatestForMonitorIDs(ctx, ids)
	if err != nil {
		slog.Warn("model_marketplace_monitor: batch load latest failed", "error", err)
		latestMap = map[int64][]*ModelMarketplaceMonitorLatest{}
	}
	availMap, err := s.repo.ComputeAvailabilityForMonitors(ctx, ids, modelMarketplaceAvailability7Days)
	if err != nil {
		slog.Warn("model_marketplace_monitor: batch compute availability failed", "error", err)
		availMap = map[int64][]*ModelMarketplaceMonitorAvailability{}
	}
	for _, id := range ids {
		out[id] = buildModelMarketplaceStatusSummary(
			indexModelMarketplaceLatestByModel(latestMap[id]),
			indexModelMarketplaceAvailabilityByModel(availMap[id]),
			primaryByID[id],
			extrasByID[id],
		)
	}
	return out
}

func (s *ModelMarketplaceMonitorService) decryptInPlace(m *ModelMarketplaceMonitor) {
	if m == nil || m.APIKey == "" {
		return
	}
	plain, err := s.encryptor.Decrypt(m.APIKey)
	if err != nil {
		slog.Warn("model_marketplace_monitor: decrypt api key failed", "monitor_id", m.ID, "error", err)
		m.APIKey = ""
		m.APIKeyDecryptFailed = true
		return
	}
	m.APIKey = plain
}

func applyModelMarketplaceMonitorUpdate(existing *ModelMarketplaceMonitor, p ModelMarketplaceMonitorUpdateParams) error {
	if p.Name != nil {
		existing.Name = strings.TrimSpace(*p.Name)
	}
	if p.Provider != nil {
		if err := validateModelMarketplaceProvider(*p.Provider); err != nil {
			return err
		}
		existing.Provider = *p.Provider
	}
	if p.Endpoint != nil {
		if err := validateModelMarketplaceEndpoint(*p.Endpoint); err != nil {
			return err
		}
		existing.Endpoint = normalizeModelMarketplaceEndpoint(*p.Endpoint)
	}
	if p.PrimaryModel != nil {
		existing.PrimaryModel = strings.TrimSpace(*p.PrimaryModel)
	}
	if p.ExtraModels != nil {
		existing.ExtraModels = normalizeModelMarketplaceModels(*p.ExtraModels)
	}
	if p.GroupName != nil {
		existing.GroupName = strings.TrimSpace(*p.GroupName)
	}
	if p.Enabled != nil {
		existing.Enabled = *p.Enabled
	}
	if p.IntervalSeconds != nil {
		if err := validateModelMarketplaceInterval(*p.IntervalSeconds); err != nil {
			return err
		}
		existing.IntervalSeconds = *p.IntervalSeconds
	}
	return applyModelMarketplaceAdvancedUpdate(existing, p)
}

func applyModelMarketplaceAdvancedUpdate(existing *ModelMarketplaceMonitor, p ModelMarketplaceMonitorUpdateParams) error {
	if p.ClearTemplate {
		existing.TemplateID = nil
	} else if p.TemplateID != nil {
		id := *p.TemplateID
		existing.TemplateID = &id
	}
	if p.ExtraHeaders != nil {
		if err := validateModelMarketplaceExtraHeaders(*p.ExtraHeaders); err != nil {
			return err
		}
		existing.ExtraHeaders = emptyModelMarketplaceHeadersIfNil(*p.ExtraHeaders)
	}
	newMode := existing.BodyOverrideMode
	newBody := existing.BodyOverride
	if p.BodyOverrideMode != nil {
		newMode = *p.BodyOverrideMode
	}
	if p.BodyOverride != nil {
		newBody = *p.BodyOverride
	}
	if p.BodyOverrideMode != nil || p.BodyOverride != nil {
		if err := validateModelMarketplaceBodyModeParams(newMode, newBody); err != nil {
			return err
		}
		existing.BodyOverrideMode = defaultModelMarketplaceBodyMode(newMode)
		existing.BodyOverride = newBody
	}
	return nil
}

func indexModelMarketplaceLatestByModel(rows []*ModelMarketplaceMonitorLatest) map[string]*ModelMarketplaceMonitorLatest {
	m := make(map[string]*ModelMarketplaceMonitorLatest, len(rows))
	for _, r := range rows {
		m[r.Model] = r
	}
	return m
}

func indexModelMarketplaceAvailabilityByModel(rows []*ModelMarketplaceMonitorAvailability) map[string]*ModelMarketplaceMonitorAvailability {
	m := make(map[string]*ModelMarketplaceMonitorAvailability, len(rows))
	for _, r := range rows {
		m[r.Model] = r
	}
	return m
}

func buildModelMarketplaceStatusSummary(
	latestByModel map[string]*ModelMarketplaceMonitorLatest,
	availByModel map[string]*ModelMarketplaceMonitorAvailability,
	primary string,
	extras []string,
) ModelMarketplaceMonitorStatusSummary {
	summary := ModelMarketplaceMonitorStatusSummary{ExtraModels: make([]ModelMarketplaceExtraModelStatus, 0, len(extras))}
	if primary != "" {
		if l, ok := latestByModel[primary]; ok {
			summary.PrimaryStatus = l.Status
			summary.PrimaryLatencyMs = l.LatencyMs
		}
		if a, ok := availByModel[primary]; ok {
			summary.Availability7d = a.AvailabilityPct
		}
	}
	for _, model := range extras {
		entry := ModelMarketplaceExtraModelStatus{Model: model}
		if l, ok := latestByModel[model]; ok {
			entry.Status = l.Status
			entry.LatencyMs = l.LatencyMs
		}
		summary.ExtraModels = append(summary.ExtraModels, entry)
	}
	return summary
}
