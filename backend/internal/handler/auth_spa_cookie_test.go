package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSPAProtectTokenCookieLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	ctx.Request.Header.Set("X-Forwarded-Proto", "https")

	setSPAProtectTokenCookie(ctx, 123)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	require.Equal(t, spaProtectTokenCookieName, cookie.Name)
	require.Equal(t, "1", cookie.Value)
	require.Equal(t, "/", cookie.Path)
	require.Equal(t, 123, cookie.MaxAge)
	require.True(t, cookie.HttpOnly)
	require.True(t, cookie.Secure)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)

	rec = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)

	clearSPAProtectTokenCookie(ctx, false)

	cookies = rec.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie = cookies[0]
	require.Equal(t, spaProtectTokenCookieName, cookie.Name)
	require.Equal(t, "", cookie.Value)
	require.Equal(t, "/", cookie.Path)
	require.Equal(t, -1, cookie.MaxAge)
	require.True(t, cookie.HttpOnly)
	require.False(t, cookie.Secure)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
}
