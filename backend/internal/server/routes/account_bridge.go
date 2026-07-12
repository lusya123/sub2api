package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerAccountBridgeRoutes(admin *gin.RouterGroup, h *handler.Handlers) {
	bridge := admin.Group("/account-bridge")
	bridge.POST("/verify-password", h.Auth.VerifyPasswordForAccountBridge)
}
