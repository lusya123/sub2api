package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type chatAuthServiceStub struct{}

func (chatAuthServiceStub) GenerateToken(context.Context, *service.User) (string, error) {
	return "test-token", nil
}

func (chatAuthServiceStub) GetAccessTokenExpiresIn() int { return 3600 }

type chatUserServiceStub struct{}

func (chatUserServiceStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Email: "user@example.com", Role: service.RoleUser, Status: service.StatusActive}, nil
}

func TestCreateChatLaunchWithoutConfigurationReturnsServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ChatHandler{
		cfg:         &config.Config{},
		authSvc:     chatAuthServiceStub{},
		userService: chatUserServiceStub{},
	}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 7})
		c.Next()
	})
	router.POST("/api/v1/chat/launch", h.CreateLaunch)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/launch", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	var got response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "CHAT_UNAVAILABLE", got.Reason)
	require.NotContains(t, got.Message, "lobe.chat_url")
}

func TestChatSignInURLUsesConfiguredChatURL(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.Lobe.ChatURL = "https://chat.example.com/"

	got, err := h.chatSignInURL(context.Background())
	if err != nil {
		t.Fatalf("chatSignInURL returned error: %v", err)
	}
	want := "https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox&source=sub2api"
	if got != want {
		t.Fatalf("chatSignInURL = %q, want %q", got, want)
	}
}

func TestChatSignInURLUsesSettingsChatURLBeforeConfig(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.Lobe.ChatURL = "https://configured.example.com"

	got, err := h.chatSignInURL(context.Background(), "https://settings.example.com/")
	if err != nil {
		t.Fatalf("chatSignInURL returned error: %v", err)
	}
	want := "https://settings.example.com/signin?callbackUrl=%2Fagent%2Finbox&source=sub2api"
	if got != want {
		t.Fatalf("chatSignInURL = %q, want %q", got, want)
	}
}

func TestChatSignInURLFallsBackToOIDCRedirectOrigin(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.OIDCIssuer.RedirectURIs = []string{"https://chat.example.com/api/auth/callback/generic-oidc"}

	got, err := h.chatSignInURL(context.Background())
	if err != nil {
		t.Fatalf("chatSignInURL returned error: %v", err)
	}
	want := "https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox&source=sub2api"
	if got != want {
		t.Fatalf("chatSignInURL = %q, want %q", got, want)
	}
}

func TestChatSignInURLFallsBackToChatSubdomain(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.Server.FrontendURL = "https://app.example.com"

	got, err := h.chatSignInURL(context.Background())
	if err != nil {
		t.Fatalf("chatSignInURL returned error: %v", err)
	}
	want := "https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox&source=sub2api"
	if got != want {
		t.Fatalf("chatSignInURL = %q, want %q", got, want)
	}
}

func TestChatSignInURLWithPreferenceUsesTopLevelCallback(t *testing.T) {
	h := &ChatHandler{cfg: &config.Config{}}
	h.cfg.Lobe.ChatURL = "https://chat.example.com"

	got, err := h.chatSignInURLWithPreference(context.Background(), &chatLaunchPreference{
		Provider: "sub2api-group-7",
		Model:    "gpt-5.4-mini",
	})
	if err != nil {
		t.Fatalf("chatSignInURLWithPreference returned error: %v", err)
	}

	signInURL, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse sign-in URL: %v", err)
	}
	if embed := signInURL.Query().Get("embed"); embed != "" {
		t.Fatalf("sign-in URL unexpectedly enables embed mode: %q", embed)
	}
	if source := signInURL.Query().Get("source"); source != "sub2api" {
		t.Fatalf("sign-in source = %q, want %q", source, "sub2api")
	}

	callbackURL, err := url.Parse(signInURL.Query().Get("callbackUrl"))
	if err != nil {
		t.Fatalf("parse callback URL: %v", err)
	}
	if callbackURL.Path != "/agent/inbox" {
		t.Fatalf("callback path = %q, want %q", callbackURL.Path, "/agent/inbox")
	}
	if embed := callbackURL.Query().Get("embed"); embed != "" {
		t.Fatalf("callback URL unexpectedly enables embed mode: %q", embed)
	}
	if provider := callbackURL.Query().Get("provider"); provider != "sub2api-group-7" {
		t.Fatalf("callback provider = %q, want %q", provider, "sub2api-group-7")
	}
	if model := callbackURL.Query().Get("modelId"); model != "gpt-5.4-mini" {
		t.Fatalf("callback model = %q, want %q", model, "gpt-5.4-mini")
	}
}

func TestSyncLobeUserConfig(t *testing.T) {
	t.Parallel()

	var gotAuthorization string
	var gotBody map[string]string
	client := chatHTTPClientFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/api/internal/sync-user" {
			t.Fatalf("path = %q, want /api/internal/sync-user", r.URL.Path)
		}
		if r.URL.Scheme != "https" || r.URL.Host != "chat.example.com" || r.URL.RawQuery != "" {
			t.Fatalf("sync URL = %q, want https://chat.example.com/api/internal/sync-user", r.URL.String())
		}
		gotAuthorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		return &http.Response{
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			StatusCode: http.StatusOK,
		}, nil
	})

	h := &ChatHandler{cfg: &config.Config{}, lobeSyncClient: client}
	h.cfg.Lobe.InternalSharedSecret = "shared-secret"

	err := h.syncLobeUserConfig(context.Background(), 2412, "https://chat.example.com/signin?source=sub2api")
	if err != nil {
		t.Fatalf("syncLobeUserConfig returned error: %v", err)
	}
	if gotAuthorization != "Bearer shared-secret" {
		t.Fatalf("Authorization = %q, want Bearer shared-secret", gotAuthorization)
	}
	if gotBody["user_id"] != "2412" {
		t.Fatalf("user_id = %q, want 2412", gotBody["user_id"])
	}
}

func TestSyncLobeUserConfigDoesNotExposeResponseBody(t *testing.T) {
	t.Parallel()

	h := &ChatHandler{
		cfg: &config.Config{},
		lobeSyncClient: chatHTTPClientFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				Body:       io.NopCloser(strings.NewReader("SECRET_RESPONSE_BODY")),
				StatusCode: http.StatusBadGateway,
			}, nil
		}),
	}
	h.cfg.Lobe.InternalSharedSecret = "shared-secret"

	err := h.syncLobeUserConfig(context.Background(), 2412, "https://chat.example.com/signin")
	if err == nil {
		t.Fatal("syncLobeUserConfig returned nil error")
	}
	if strings.Contains(err.Error(), "SECRET_RESPONSE_BODY") {
		t.Fatalf("error exposes response body: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("error = %q, want HTTP 502", err.Error())
	}
}

type chatHTTPClientFunc func(req *http.Request) (*http.Response, error)

func (f chatHTTPClientFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSyncLobeUserConfigSkipsWhenSecretIsMissing(t *testing.T) {
	t.Parallel()

	called := false
	h := &ChatHandler{
		cfg: &config.Config{},
		lobeSyncClient: chatHTTPClientFunc(func(*http.Request) (*http.Response, error) {
			called = true
			return nil, nil
		}),
	}

	err := h.syncLobeUserConfig(context.Background(), 2412, "https://chat.example.com/signin")
	if err != nil {
		t.Fatalf("syncLobeUserConfig returned error: %v", err)
	}
	if called {
		t.Fatal("HTTP client was called without a shared secret")
	}
}
