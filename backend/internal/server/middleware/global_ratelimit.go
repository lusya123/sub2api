package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type GlobalRateLimiter struct {
	rdb *redis.Client
}

func NewGlobalRateLimiter(rdb *redis.Client) *GlobalRateLimiter {
	return &GlobalRateLimiter{rdb: rdb}
}

func (g *GlobalRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED") == "false" || g == nil || g.rdb == nil {
			c.Next()
			return
		}

		switch GetTrustTier(c) {
		case TierAPIKey:
			c.Next()
			return
		case TierUser:
			key := "rl:user:" + userRateLimitKey(c)
			if !g.allow(c.Request.Context(), key, 600, time.Minute) {
				abortDefenseRateLimit(c, http.StatusTooManyRequests, "rate limited (user)")
				return
			}
		default:
			fp := ClientFingerprint(c)
			if !g.allow(c.Request.Context(), "rl:anon:"+fp, 30, time.Minute) {
				abortDefenseRateLimit(c, http.StatusTooManyRequests, "rate limited (anon)")
				return
			}
			bucket := time.Now().UTC().Format("20060102150405")
			if !g.allow(c.Request.Context(), "rl:anon:global:"+bucket, 2000, 2*time.Second) {
				abortDefenseRateLimit(c, http.StatusTooManyRequests, "rate limited (global)")
				return
			}
		}

		c.Next()
	}
}

func (g *GlobalRateLimiter) allow(ctx context.Context, key string, limit int, window time.Duration) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	cnt, err := g.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false
	}
	if cnt == 1 {
		_ = g.rdb.Expire(ctx, key, window).Err()
	}
	return cnt <= int64(limit)
}

func ClientFingerprint(c *gin.Context) string {
	ip := strings.TrimSpace(c.GetHeader("X-Real-IP"))
	if ip == "" {
		ip = strings.TrimSpace(c.GetHeader("X-Forwarded-For"))
		if i := strings.Index(ip, ","); i > 0 {
			ip = strings.TrimSpace(ip[:i])
		}
	}
	if ip == "" {
		ip = c.ClientIP()
	}
	ua := c.GetHeader("User-Agent")
	lang := c.GetHeader("Accept-Language")
	h := sha256.Sum256([]byte(ip + "|" + ua + "|" + lang))
	return hex.EncodeToString(h[:8])
}

func userRateLimitKey(c *gin.Context) string {
	if subject, ok := GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		return strconv.FormatInt(subject.UserID, 10)
	}
	token := strings.TrimSpace(c.GetHeader("Authorization"))
	if token == "" {
		token = strings.TrimSpace(c.GetHeader("Cookie"))
	}
	if token == "" {
		return ClientFingerprint(c)
	}
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:8])
}

func abortDefenseRateLimit(c *gin.Context, status int, reason string) {
	c.AbortWithStatusJSON(status, gin.H{"error": reason})
}
