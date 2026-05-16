package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const spaProtectTestJWTSecret = "spa-protect-test-secret-32-bytes!!"

func TestSPAProtectLimitsAnonymousSPARoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SPA_PROTECT_ENABLED", "true")
	t.Setenv("SPA_PROTECT_MAX_PER_MINUTE", "2")
	t.Setenv("SPA_PROTECT_BAN_SECONDS", "60")

	rdb, cleanup := newSPAProtectTestRedis(t)
	defer cleanup()

	router := newSPAProtectTestRouter(rdb)

	require.Equal(t, http.StatusOK, performSPAProtectRequest(router, "/model-marketplace", nil).Code)
	require.Equal(t, http.StatusOK, performSPAProtectRequest(router, "/model-marketplace", nil).Code)

	rec := performSPAProtectRequest(router, "/model-marketplace", nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "rate limit exceeded")

	rec = performSPAProtectRequest(router, "/dashboard", nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "blocked")

	exists, err := rdb.Exists(context.Background(), "spa:ban:203.0.113.10").Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), exists)
}

func TestSPAProtectClearsRateCounterWhenBanStarts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SPA_PROTECT_ENABLED", "true")
	t.Setenv("SPA_PROTECT_MAX_PER_MINUTE", "1")
	t.Setenv("SPA_PROTECT_BAN_SECONDS", "2")

	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		s.Close()
	})

	router := newSPAProtectTestRouter(rdb)
	require.Equal(t, http.StatusOK, performSPAProtectRequest(router, "/model-marketplace", nil).Code)
	require.Equal(t, http.StatusTooManyRequests, performSPAProtectRequest(router, "/model-marketplace", nil).Code)

	exists, err := rdb.Exists(context.Background(), "spa:rate:203.0.113.10").Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), exists)

	s.FastForward(3 * time.Second)
	require.Equal(t, http.StatusOK, performSPAProtectRequest(router, "/model-marketplace", nil).Code)
}

func TestSPAProtectBypassesTrustedTraffic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SPA_PROTECT_ENABLED", "true")
	t.Setenv("SPA_PROTECT_MAX_PER_MINUTE", "1")

	tests := []struct {
		name string
		path string
		set  func(*http.Request)
	}{
		{
			name: "token cookie",
			path: "/dashboard",
			set:  func(req *http.Request) { req.Header.Set("Cookie", "token=fake-jwt-for-test") },
		},
		{
			name: "bearer jwt candidate",
			path: "/dashboard",
			set:  func(req *http.Request) { req.Header.Set("Authorization", "Bearer jwt-token") },
		},
		{
			name: "bearer api key",
			path: "/model-marketplace",
			set:  func(req *http.Request) { req.Header.Set("Authorization", "Bearer sk-test") },
		},
		{
			name: "x api key",
			path: "/model-marketplace",
			set:  func(req *http.Request) { req.Header.Set("x-api-key", "sk-test") },
		},
		{
			name: "gateway api path",
			path: "/v1/messages",
		},
		{
			name: "application api path",
			path: "/api/v1/auth/login",
		},
		{
			name: "static asset",
			path: "/assets/index.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdb, cleanup := newSPAProtectTestRedis(t)
			defer cleanup()

			router := newSPAProtectTestRouter(rdb)
			for i := 0; i < 3; i++ {
				rec := performSPAProtectRequest(router, tt.path, tt.set)
				require.Equal(t, http.StatusOK, rec.Code, "request %d should bypass SPA protection", i+1)
			}

			keys, err := rdb.Keys(context.Background(), "spa:*").Result()
			require.NoError(t, err)
			require.Empty(t, keys)
		})
	}
}

func TestSPAProtectRequiresVerifiedUserForSensitiveRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SPA_PROTECT_ENABLED", "true")
	t.Setenv("SPA_PROTECT_MAX_PER_MINUTE", "20")

	validJWT := generateSPAProtectTestJWT(t, spaProtectTestJWTSecret, time.Now().Add(time.Hour))
	expiredJWT := generateSPAProtectTestJWT(t, spaProtectTestJWTSecret, time.Now().Add(-time.Hour))

	tests := []struct {
		name string
		path string
		set  func(*http.Request)
		want int
	}{
		{
			name: "chat requires login",
			path: "/chat",
			want: http.StatusUnauthorized,
		},
		{
			name: "use token alias requires login",
			path: "/use-token",
			want: http.StatusUnauthorized,
		},
		{
			name: "nested chat route requires login",
			path: "/chat/agent",
			want: http.StatusUnauthorized,
		},
		{
			name: "legacy marker cookie is not enough",
			path: "/chat",
			set:  func(req *http.Request) { req.Header.Set("Cookie", "token=1") },
			want: http.StatusUnauthorized,
		},
		{
			name: "fake jwt cookie is not enough",
			path: "/chat",
			set:  func(req *http.Request) { req.Header.Set("Cookie", "token=fake-jwt-for-test") },
			want: http.StatusUnauthorized,
		},
		{
			name: "expired jwt cookie is not enough",
			path: "/chat",
			set:  func(req *http.Request) { req.Header.Set("Cookie", "token="+expiredJWT) },
			want: http.StatusUnauthorized,
		},
		{
			name: "api key is not page login",
			path: "/chat",
			set:  func(req *http.Request) { req.Header.Set("Authorization", "Bearer sk-test") },
			want: http.StatusUnauthorized,
		},
		{
			name: "signed jwt cookie opens chat",
			path: "/chat",
			set:  func(req *http.Request) { req.Header.Set("Cookie", "token="+validJWT) },
			want: http.StatusOK,
		},
		{
			name: "signed bearer jwt opens chat",
			path: "/use-token",
			set:  func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+validJWT) },
			want: http.StatusOK,
		},
		{
			name: "model marketplace still accepts legacy marker",
			path: "/model-marketplace",
			set:  func(req *http.Request) { req.Header.Set("Cookie", "token=1") },
			want: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rdb, cleanup := newSPAProtectTestRedis(t)
			defer cleanup()

			router := newSPAProtectTestRouter(rdb)
			rec := performSPAProtectRequest(router, tt.path, tt.set)
			require.Equal(t, tt.want, rec.Code)
		})
	}
}

func TestSPAProtectBansRepeatedSensitiveAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SPA_PROTECT_ENABLED", "true")
	t.Setenv("SPA_PROTECT_MAX_PER_MINUTE", "1")
	t.Setenv("SPA_PROTECT_BAN_SECONDS", "60")

	rdb, cleanup := newSPAProtectTestRedis(t)
	defer cleanup()

	router := newSPAProtectTestRouter(rdb)

	rec := performSPAProtectRequest(router, "/chat", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = performSPAProtectRequest(router, "/chat", nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "rate limit exceeded")

	rec = performSPAProtectRequest(router, "/model-marketplace", nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "blocked")
}

func TestSPAProtectRequiresVerifiedUserWithoutRedis(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SPA_PROTECT_ENABLED", "true")

	router := newSPAProtectTestRouter(nil)

	rec := performSPAProtectRequest(router, "/chat", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	validJWT := generateSPAProtectTestJWT(t, spaProtectTestJWTSecret, time.Now().Add(time.Hour))
	rec = performSPAProtectRequest(router, "/chat", func(req *http.Request) {
		req.Header.Set("Cookie", "token="+validJWT)
	})
	require.Equal(t, http.StatusOK, rec.Code)

	rec = performSPAProtectRequest(router, "/model-marketplace", nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSPAProtectDisabledAndRedisFailureFailOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("disabled", func(t *testing.T) {
		t.Setenv("SPA_PROTECT_ENABLED", "false")
		t.Setenv("SPA_PROTECT_MAX_PER_MINUTE", "1")

		rdb, cleanup := newSPAProtectTestRedis(t)
		defer cleanup()

		router := newSPAProtectTestRouter(rdb)
		for i := 0; i < 3; i++ {
			require.Equal(t, http.StatusOK, performSPAProtectRequest(router, "/model-marketplace", nil).Code)
		}
	})

	t.Run("redis failure", func(t *testing.T) {
		t.Setenv("SPA_PROTECT_ENABLED", "true")
		t.Setenv("SPA_PROTECT_MAX_PER_MINUTE", "1")

		rdb := redis.NewClient(&redis.Options{
			Addr:         "127.0.0.1:1",
			DialTimeout:  10 * time.Millisecond,
			ReadTimeout:  10 * time.Millisecond,
			WriteTimeout: 10 * time.Millisecond,
		})
		t.Cleanup(func() { _ = rdb.Close() })

		router := newSPAProtectTestRouter(rdb)
		require.Equal(t, http.StatusOK, performSPAProtectRequest(router, "/model-marketplace", nil).Code)
	})
}

func newSPAProtectTestRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return rdb, func() {
		_ = rdb.Close()
		s.Close()
	}
}

func newSPAProtectTestRouter(rdb *redis.Client) *gin.Engine {
	router := gin.New()
	router.Use(SPAProtect(rdb, spaProtectTestJWTSecret))
	router.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return router
}

func generateSPAProtectTestJWT(t *testing.T, secret string, expiresAt time.Time) string {
	t.Helper()
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return signed
}

func performSPAProtectRequest(router *gin.Engine, path string, set func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("User-Agent", "Mozilla/5.0 spa-protect-test")
	req.Header.Set("Accept-Language", "en-US")
	if set != nil {
		set(req)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
