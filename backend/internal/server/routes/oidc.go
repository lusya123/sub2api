package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterOIDCRoutes(r *gin.Engine, h *handler.Handlers) {
	r.GET("/.well-known/openid-configuration", h.OIDC.Discovery)
	r.GET("/.well-known/jwks.json", h.OIDC.JWKS)
	r.GET("/oauth2/authorize", h.OIDC.Authorize)
	r.POST("/oauth2/token", h.OIDC.Token)
	r.GET("/oauth2/userinfo", h.OIDC.UserInfo)
}
