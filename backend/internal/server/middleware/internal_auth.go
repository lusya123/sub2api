package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
)

func InternalAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		expected := ""
		if cfg != nil {
			expected = strings.TrimSpace(cfg.Lobe.InternalSharedSecret)
		}
		if expected == "" {
			AbortWithError(c, 503, "INTERNAL_AUTH_NOT_CONFIGURED", "internal auth is not configured")
			return
		}

		authHeader := c.GetHeader("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			AbortWithError(c, 401, "INVALID_AUTH_HEADER", "Authorization header format must be 'Bearer {token}'")
			return
		}
		token := strings.TrimSpace(parts[1])
		if subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			AbortWithError(c, 401, "UNAUTHORIZED", "invalid internal token")
			return
		}
		c.Next()
	}
}
