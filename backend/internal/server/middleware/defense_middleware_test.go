package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestTrustTierDetector(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")

	tests := []struct {
		name string
		set  func(*http.Request)
		want string
	}{
		{
			name: "x-api-key on gateway path",
			set: func(r *http.Request) {
				r.URL.Path = "/v1/messages"
				r.RequestURI = "/v1/messages"
				r.Header.Set("x-api-key", "sk-test-valid-candidate-0000")
			},
			want: TierAPIKey,
		},
		{
			name: "x-goog-api-key on gemini path",
			set: func(r *http.Request) {
				r.URL.Path = "/v1beta/models"
				r.RequestURI = "/v1beta/models"
				r.Header.Set("x-goog-api-key", "sk-test-valid-candidate-0000")
			},
			want: TierAPIKey,
		},
		{
			name: "bearer api key on alias path",
			set: func(r *http.Request) {
				r.URL.Path = "/chat/completions"
				r.RequestURI = "/chat/completions"
				r.Header.Set("Authorization", "Bearer sk-test-valid-candidate-0000")
			},
			want: TierAPIKey,
		},
		{
			name: "short fake api key on gateway path stays anonymous",
			set: func(r *http.Request) {
				r.URL.Path = "/v1/messages"
				r.RequestURI = "/v1/messages"
				r.Header.Set("x-api-key", "sk-short")
			},
			want: TierAnonymous,
		},
		{
			name: "malformed fake api key on gateway path stays anonymous",
			set: func(r *http.Request) {
				r.URL.Path = "/v1/messages"
				r.RequestURI = "/v1/messages"
				r.Header.Set("x-api-key", "sk-invalid space")
			},
			want: TierAnonymous,
		},
		{
			name: "fake api key on public path stays anonymous",
			set:  func(r *http.Request) { r.Header.Set("x-api-key", "sk-fake") },
			want: TierAnonymous,
		},
		{
			name: "bearer jwt candidate",
			set:  func(r *http.Request) { r.Header.Set("Authorization", "Bearer jwt-token") },
			want: TierUser,
		},
		{
			name: "token cookie user candidate",
			set:  func(r *http.Request) { r.Header.Set("Cookie", "token=abc") },
			want: TierUser,
		},
		{
			name: "visitor cookie only stays anonymous",
			set:  func(r *http.Request) { r.Header.Set("Cookie", visitorCookieName+"=abc") },
			want: TierAnonymous,
		},
		{
			name: "visitor fingerprint cookie stays anonymous",
			set: func(r *http.Request) {
				r.Header.Set("Cookie", visitorCookieName+"=abc; "+visitorFingerprintName+"=def")
			},
			want: TierAnonymous,
		},
		{
			name: "visitor cookie plus token cookie stays user",
			set:  func(r *http.Request) { r.Header.Set("Cookie", visitorCookieName+"=abc; token=def") },
			want: TierUser,
		},
		{
			name: "unsupported authorization scheme stays anonymous",
			set:  func(r *http.Request) { r.Header.Set("Authorization", "Token abc") },
			want: TierAnonymous,
		},
		{
			name: "anonymous",
			set:  func(r *http.Request) {},
			want: TierAnonymous,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(TrustTierDetector())
			router.GET("/t", func(c *gin.Context) {
				c.String(http.StatusOK, GetTrustTier(c))
			})
			router.NoRoute(func(c *gin.Context) {
				c.String(http.StatusOK, GetTrustTier(c))
			})

			req := httptest.NewRequest(http.MethodGet, "/t", nil)
			tt.set(req)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.want, rec.Body.String())
		})
	}
}

func TestGlobalRateLimiterTiers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/v1/messages", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 40; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/v1/messages", nil, func(req *http.Request) {
			req.Header.Set("x-api-key", "sk-legit-valid-candidate-0000")
		})
		require.Equal(t, http.StatusOK, rec.Code, "API key request %d should bypass defense limiter", i+1)
	}

	for i := 0; i < 30; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, nil)
		require.Equal(t, http.StatusOK, rec.Code, "anonymous request %d should be under limit", i+1)
	}
	rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "rate limited (anon)")

	rec = performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, func(req *http.Request) {
		req.Header.Set("x-api-key", "sk-fake")
	})
	require.Equal(t, http.StatusTooManyRequests, rec.Code, "fake API key on public path must not bypass anonymous limit")
}

func TestGlobalRateLimiterBypassesGatewayAPIKeyTraffic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_APIKEY_CANDIDATE_GLOBAL_PER_2S", "2")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/v1/messages", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/v1/messages", nil, func(req *http.Request) {
			req.Header.Set("x-api-key", "sk-legit-valid-candidate-0000")
		})
		require.Equal(t, http.StatusOK, rec.Code, "gateway API key request %d must bypass defense limits", i+1)
	}
}

func TestGlobalRateLimiterBypassesHealthAndSetupStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/setup/status", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, path := range []string{"/health", "/setup/status"} {
		for i := 0; i < 40; i++ {
			rec := performDefenseRequest(router, http.MethodGet, path, nil, nil)
			require.Equal(t, http.StatusOK, rec.Code, "%s request %d should bypass global defense limiter", path, i+1)
		}
	}
}

func TestGlobalRateLimiterBypassesVisitorCookieEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.POST("/api/public/visitor/challenge", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.POST("/api/public/visitor/issue-cookie", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, path := range []string{"/api/public/visitor/challenge", "/api/public/visitor/issue-cookie"} {
		for i := 0; i < 40; i++ {
			rec := performDefenseRequest(router, http.MethodPost, path, nil, nil)
			require.Equal(t, http.StatusOK, rec.Code, "%s request %d should bypass anonymous global limiter", path, i+1)
		}
	}
}

func TestAttackDetectorBypassesGatewayAPIPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_ATTACK_DETECTOR_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	credit := NewCookieCreditSystem(rdb)
	router := gin.New()
	router.Use(NewAttackDetector(rdb, credit).Middleware())
	router.POST("/v1/messages", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	body := []byte(`{"email":"123123123123@qq.com"}`)
	rec := performDefenseRequest(router, http.MethodPost, "/v1/messages", body, nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAttackDetectorDeductsCookieCreditForAttackPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_ATTACK_DETECTOR_ENABLED", "true")
	t.Setenv("DEFENSE_CREDIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	cookieValue, _ := mgr.IssueCookieWithFingerprint("fp")
	credit := NewCookieCreditSystem(rdb)
	router := gin.New()
	router.Use(NewAttackDetector(rdb, credit).Middleware())
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	body := []byte(`{"email":"123123123123@qq.com"}`)
	rec := performDefenseRequest(router, http.MethodPost, "/api/v1/auth/login", body, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: cookieValue})
	})
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, 0, credit.GetCredit(context.Background(), CookieHash(cookieValue)))
}

func TestPathLevelRateLimiterLimitsConfiguredPathsAndBypassesGateway(t *testing.T) {
	t.Setenv("DEFENSE_PATH_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	limiter := NewPathLevelRateLimiter(rdb)
	limiter.limits["/api/v1/auth/login"] = pathLimit{limit: 1, window: time.Minute}
	limiter.limits["/api/public/visitor/challenge"] = pathLimit{limit: 1, window: time.Minute}

	require.True(t, limiter.Allow(context.Background(), "/api/v1/auth/login"))
	require.False(t, limiter.Allow(context.Background(), "/api/v1/auth/login"))
	require.True(t, limiter.Allow(context.Background(), "/v1/messages"))
	require.True(t, limiter.Allow(context.Background(), "/v1/messages"))
	require.True(t, limiter.Allow(context.Background(), "/api/public/visitor/challenge"))
	require.True(t, limiter.Allow(context.Background(), "/api/public/visitor/challenge"))
	require.True(t, limiter.Allow(context.Background(), "/api/public/visitor/issue-cookie"))
}

func TestGlobalRateLimiterBypassesPublicLogoAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/api/v1/settings/logo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.HEAD("/api/v1/settings/logo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.POST("/api/v1/settings/logo", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 100; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/logo", nil, nil)
		require.Equal(t, http.StatusOK, rec.Code, "public logo GET %d should bypass anonymous limiter", i+1)
	}
	for i := 0; i < 100; i++ {
		rec := performDefenseRequest(router, http.MethodHead, "/api/v1/settings/logo", nil, nil)
		require.Equal(t, http.StatusOK, rec.Code, "public logo HEAD %d should bypass anonymous limiter", i+1)
	}

	for i := 0; i < 30; i++ {
		rec := performDefenseRequest(router, http.MethodPost, "/api/v1/settings/logo", nil, nil)
		require.Equal(t, http.StatusOK, rec.Code, "public logo POST %d should still use anonymous limiter", i+1)
	}
	rec := performDefenseRequest(router, http.MethodPost, "/api/v1/settings/logo", nil, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestGlobalRateLimiterUsesGlobalOnlyLimitForFrontendDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "40")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/model-marketplace", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 40; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/model-marketplace", nil, nil)
		require.Equal(t, http.StatusOK, rec.Code, "SPA request %d should be under global frontend limit", i+1)
	}
	rec := performDefenseRequest(router, http.MethodGet, "/model-marketplace", nil, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "rate limited (frontend global)")
}

func TestGlobalRateLimiterFrontendDocumentGlobalFailOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "1")

	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/model-marketplace", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/model-marketplace", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestGlobalRateLimiterFailOpenOnRedisError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/api/v1/model-marketplace", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/api/v1/model-marketplace", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestGlobalRateLimiterProtectsFrontendDocumentsWhenSPAProtectDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("SPA_PROTECT_ENABLED", "false")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "30")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/model-marketplace", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 30; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/model-marketplace", nil, nil)
		require.Equal(t, http.StatusOK, rec.Code, "SPA request %d should remain under fallback global limit", i+1)
	}
	rec := performDefenseRequest(router, http.MethodGet, "/model-marketplace", nil, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestBodyFingerprintBlocksAnonymousReplayAndSkipsAPIPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_BODY_FINGERPRINT_ENABLED", "true")
	t.Setenv("DEFENSE_DYNAMIC_BAN_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewBodyFingerprint(rdb).Middleware())
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})
	router.POST("/v1/messages", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	body := []byte(`{"email":"123123123123@qq.com","password":"123123123123"}`)
	for i := 0; i < 50; i++ {
		rec := performDefenseRequest(router, http.MethodPost, "/api/v1/auth/login", body, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code, "anonymous repeated body %d should pass until threshold", i+1)
	}
	rec := performDefenseRequest(router, http.MethodPost, "/api/v1/auth/login", body, nil)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "abnormal pattern")

	for i := 0; i < 55; i++ {
		rec := performDefenseRequest(router, http.MethodPost, "/v1/messages", body, nil)
		require.Equal(t, http.StatusOK, rec.Code, "/v1 API path should skip body fingerprint")
	}

	rec = performDefenseRequest(router, http.MethodPost, "/api/v1/auth/login", body, func(req *http.Request) {
		req.Header.Set("x-api-key", "sk-fake")
	})
	require.Equal(t, http.StatusForbidden, rec.Code, "fake API key on login path must not bypass body fingerprint")
}

func TestBanCheckIsDisabledForCDNFriendlyDefense(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_DYNAMIC_BAN_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewBanCheck(rdb))
	router.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/v1/messages", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, func(req *http.Request) {
		req.Header.Set("x-api-key", "sk-fake")
	})
	require.Equal(t, http.StatusOK, rec.Code)

	rec = performDefenseRequest(router, http.MethodGet, "/v1/messages", nil, func(req *http.Request) {
		req.Header.Set("x-api-key", "sk-legit-valid-candidate-0000")
	})
	require.Equal(t, http.StatusOK, rec.Code)
}

func newDefenseTestRedis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	return rdb, func() {
		_ = rdb.Close()
		s.Close()
	}
}

func performDefenseRequest(router *gin.Engine, method, path string, body []byte, set func(*http.Request)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("User-Agent", "Mozilla/5.0 defense-test")
	req.Header.Set("Accept-Language", "en-US")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if set != nil {
		set(req)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
