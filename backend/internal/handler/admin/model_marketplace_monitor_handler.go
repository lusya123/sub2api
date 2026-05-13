package admin

import (
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ModelMarketplaceMonitorHandler struct {
	monitorService *service.ModelMarketplaceMonitorService
}

func NewModelMarketplaceMonitorHandler(monitorService *service.ModelMarketplaceMonitorService) *ModelMarketplaceMonitorHandler {
	return &ModelMarketplaceMonitorHandler{monitorService: monitorService}
}

const (
	modelMarketplaceMonitorMaxPageSize    = 100
	modelMarketplaceAPIKeyMaskPrefix      = 4
	modelMarketplaceAPIKeyMaskPlaceholder = "***"
)

type modelMarketplaceMonitorCreateRequest struct {
	Name              string                                              `json:"name" binding:"required,max=100"`
	Provider          string                                              `json:"provider" binding:"required,max=50"`
	Endpoint          string                                              `json:"endpoint" binding:"required,max=500"`
	APIKey            string                                              `json:"api_key" binding:"required,max=2000"`
	PrimaryModel      string                                              `json:"primary_model" binding:"required,max=200"`
	ExtraModels       []string                                            `json:"extra_models"`
	ModelDisplayNames map[string]service.ModelMarketplaceModelDisplayName `json:"model_display_names"`
	ModelCallConfigs  map[string]service.ModelMarketplaceModelCallConfig  `json:"model_call_configs"`
	GroupName         string                                              `json:"group_name" binding:"max=100"`
	EffectiveRate     *float64                                            `json:"effective_rate"`
	Enabled           *bool                                               `json:"enabled"`
	IntervalSeconds   int                                                 `json:"interval_seconds" binding:"required,min=15,max=3600"`
	TemplateID        *int64                                              `json:"template_id"`
	ExtraHeaders      map[string]string                                   `json:"extra_headers"`
	BodyOverrideMode  string                                              `json:"body_override_mode" binding:"omitempty,oneof=off merge replace"`
	BodyOverride      map[string]any                                      `json:"body_override"`
}

type modelMarketplaceMonitorUpdateRequest struct {
	Name              *string                                              `json:"name" binding:"omitempty,max=100"`
	Provider          *string                                              `json:"provider" binding:"omitempty,max=50"`
	Endpoint          *string                                              `json:"endpoint" binding:"omitempty,max=500"`
	APIKey            *string                                              `json:"api_key" binding:"omitempty,max=2000"`
	PrimaryModel      *string                                              `json:"primary_model" binding:"omitempty,max=200"`
	ExtraModels       *[]string                                            `json:"extra_models"`
	ModelDisplayNames *map[string]service.ModelMarketplaceModelDisplayName `json:"model_display_names"`
	ModelCallConfigs  *map[string]service.ModelMarketplaceModelCallConfig  `json:"model_call_configs"`
	GroupName         *string                                              `json:"group_name" binding:"omitempty,max=100"`
	EffectiveRate     *float64                                             `json:"effective_rate"`
	Enabled           *bool                                                `json:"enabled"`
	IntervalSeconds   *int                                                 `json:"interval_seconds" binding:"omitempty,min=15,max=3600"`
	TemplateID        *int64                                               `json:"template_id"`
	ClearTemplate     bool                                                 `json:"clear_template"`
	ExtraHeaders      *map[string]string                                   `json:"extra_headers"`
	BodyOverrideMode  *string                                              `json:"body_override_mode" binding:"omitempty,oneof=off merge replace"`
	BodyOverride      *map[string]any                                      `json:"body_override"`
}

type modelMarketplaceExtraModelStatusResponse struct {
	Model     string `json:"model"`
	Status    string `json:"status"`
	LatencyMs *int   `json:"latency_ms"`
}

type modelMarketplaceMonitorResponse struct {
	ID                  int64                                               `json:"id"`
	Name                string                                              `json:"name"`
	Provider            string                                              `json:"provider"`
	Endpoint            string                                              `json:"endpoint"`
	APIKeyMasked        string                                              `json:"api_key_masked"`
	APIKeyDecryptFailed bool                                                `json:"api_key_decrypt_failed"`
	PrimaryModel        string                                              `json:"primary_model"`
	ExtraModels         []string                                            `json:"extra_models"`
	ModelDisplayNames   map[string]service.ModelMarketplaceModelDisplayName `json:"model_display_names"`
	ModelCallConfigs    map[string]service.ModelMarketplaceModelCallConfig  `json:"model_call_configs"`
	GroupName           string                                              `json:"group_name"`
	EffectiveRate       float64                                             `json:"effective_rate"`
	Enabled             bool                                                `json:"enabled"`
	IntervalSeconds     int                                                 `json:"interval_seconds"`
	LastCheckedAt       *string                                             `json:"last_checked_at"`
	CreatedBy           int64                                               `json:"created_by"`
	CreatedAt           string                                              `json:"created_at"`
	UpdatedAt           string                                              `json:"updated_at"`
	PrimaryStatus       string                                              `json:"primary_status"`
	PrimaryLatencyMs    *int                                                `json:"primary_latency_ms"`
	Availability7d      float64                                             `json:"availability_7d"`
	ExtraModelsStatus   []modelMarketplaceExtraModelStatusResponse          `json:"extra_models_status"`
	TemplateID          *int64                                              `json:"template_id"`
	ExtraHeaders        map[string]string                                   `json:"extra_headers"`
	BodyOverrideMode    string                                              `json:"body_override_mode"`
	BodyOverride        map[string]any                                      `json:"body_override"`
}

type modelMarketplaceCheckResultResponse struct {
	Model         string `json:"model"`
	Status        string `json:"status"`
	LatencyMs     *int   `json:"latency_ms"`
	PingLatencyMs *int   `json:"ping_latency_ms"`
	Message       string `json:"message"`
	CheckedAt     string `json:"checked_at"`
}

type modelMarketplaceHistoryItemResponse struct {
	ID            int64  `json:"id"`
	Model         string `json:"model"`
	Status        string `json:"status"`
	LatencyMs     *int   `json:"latency_ms"`
	PingLatencyMs *int   `json:"ping_latency_ms"`
	Message       string `json:"message"`
	CheckedAt     string `json:"checked_at"`
}

func modelMarketplaceMonitorToResponse(m *service.ModelMarketplaceMonitor) *modelMarketplaceMonitorResponse {
	if m == nil {
		return nil
	}
	extras := m.ExtraModels
	if extras == nil {
		extras = []string{}
	}
	headers := m.ExtraHeaders
	if headers == nil {
		headers = map[string]string{}
	}
	resp := &modelMarketplaceMonitorResponse{
		ID:                  m.ID,
		Name:                m.Name,
		Provider:            m.Provider,
		Endpoint:            m.Endpoint,
		APIKeyMasked:        maskModelMarketplaceAPIKey(m.APIKey),
		APIKeyDecryptFailed: m.APIKeyDecryptFailed,
		PrimaryModel:        m.PrimaryModel,
		ExtraModels:         extras,
		ModelDisplayNames:   emptyModelMarketplaceDisplayNamesResponse(m.ModelDisplayNames),
		ModelCallConfigs:    emptyModelMarketplaceCallConfigsResponse(m.ModelCallConfigs),
		GroupName:           m.GroupName,
		EffectiveRate:       m.EffectiveRate,
		Enabled:             m.Enabled,
		IntervalSeconds:     m.IntervalSeconds,
		CreatedBy:           m.CreatedBy,
		CreatedAt:           m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           m.UpdatedAt.UTC().Format(time.RFC3339),
		TemplateID:          m.TemplateID,
		ExtraHeaders:        headers,
		BodyOverrideMode:    m.BodyOverrideMode,
		BodyOverride:        m.BodyOverride,
	}
	if m.LastCheckedAt != nil {
		s := m.LastCheckedAt.UTC().Format(time.RFC3339)
		resp.LastCheckedAt = &s
	}
	return resp
}

func emptyModelMarketplaceDisplayNamesResponse(in map[string]service.ModelMarketplaceModelDisplayName) map[string]service.ModelMarketplaceModelDisplayName {
	if in == nil {
		return map[string]service.ModelMarketplaceModelDisplayName{}
	}
	return in
}

func emptyModelMarketplaceCallConfigsResponse(in map[string]service.ModelMarketplaceModelCallConfig) map[string]service.ModelMarketplaceModelCallConfig {
	if in == nil {
		return map[string]service.ModelMarketplaceModelCallConfig{}
	}
	return in
}

func modelMarketplaceCheckResultToResponse(r *service.ModelMarketplaceCheckResult) modelMarketplaceCheckResultResponse {
	return modelMarketplaceCheckResultResponse{
		Model:         r.Model,
		Status:        r.Status,
		LatencyMs:     r.LatencyMs,
		PingLatencyMs: r.PingLatencyMs,
		Message:       r.Message,
		CheckedAt:     r.CheckedAt.UTC().Format(time.RFC3339),
	}
}

func modelMarketplaceHistoryEntryToResponse(e *service.ModelMarketplaceMonitorHistoryEntry) modelMarketplaceHistoryItemResponse {
	return modelMarketplaceHistoryItemResponse{
		ID:            e.ID,
		Model:         e.Model,
		Status:        e.Status,
		LatencyMs:     e.LatencyMs,
		PingLatencyMs: e.PingLatencyMs,
		Message:       e.Message,
		CheckedAt:     e.CheckedAt.UTC().Format(time.RFC3339),
	}
}

func ParseModelMarketplaceMonitorID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_MODEL_MARKETPLACE_MONITOR_ID", "invalid model marketplace monitor id"))
		return 0, false
	}
	return id, true
}

func maskModelMarketplaceAPIKey(plain string) string {
	if len(plain) <= modelMarketplaceAPIKeyMaskPrefix {
		return modelMarketplaceAPIKeyMaskPlaceholder
	}
	return plain[:modelMarketplaceAPIKeyMaskPrefix] + modelMarketplaceAPIKeyMaskPlaceholder
}

func parseModelMarketplaceListEnabled(raw string) *bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes":
		v := true
		return &v
	case "false", "0", "no":
		v := false
		return &v
	default:
		return nil
	}
}

func parseModelMarketplaceHistoryLimit(raw string) int {
	if strings.TrimSpace(raw) == "" {
		return service.ModelMarketplaceMonitorHistoryDefaultLimit
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return service.ModelMarketplaceMonitorHistoryDefaultLimit
	}
	if v > service.ModelMarketplaceMonitorHistoryMaxLimit {
		return service.ModelMarketplaceMonitorHistoryMaxLimit
	}
	return v
}

func (h *ModelMarketplaceMonitorHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > modelMarketplaceMonitorMaxPageSize {
		pageSize = modelMarketplaceMonitorMaxPageSize
	}
	params := service.ModelMarketplaceMonitorListParams{
		Page:     page,
		PageSize: pageSize,
		Provider: strings.TrimSpace(c.Query("provider")),
		Enabled:  parseModelMarketplaceListEnabled(c.Query("enabled")),
		Search:   strings.TrimSpace(c.Query("search")),
	}
	items, total, err := h.monitorService.List(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	summaries := h.batchSummaryFor(c, items)
	out := make([]*modelMarketplaceMonitorResponse, 0, len(items))
	for _, m := range items {
		out = append(out, buildModelMarketplaceListItemResponse(m, summaries[m.ID]))
	}
	response.Paginated(c, out, total, page, pageSize)
}

func (h *ModelMarketplaceMonitorHandler) batchSummaryFor(c *gin.Context, items []*service.ModelMarketplaceMonitor) map[int64]service.ModelMarketplaceMonitorStatusSummary {
	ids := make([]int64, 0, len(items))
	primaryByID := make(map[int64]string, len(items))
	extrasByID := make(map[int64][]string, len(items))
	for _, m := range items {
		ids = append(ids, m.ID)
		primaryByID[m.ID] = m.PrimaryModel
		extrasByID[m.ID] = m.ExtraModels
	}
	return h.monitorService.BatchMonitorStatusSummary(c.Request.Context(), ids, primaryByID, extrasByID)
}

func buildModelMarketplaceListItemResponse(m *service.ModelMarketplaceMonitor, summary service.ModelMarketplaceMonitorStatusSummary) *modelMarketplaceMonitorResponse {
	resp := modelMarketplaceMonitorToResponse(m)
	resp.PrimaryStatus = summary.PrimaryStatus
	resp.PrimaryLatencyMs = summary.PrimaryLatencyMs
	resp.Availability7d = summary.Availability7d
	resp.ExtraModelsStatus = make([]modelMarketplaceExtraModelStatusResponse, 0, len(summary.ExtraModels))
	for _, e := range summary.ExtraModels {
		resp.ExtraModelsStatus = append(resp.ExtraModelsStatus, modelMarketplaceExtraModelStatusResponse{
			Model:     e.Model,
			Status:    e.Status,
			LatencyMs: e.LatencyMs,
		})
	}
	return resp
}

func (h *ModelMarketplaceMonitorHandler) Get(c *gin.Context) {
	id, ok := ParseModelMarketplaceMonitorID(c)
	if !ok {
		return
	}
	m, err := h.monitorService.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, modelMarketplaceMonitorToResponse(m))
}

func (h *ModelMarketplaceMonitorHandler) Create(c *gin.Context) {
	var req modelMarketplaceMonitorCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	subject, _ := middleware2.GetAuthSubjectFromContext(c)
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	m, err := h.monitorService.Create(c.Request.Context(), service.ModelMarketplaceMonitorCreateParams{
		Name:              req.Name,
		Provider:          req.Provider,
		Endpoint:          req.Endpoint,
		APIKey:            req.APIKey,
		PrimaryModel:      req.PrimaryModel,
		ExtraModels:       req.ExtraModels,
		ModelDisplayNames: req.ModelDisplayNames,
		ModelCallConfigs:  req.ModelCallConfigs,
		GroupName:         req.GroupName,
		EffectiveRate:     req.EffectiveRate,
		Enabled:           enabled,
		IntervalSeconds:   req.IntervalSeconds,
		CreatedBy:         subject.UserID,
		TemplateID:        req.TemplateID,
		ExtraHeaders:      req.ExtraHeaders,
		BodyOverrideMode:  req.BodyOverrideMode,
		BodyOverride:      req.BodyOverride,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, modelMarketplaceMonitorToResponse(m))
}

func (h *ModelMarketplaceMonitorHandler) Update(c *gin.Context) {
	id, ok := ParseModelMarketplaceMonitorID(c)
	if !ok {
		return
	}
	var req modelMarketplaceMonitorUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	m, err := h.monitorService.Update(c.Request.Context(), id, service.ModelMarketplaceMonitorUpdateParams{
		Name:              req.Name,
		Provider:          req.Provider,
		Endpoint:          req.Endpoint,
		APIKey:            req.APIKey,
		PrimaryModel:      req.PrimaryModel,
		ExtraModels:       req.ExtraModels,
		ModelDisplayNames: req.ModelDisplayNames,
		ModelCallConfigs:  req.ModelCallConfigs,
		GroupName:         req.GroupName,
		EffectiveRate:     req.EffectiveRate,
		Enabled:           req.Enabled,
		IntervalSeconds:   req.IntervalSeconds,
		TemplateID:        req.TemplateID,
		ClearTemplate:     req.ClearTemplate,
		ExtraHeaders:      req.ExtraHeaders,
		BodyOverrideMode:  req.BodyOverrideMode,
		BodyOverride:      req.BodyOverride,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, modelMarketplaceMonitorToResponse(m))
}

func (h *ModelMarketplaceMonitorHandler) Delete(c *gin.Context) {
	id, ok := ParseModelMarketplaceMonitorID(c)
	if !ok {
		return
	}
	if err := h.monitorService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ModelMarketplaceMonitorHandler) Run(c *gin.Context) {
	id, ok := ParseModelMarketplaceMonitorID(c)
	if !ok {
		return
	}
	results, err := h.monitorService.RunCheck(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]modelMarketplaceCheckResultResponse, 0, len(results))
	for _, r := range results {
		out = append(out, modelMarketplaceCheckResultToResponse(r))
	}
	response.Success(c, gin.H{"results": out})
}

func (h *ModelMarketplaceMonitorHandler) History(c *gin.Context) {
	id, ok := ParseModelMarketplaceMonitorID(c)
	if !ok {
		return
	}
	limit := parseModelMarketplaceHistoryLimit(c.Query("limit"))
	model := strings.TrimSpace(c.Query("model"))
	entries, err := h.monitorService.ListHistory(c.Request.Context(), id, model, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]modelMarketplaceHistoryItemResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, modelMarketplaceHistoryEntryToResponse(e))
	}
	response.Success(c, gin.H{"items": out})
}
