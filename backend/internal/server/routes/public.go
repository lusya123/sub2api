package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/public"

	"github.com/gin-gonic/gin"
)

// RegisterPublicGlobeRoutes wires the /api/public/globe/* endpoints that
// back the live world-map dashboard. Anonymous-safe: payloads expose
// country / city / lat-lng aggregates and never raw user identifiers.
func RegisterPublicGlobeRoutes(r *gin.Engine, h *public.GlobeHandler) {
	if h == nil {
		return
	}
	group := r.Group("/api/public/globe")
	{
		group.GET("/snapshot", h.Snapshot)
		group.GET("/summary", h.Summary)
		group.GET("/stream", h.Stream)
	}
}
