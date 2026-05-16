package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	defaultVerifyCodeTargetWindowSeconds = 60
	defaultVerifyCodeGlobalPerSecond     = 50
	defaultVerifyCodeClientWindowSeconds = 60
	defaultVerifyCodeClientMax           = 5
)

// VerifyCodeProtect protects public verification-code style endpoints before
// they reach the heavier handler path.
func VerifyCodeProtect(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("VERIFY_CODE_PROTECT_ENABLED") == "false" || rdb == nil {
			c.Next()
			return
		}

		target, ok := verifyCodeTargetFromBody(c)
		if !ok {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)

		clientMax := envInt("VERIFY_CODE_PROTECT_CLIENT_MAX", defaultVerifyCodeClientMax)
		clientWindowSeconds := envInt("VERIFY_CODE_PROTECT_CLIENT_WINDOW_SECONDS", defaultVerifyCodeClientWindowSeconds)
		if clientMax > 0 && clientWindowSeconds > 0 {
			clientKey := "verify_code:client:" + verifyCodeClientFingerprint(c)
			allowed, err := verifyCodeAllowFixedWindow(ctx, rdb, clientKey, clientMax, time.Duration(clientWindowSeconds)*time.Second)
			if err != nil {
				cancel()
				c.Next()
				return
			}
			if !allowed {
				cancel()
				abortVerifyCodeProtect(c, http.StatusTooManyRequests, "rate limit exceeded", "too many verification requests, please try again later")
				return
			}
		}

		targetHash := hashVerifyCodeTarget(target)
		rateKey := "verify_code:target:" + targetHash
		windowSeconds := envInt("VERIFY_CODE_PROTECT_TARGET_WINDOW_SECONDS", defaultVerifyCodeTargetWindowSeconds)
		if windowSeconds > 0 {
			exists, err := rdb.Exists(ctx, rateKey).Result()
			if err != nil {
				cancel()
				c.Next()
				return
			}
			if exists > 0 {
				cancel()
				abortVerifyCodeProtect(c, http.StatusTooManyRequests, "too frequent", "please wait before requesting another code")
				return
			}
		}

		globalLimit := envInt("VERIFY_CODE_PROTECT_GLOBAL_PER_SECOND", defaultVerifyCodeGlobalPerSecond)
		if globalLimit > 0 {
			globalKey := "verify_code:global:" + time.Now().UTC().Format("20060102150405")
			allowed, err := verifyCodeAllowFixedWindow(ctx, rdb, globalKey, globalLimit, 2*time.Second)
			if err != nil {
				cancel()
				c.Next()
				return
			}
			if !allowed {
				cancel()
				abortVerifyCodeProtect(c, http.StatusServiceUnavailable, "service busy", "too many verification requests, please retry later")
				return
			}
		}
		cancel()

		c.Next()

		if windowSeconds <= 0 || c.Writer.Status() >= http.StatusBadRequest {
			return
		}
		postCtx, postCancel := context.WithTimeout(context.Background(), time.Second)
		defer postCancel()
		_ = rdb.Set(postCtx, rateKey, "1", time.Duration(windowSeconds)*time.Second).Err()
	}
}

func verifyCodeTargetFromBody(c *gin.Context) (string, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return "", false
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	var req struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", false
	}

	target := strings.ToLower(strings.TrimSpace(req.Email))
	if target == "" {
		target = strings.TrimSpace(req.Phone)
	}
	return target, target != ""
}

func hashVerifyCodeTarget(target string) string {
	sum := sha256.Sum256([]byte(target))
	return hex.EncodeToString(sum[:])
}

func verifyCodeClientFingerprint(c *gin.Context) string {
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
	if ip == "" && c != nil {
		ip = c.ClientIP()
	}
	ua := ""
	lang := ""
	if c != nil {
		ua = c.GetHeader("User-Agent")
		lang = c.GetHeader("Accept-Language")
	}
	sum := sha256.Sum256([]byte(ip + "|" + ua + "|" + lang))
	return hex.EncodeToString(sum[:])
}

func verifyCodeAllowFixedWindow(ctx context.Context, rdb *redis.Client, key string, limit int, ttl time.Duration) (bool, error) {
	n, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return true, err
	}
	if n == 1 {
		_ = rdb.Expire(ctx, key, ttl).Err()
	}
	return n <= int64(limit), nil
}

func abortVerifyCodeProtect(c *gin.Context, status int, errMsg, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error":   errMsg,
		"message": message,
	})
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
