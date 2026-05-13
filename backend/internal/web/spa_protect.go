package web

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// SPAProtect protects anonymous SPA document routes such as /model-marketplace
// and /dashboard from high-frequency scraping/CC traffic.
//
// Design rules:
//   - Requests with a login marker cookie are allowed through so logged-in users see
//     fresh frontend builds immediately.
//   - Requests with an sk-* API key are allowed through so gateway API users are
//     never affected by frontend protection.
//   - API routes and static assets are allowed through to their own handlers.
//   - Anonymous SPA routes are rate-limited per client IP and temporarily banned
//     after exceeding the configured threshold.
//
// Emergency rollback: set SPA_PROTECT_ENABLED=false.
func SPAProtect(rdb *redis.Client) gin.HandlerFunc {
	if rdb == nil {
		return func(c *gin.Context) { c.Next() }
	}

	maxPerMinute := spaProtectEnvInt("SPA_PROTECT_MAX_PER_MINUTE", 100)
	banSeconds := spaProtectEnvInt("SPA_PROTECT_BAN_SECONDS", 300)
	banTTL := time.Duration(banSeconds) * time.Second

	const redisTimeout = time.Second

	return func(c *gin.Context) {
		if os.Getenv("SPA_PROTECT_ENABLED") == "false" {
			c.Next()
			return
		}

		if hasSPAUserCredential(c) || hasSPAAPIKey(c) {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if spaProtectBypassPath(path) || spaProtectStaticAsset(path) {
			c.Next()
			return
		}

		ip := spaProtectClientIP(c)
		if ip == "" {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), redisTimeout)
		defer cancel()

		banKey := "spa:ban:" + ip
		rateKey := "spa:rate:" + ip
		result, err := spaProtectRateScript.Run(ctx, rdb, []string{banKey, rateKey}, maxPerMinute, banSeconds).Int64()
		if err != nil {
			c.Next()
			return
		}

		switch result {
		case spaProtectAlreadyBanned:
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "too many requests",
				"message": "blocked, please try again later",
			})
			return
		case spaProtectNewlyBanned:
			log.Printf("spa:ban ip=%s path=%s limit=%d ban_seconds=%d", ip, path, maxPerMinute, int(banTTL/time.Second))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "too many requests",
				"message": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}

const (
	spaProtectAlreadyBanned int64 = -1
	spaProtectNewlyBanned   int64 = -2
)

var spaProtectRateScript = redis.NewScript(`
if redis.call("EXISTS", KEYS[1]) > 0 then
	return -1
end

local current = redis.call("INCR", KEYS[2])
if current == 1 then
	redis.call("EXPIRE", KEYS[2], 60)
end

if current > tonumber(ARGV[1]) then
	redis.call("SET", KEYS[1], "1", "EX", tonumber(ARGV[2]))
	redis.call("DEL", KEYS[2])
	return -2
end

return current
`)

func spaProtectEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

func hasSPAUserCredential(c *gin.Context) bool {
	if cookie, err := c.Cookie("token"); err == nil && strings.TrimSpace(cookie) != "" {
		return true
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	token := strings.TrimSpace(parts[1])
	return token != "" && !strings.HasPrefix(token, "sk-")
}

func hasSPAAPIKey(c *gin.Context) bool {
	if key := strings.TrimSpace(c.GetHeader("x-api-key")); strings.HasPrefix(key, "sk-") {
		return true
	}
	if key := strings.TrimSpace(c.GetHeader("x-goog-api-key")); strings.HasPrefix(key, "sk-") {
		return true
	}

	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.SplitN(auth, " ", 2)
	return len(parts) == 2 &&
		strings.EqualFold(parts[0], "Bearer") &&
		strings.HasPrefix(strings.TrimSpace(parts[1]), "sk-")
}

func spaProtectBypassPath(path string) bool {
	trimmed := strings.TrimSpace(path)
	return trimmed == "/v1" ||
		strings.HasPrefix(trimmed, "/v1/") ||
		trimmed == "/v1beta" ||
		strings.HasPrefix(trimmed, "/v1beta/") ||
		trimmed == "/responses" ||
		strings.HasPrefix(trimmed, "/responses/") ||
		strings.HasPrefix(trimmed, "/api/") ||
		strings.HasPrefix(trimmed, "/backend-api/") ||
		strings.HasPrefix(trimmed, "/openai/") ||
		strings.HasPrefix(trimmed, "/antigravity/") ||
		strings.HasPrefix(trimmed, "/setup/") ||
		trimmed == "/health"
}

func spaProtectStaticAsset(path string) bool {
	if path == "" {
		return false
	}
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

func spaProtectClientIP(c *gin.Context) string {
	if ip := strings.TrimSpace(c.GetHeader("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(c.GetHeader("X-Real-IP")); ip != "" {
		return ip
	}
	if xff := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return xff
	}
	return strings.TrimSpace(c.ClientIP())
}
