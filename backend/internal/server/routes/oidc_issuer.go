package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterOIDCIssuerRoutes registers Sub2API-as-provider OIDC endpoints.
func RegisterOIDCIssuerRoutes(r *gin.Engine, h *handler.Handlers) {
	r.GET("/.well-known/openid-configuration", h.OIDCIssuer.Discovery)
	r.GET("/.well-known/jwks.json", h.OIDCIssuer.JWKS)
	r.GET("/oauth2/authorize", h.OIDCIssuer.Authorize)
	r.POST("/oauth2/token", h.OIDCIssuer.Token)
	r.GET("/oauth2/userinfo", h.OIDCIssuer.UserInfo)
}
