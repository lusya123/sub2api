package admin

import (
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type ModelMarketplaceTemplateHandler struct {
	templateService *service.ModelMarketplaceTemplateService
}

func NewModelMarketplaceTemplateHandler(templateService *service.ModelMarketplaceTemplateService) *ModelMarketplaceTemplateHandler {
	return &ModelMarketplaceTemplateHandler{templateService: templateService}
}

type modelMarketplaceTemplateCreateRequest struct {
	Name             string            `json:"name" binding:"required,max=100"`
	Provider         string            `json:"provider" binding:"required,max=50"`
	Description      string            `json:"description" binding:"max=500"`
	ExtraHeaders     map[string]string `json:"extra_headers"`
	BodyOverrideMode string            `json:"body_override_mode" binding:"omitempty,oneof=off merge replace"`
	BodyOverride     map[string]any    `json:"body_override"`
}

type modelMarketplaceTemplateUpdateRequest struct {
	Name             *string            `json:"name" binding:"omitempty,max=100"`
	Description      *string            `json:"description" binding:"omitempty,max=500"`
	ExtraHeaders     *map[string]string `json:"extra_headers"`
	BodyOverrideMode *string            `json:"body_override_mode" binding:"omitempty,oneof=off merge replace"`
	BodyOverride     *map[string]any    `json:"body_override"`
}

type modelMarketplaceTemplateResponse struct {
	ID                 int64             `json:"id"`
	Name               string            `json:"name"`
	Provider           string            `json:"provider"`
	Description        string            `json:"description"`
	ExtraHeaders       map[string]string `json:"extra_headers"`
	BodyOverrideMode   string            `json:"body_override_mode"`
	BodyOverride       map[string]any    `json:"body_override"`
	CreatedAt          string            `json:"created_at"`
	UpdatedAt          string            `json:"updated_at"`
	AssociatedMonitors int64             `json:"associated_monitors"`
}

type modelMarketplaceTemplateApplyRequest struct {
	MonitorIDs []int64 `json:"monitor_ids"`
}

func modelMarketplaceTemplateToResponse(t *service.ModelMarketplaceTemplate, associated int64) modelMarketplaceTemplateResponse {
	headers := t.ExtraHeaders
	if headers == nil {
		headers = map[string]string{}
	}
	return modelMarketplaceTemplateResponse{
		ID:                 t.ID,
		Name:               t.Name,
		Provider:           t.Provider,
		Description:        t.Description,
		ExtraHeaders:       headers,
		BodyOverrideMode:   t.BodyOverrideMode,
		BodyOverride:       t.BodyOverride,
		CreatedAt:          t.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:          t.UpdatedAt.UTC().Format(time.RFC3339),
		AssociatedMonitors: associated,
	}
}

func parseModelMarketplaceTemplateID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_MODEL_MARKETPLACE_TEMPLATE_ID", "invalid model marketplace template id"))
		return 0, false
	}
	return id, true
}

func (h *ModelMarketplaceTemplateHandler) List(c *gin.Context) {
	items, err := h.templateService.List(c.Request.Context(), service.ModelMarketplaceTemplateListParams{
		Provider: strings.TrimSpace(c.Query("provider")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]modelMarketplaceTemplateResponse, 0, len(items))
	for _, item := range items {
		count, _ := h.templateService.CountAssociatedMonitors(c.Request.Context(), item.ID)
		out = append(out, modelMarketplaceTemplateToResponse(item, count))
	}
	response.Success(c, gin.H{"items": out})
}

func (h *ModelMarketplaceTemplateHandler) Get(c *gin.Context) {
	id, ok := parseModelMarketplaceTemplateID(c)
	if !ok {
		return
	}
	item, err := h.templateService.Get(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	count, _ := h.templateService.CountAssociatedMonitors(c.Request.Context(), item.ID)
	response.Success(c, modelMarketplaceTemplateToResponse(item, count))
}

func (h *ModelMarketplaceTemplateHandler) Create(c *gin.Context) {
	var req modelMarketplaceTemplateCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	item, err := h.templateService.Create(c.Request.Context(), service.ModelMarketplaceTemplateCreateParams{
		Name:             req.Name,
		Provider:         req.Provider,
		Description:      req.Description,
		ExtraHeaders:     req.ExtraHeaders,
		BodyOverrideMode: req.BodyOverrideMode,
		BodyOverride:     req.BodyOverride,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, modelMarketplaceTemplateToResponse(item, 0))
}

func (h *ModelMarketplaceTemplateHandler) Update(c *gin.Context) {
	id, ok := parseModelMarketplaceTemplateID(c)
	if !ok {
		return
	}
	var req modelMarketplaceTemplateUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	item, err := h.templateService.Update(c.Request.Context(), id, service.ModelMarketplaceTemplateUpdateParams{
		Name:             req.Name,
		Description:      req.Description,
		ExtraHeaders:     req.ExtraHeaders,
		BodyOverrideMode: req.BodyOverrideMode,
		BodyOverride:     req.BodyOverride,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	count, _ := h.templateService.CountAssociatedMonitors(c.Request.Context(), item.ID)
	response.Success(c, modelMarketplaceTemplateToResponse(item, count))
}

func (h *ModelMarketplaceTemplateHandler) Delete(c *gin.Context) {
	id, ok := parseModelMarketplaceTemplateID(c)
	if !ok {
		return
	}
	if err := h.templateService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *ModelMarketplaceTemplateHandler) AssociatedMonitors(c *gin.Context) {
	id, ok := parseModelMarketplaceTemplateID(c)
	if !ok {
		return
	}
	items, err := h.templateService.ListAssociatedMonitors(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *ModelMarketplaceTemplateHandler) Apply(c *gin.Context) {
	id, ok := parseModelMarketplaceTemplateID(c)
	if !ok {
		return
	}
	var req modelMarketplaceTemplateApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	affected, err := h.templateService.ApplyToMonitors(c.Request.Context(), id, req.MonitorIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"affected": affected})
}
