package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterInternalRoutes(r *gin.Engine, h *handler.Handlers, cfg *config.Config) {
	internal := r.Group("/internal/v1")
	internal.Use(middleware.InternalAuth(cfg))
	{
		internal.GET("/users/:user_id/lobe-config", h.LobeConfig.GetUserConfig)
	}
}
