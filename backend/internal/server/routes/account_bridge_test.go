package routes

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const routeTestShopAccountBridgeSecret = "0123456789abcdef0123456789abcdef"

func signedRouteShopAccountBridgeRequest(path string, body []byte, now time.Time) *http.Request {
	timestamp := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(routeTestShopAccountBridgeSecret))
	_, _ = mac.Write([]byte(http.MethodPost + "\n" + path + "\n" + timestamp + "\n"))
	_, _ = mac.Write(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Shop-Bridge-Timestamp", timestamp)
	req.Header.Set("X-Shop-Bridge-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	return req
}

func TestShopAccountBridgeRoutesAreSeparateFromAdminScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(servermiddleware.ShopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{
		Enabled: true, SharedSecret: routeTestShopAccountBridgeSecret, ClockSkewSeconds: 60,
	}))
	RegisterShopAccountBridgeRoutes(router.Group("/api/v1"), &handler.Handlers{Auth: &handler.AuthHandler{}})
	body := []byte(`{"email":"bridge@example.com"}`)
	now := time.Now().UTC()

	// A correctly signed request reaches only the scoped handler. The zero-value
	// handler returns 500, proving the HMAC middleware accepted it without admin auth.
	accepted := httptest.NewRecorder()
	router.ServeHTTP(accepted, signedRouteShopAccountBridgeRequest(
		"/api/v1/integrations/shop/account-bridge/lookup", body, now,
	))
	require.Equal(t, http.StatusInternalServerError, accepted.Code)
	require.Equal(t, "no-store", accepted.Header().Get("Cache-Control"))

	// The retired admin bridge and generic admin resources are not registered by
	// this credential scope, even with a valid bridge signature.
	for _, path := range []string{
		"/api/v1/admin/users",
	} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, signedRouteShopAccountBridgeRequest(path, body, now))
		require.Equal(t, http.StatusNotFound, recorder.Code, path)
	}

	// Conversely, an administrator API-key header alone cannot authenticate the
	// integrations route.
	adminKeyOnly := httptest.NewRequest(http.MethodPost,
		"/api/v1/integrations/shop/account-bridge/lookup", strings.NewReader(string(body)))
	adminKeyOnly.Header.Set("Content-Type", "application/json")
	adminKeyOnly.Header.Set("x-api-key", "admin-key-must-not-work-here")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, adminKeyOnly)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestShopAccountBridgeRoutesDefaultDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(servermiddleware.ShopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{}))
	RegisterShopAccountBridgeRoutes(router.Group("/api/v1"), &handler.Handlers{Auth: &handler.AuthHandler{}})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost,
		"/api/v1/integrations/shop/account-bridge/lookup", strings.NewReader(`{}`)))
	require.Equal(t, http.StatusNotFound, recorder.Code)
}
