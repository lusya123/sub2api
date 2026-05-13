package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
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

const (
	modelMarketplaceExchangeRateFallback = 7.2
	modelMarketplaceExchangeRateTTL      = time.Hour
	modelMarketplaceExchangeRateTimeout  = 3 * time.Second
	modelMarketplaceExchangeRateSource   = "frankfurter"
	modelMarketplaceExchangeRateURL      = "https://api.frankfurter.app/latest?from=USD&to=CNY"
)

type modelMarketplaceExchangeRateCache struct {
	mu        sync.Mutex
	rate      float64
	updatedAt time.Time
	fetchedAt time.Time
	source    string
}

//nolint:gochecknoglobals // Process-local cache avoids calling the public FX API for every user page view.
var modelMarketplaceFXCache modelMarketplaceExchangeRateCache

type modelMarketplaceExchangeRateResponse struct {
	Base      string  `json:"base"`
	Quote     string  `json:"quote"`
	Rate      float64 `json:"rate"`
	Source    string  `json:"source"`
	UpdatedAt string  `json:"updated_at"`
	Fallback  bool    `json:"fallback"`
}

type modelMarketplaceUserListItem struct {
	ID                   int64                                  `json:"id"`
	Name                 string                                 `json:"name"`
	Provider             string                                 `json:"provider"`
	GroupName            string                                 `json:"group_name"`
	EffectiveRate        float64                                `json:"effective_rate"`
	PrimaryModel         string                                 `json:"primary_model"`
	PrimaryDisplayNameZh string                                 `json:"primary_display_name_zh"`
	PrimaryDisplayNameEn string                                 `json:"primary_display_name_en"`
	PrimaryCallModel     string                                 `json:"primary_call_model"`
	PrimaryRequestURL    string                                 `json:"primary_request_url"`
	PrimaryStatus        string                                 `json:"primary_status"`
	PrimaryLatencyMs     *int                                   `json:"primary_latency_ms"`
	PrimaryPingLatencyMs *int                                   `json:"primary_ping_latency_ms"`
	Availability7d       float64                                `json:"availability_7d"`
	PrimaryPricing       *userSupportedModelPricing             `json:"primary_pricing"`
	ExtraModels          []modelMarketplaceUserExtraModelStatus `json:"extra_models"`
	Timeline             []modelMarketplaceUserTimelinePoint    `json:"timeline"`
}

type modelMarketplaceUserExtraModelStatus struct {
	Model          string                              `json:"model"`
	DisplayNameZh  string                              `json:"display_name_zh"`
	DisplayNameEn  string                              `json:"display_name_en"`
	CallModel      string                              `json:"call_model"`
	RequestURL     string                              `json:"request_url"`
	Status         string                              `json:"status"`
	LatencyMs      *int                                `json:"latency_ms"`
	PingLatencyMs  *int                                `json:"ping_latency_ms"`
	Availability7d float64                             `json:"availability_7d"`
	Pricing        *userSupportedModelPricing          `json:"pricing"`
	Timeline       []modelMarketplaceUserTimelinePoint `json:"timeline"`
}

type modelMarketplaceUserTimelinePoint struct {
	Status        string `json:"status"`
	LatencyMs     *int   `json:"latency_ms"`
	PingLatencyMs *int   `json:"ping_latency_ms"`
	CheckedAt     string `json:"checked_at"`
}

type modelMarketplaceUserDetailResponse struct {
	ID            int64                           `json:"id"`
	Name          string                          `json:"name"`
	Provider      string                          `json:"provider"`
	GroupName     string                          `json:"group_name"`
	EffectiveRate float64                         `json:"effective_rate"`
	Models        []modelMarketplaceUserModelStat `json:"models"`
}

type modelMarketplaceUserModelStat struct {
	Model           string                     `json:"model"`
	DisplayNameZh   string                     `json:"display_name_zh"`
	DisplayNameEn   string                     `json:"display_name_en"`
	CallModel       string                     `json:"call_model"`
	RequestURL      string                     `json:"request_url"`
	LatestStatus    string                     `json:"latest_status"`
	LatestLatencyMs *int                       `json:"latest_latency_ms"`
	Availability7d  float64                    `json:"availability_7d"`
	Availability15d float64                    `json:"availability_15d"`
	Availability30d float64                    `json:"availability_30d"`
	AvgLatency7dMs  *int                       `json:"avg_latency_7d_ms"`
	Pricing         *userSupportedModelPricing `json:"pricing"`
}

func modelMarketplaceUserViewToItem(v *service.ModelMarketplaceUserMonitorView) modelMarketplaceUserListItem {
	extras := make([]modelMarketplaceUserExtraModelStatus, 0, len(v.ExtraModels))
	for _, e := range v.ExtraModels {
		extras = append(extras, modelMarketplaceUserExtraModelStatus{
			Model:          e.Model,
			DisplayNameZh:  e.DisplayNameZh,
			DisplayNameEn:  e.DisplayNameEn,
			CallModel:      e.CallModel,
			RequestURL:     e.RequestURL,
			Status:         e.Status,
			LatencyMs:      e.LatencyMs,
			PingLatencyMs:  e.PingLatencyMs,
			Availability7d: e.Availability7d,
			Pricing:        toUserPricing(e.Pricing),
			Timeline:       modelMarketplaceTimelinePointsToResponse(e.Timeline),
		})
	}
	return modelMarketplaceUserListItem{
		ID:                   v.ID,
		Name:                 v.Name,
		Provider:             v.Provider,
		GroupName:            v.GroupName,
		EffectiveRate:        v.EffectiveRate,
		PrimaryModel:         v.PrimaryModel,
		PrimaryDisplayNameZh: v.PrimaryDisplayNameZh,
		PrimaryDisplayNameEn: v.PrimaryDisplayNameEn,
		PrimaryCallModel:     v.PrimaryCallModel,
		PrimaryRequestURL:    v.PrimaryRequestURL,
		PrimaryStatus:        v.PrimaryStatus,
		PrimaryLatencyMs:     v.PrimaryLatencyMs,
		PrimaryPingLatencyMs: v.PrimaryPingLatencyMs,
		Availability7d:       v.Availability7d,
		PrimaryPricing:       toUserPricing(v.PrimaryPricing),
		ExtraModels:          extras,
		Timeline:             modelMarketplaceTimelinePointsToResponse(v.Timeline),
	}
}

func modelMarketplaceTimelinePointsToResponse(points []service.ModelMarketplaceUserTimelinePoint) []modelMarketplaceUserTimelinePoint {
	timeline := make([]modelMarketplaceUserTimelinePoint, 0, len(points))
	for _, p := range points {
		timeline = append(timeline, modelMarketplaceUserTimelinePoint{
			Status:        p.Status,
			LatencyMs:     p.LatencyMs,
			PingLatencyMs: p.PingLatencyMs,
			CheckedAt:     p.CheckedAt.UTC().Format(time.RFC3339),
		})
	}
	return timeline
}

func modelMarketplaceUserDetailToResponse(d *service.ModelMarketplaceUserMonitorDetail) *modelMarketplaceUserDetailResponse {
	models := make([]modelMarketplaceUserModelStat, 0, len(d.Models))
	for _, m := range d.Models {
		models = append(models, modelMarketplaceUserModelStat{
			Model:           m.Model,
			DisplayNameZh:   m.DisplayNameZh,
			DisplayNameEn:   m.DisplayNameEn,
			CallModel:       m.CallModel,
			RequestURL:      m.RequestURL,
			LatestStatus:    m.LatestStatus,
			LatestLatencyMs: m.LatestLatencyMs,
			Availability7d:  m.Availability7d,
			Availability15d: m.Availability15d,
			Availability30d: m.Availability30d,
			AvgLatency7dMs:  m.AvgLatency7dMs,
			Pricing:         toUserPricing(m.Pricing),
		})
	}
	return &modelMarketplaceUserDetailResponse{
		ID:            d.ID,
		Name:          d.Name,
		Provider:      d.Provider,
		GroupName:     d.GroupName,
		EffectiveRate: d.EffectiveRate,
		Models:        models,
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

// ExchangeRate GET /api/v1/model-marketplace/exchange-rate
func (h *ModelMarketplaceUserHandler) ExchangeRate(c *gin.Context) {
	rate, source, updatedAt, fallback := currentModelMarketplaceExchangeRate(c.Request.Context())
	response.Success(c, modelMarketplaceExchangeRateResponse{
		Base:      "USD",
		Quote:     "CNY",
		Rate:      rate,
		Source:    source,
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
		Fallback:  fallback,
	})
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

func currentModelMarketplaceExchangeRate(ctx context.Context) (float64, string, time.Time, bool) {
	modelMarketplaceFXCache.mu.Lock()
	if modelMarketplaceFXCache.rate > 0 && time.Since(modelMarketplaceFXCache.fetchedAt) < modelMarketplaceExchangeRateTTL {
		rate := modelMarketplaceFXCache.rate
		source := modelMarketplaceFXCache.source
		updatedAt := modelMarketplaceFXCache.updatedAt
		modelMarketplaceFXCache.mu.Unlock()
		return rate, source, updatedAt, false
	}
	modelMarketplaceFXCache.mu.Unlock()

	rate, updatedAt, err := fetchModelMarketplaceExchangeRate(ctx)
	if err == nil && rate > 0 {
		modelMarketplaceFXCache.mu.Lock()
		modelMarketplaceFXCache.rate = rate
		modelMarketplaceFXCache.updatedAt = updatedAt
		modelMarketplaceFXCache.fetchedAt = time.Now()
		modelMarketplaceFXCache.source = modelMarketplaceExchangeRateSource
		modelMarketplaceFXCache.mu.Unlock()
		return rate, modelMarketplaceExchangeRateSource, updatedAt, false
	}

	return fallbackModelMarketplaceExchangeRate(), "fallback", time.Now(), true
}

func fetchModelMarketplaceExchangeRate(ctx context.Context) (float64, time.Time, error) {
	reqCtx, cancel := context.WithTimeout(ctx, modelMarketplaceExchangeRateTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, modelMarketplaceExchangeRateURL, nil)
	if err != nil {
		return 0, time.Time{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, time.Time{}, strconv.ErrSyntax
	}
	var payload struct {
		Date  string             `json:"date"`
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, time.Time{}, err
	}
	rate := payload.Rates["CNY"]
	if rate <= 0 {
		return 0, time.Time{}, strconv.ErrSyntax
	}
	updatedAt := time.Now()
	if payload.Date != "" {
		if parsed, err := time.Parse("2006-01-02", payload.Date); err == nil {
			updatedAt = parsed
		}
	}
	return rate, updatedAt, nil
}

func fallbackModelMarketplaceExchangeRate() float64 {
	raw := os.Getenv("MODEL_MARKETPLACE_USD_CNY_RATE")
	if raw == "" {
		raw = os.Getenv("VITE_MODEL_MARKETPLACE_USD_CNY_RATE")
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil && v > 0 {
		return v
	}
	return modelMarketplaceExchangeRateFallback
}
