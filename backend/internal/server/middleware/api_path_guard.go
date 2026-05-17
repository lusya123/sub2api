package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var apiPathGuardAPIPrefixes = []string{
	"/api/",
	"/v1/",
	"/v1beta/",
	"/responses",
	"/backend-api/",
	"/openai/",
	"/antigravity/",
	"/setup/",
	"/health",
	"/internal/",
	"/oauth2/",
	"/.well-known/",
}

var apiPathGuardFrontendExactPaths = map[string]struct{}{
	"/auth/callback":                {},
	"/auth/oauth/callback":          {},
	"/auth/linuxdo/callback":        {},
	"/auth/wechat/callback":         {},
	"/auth/wechat/payment/callback": {},
	"/auth/oidc/callback":           {},
	"/email-verify":                 {},
}

var apiPathGuardAdminFrontendPrefixes = []string{
	"/admin/dashboard",
	"/admin/ops",
	"/admin/operations",
	"/admin/globe",
	"/admin/audit-logs",
	"/admin/model-marketplace",
	"/admin/model-health",
	"/admin/users",
	"/admin/groups",
	"/admin/channels",
	"/admin/subscriptions",
	"/admin/accounts",
	"/admin/announcements",
	"/admin/proxies",
	"/admin/redeem",
	"/admin/promo-codes",
	"/admin/settings",
	"/admin/risk-control",
	"/admin/usage",
	"/admin/affiliates",
	"/admin/orders",
}

var apiPathGuardBlockedPrefixes = []string{
	"/auth/",
	"/admin/",
	"/oauth/",
}

func APIPathGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("DEFENSE_API_PATH_GUARD_ENABLED") == "false" {
			c.Next()
			return
		}
		if c == nil || c.Request == nil || c.Request.URL == nil {
			c.Next()
			return
		}

		path := strings.TrimSpace(c.Request.URL.Path)
		if path == "" {
			c.Next()
			return
		}
		if isAPIPathGuardLegitimateAPI(path) || isAPIPathGuardKnownFrontend(path) {
			c.Next()
			return
		}

		for _, prefix := range apiPathGuardBlockedPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
		}
		c.Next()
	}
}

func isAPIPathGuardLegitimateAPI(path string) bool {
	if path == "/api" {
		return true
	}
	for _, prefix := range apiPathGuardAPIPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func isAPIPathGuardKnownFrontend(path string) bool {
	if _, ok := apiPathGuardFrontendExactPaths[path]; ok {
		return true
	}
	if path == "/admin" {
		return true
	}
	for _, prefix := range apiPathGuardAdminFrontendPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
