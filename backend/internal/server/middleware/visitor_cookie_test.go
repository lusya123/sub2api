package middleware

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVisitorCookieIssuerSetsCookieOnlyForFrontendDocuments(t *testing.T) {
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
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, visitorCookieName, cookies[0].Name)
	require.True(t, cookies[0].HttpOnly)
	require.True(t, cookies[0].Secure)
	require.Equal(t, http.SameSiteLaxMode, cookies[0].SameSite)
	require.True(t, mgr.VerifyCookie(cookies[0].Value))

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

func TestGlobalRateLimiterVisitorCookieBypassesFrontendGlobalAndLimitsReuse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "1")
	t.Setenv("DEFENSE_VISITOR_COOKIE_PER_MINUTE", "2")

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

func TestVisitorCookieIssuerAfterGlobalDoesNotCookieBlockedRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_GLOBAL_RATELIMIT_ENABLED", "true")
	t.Setenv("DEFENSE_VISITOR_COOKIE_ENABLED", "true")
	t.Setenv("DEFENSE_FRONTEND_DOC_GLOBAL_PER_2S", "1")

	rdb, cleanup := newDefenseTestRedis(t)
	defer cleanup()

	mgr := NewVisitorCookieManagerWithSecret(rdb, "visitor-test-secret")
	router := gin.New()
	router.Use(NewGlobalRateLimiter(rdb, mgr).Middleware())
	router.Use(VisitorCookieIssuerMiddleware(mgr))
	router.GET("/dashboard", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := performDefenseRequest(router, http.MethodGet, "/dashboard", nil, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEmpty(t, rec.Result().Cookies())

	rec = performDefenseRequest(router, http.MethodGet, "/dashboard", nil, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Empty(t, rec.Result().Cookies())
}

func TestVisitorCookieRejectsExpiredCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("DEFENSE_VISITOR_COOKIE_TTL_SECONDS", "60")

	mgr := NewVisitorCookieManagerWithSecret(nil, "visitor-test-secret")
	oldTS := strconv.FormatInt(time.Now().Add(-2*time.Minute).Unix(), 10)
	expired := oldTS + "." + mgr.sign(oldTS)

	require.False(t, mgr.VerifyCookie(expired))
}
