package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/public"

	"github.com/gin-gonic/gin"
)

// RegisterPublicStatusRoutes wires the /api/public/status/* endpoints that
// back the kuma-mieru-styled public status page.
//
// Mounted at the top of the router tree with no auth middleware and no rate
// limiting beyond gin's defaults. The handlers must stay read-only — see the
// note in handler/public.
func RegisterPublicStatusRoutes(r *gin.Engine, h *public.PublicStatusHandler) {
	if h == nil {
		return
	}
	group := r.Group("/api/public/status")
	{
		group.GET("/models", h.ListModels)
		group.GET("/model/:name", h.GetModelDetail)
	}
}
