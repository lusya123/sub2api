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
		if GetTrustTier(c) != TierAnonymous || c.Request.Method != http.MethodPost {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/v1beta/") {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		if len(body) == 0 || len(body) > 4096 {
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
			TriggerDynamicBan(b.rdb, ClientFingerprint(c), "body_replay")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "abnormal pattern"})
			return
		}

		c.Next()
	}
}
