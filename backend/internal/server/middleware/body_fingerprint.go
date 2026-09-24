package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const bodyFingerprintMaxBodyBytes = 4096

type replayableRequestBody struct {
	io.Reader
	io.Closer
}

type BodyFingerprint struct {
	rdb *redis.Client
}

func NewBodyFingerprint(rdb *redis.Client) *BodyFingerprint {
	return &BodyFingerprint{rdb: rdb}
}

func (b *BodyFingerprint) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("DEFENSE_BODY_FINGERPRINT_ENABLED") == "false" || b == nil || b.rdb == nil {
			c.Next()
			return
		}
		if c == nil || c.Request == nil || c.Request.Body == nil ||
			IsShopAccountBridgeAuthenticated(c) ||
			GetTrustTier(c) != TierAnonymous || c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/v1beta/") {
			c.Next()
			return
		}

		if c.Request.ContentLength > bodyFingerprintMaxBodyBytes {
			c.Next()
			return
		}

		originalBody := c.Request.Body
		body, err := io.ReadAll(io.LimitReader(originalBody, bodyFingerprintMaxBodyBytes+1))
		// Preserve both the bytes already consumed and the unread suffix. This is
		// important for chunked/unknown-length bodies that exceed the fingerprint
		// limit: downstream handlers still receive the exact original stream.
		c.Request.Body = &replayableRequestBody{
			Reader: io.MultiReader(bytes.NewReader(body), originalBody),
			Closer: originalBody,
		}
		if err != nil {
			c.Next()
			return
		}

		if len(body) == 0 || len(body) > bodyFingerprintMaxBodyBytes {
			c.Next()
			return
		}

		sum := sha256.Sum256(body)
		bodyHash := hex.EncodeToString(sum[:8])
		c.Set("defense_body_hash", bodyHash)
		key := "fp:body:" + bodyHash
		n, err := b.rdb.Incr(c.Request.Context(), key).Result()
		if err != nil {
			c.Next()
			return
		}
		if n == 1 {
			_ = b.rdb.Expire(c.Request.Context(), key, time.Minute).Err()
		}
		if n > 50 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "abnormal pattern"})
			return
		}

		c.Next()
	}
}
