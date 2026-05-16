package middleware

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIPathGuardBlocksPseudoAPIPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_API_PATH_GUARD_ENABLED", "true")

	router := gin.New()
	router.Use(APIPathGuard())
	router.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "spa")
	})

	for _, path := range []string{
		"/auth/send-verify-code",
		"/auth/forgot-password",
		"/oauth/token",
		"/admin/foo",
	} {
		rec := performDefenseRequest(router, http.MethodGet, path, nil, nil)
		require.Equal(t, http.StatusNotFound, rec.Code, path)
		require.Contains(t, rec.Body.String(), "not found")
	}
}

func TestAPIPathGuardAllowsLegitimateAPIAndKnownFrontendPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_API_PATH_GUARD_ENABLED", "true")

	router := gin.New()
	router.Use(APIPathGuard())
	router.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "spa")
	})

	for _, path := range []string{
		"/api/v1/auth/send-verify-code",
		"/v1/messages",
		"/v1beta/models",
		"/responses",
		"/backend-api/ping",
		"/openai/oauth/callback",
		"/antigravity/oauth/callback",
		"/setup/status",
		"/health",
		"/auth/callback",
		"/auth/linuxdo/callback",
		"/auth/wechat/payment/callback",
		"/auth/oidc/callback",
		"/email-verify",
		"/admin/dashboard",
		"/admin/model-marketplace",
		"/admin/orders/plans",
		"/login",
		"/dashboard",
	} {
		rec := performDefenseRequest(router, http.MethodGet, path, nil, nil)
		require.Equal(t, http.StatusOK, rec.Code, path)
	}
}

func TestAPIPathGuardCanBeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_API_PATH_GUARD_ENABLED", "false")

	router := gin.New()
	router.Use(APIPathGuard())
	router.NoRoute(func(c *gin.Context) {
		c.String(http.StatusOK, "spa")
	})

	rec := performDefenseRequest(router, http.MethodGet, "/auth/send-verify-code", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
}
