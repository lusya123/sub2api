package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.Greater(t, retryAfter, 0)
	require.LessOrEqual(t, retryAfter, 60)

	rec = performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, func(req *http.Request) {
		req.Header.Set("x-api-key", "sk-fake")
	})
	require.Equal(t, http.StatusTooManyRequests, rec.Code, "fake API key on public path must not bypass anonymous limit")
}

func TestGlobalRateLimiterRepairsMissingTTLAndReturnsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 30; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, nil)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	ctx := context.Background()
	keys, err := rdb.Keys(ctx, "rl:anon:????????????????").Result()
	require.NoError(t, err)
	require.Len(t, keys, 1)
	persisted, err := rdb.Persist(ctx, keys[0]).Result()
	require.NoError(t, err)
	require.True(t, persisted)

	rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.Greater(t, retryAfter, 0)
	require.LessOrEqual(t, retryAfter, 60)

	ttl, err := rdb.PTTL(ctx, keys[0]).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
	require.LessOrEqual(t, ttl, time.Minute)
}

func TestGlobalRateLimiterRedisFailureRemainsFailOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()
	require.NoError(t, rdb.Close())

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Retry-After"))
}

func TestDefenseRateLimitTTLDecreasesWithoutRefresh(t *testing.T) {
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = rdb.Close()
		redisServer.Close()
	})
	ctx := context.Background()

	allowed, retryAfter := allowDefenseRateLimit(ctx, rdb, "rl:test:no-refresh", 10, time.Minute)
	require.True(t, allowed)
	require.Zero(t, retryAfter)
	ttlBefore, err := rdb.PTTL(ctx, "rl:test:no-refresh").Result()
	require.NoError(t, err)
	require.Greater(t, ttlBefore, time.Duration(0))

	redisServer.FastForward(10 * time.Second)
	allowed, retryAfter = allowDefenseRateLimit(ctx, rdb, "rl:test:no-refresh", 10, time.Minute)
	require.True(t, allowed)
	require.Zero(t, retryAfter)
	ttlAfter, err := rdb.PTTL(ctx, "rl:test:no-refresh").Result()
	require.NoError(t, err)
	require.Less(t, ttlAfter, ttlBefore)
	require.LessOrEqual(t, ttlAfter, 50*time.Second)
}

func TestGlobalRateLimiterUserTierReturnsRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	router.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	setUser := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer jwt-test-token")
	}
	for i := 0; i < 600; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, setUser)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, setUser)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.Greater(t, retryAfter, 0)
	require.LessOrEqual(t, retryAfter, 60)
}

func TestDefenseRetryAfterSecondsRoundsUpAndStaysPositive(t *testing.T) {
	tests := []struct {
		name  string
		delay time.Duration
		want  int
	}{
		{name: "zero", delay: 0, want: 1},
		{name: "one millisecond", delay: time.Millisecond, want: 1},
		{name: "one second", delay: time.Second, want: 1},
		{name: "just over one second", delay: time.Second + time.Millisecond, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, defenseRetryAfterSeconds(tt.delay))
		})
	}
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

func TestGlobalRateLimiterBypassesInstallerDownloads(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewGlobalRateLimiter(rdb).Middleware())
	for _, path := range []string{
		"/install-claude-ccswitch.sh",
		"/install-codex-win-bootstrap.ps1",
		"/install-openclaw.sh",
		"/downloads/cc-switch/CC-Switch-XDT-Linux-x64.deb",
	} {
		path := path
		router.GET(path, func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
	}

	for _, path := range []string{
		"/install-claude-ccswitch.sh",
		"/install-codex-win-bootstrap.ps1",
		"/install-openclaw.sh",
		"/downloads/cc-switch/CC-Switch-XDT-Linux-x64.deb",
	} {
		for i := 0; i < 100; i++ {
			rec := performDefenseRequest(router, http.MethodGet, path, nil, nil)
			require.Equal(t, http.StatusOK, rec.Code, "%s request %d should bypass anonymous limiter", path, i+1)
		}
	}
}

func TestGlobalRateLimiterDoesNotBypassUnknownInstallerLikePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "30")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(newFixedGlobalRateLimiter(rdb).Middleware())
	router.GET("/unknown-installer.sh", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 30; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/unknown-installer.sh", nil, nil)
		require.Equal(t, http.StatusOK, rec.Code, "unknown installer-like request %d should be under frontend document limit", i+1)
	}
	rec := performDefenseRequest(router, http.MethodGet, "/unknown-installer.sh", nil, nil)
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
	router.Use(newFixedGlobalRateLimiter(rdb).Middleware())
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
	require.Equal(t, "1", rec.Header().Get("Retry-After"))
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
	router.Use(newFixedGlobalRateLimiter(rdb).Middleware())
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
	router.Use(newFixedGlobalRateLimiter(rdb).Middleware())
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

func TestGlobalRateLimiterAnonymousAggregateReturnsBoundaryRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()
	limiter := newFixedGlobalRateLimiter(rdb)
	globalKey := "rl:anon:global:" + limiter.currentTime().Format("20060102150405")
	require.NoError(t, rdb.Set(context.Background(), globalKey, 2000, 2*time.Second).Err())

	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(limiter.Middleware())
	router.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "rate limited (global)")
	require.Equal(t, "1", rec.Header().Get("Retry-After"))
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

func TestBodyFingerprintPreservesUnknownLengthBodyBeyondInspectionLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_TRUST_TIER_ENABLED", "true")
	t.Setenv("DEFENSE_BODY_FINGERPRINT_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	body := bytes.Repeat([]byte("b"), bodyFingerprintMaxBodyBytes+2048)
	router := gin.New()
	router.Use(TrustTierDetector())
	router.Use(NewBodyFingerprint(rdb).Middleware())
	router.POST("/api/v1/upload", func(c *gin.Context) {
		got, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, body, got)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", bytes.NewReader(body))
	req.ContentLength = -1 // exercise the bounded-read path used for chunked bodies
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusNoContent, recorder.Code)
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

func newFixedGlobalRateLimiter(rdb *redis.Client, visitorCookie ...*VisitorCookieManager) *GlobalRateLimiter {
	limiter := NewGlobalRateLimiter(rdb, visitorCookie...)
	limiter.now = func() time.Time {
		// 250 ms before both the next one-second and two-second bucket boundary.
		return time.Unix(1_700_000_001, 750_000_000).UTC()
	}
	return limiter
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
