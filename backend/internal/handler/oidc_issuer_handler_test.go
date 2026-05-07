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
	cfg.OIDCIssuer.Issuer = "https://app.example.com"
	cfg.OIDCIssuer.ClientID = "lobe"
	cfg.OIDCIssuer.ClientSecret = "secret"
	cfg.OIDCIssuer.RedirectURIs = []string{"https://chat.example.com/api/auth/callback/generic-oidc"}

	oidcIssuerService, err := service.NewOIDCIssuerService(cfg, nil, nil)
	if err != nil {
		t.Fatalf("NewOIDCIssuerService returned error: %v", err)
	}
	h := NewOIDCIssuerHandler(oidcIssuerService)

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
