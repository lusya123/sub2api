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
		if isPublicCacheableAssetRequest(c) {
			c.Next()
			return
		}
		if isFrontendDocumentRequest(c) {
			if !g.allowFrontendDocumentGlobal(c) {
				abortDefenseRateLimit(c, http.StatusTooManyRequests, "rate limited (frontend global)")
				return
			}
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
		return true
	}
	if cnt == 1 {
		_ = g.rdb.Expire(ctx, key, window).Err()
	}
	return cnt <= int64(limit)
}

func (g *GlobalRateLimiter) allowFrontendDocumentGlobal(c *gin.Context) bool {
	limit := defenseEnvInt("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", 2000)
	if limit <= 0 {
		return true
	}
	bucket := strconv.FormatInt(time.Now().UTC().Unix()/2, 10)
	ctx := c.Request.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	key := "rl:frontend:global:" + bucket
	cnt, err := g.rdb.Incr(ctx, key).Result()
	if err != nil {
		return true
	}
	if cnt == 1 {
		_ = g.rdb.Expire(ctx, key, 2*time.Second).Err()
	}
	return cnt <= int64(limit)
}

func defenseEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func isFrontendDocumentRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		return false
	}

	path := strings.TrimSpace(c.Request.URL.Path)
	if globalRateLimitBypassFrontend(path) || globalRateLimitStaticAsset(path) {
		return false
	}
	return true
}

func globalRateLimitBypassFrontend(path string) bool {
	return path == "/v1" ||
		strings.HasPrefix(path, "/v1/") ||
		path == "/v1beta" ||
		strings.HasPrefix(path, "/v1beta/") ||
		path == "/responses" ||
		strings.HasPrefix(path, "/responses/") ||
		strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/backend-api/") ||
		strings.HasPrefix(path, "/openai/") ||
		strings.HasPrefix(path, "/antigravity/") ||
		strings.HasPrefix(path, "/setup/") ||
		path == "/health"
}

func globalRateLimitStaticAsset(path string) bool {
	if strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/static/") ||
		strings.HasPrefix(path, "/images/") {
		return true
	}
	switch path {
	case "/favicon.ico", "/logo.png", "/robots.txt", "/sitemap.xml", "/manifest.json":
		return true
	}
	staticExts := []string{
		".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp",
		".woff", ".woff2", ".ttf", ".eot", ".ico", ".map",
	}
	for _, ext := range staticExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func isPublicCacheableAssetRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		return false
	}
	return c.Request.URL.Path == "/api/v1/settings/logo"
}

func ClientFingerprint(c *gin.Context) string {
	ip := strings.TrimSpace(c.GetHeader("CF-Connecting-IP"))
	if ip == "" {
		ip = strings.TrimSpace(c.GetHeader("X-Real-IP"))
	}
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
