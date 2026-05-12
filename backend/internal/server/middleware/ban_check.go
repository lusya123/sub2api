package middleware

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/security"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func NewBanCheck(rdb *redis.Client) gin.HandlerFunc {
	db := security.NewDynamicBan(rdb)
	return func(c *gin.Context) {
		if GetTrustTier(c) == TierAPIKey {
			c.Next()
			return
		}
		if db.IsBanned(c.Request.Context(), ClientFingerprint(c)) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "blocked"})
			return
		}
		c.Next()
	}
}

func TriggerDynamicBan(rdb *redis.Client, fingerprint, reason string) {
	security.NewDynamicBan(rdb).Trigger(nil, fingerprint, reason)
}
