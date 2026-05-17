package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func NewBanCheck(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func TriggerDynamicBan(rdb *redis.Client, fingerprint, reason string) {
}
