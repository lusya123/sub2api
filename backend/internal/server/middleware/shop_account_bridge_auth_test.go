package middleware

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const testShopAccountBridgeSecret = "0123456789abcdef0123456789abcdef"

func signedShopAccountBridgeRequest(method, path string, body []byte, timestamp time.Time, secret string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ts := strconv.FormatInt(timestamp.Unix(), 10)
	req.Header.Set(shopAccountBridgeTimestampHeader, ts)
	req.Header.Set(shopAccountBridgeSignatureHeader, "sha256="+hex.EncodeToString(
		shopAccountBridgeHMAC([]byte(secret), method, req.URL.EscapedPath(), ts, body),
	))
	return req
}

func TestShopAccountBridgeHMACAuth_AcceptsValidSignedBodyAndRestoresIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 8, 2, 2, 3, 4, 0, time.UTC)
	body := []byte(`{"email":"bridge@example.com"}`)
	router := gin.New()
	router.Use(shopAccountBridgeHMACAuth(config.ShopAccountBridgeConfig{
		Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
	}, func() time.Time { return now }))
	router.POST("/api/v1/integrations/shop/account-bridge/lookup", func(c *gin.Context) {
		var payload map[string]string
		require.NoError(t, c.ShouldBindJSON(&payload))
		require.Equal(t, "bridge@example.com", payload["email"])
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, signedShopAccountBridgeRequest(
		http.MethodPost,
		"/api/v1/integrations/shop/account-bridge/lookup",
		body,
		now,
		testShopAccountBridgeSecret,
	))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
}

func TestShopAccountBridgeHMACAuth_RejectsInvalidSignatureReplayWindowAndPathTamper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 8, 2, 2, 3, 4, 0, time.UTC)
	body := []byte(`{"email":"bridge@example.com"}`)
	endpoint := "/api/v1/integrations/shop/account-bridge/lookup"

	tests := []struct {
		name       string
		request    func() *http.Request
		wantStatus int
	}{
		{
			name: "wrong secret",
			request: func() *http.Request {
				return signedShopAccountBridgeRequest(http.MethodPost, endpoint, body, now, "abcdef0123456789abcdef0123456789")
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "stale replay",
			request: func() *http.Request {
				return signedShopAccountBridgeRequest(http.MethodPost, endpoint, body, now.Add(-61*time.Second), testShopAccountBridgeSecret)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "future timestamp",
			request: func() *http.Request {
				return signedShopAccountBridgeRequest(http.MethodPost, endpoint, body, now.Add(61*time.Second), testShopAccountBridgeSecret)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "path tamper",
			request: func() *http.Request {
				req := signedShopAccountBridgeRequest(http.MethodPost, endpoint, body, now, testShopAccountBridgeSecret)
				req.URL.Path = "/api/v1/integrations/shop/account-bridge/users"
				return req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "body tamper",
			request: func() *http.Request {
				req := signedShopAccountBridgeRequest(http.MethodPost, endpoint, body, now, testShopAccountBridgeSecret)
				tampered := []byte(`{"email":"attacker@example.com"}`)
				req.Body = io.NopCloser(bytes.NewReader(tampered))
				req.ContentLength = int64(len(tampered))
				return req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "query rejected",
			request: func() *http.Request {
				return signedShopAccountBridgeRequest(http.MethodPost, endpoint+"?email=pii@example.com", body, now, testShopAccountBridgeSecret)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "duplicate signature header",
			request: func() *http.Request {
				req := signedShopAccountBridgeRequest(http.MethodPost, endpoint, body, now, testShopAccountBridgeSecret)
				req.Header.Add(shopAccountBridgeSignatureHeader, req.Header.Get(shopAccountBridgeSignatureHeader))
				return req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "duplicate timestamp header",
			request: func() *http.Request {
				req := signedShopAccountBridgeRequest(http.MethodPost, endpoint, body, now, testShopAccountBridgeSecret)
				req.Header.Add(shopAccountBridgeTimestampHeader, req.Header.Get(shopAccountBridgeTimestampHeader))
				return req
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			router := gin.New()
			router.Use(shopAccountBridgeHMACAuth(config.ShopAccountBridgeConfig{
				Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
			}, func() time.Time { return now }))
			router.POST(endpoint, func(c *gin.Context) { called = true; c.Status(http.StatusNoContent) })
			router.POST("/api/v1/integrations/shop/account-bridge/users", func(c *gin.Context) { called = true; c.Status(http.StatusNoContent) })
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, tc.request())
			require.Equal(t, tc.wantStatus, recorder.Code)
			require.False(t, called, "invalid HMAC request reached the database-capable handler")
		})
	}
}

func TestShopAccountBridgeHMACAuth_RejectsOversizedBodyBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 8, 2, 2, 3, 4, 0, time.UTC)
	body := bytes.Repeat([]byte("x"), shopAccountBridgeMaxBodyBytes+1)
	endpoint := "/api/v1/integrations/shop/account-bridge/verify-password"
	called := false
	router := gin.New()
	router.Use(shopAccountBridgeHMACAuth(config.ShopAccountBridgeConfig{
		Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
	}, func() time.Time { return now }))
	router.POST(endpoint, func(c *gin.Context) { called = true })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, signedShopAccountBridgeRequest(http.MethodPost, endpoint, body, now, testShopAccountBridgeSecret))
	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.False(t, called)
}

func TestShopAccountBridgeIngressHMACAuth_DefaultDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ShopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{}))
	router.POST(shopAccountBridgeLookupPath, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, shopAccountBridgeLookupPath, bytes.NewReader([]byte(`{}`))))
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestShopAccountBridgeIngressAuthRunsBeforeReplayDefenses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_ATTACK_DETECTOR_ENABLED", "true")
	t.Setenv("DEFENSE_BODY_FINGERPRINT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	now := time.Date(2026, 8, 2, 2, 3, 4, 0, time.UTC)
	body := []byte(`{"email":"bridge@example.com"}`)
	called := 0
	router := gin.New()
	router.Use(shopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{
		Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
	}, func() time.Time { return now }))
	router.Use(TrustTierDetector())
	router.Use(NewAttackDetector(rdb, NewCookieCreditSystem(rdb)).Middleware())
	router.Use(NewBodyFingerprint(rdb).Middleware())
	bridge := router.Group("/api/v1/integrations/shop/account-bridge")
	bridge.Use(RequireShopAccountBridgeAuthenticated())
	bridge.POST("/lookup", func(c *gin.Context) {
		called++
		var payload map[string]string
		require.NoError(t, c.ShouldBindJSON(&payload))
		require.Equal(t, "bridge@example.com", payload["email"])
		c.Status(http.StatusNoContent)
	})

	// Repeating an unsigned request with the exact eventual signed body must not
	// pre-populate either global replay detector's key.
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest(http.MethodPost, shopAccountBridgeLookupPath, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusUnauthorized, recorder.Code, "unsigned request %d", i+1)
	}
	for _, pattern := range []string{"fp:body:*", "atk:body:*"} {
		keys, err := rdb.Keys(context.Background(), pattern).Result()
		require.NoError(t, err)
		require.Empty(t, keys, pattern)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, signedShopAccountBridgeRequest(
		http.MethodPost, shopAccountBridgeLookupPath, body, now, testShopAccountBridgeSecret,
	))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, 1, called)
}

func TestAuthenticatedShopBridgeBypassesAllBrowserDefenses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_ATTACK_DETECTOR_ENABLED", "true")
	t.Setenv("DEFENSE_BODY_FINGERPRINT_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_PATH_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")
	t.Setenv("DEFENSE_CREDIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	now := time.Date(2026, 8, 2, 2, 3, 4, 0, time.UTC)
	// This address is an explicit AttackDetector signature on ordinary routes.
	body := []byte(`{"email":"admin@admin.com"}`)
	visitorMgr := NewVisitorCookieManagerWithSecret(rdb, "bridge-defense-test-secret")
	credit := NewCookieCreditSystem(rdb)
	blockedCookie, _ := visitorMgr.IssueCookieWithFingerprint("blocked-fingerprint")
	require.NoError(t, rdb.Set(context.Background(), "credit:"+CookieHash(blockedCookie), 0, time.Hour).Err())
	pathLimiter := NewPathLevelRateLimiter(rdb)
	pathLimiter.limits[shopAccountBridgeLookupPath] = pathLimit{limit: 1, window: time.Minute}

	called := 0
	router := gin.New()
	router.Use(shopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{
		Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
	}, func() time.Time { return now }))
	router.Use(TrustTierDetector())
	router.Use(NewAttackDetector(rdb, credit).Middleware())
	router.Use(CookieReputationGuard(credit))
	router.Use(VisitorCookieIssuerMiddleware(visitorMgr))
	router.Use(NewGlobalRateLimiter(rdb, visitorMgr).Middleware())
	router.Use(pathLimiter.Middleware())
	router.Use(NewBodyFingerprint(rdb).Middleware())
	bridge := router.Group("/api/v1/integrations/shop/account-bridge")
	bridge.Use(RequireShopAccountBridgeAuthenticated())
	bridge.POST("/lookup", func(c *gin.Context) {
		require.Equal(t, TierShopBridge, GetTrustTier(c))
		called++
		c.Status(http.StatusNoContent)
	})

	// Well above the anonymous 30/minute, replay 50/minute, and forced path
	// 1/minute limits: every request must still reach the scoped handler after
	// its own HMAC succeeds, even with a blocked browser cookie attached.
	for i := 0; i < 120; i++ {
		req := signedShopAccountBridgeRequest(
			http.MethodPost, shopAccountBridgeLookupPath, body, now, testShopAccountBridgeSecret,
		)
		req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: blockedCookie})
		req.AddCookie(&http.Cookie{Name: visitorFingerprintName, Value: "blocked-fingerprint"})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusNoContent, recorder.Code, "signed request %d", i+1)
	}
	require.Equal(t, 120, called)
	for _, pattern := range []string{"fp:body:*", "atk:body:*", "rl:anon:*", "path:*"} {
		keys, err := rdb.Keys(context.Background(), pattern).Result()
		require.NoError(t, err)
		require.Empty(t, keys, pattern)
	}
}

func TestShopAccountBridgeIngressRejectsOversizeBeforeReplayAndHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_ATTACK_DETECTOR_ENABLED", "true")
	t.Setenv("DEFENSE_BODY_FINGERPRINT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	now := time.Date(2026, 8, 2, 2, 3, 4, 0, time.UTC)
	body := bytes.Repeat([]byte("x"), shopAccountBridgeMaxBodyBytes+1)
	called := false
	router := gin.New()
	router.Use(shopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{
		Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
	}, func() time.Time { return now }))
	router.Use(TrustTierDetector())
	router.Use(NewAttackDetector(rdb, NewCookieCreditSystem(rdb)).Middleware())
	router.Use(NewBodyFingerprint(rdb).Middleware())
	bridge := router.Group("/api/v1/integrations/shop/account-bridge")
	bridge.Use(RequireShopAccountBridgeAuthenticated())
	bridge.POST("/verify-password", func(c *gin.Context) { called = true })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, signedShopAccountBridgeRequest(
		http.MethodPost, shopAccountBridgeVerifyPasswordPath, body, now, testShopAccountBridgeSecret,
	))
	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.False(t, called, "oversized request reached the database-capable handler")
	for _, pattern := range []string{"fp:body:*", "atk:body:*"} {
		keys, err := rdb.Keys(context.Background(), pattern).Result()
		require.NoError(t, err)
		require.Empty(t, keys, pattern)
	}
}

func TestShopAccountBridgeIngressDoesNotBypassOrdinaryRouteFingerprinting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_ATTACK_DETECTOR_ENABLED", "true")
	t.Setenv("DEFENSE_BODY_FINGERPRINT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(ShopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{
		Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
	}))
	router.Use(TrustTierDetector())
	router.Use(NewAttackDetector(rdb, NewCookieCreditSystem(rdb)).Middleware())
	router.Use(NewBodyFingerprint(rdb).Middleware())
	router.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusUnauthorized) })

	body := []byte(`{"email":"normal@example.com"}`)
	for i := 0; i < 50; i++ {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body)))
		require.Equal(t, http.StatusUnauthorized, recorder.Code, "ordinary request %d", i+1)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body)))
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "abnormal pattern")
}

func TestShopAccountBridgeIngressDoesNotBypassOrdinaryRateLimitOrAttackDetection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_ATTACK_DETECTOR_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	t.Run("known attack payload remains blocked", func(t *testing.T) {
		rdb, cleanup := newDefenseTestRedis(t)
		defer cleanup()

		router := gin.New()
		router.Use(ShopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{
			Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
		}))
		router.Use(TrustTierDetector())
		router.Use(NewAttackDetector(rdb, NewCookieCreditSystem(rdb)).Middleware())
		router.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusNoContent) })

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
			bytes.NewReader([]byte(`{"email":"admin@admin.com"}`))))
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("anonymous global rate limit remains active", func(t *testing.T) {
		rdb, cleanup := newDefenseTestRedis(t)
		defer cleanup()

		router := gin.New()
		router.Use(ShopAccountBridgeIngressHMACAuth(config.ShopAccountBridgeConfig{
			Enabled: true, SharedSecret: testShopAccountBridgeSecret, ClockSkewSeconds: 60,
		}))
		router.Use(TrustTierDetector())
		router.Use(NewGlobalRateLimiter(rdb).Middleware())
		router.POST("/api/v1/auth/login", func(c *gin.Context) { c.Status(http.StatusUnauthorized) })

		for i := 0; i < 30; i++ {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(`{}`))))
			require.Equal(t, http.StatusUnauthorized, recorder.Code, "ordinary request %d", i+1)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(`{}`))))
		require.Equal(t, http.StatusTooManyRequests, recorder.Code)
		require.Contains(t, recorder.Body.String(), "rate limited (anon)")
	})
}
