package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
//   - Anonymous SPA routes are rate-limited by signed visitor cookie when
//     available, otherwise by path-level buckets. No IP address is banned.
//
// Sensitive authenticated SPA routes, such as /chat, require a signed user
// JWT before the frontend document is served.
//
// Emergency rollback: set SPA_PROTECT_ENABLED=false.
func SPAProtect(rdb *redis.Client, jwtSecret string) gin.HandlerFunc {
	maxPerMinute := spaProtectEnvInt("SPA_PROTECT_MAX_PER_MINUTE", 100)
	banSeconds := spaProtectEnvInt("SPA_PROTECT_BAN_SECONDS", 300)
	banTTL := time.Duration(banSeconds) * time.Second

	const redisTimeout = time.Second

	return func(c *gin.Context) {
		if os.Getenv("SPA_PROTECT_ENABLED") == "false" {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if spaProtectBypassPath(path) || spaProtectStaticAsset(path) {
			c.Next()
			return
		}

		if spaProtectRequiresVerifiedUser(path) {
			if hasSPAVerifiedUserCredential(c, jwtSecret) {
				c.Next()
				return
			}
			if blocked := spaProtectRateLimit(c, rdb, path, jwtSecret, maxPerMinute, banSeconds, banTTL, redisTimeout); blocked {
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "authentication required",
				"message": "please log in before opening this page",
			})
			return
		}

		if hasSPAUserCredential(c) || hasSPAAPIKey(c) {
			c.Next()
			return
		}

		if blocked := spaProtectRateLimit(c, rdb, path, jwtSecret, maxPerMinute, banSeconds, banTTL, redisTimeout); blocked {
			return
		}

		c.Next()
	}
}

func spaProtectRateLimit(
	c *gin.Context,
	rdb *redis.Client,
	path string,
	jwtSecret string,
	maxPerMinute int,
	banSeconds int,
	banTTL time.Duration,
	redisTimeout time.Duration,
) bool {
	if rdb == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), redisTimeout)
	defer cancel()

	rateKey := spaProtectRateKey(c, rdb, path, jwtSecret)
	result, err := spaProtectRateScript.Run(ctx, rdb, []string{rateKey}, maxPerMinute).Int64()
	if err != nil {
		return false
	}

	_, _ = banSeconds, banTTL
	if result == spaProtectRateLimited {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error":   "too many requests",
			"message": "rate limit exceeded",
		})
		return true
	}
	return false
}

const spaProtectRateLimited int64 = -1

var spaProtectRateScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
	redis.call("EXPIRE", KEYS[1], 60)
end

if current > tonumber(ARGV[1]) then
	return -1
end

return current
`)

func spaProtectRateKey(c *gin.Context, rdb *redis.Client, path, jwtSecret string) string {
	if cookie, err := c.Cookie("_xdt_v"); err == nil && strings.TrimSpace(cookie) != "" {
		mgr := servermiddleware.NewVisitorCookieManagerWithSecret(rdb, jwtSecret)
		fingerprint, _ := c.Cookie("_xdt_fp")
		if mgr.VerifyCookieWithFingerprint(cookie, fingerprint) {
			return "spa:rate:cookie:" + servermiddleware.CookieHash(cookie)
		}
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(path)))
	return "spa:rate:path:" + hex.EncodeToString(sum[:8])
}

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

func hasSPAVerifiedUserCredential(c *gin.Context, jwtSecret string) bool {
	cookie, cookieErr := c.Cookie("token")
	if cookieErr == nil && validateSPAJWT(strings.TrimSpace(cookie), jwtSecret) {
		return true
	}

	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" || strings.HasPrefix(token, "sk-") {
		return false
	}
	return validateSPAJWT(token, jwtSecret)
}

func validateSPAJWT(tokenString string, jwtSecret string) bool {
	if tokenString == "" || jwtSecret == "" || len(tokenString) > 8192 {
		return false
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{
		jwt.SigningMethodHS256.Name,
		jwt.SigningMethodHS384.Name,
		jwt.SigningMethodHS512.Name,
	}))
	token, err := parser.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(jwtSecret), nil
	})
	return err == nil && token.Valid
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
		strings.HasPrefix(path, "/images/") ||
		strings.HasPrefix(path, "/downloads/") {
		return true
	}
	switch path {
	case "/favicon.ico", "/logo.png", "/robots.txt", "/sitemap.xml", "/manifest.json",
		"/install-claude.sh", "/install-claude-ccswitch.sh",
		"/install-claude-win.ps1", "/install-claude-ccswitch-win.ps1",
		"/install-codex.sh", "/install-codex-win.ps1", "/install-codex-win-bootstrap.ps1",
		"/install-openclaw.sh", "/install-openclaw-win.ps1", "/install-openclaw.js":
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

func spaProtectRequiresVerifiedUser(path string) bool {
	trimmed := strings.TrimSpace(path)
	return trimmed == "/chat" ||
		strings.HasPrefix(trimmed, "/chat/") ||
		trimmed == "/use-token" ||
		strings.HasPrefix(trimmed, "/use-token/")
}
