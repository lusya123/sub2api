package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestOIDCAuthorizeRejectsInvalidRedirectURIWithoutRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{}
	cfg.OIDCServer.Issuer = "https://app.example.com"
	cfg.OIDCServer.ClientID = "lobe"
	cfg.OIDCServer.ClientSecret = "secret"
	cfg.OIDCServer.RedirectURIs = []string{"https://chat.example.com/api/auth/callback/generic-oidc"}

	oidcService, err := service.NewOIDCService(cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewOIDCService returned error: %v", err)
	}
	h := NewOIDCHandler(oidcService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/oauth2/authorize?response_type=code&client_id=lobe&redirect_uri=https%3A%2F%2Fevil.example%2Fcallback&scope=openid&state=abc",
		nil,
	)

	h.Authorize(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if location := w.Header().Get("Location"); location != "" {
		t.Fatalf("Location header = %q, want empty", location)
	}
}
