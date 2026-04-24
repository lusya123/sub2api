// Package public hosts handlers mounted without auth middleware. The status
// page (kuma-mieru style heartbeat dashboard) lives here — it is explicitly
// public so anonymous visitors can check "is sub2api up?" without logging in.
//
// Handlers here must never:
//   - Return account credentials, IPs, or emails (see StatusPageService.mask).
//   - Require an API key.
//   - Do anything mutating.
package public

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// statusCacheControl is the response header used for both list and detail
// endpoints. 30s matches StatusPageService.statusCacheTTL so anything served
// from a CDN / reverse proxy expires at the same cadence as our internal
// cache.
const statusCacheControl = "public, max-age=30"

// PublicStatusHandler serves /api/public/status/* endpoints. It holds no state
// of its own — all aggregation is done inside StatusPageService.
type PublicStatusHandler struct {
	svc *service.StatusPageService
}

// NewPublicStatusHandler is the wire constructor.
func NewPublicStatusHandler(svc *service.StatusPageService) *PublicStatusHandler {
	return &PublicStatusHandler{svc: svc}
}

// ListModels handles GET /api/public/status/models.
//
// Response shape is a JSON object `{"models": [...]}` — mirrors the object
// response style used by GetModelDetail and keeps the frontend client free
// to add sibling fields (pagination, generated_at, etc.) later without
// breaking callers that already destructure `data.models`.
func (h *PublicStatusHandler) ListModels(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "status service not available"})
		return
	}
	models, err := h.svc.ListModels(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if models == nil {
		models = []service.StatusModel{}
	}
	// 30s cache — matches the in-process StatusPageService TTL. Vary on
	// Accept-Language so future i18n of the `note` field doesn't serve
	// stale translations across users.
	c.Header("Cache-Control", statusCacheControl)
	c.Header("Vary", "Accept-Language")
	c.JSON(http.StatusOK, gin.H{"models": models})
}

// GetModelDetail handles GET /api/public/status/model/:name.
//
// The name path parameter is taken verbatim as the model id. Unknown models
// still return 200 with an empty-heartbeat shell so the frontend can render
// the pricing/metadata card even when a combo has never been probed.
func (h *PublicStatusHandler) GetModelDetail(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "status service not available"})
		return
	}
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model name required"})
		return
	}
	detail, err := h.svc.GetModelDetail(c.Request.Context(), name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", statusCacheControl)
	c.Header("Vary", "Accept-Language")
	c.JSON(http.StatusOK, detail)
}
