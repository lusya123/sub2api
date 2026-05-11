package handler

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelMarketplaceUserHandler exposes model marketplace status to regular users.
type ModelMarketplaceUserHandler struct {
	monitorService *service.ModelMarketplaceMonitorService
}

func NewModelMarketplaceUserHandler(monitorService *service.ModelMarketplaceMonitorService) *ModelMarketplaceUserHandler {
	return &ModelMarketplaceUserHandler{monitorService: monitorService}
}

type modelMarketplaceUserListItem struct {
	ID                   int64                                  `json:"id"`
	Name                 string                                 `json:"name"`
	Provider             string                                 `json:"provider"`
	GroupName            string                                 `json:"group_name"`
	PrimaryModel         string                                 `json:"primary_model"`
	PrimaryStatus        string                                 `json:"primary_status"`
	PrimaryLatencyMs     *int                                   `json:"primary_latency_ms"`
	PrimaryPingLatencyMs *int                                   `json:"primary_ping_latency_ms"`
	Availability7d       float64                                `json:"availability_7d"`
	ExtraModels          []modelMarketplaceUserExtraModelStatus `json:"extra_models"`
	Timeline             []modelMarketplaceUserTimelinePoint    `json:"timeline"`
}

type modelMarketplaceUserExtraModelStatus struct {
	Model     string `json:"model"`
	Status    string `json:"status"`
	LatencyMs *int   `json:"latency_ms"`
}

type modelMarketplaceUserTimelinePoint struct {
	Status        string `json:"status"`
	LatencyMs     *int   `json:"latency_ms"`
	PingLatencyMs *int   `json:"ping_latency_ms"`
	CheckedAt     string `json:"checked_at"`
}

type modelMarketplaceUserDetailResponse struct {
	ID        int64                           `json:"id"`
	Name      string                          `json:"name"`
	Provider  string                          `json:"provider"`
	GroupName string                          `json:"group_name"`
	Models    []modelMarketplaceUserModelStat `json:"models"`
}

type modelMarketplaceUserModelStat struct {
	Model           string  `json:"model"`
	LatestStatus    string  `json:"latest_status"`
	LatestLatencyMs *int    `json:"latest_latency_ms"`
	Availability7d  float64 `json:"availability_7d"`
	Availability15d float64 `json:"availability_15d"`
	Availability30d float64 `json:"availability_30d"`
	AvgLatency7dMs  *int    `json:"avg_latency_7d_ms"`
}

func modelMarketplaceUserViewToItem(v *service.ModelMarketplaceUserMonitorView) modelMarketplaceUserListItem {
	extras := make([]modelMarketplaceUserExtraModelStatus, 0, len(v.ExtraModels))
	for _, e := range v.ExtraModels {
		extras = append(extras, modelMarketplaceUserExtraModelStatus{
			Model:     e.Model,
			Status:    e.Status,
			LatencyMs: e.LatencyMs,
		})
	}
	timeline := make([]modelMarketplaceUserTimelinePoint, 0, len(v.Timeline))
	for _, p := range v.Timeline {
		timeline = append(timeline, modelMarketplaceUserTimelinePoint{
			Status:        p.Status,
			LatencyMs:     p.LatencyMs,
			PingLatencyMs: p.PingLatencyMs,
			CheckedAt:     p.CheckedAt.UTC().Format(time.RFC3339),
		})
	}
	return modelMarketplaceUserListItem{
		ID:                   v.ID,
		Name:                 v.Name,
		Provider:             v.Provider,
		GroupName:            v.GroupName,
		PrimaryModel:         v.PrimaryModel,
		PrimaryStatus:        v.PrimaryStatus,
		PrimaryLatencyMs:     v.PrimaryLatencyMs,
		PrimaryPingLatencyMs: v.PrimaryPingLatencyMs,
		Availability7d:       v.Availability7d,
		ExtraModels:          extras,
		Timeline:             timeline,
	}
}

func modelMarketplaceUserDetailToResponse(d *service.ModelMarketplaceUserMonitorDetail) *modelMarketplaceUserDetailResponse {
	models := make([]modelMarketplaceUserModelStat, 0, len(d.Models))
	for _, m := range d.Models {
		models = append(models, modelMarketplaceUserModelStat{
			Model:           m.Model,
			LatestStatus:    m.LatestStatus,
			LatestLatencyMs: m.LatestLatencyMs,
			Availability7d:  m.Availability7d,
			Availability15d: m.Availability15d,
			Availability30d: m.Availability30d,
			AvgLatency7dMs:  m.AvgLatency7dMs,
		})
	}
	return &modelMarketplaceUserDetailResponse{
		ID:        d.ID,
		Name:      d.Name,
		Provider:  d.Provider,
		GroupName: d.GroupName,
		Models:    models,
	}
}

// List GET /api/v1/model-marketplace
func (h *ModelMarketplaceUserHandler) List(c *gin.Context) {
	views, err := h.monitorService.ListUserView(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]modelMarketplaceUserListItem, 0, len(views))
	for _, v := range views {
		items = append(items, modelMarketplaceUserViewToItem(v))
	}
	response.Success(c, gin.H{"items": items})
}

// GetStatus GET /api/v1/model-marketplace/:id/status
func (h *ModelMarketplaceUserHandler) GetStatus(c *gin.Context) {
	id, ok := admin.ParseModelMarketplaceMonitorID(c)
	if !ok {
		return
	}
	detail, err := h.monitorService.GetUserDetail(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, modelMarketplaceUserDetailToResponse(detail))
}
