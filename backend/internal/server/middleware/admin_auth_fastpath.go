package middleware

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	pkgip "github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	adminAuthFastPathPrefix     = "/api/v1/admin/"
	adminAuthFastPathRoot       = "/api/v1/admin"
	adminAuthFastPathDefaultMin = 16
	adminAuthFastPathDefaultMax = 4096
	adminAuthFastPathLogSample  = 100
	adminFailBanThreshold       = 30
	adminFailBanWindowSeconds   = 60
	adminFailBanDurationSeconds = 600
	adminFailBanRedisTimeout    = 200 * time.Millisecond
)

type AdminAuthFastPathReason string

const (
	AdminAuthFastPathRejectNoHeader   AdminAuthFastPathReason = "reject_no_header"
	AdminAuthFastPathRejectBadScheme  AdminAuthFastPathReason = "reject_bad_scheme"
	AdminAuthFastPathRejectBadLength  AdminAuthFastPathReason = "reject_bad_length"
	AdminAuthFastPathRejectBadCharset AdminAuthFastPathReason = "reject_bad_charset"
	AdminAuthFastPathRejectBanned     AdminAuthFastPathReason = "reject_banned"
)

type AdminAuthFastPathMetrics struct {
	RejectNoHeader   uint64 `json:"reject_no_header"`
	RejectBadScheme  uint64 `json:"reject_bad_scheme"`
	RejectBadLength  uint64 `json:"reject_bad_length"`
	RejectBadCharset uint64 `json:"reject_bad_charset"`
	RejectBanned     uint64 `json:"reject_banned"`
}

var adminAuthFastPathCounters struct {
	noHeader   atomic.Uint64
	badScheme  atomic.Uint64
	badLength  atomic.Uint64
	badCharset atomic.Uint64
	banned     atomic.Uint64
}

// AdminAuthFastPath rejects malformed admin credentials before the request enters
// the heavier logging, rate-limit, and business-auth middleware chain.
func AdminAuthFastPath(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !adminAuthFastPathEnabled() || c == nil || c.Request == nil || !isAdminAuthFastPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		if isAdminAuthFastPathBanned(c, rdb) {
			rejectAdminAuthFastPath(c, AdminAuthFastPathRejectBanned, nil)
			return
		}

		if apiKey := strings.TrimSpace(c.GetHeader("x-api-key")); apiKey != "" {
			if reason := validAdminAuthFastPathToken(apiKey); reason != "" {
				rejectAdminAuthFastPath(c, reason, rdb)
				return
			}
			c.Next()
			return
		}
		if isWebSocketUpgradeRequest(c) {
			token := extractJWTFromWebSocketSubprotocol(c)
			if token != "" && validAdminAuthFastPathToken(token) == "" {
				c.Next()
				return
			}
			rejectAdminAuthFastPath(c, AdminAuthFastPathRejectNoHeader, rdb)
			return
		}

		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			rejectAdminAuthFastPath(c, AdminAuthFastPathRejectNoHeader, rdb)
			return
		}

		token, ok := splitBearerToken(authHeader)
		if !ok {
			rejectAdminAuthFastPath(c, AdminAuthFastPathRejectBadScheme, rdb)
			return
		}

		if reason := validAdminAuthFastPathToken(token); reason != "" {
			rejectAdminAuthFastPath(c, reason, rdb)
			return
		}

		c.Next()
	}
}

func AdminAuthFastPathMetricsSnapshot() AdminAuthFastPathMetrics {
	return AdminAuthFastPathMetrics{
		RejectNoHeader:   adminAuthFastPathCounters.noHeader.Load(),
		RejectBadScheme:  adminAuthFastPathCounters.badScheme.Load(),
		RejectBadLength:  adminAuthFastPathCounters.badLength.Load(),
		RejectBadCharset: adminAuthFastPathCounters.badCharset.Load(),
		RejectBanned:     adminAuthFastPathCounters.banned.Load(),
	}
}

// AdminAuthFailureRecorder records admin authentication failures that survive
// the fast path and are rejected by the real admin auth middleware.
func AdminAuthFailureRecorder(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() == http.StatusUnauthorized {
			recordAdminAuthFailure(c, rdb)
		}
	}
}

func isAdminAuthFastPath(path string) bool {
	return path == adminAuthFastPathRoot || strings.HasPrefix(path, adminAuthFastPathPrefix)
}

func splitBearerToken(authHeader string) (string, bool) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	return token, token != ""
}

func validAdminAuthFastPathToken(token string) AdminAuthFastPathReason {
	minLen := adminAuthFastPathEnvInt("ADMIN_FASTPATH_MIN_TOKEN_LEN", adminAuthFastPathDefaultMin)
	maxLen := adminAuthFastPathEnvInt("ADMIN_FASTPATH_MAX_TOKEN_LEN", adminAuthFastPathDefaultMax)
	if minLen < 0 {
		minLen = 0
	}
	if maxLen <= 0 {
		maxLen = adminAuthFastPathDefaultMax
	}
	if maxLen < minLen {
		maxLen = minLen
	}
	if len(token) < minLen || len(token) > maxLen {
		return AdminAuthFastPathRejectBadLength
	}
	if !validAdminAuthFastPathTokenCharset(token) {
		return AdminAuthFastPathRejectBadCharset
	}
	return ""
}

func validAdminAuthFastPathTokenCharset(token string) bool {
	for i := 0; i < len(token); i++ {
		ch := token[i]
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '.' || ch == '_' || ch == '-' || ch == '~':
		case ch == '+' || ch == '/' || ch == '=':
		default:
			return false
		}
	}
	return true
}

func rejectAdminAuthFastPath(c *gin.Context, reason AdminAuthFastPathReason, rdb *redis.Client) {
	count := incrementAdminAuthFastPathCounter(reason)
	if reason != AdminAuthFastPathRejectBanned {
		recordAdminAuthFailure(c, rdb)
	}
	sample := adminAuthFastPathEnvInt("ADMIN_FASTPATH_LOG_SAMPLE", adminAuthFastPathLogSample)
	if sample > 0 && count%uint64(sample) == 0 {
		logger.With(
			zap.String("component", "http.admin_fastpath"),
			zap.String("reason", string(reason)),
			zap.String("path", c.Request.URL.Path),
			zap.String("method", c.Request.Method),
			zap.Uint64("count", count),
		).Warn("admin auth fastpath rejected request")
	}

	c.Header("Content-Length", "0")
	c.Header("Connection", "close")
	status := http.StatusUnauthorized
	if reason == AdminAuthFastPathRejectBanned {
		status = http.StatusForbidden
	}
	c.Status(status)
	c.Writer.WriteHeaderNow()
	c.Abort()
}

func incrementAdminAuthFastPathCounter(reason AdminAuthFastPathReason) uint64 {
	switch reason {
	case AdminAuthFastPathRejectNoHeader:
		return adminAuthFastPathCounters.noHeader.Add(1)
	case AdminAuthFastPathRejectBadScheme:
		return adminAuthFastPathCounters.badScheme.Add(1)
	case AdminAuthFastPathRejectBadLength:
		return adminAuthFastPathCounters.badLength.Add(1)
	case AdminAuthFastPathRejectBadCharset:
		return adminAuthFastPathCounters.badCharset.Add(1)
	case AdminAuthFastPathRejectBanned:
		return adminAuthFastPathCounters.banned.Add(1)
	default:
		return 0
	}
}

func isAdminAuthFastPathBanned(c *gin.Context, rdb *redis.Client) bool {
	if rdb == nil || c == nil || c.Request == nil || adminFailBanWhitelisted(c) {
		return false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), adminFailBanRedisTimeout)
	defer cancel()

	banned, err := rdb.Get(ctx, adminFailBanKey(adminFailBanClientIP(c))).Result()
	return err == nil && banned == "1"
}

func recordAdminAuthFailure(c *gin.Context, rdb *redis.Client) {
	if rdb == nil || c == nil || c.Request == nil || adminFailBanWhitelisted(c) {
		return
	}

	threshold := adminAuthFastPathEnvInt("ADMIN_FAIL_BAN_THRESHOLD", adminFailBanThreshold)
	windowSeconds := adminAuthFastPathEnvInt("ADMIN_FAIL_BAN_WINDOW", adminFailBanWindowSeconds)
	banSeconds := adminAuthFastPathEnvInt("ADMIN_FAIL_BAN_DURATION", adminFailBanDurationSeconds)
	if threshold <= 0 || windowSeconds <= 0 || banSeconds <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), adminFailBanRedisTimeout)
	defer cancel()

	clientIP := adminFailBanClientIP(c)
	count, err := rdb.Incr(ctx, adminFailCounterKey(clientIP)).Result()
	if err != nil {
		return
	}
	if count == 1 {
		_ = rdb.Expire(ctx, adminFailCounterKey(clientIP), time.Duration(windowSeconds)*time.Second).Err()
	}
	if count > int64(threshold) {
		_ = rdb.Set(ctx, adminFailBanKey(clientIP), "1", time.Duration(banSeconds)*time.Second).Err()
	}
}

func adminFailBanWhitelisted(c *gin.Context) bool {
	clientIP := adminFailBanClientIP(c)
	if clientIP == "" {
		return false
	}
	return pkgip.MatchesAnyPattern(clientIP, splitAdminFastPathCSVEnv("ADMIN_FAIL_BAN_WHITELIST"))
}

func adminFailBanClientIP(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.ClientIP())
}

func adminFailCounterKey(clientIP string) string {
	return "fail:admin:" + clientIP
}

func adminFailBanKey(clientIP string) string {
	return "ban:admin:" + clientIP
}

func splitAdminFastPathCSVEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func adminAuthFastPathEnabled() bool {
	return !strings.EqualFold(strings.TrimSpace(os.Getenv("ADMIN_FASTPATH_ENABLED")), "false")
}

func adminAuthFastPathEnvInt(key string, defaultVal int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func resetAdminAuthFastPathMetricsForTest() {
	adminAuthFastPathCounters.noHeader.Store(0)
	adminAuthFastPathCounters.badScheme.Store(0)
	adminAuthFastPathCounters.badLength.Store(0)
	adminAuthFastPathCounters.badCharset.Store(0)
	adminAuthFastPathCounters.banned.Store(0)
}
