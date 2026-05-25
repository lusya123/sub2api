package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestAdminAuthFastPathRejectsMalformedAdminAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_FASTPATH_ENABLED", "true")
	t.Setenv("ADMIN_FASTPATH_LOG_SAMPLE", "0")
	resetAdminAuthFastPathMetricsForTest()

	tests := []struct {
		name      string
		auth      string
		wantField func(AdminAuthFastPathMetrics) uint64
	}{
		{
			name:      "no authorization header",
			wantField: func(m AdminAuthFastPathMetrics) uint64 { return m.RejectNoHeader },
		},
		{
			name:      "bad scheme",
			auth:      "hello",
			wantField: func(m AdminAuthFastPathMetrics) uint64 { return m.RejectBadScheme },
		},
		{
			name:      "short bearer token",
			auth:      "Bearer abc",
			wantField: func(m AdminAuthFastPathMetrics) uint64 { return m.RejectBadLength },
		},
		{
			name:      "long bearer token",
			auth:      "Bearer " + strings.Repeat("a", 4097),
			wantField: func(m AdminAuthFastPathMetrics) uint64 { return m.RejectBadLength },
		},
		{
			name:      "bad charset",
			auth:      "Bearer valid-length-token!",
			wantField: func(m AdminAuthFastPathMetrics) uint64 { return m.RejectBadCharset },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newAdminAuthFastPathTestRouter(t)
			before := tt.wantField(AdminAuthFastPathMetricsSnapshot())

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/models", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusUnauthorized, w.Code)
			require.Empty(t, w.Body.String())
			require.Equal(t, "0", w.Header().Get("Content-Length"))
			require.Equal(t, "close", w.Header().Get("Connection"))
			require.Equal(t, before+1, tt.wantField(AdminAuthFastPathMetricsSnapshot()))
		})
	}
}

func TestAdminAuthFastPathAllowsSupportedCredentialsAndNonAdminPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_FASTPATH_ENABLED", "true")
	t.Setenv("ADMIN_FASTPATH_LOG_SAMPLE", "0")

	validJWT := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature"
	validOpaque := "0123456789abcdef"

	tests := []struct {
		name    string
		path    string
		headers map[string]string
	}{
		{
			name: "valid bearer token",
			path: "/api/v1/admin/dashboard/models",
			headers: map[string]string{
				"Authorization": "Bearer " + validJWT,
			},
		},
		{
			name: "lowercase bearer remains compatible",
			path: "/api/v1/admin/dashboard/models",
			headers: map[string]string{
				"Authorization": "bearer " + validJWT,
			},
		},
		{
			name: "admin api key bypasses bearer precheck",
			path: "/api/v1/admin/dashboard/models",
			headers: map[string]string{
				"x-api-key": "admin-" + strings.Repeat("a", 64),
			},
		},
		{
			name: "admin websocket subprotocol jwt",
			path: "/api/v1/admin/ops/ws/qps",
			headers: map[string]string{
				"Upgrade":                "websocket",
				"Connection":             "Upgrade",
				"Sec-WebSocket-Protocol": "sub2api-admin, jwt." + validJWT,
			},
		},
		{
			name: "non admin path",
			path: "/api/v1/user/profile",
		},
		{
			name: "admin root path",
			path: "/api/v1/admin",
			headers: map[string]string{
				"Authorization": "Bearer " + validOpaque,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newAdminAuthFastPathTestRouter(t)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			require.JSONEq(t, `{"ok":true}`, w.Body.String())
		})
	}
}

func TestAdminAuthFastPathCanBeDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_FASTPATH_ENABLED", "false")

	router := newAdminAuthFastPathTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/models", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"ok":true}`, w.Body.String())
}

func TestAdminAuthFastPathRejectsWebSocketWithoutJWTSubprotocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_FASTPATH_ENABLED", "true")
	t.Setenv("ADMIN_FASTPATH_LOG_SAMPLE", "0")

	router := newAdminAuthFastPathTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ops/ws/qps", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Empty(t, w.Body.String())
}

func TestAdminAuthFastPathRejectsMalformedAdminAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_FASTPATH_ENABLED", "true")
	t.Setenv("ADMIN_FASTPATH_LOG_SAMPLE", "0")
	resetAdminAuthFastPathMetricsForTest()

	router := newAdminAuthFastPathTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/models", nil)
	req.Header.Set("x-api-key", "short")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Empty(t, w.Body.String())
	require.Equal(t, uint64(1), AdminAuthFastPathMetricsSnapshot().RejectBadLength)
}

func TestAdminAuthFastPathBansRepeatedFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_FASTPATH_ENABLED", "true")
	t.Setenv("ADMIN_FASTPATH_LOG_SAMPLE", "0")
	t.Setenv("ADMIN_FAIL_BAN_THRESHOLD", "2")
	t.Setenv("ADMIN_FAIL_BAN_WINDOW", "60")
	t.Setenv("ADMIN_FAIL_BAN_DURATION", "600")
	resetAdminAuthFastPathMetricsForTest()

	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	router := newAdminAuthFastPathTestRouterWithRedis(t, rdb)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/models", nil)
		req.RemoteAddr = "203.0.113.10:12345"
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/models", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, w.Body.String())
	require.Equal(t, uint64(1), AdminAuthFastPathMetricsSnapshot().RejectBanned)
}

func TestAdminAuthFailureRecorderCountsRealAuth401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_FAIL_BAN_THRESHOLD", "1")
	t.Setenv("ADMIN_FAIL_BAN_WINDOW", "60")
	t.Setenv("ADMIN_FAIL_BAN_DURATION", "600")

	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	router := gin.New()
	router.Use(AdminAuthFailureRecorder(rdb))
	router.GET("/api/v1/admin/dashboard/models", func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/models", nil)
	req.RemoteAddr = "203.0.113.11:12345"
	router.ServeHTTP(httptest.NewRecorder(), req)

	got, err := s.Get("fail:admin:203.0.113.11")
	require.NoError(t, err)
	require.Equal(t, "1", got)
}

func newAdminAuthFastPathTestRouter(t *testing.T) *gin.Engine {
	return newAdminAuthFastPathTestRouterWithRedis(t, nil)
}

func newAdminAuthFastPathTestRouterWithRedis(t *testing.T, rdb *redis.Client) *gin.Engine {
	t.Helper()

	router := gin.New()
	router.Use(AdminAuthFastPath(rdb))
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}
