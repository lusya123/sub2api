package middleware

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	TrustTierContextKey = "trust_tier"

	TierAnonymous = "anonymous"
	TierUser      = "user"
	TierAPIKey    = "apikey"

	minAPIKeyCredentialLength = 16
)

// TrustTierDetector marks the request with its coarse trust tier. It does not
// authenticate credentials; route auth middleware remains authoritative.
func TrustTierDetector() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("DEFENSE_TRUST_TIER_ENABLED") != "false" {
			c.Set(TrustTierContextKey, InferTrustTier(c))
		}
		c.Next()
	}
}

func InferTrustTier(c *gin.Context) string {
	if hasAPIKeyCredential(c) && isGatewayAPIPath(c.Request.URL.Path) {
		return TierAPIKey
	}
	if hasUserCredential(c) {
		return TierUser
	}
	return TierAnonymous
}

func GetTrustTier(c *gin.Context) string {
	if v, ok := c.Get(TrustTierContextKey); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return InferTrustTier(c)
}

func MarkTrustTier(c *gin.Context, tier string) {
	if os.Getenv("DEFENSE_TRUST_TIER_ENABLED") == "false" {
		return
	}
	c.Set(TrustTierContextKey, tier)
}

func hasAPIKeyCredential(c *gin.Context) bool {
	if key := strings.TrimSpace(c.GetHeader("x-api-key")); looksLikeAPIKeyCredential(key) {
		return true
	}
	if key := strings.TrimSpace(c.GetHeader("x-goog-api-key")); looksLikeAPIKeyCredential(key) {
		return true
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if auth == "" {
		return false
	}
	parts := strings.SplitN(auth, " ", 2)
	return len(parts) == 2 &&
		strings.EqualFold(parts[0], "Bearer") &&
		looksLikeAPIKeyCredential(strings.TrimSpace(parts[1]))
}

func looksLikeAPIKeyCredential(key string) bool {
	if !strings.HasPrefix(key, "sk-") || len(key) < minAPIKeyCredentialLength {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func hasUserCredential(c *gin.Context) bool {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		token := strings.TrimSpace(parts[1])
		if token != "" && !strings.HasPrefix(token, "sk-") {
			return true
		}
	}
	if token, err := c.Cookie("token"); err == nil && strings.TrimSpace(token) != "" {
		return true
	}
	return false
}

func isGatewayAPIPath(path string) bool {
	return IsGatewayAPIPath(path)
}
