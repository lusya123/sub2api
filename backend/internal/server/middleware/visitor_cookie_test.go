package middleware

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVisitorCookieIssuerDoesNotIssueWithoutPoW(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	router := gin.New()
	router.Use(VisitorCookieIssuerMiddleware(mgr))
	router.GET("/dashboard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/api/v1/settings/public", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Result().Cookies())

	rec = performDefenseRequest(router, http.MethodGet, "/api/v1/settings/public", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}

func TestVisitorCookieIssuerDoesNotReplaceValidCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	value, _ := mgr.IssueCookie()
	router := gin.New()
	router.Use(VisitorCookieIssuerMiddleware(mgr))
	router.GET("/dashboard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: value})
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}

func TestVisitorCookieIssuerGeneratesUniqueCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	value1, _ := mgr.IssueCookie()
	value2, _ := mgr.IssueCookie()

	require.NotEqual(t, value1, value2)
	require.True(t, mgr.VerifyCookie(value1))
	require.True(t, mgr.VerifyCookie(value2))
}

func TestVisitorCookieFingerprintBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_VISITOR_COOKIE_BIND_FINGERPRINT", "true")

	mgr := NewVisitorCookieManagerWithSecret(nil, "visitor-test-secret")
	value, _ := mgr.IssueCookieWithFingerprint("fingerprint-a")

	require.True(t, mgr.VerifyCookieWithFingerprint(value, "fingerprint-a"))
	require.False(t, mgr.VerifyCookieWithFingerprint(value, "fingerprint-b"))
	require.False(t, mgr.VerifyCookieWithFingerprint(value, ""))
}

func TestVisitorCookieChallengeConsumeOnce(t *testing.T) {
	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	challenge := strconv.FormatInt(time.Now().UnixNano(), 36) + ".abcdef"

	mgr.StoreChallenge(context.Background(), challenge, time.Minute)
	consumed, err := mgr.ConsumeChallenge(context.Background(), challenge)
	require.NoError(t, err)
	require.True(t, consumed)

	consumed, err = mgr.ConsumeChallenge(context.Background(), challenge)
	require.NoError(t, err)
	require.False(t, consumed)
}

func TestGlobalRateLimiterVisitorCookieBypassesFrontendGlobalAndLimitsReuse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "1")
	t.Setenv("DEFENSE_VISITOR_COOKIE_PER_MINUTE", "2")
	t.Setenv("DEFENSE_VISITOR_COOKIE_GLOBAL_PER_2S", "0")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	value, _ := mgr.IssueCookie()
	router := gin.New()
	router.Use(NewGlobalRateLimiter(rdb, mgr).Middleware())
	router.GET("/dashboard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, func(req *http.Request) {
			req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: value})
		})
		require.Equal(t, http.StatusOK, rec.Code, "valid visitor cookie request %d should bypass frontend global", i+1)
	}

	rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: value})
	})
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "rate limited (visitor cookie)")
}

func TestGlobalRateLimiterVisitorCookieGlobalLimitAcrossCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "1")
	t.Setenv("DEFENSE_VISITOR_COOKIE_PER_MINUTE", "100")
	t.Setenv("DEFENSE_VISITOR_COOKIE_GLOBAL_PER_2S", "2")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	router := gin.New()
	router.Use(NewGlobalRateLimiter(rdb, mgr).Middleware())
	router.GET("/dashboard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for i := 0; i < 2; i++ {
		value, _ := mgr.IssueCookie()
		rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, func(req *http.Request) {
			req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: value})
		})
		require.Equal(t, http.StatusOK, rec.Code, "valid visitor cookie request %d should be under global visitor limit", i+1)
	}

	value, _ := mgr.IssueCookie()
	rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: value})
	})
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "rate limited (visitor cookie global)")
}

func TestGlobalRateLimiterForgedVisitorCookieFallsBackToFrontendGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "1")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	router := gin.New()
	router.Use(NewGlobalRateLimiter(rdb, mgr).Middleware())
	router.GET("/dashboard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	forged := strconv.FormatInt(time.Now().Unix(), 10) + ".fake_signature"
	rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: forged})
	})
	require.Equal(t, http.StatusOK, rec.Code)

	rec = performDefenseRequest(router, http.MethodGet, "/dashboard", nil, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: forged})
	})
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "rate limited (frontend global)")
}

func TestPoWVisitorCookieLetsBlockedFirstVisitRefreshThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "1")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	router := gin.New()
	router.Use(VisitorCookieIssuerMiddleware(mgr))
	router.Use(NewGlobalRateLimiter(rdb, mgr).Middleware())
	router.GET("/dashboard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Result().Cookies())

	rec = performDefenseRequest(router, http.MethodGet, "/dashboard", nil, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	value, _ := mgr.IssueCookieWithFingerprint("test-fingerprint")
	rec = performDefenseRequest(router, http.MethodGet, "/dashboard", nil, func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: visitorCookieName, Value: value})
		req.AddCookie(&http.Cookie{Name: visitorFingerprintName, Value: "test-fingerprint"})
	})
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestVisitorCookieRejectsExpiredCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_VISITOR_COOKIE_TTL_SECONDS", "60")

	mgr := NewVisitorCookieManagerWithSecret(nil, "visitor-test-secret")
	oldTS := strconv.FormatInt(time.Now().Add(-2*time.Minute).Unix(), 10)
	expired := oldTS + "." + mgr.sign(oldTS)

	require.False(t, mgr.VerifyCookie(expired))
}
