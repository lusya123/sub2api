package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/gin-gonic/gin"
)

const (
	shopAccountBridgeTimestampHeader = "X-Shop-Bridge-Timestamp"
	shopAccountBridgeSignatureHeader = "X-Shop-Bridge-Signature"
	shopAccountBridgeMaxBodyBytes    = 16 * 1024

	shopAccountBridgeLookupPath         = "/api/v1/integrations/shop/account-bridge/lookup"
	shopAccountBridgeUsersPath          = "/api/v1/integrations/shop/account-bridge/users"
	shopAccountBridgeVerifyPasswordPath = "/api/v1/integrations/shop/account-bridge/verify-password"

	shopAccountBridgeAuthenticatedContextKey = "shop_account_bridge_authenticated"
)

// ShopAccountBridgeIngressHMACAuth authenticates the three dedicated Shop
// identity endpoints before global request-body inspection and replay defenses.
// It ignores every other path. The credential is intentionally unrelated to
// JWTs and global administrator API keys, so possession of this secret grants
// no admin-route capability.
func ShopAccountBridgeIngressHMACAuth(cfg config.ShopAccountBridgeConfig) gin.HandlerFunc {
	return shopAccountBridgeIngressHMACAuth(cfg, time.Now)
}

func shopAccountBridgeIngressHMACAuth(cfg config.ShopAccountBridgeConfig, now func() time.Time) gin.HandlerFunc {
	authenticate := shopAccountBridgeHMACAuth(cfg, now)
	return func(c *gin.Context) {
		if !isShopAccountBridgeEndpoint(c) {
			c.Next()
			return
		}
		authenticate(c)
	}
}

// RequireShopAccountBridgeAuthenticated is a fail-closed route guard. The
// actual HMAC is intentionally evaluated only once by the ingress middleware.
func RequireShopAccountBridgeAuthenticated() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsShopAccountBridgeAuthenticated(c) {
			abortShopAccountBridgeUnauthorized(c)
			return
		}
		c.Next()
	}
}

func shopAccountBridgeHMACAuth(cfg config.ShopAccountBridgeConfig, now func() time.Time) gin.HandlerFunc {
	secret := []byte(strings.TrimSpace(cfg.SharedSecret))
	skew := time.Duration(cfg.ClockSkewSeconds) * time.Second
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if !cfg.Enabled || len(secret) < 32 || skew <= 0 || skew > 300*time.Second || now == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if c.Request == nil || c.Request.Body == nil || c.Request.Method != http.MethodPost || c.Request.URL == nil || c.Request.URL.RawQuery != "" {
			abortShopAccountBridgeUnauthorized(c)
			return
		}
		mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
		if err != nil || !strings.EqualFold(mediaType, "application/json") {
			c.AbortWithStatus(http.StatusUnsupportedMediaType)
			return
		}
		if c.Request.ContentLength > shopAccountBridgeMaxBodyBytes {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, shopAccountBridgeMaxBodyBytes+1))
		if err != nil || len(body) > shopAccountBridgeMaxBodyBytes {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		timestampValues := c.Request.Header.Values(shopAccountBridgeTimestampHeader)
		signatureValues := c.Request.Header.Values(shopAccountBridgeSignatureHeader)
		if len(timestampValues) != 1 || len(signatureValues) != 1 {
			abortShopAccountBridgeUnauthorized(c)
			return
		}
		timestamp := strings.TrimSpace(timestampValues[0])
		timestampUnix, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			abortShopAccountBridgeUnauthorized(c)
			return
		}
		nowUnix := now().UTC().Unix()
		skewSeconds := int64(skew / time.Second)
		if timestampUnix < nowUnix-skewSeconds || timestampUnix > nowUnix+skewSeconds {
			abortShopAccountBridgeUnauthorized(c)
			return
		}

		providedRaw := strings.TrimSpace(signatureValues[0])
		if !strings.HasPrefix(providedRaw, "sha256=") {
			abortShopAccountBridgeUnauthorized(c)
			return
		}
		provided, err := hex.DecodeString(strings.TrimPrefix(providedRaw, "sha256="))
		if err != nil || len(provided) != sha256.Size {
			abortShopAccountBridgeUnauthorized(c)
			return
		}
		path := c.Request.URL.EscapedPath()
		if path == "" {
			path = "/"
		}
		expected := shopAccountBridgeHMAC(secret, c.Request.Method, path, timestamp, body)
		if !hmac.Equal(provided, expected) {
			abortShopAccountBridgeUnauthorized(c)
			return
		}
		c.Set(shopAccountBridgeAuthenticatedContextKey, true)
		c.Next()
	}
}

func isShopAccountBridgeEndpoint(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	switch c.Request.URL.Path {
	case shopAccountBridgeLookupPath, shopAccountBridgeUsersPath, shopAccountBridgeVerifyPasswordPath:
		return true
	default:
		return false
	}
}

// IsShopAccountBridgeAuthenticated reports whether the exact bridge ingress
// HMAC has already succeeded for this request. It must never be inferred from
// a caller-controlled header or from a generic trust tier.
func IsShopAccountBridgeAuthenticated(c *gin.Context) bool {
	if c == nil {
		return false
	}
	authenticated, exists := c.Get(shopAccountBridgeAuthenticatedContextKey)
	return exists && authenticated == true
}

func shopAccountBridgeHMAC(secret []byte, method, path, timestamp string, body []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(strings.ToUpper(method)))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(path))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write(body)
	return mac.Sum(nil)
}

func abortShopAccountBridgeUnauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":    http.StatusUnauthorized,
		"message": "Unauthorized",
	})
}
