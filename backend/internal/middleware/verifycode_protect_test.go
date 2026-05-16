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

func newVerifyCodeProtectTestRouter(t *testing.T, rdb *redis.Client) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/send", VerifyCodeProtect(rdb), func(c *gin.Context) {
		if strings.Contains(c.GetHeader("X-Test-Force-Status"), "403") {
			c.JSON(http.StatusForbidden, gin.H{"error": "verification failed"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func TestVerifyCodeProtectLimitsSameTarget(t *testing.T) {
	t.Setenv("VERIFY_CODE_PROTECT_ENABLED", "true")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	router := newVerifyCodeProtectTestRouter(t, rdb)

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":"User@Example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(first, req)
	require.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":" user@example.com "}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(second, req)
	require.Equal(t, http.StatusTooManyRequests, second.Code)
	require.Contains(t, second.Body.String(), "too frequent")
}

func TestVerifyCodeProtectGlobalLimit(t *testing.T) {
	t.Setenv("VERIFY_CODE_PROTECT_ENABLED", "true")
	t.Setenv("VERIFY_CODE_PROTECT_CLIENT_MAX", "0")
	t.Setenv("VERIFY_CODE_PROTECT_GLOBAL_PER_SECOND", "1")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	router := newVerifyCodeProtectTestRouter(t, rdb)

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":"one@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(first, req)
	require.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":"two@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(second, req)
	require.Equal(t, http.StatusServiceUnavailable, second.Code)
	require.Contains(t, second.Body.String(), "service busy")
}

func TestVerifyCodeProtectLimitsClientAcrossTargets(t *testing.T) {
	t.Setenv("VERIFY_CODE_PROTECT_ENABLED", "true")
	t.Setenv("VERIFY_CODE_PROTECT_CLIENT_MAX", "2")
	t.Setenv("VERIFY_CODE_PROTECT_GLOBAL_PER_SECOND", "0")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	router := newVerifyCodeProtectTestRouter(t, rdb)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":"user`+string(rune('a'+i))+`@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "same-client")
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
	}

	blocked := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":"userc@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "same-client")
	router.ServeHTTP(blocked, req)
	require.Equal(t, http.StatusTooManyRequests, blocked.Code)
	require.Contains(t, blocked.Body.String(), "rate limit exceeded")
}

func TestVerifyCodeProtectDoesNotConsumeTargetLimitOnFailedRequest(t *testing.T) {
	t.Setenv("VERIFY_CODE_PROTECT_ENABLED", "true")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	router := newVerifyCodeProtectTestRouter(t, rdb)

	failed := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":"user@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Force-Status", "403")
	router.ServeHTTP(failed, req)
	require.Equal(t, http.StatusForbidden, failed.Code)

	success := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":"user@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(success, req)
	require.Equal(t, http.StatusOK, success.Code)
}

func TestVerifyCodeProtectCanBeDisabled(t *testing.T) {
	t.Setenv("VERIFY_CODE_PROTECT_ENABLED", "false")
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	router := newVerifyCodeProtectTestRouter(t, rdb)

	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader(`{"email":"user@example.com"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, req)
		require.Equal(t, http.StatusOK, recorder.Code)
	}
}
