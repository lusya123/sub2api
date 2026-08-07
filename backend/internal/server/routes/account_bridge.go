package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterShopAccountBridgeRoutes exposes only the three identity operations
// required by Shop. The group does not inherit or accept administrator auth.
func RegisterShopAccountBridgeRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	bridge := v1.Group("/integrations/shop/account-bridge")
	bridge.Use(servermiddleware.RequireShopAccountBridgeAuthenticated())
	bridge.POST("/lookup", h.Auth.LookupShopAccountBridgeUser)
	bridge.POST("/users", h.Auth.CreateShopAccountBridgeUser)
	bridge.POST("/verify-password", h.Auth.VerifyPasswordForShopAccountBridge)
}
