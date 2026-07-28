package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyHandlerRejectsNegativeLimitsAndInvalidExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	h := NewAPIKeyHandler(nil)
	router.POST("/api/v1/api-keys", h.Create)
	router.PUT("/api/v1/api-keys/:id", h.Update)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create negative quota", method: http.MethodPost, path: "/api/v1/api-keys", body: `{"name":"test","quota":-1}`},
		{name: "create negative 5h", method: http.MethodPost, path: "/api/v1/api-keys", body: `{"name":"test","rate_limit_5h":-1}`},
		{name: "create negative 1d", method: http.MethodPost, path: "/api/v1/api-keys", body: `{"name":"test","rate_limit_1d":-1}`},
		{name: "create negative 7d", method: http.MethodPost, path: "/api/v1/api-keys", body: `{"name":"test","rate_limit_7d":-1}`},
		{name: "create zero expiry", method: http.MethodPost, path: "/api/v1/api-keys", body: `{"name":"test","expires_in_days":0}`},
		{name: "create negative expiry", method: http.MethodPost, path: "/api/v1/api-keys", body: `{"name":"test","expires_in_days":-1}`},
		{name: "create excessive expiry", method: http.MethodPost, path: "/api/v1/api-keys", body: `{"name":"test","expires_in_days":3651}`},
		{name: "update negative quota", method: http.MethodPut, path: "/api/v1/api-keys/11", body: `{"quota":-1}`},
		{name: "update negative 5h", method: http.MethodPut, path: "/api/v1/api-keys/11", body: `{"rate_limit_5h":-1}`},
		{name: "update negative 1d", method: http.MethodPut, path: "/api/v1/api-keys/11", body: `{"rate_limit_1d":-1}`},
		{name: "update negative 7d", method: http.MethodPut, path: "/api/v1/api-keys/11", body: `{"rate_limit_7d":-1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rec, req)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "Invalid request")
		})
	}
}

func TestNormalizeAPIKeySearchTrimsByRunes(t *testing.T) {
	input := "  " + strings.Repeat("界", 101) + "  "
	got := normalizeAPIKeySearch(input)
	require.True(t, utf8.ValidString(got))
	require.Equal(t, 100, utf8.RuneCountInString(got))
	require.Equal(t, strings.Repeat("界", 100), got)
}
