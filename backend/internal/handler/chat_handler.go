package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ChatHandler handles browser entry points for the hosted chat experience.
type ChatHandler struct {
	cfg               *config.Config
	authSvc           chatAuthService
	userService       chatUserService
	settingSvc        *service.SettingService
	lobeConfigService *service.LobeConfigService
	lobeSyncClient    chatHTTPClient
}

type chatHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type chatAuthService interface {
	GenerateToken(ctx context.Context, user *service.User) (string, error)
	GetAccessTokenExpiresIn() int
}

type chatUserService interface {
	GetByID(ctx context.Context, id int64) (*service.User, error)
}

type ChatLaunchResponse struct {
	URL string `json:"url"`
}

type chatLaunchPreference struct {
	Provider string
	Model    string
}

func NewChatHandler(cfg *config.Config, authService *service.AuthService, userService *service.UserService, settingService *service.SettingService, lobeConfigService *service.LobeConfigService) *ChatHandler {
	return &ChatHandler{
		cfg:               cfg,
		authSvc:           authService,
		userService:       userService,
		settingSvc:        settingService,
		lobeConfigService: lobeConfigService,
		lobeSyncClient:    newLobeSyncHTTPClient(),
	}
}

func newLobeSyncHTTPClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 5 * time.Second,
	}
}

// CreateLaunch returns the LobeChat SSO URL for an authenticated XHR request.
// The browser follows this URL as a top-level navigation after the SSO cookie
// has been refreshed by this response.
func (h *ChatHandler) CreateLaunch(c *gin.Context) {
	target, ok := h.prepareLaunch(c)
	if !ok {
		return
	}
	response.Success(c, ChatLaunchResponse{URL: target})
}

// Launch refreshes the sub2api SSO cookie and redirects the user into LobeChat.
// LobeChat then completes OIDC silently against sub2api, so users do not need a
// separate Lobe account, password, or API key configuration.
func (h *ChatHandler) Launch(c *gin.Context) {
	target, ok := h.prepareLaunch(c)
	if !ok {
		return
	}

	c.Redirect(http.StatusFound, target)
}

func (h *ChatHandler) prepareLaunch(c *gin.Context) (string, bool) {
	chatPageEnabled, chatPageURL, err := h.chatPageSettings(c.Request.Context())
	if err != nil {
		response.InternalError(c, "Failed to load chat settings")
		return "", false
	}
	if !chatPageEnabled {
		response.Forbidden(c, "Chat page is disabled")
		return "", false
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not authenticated")
		return "", false
	}

	user, err := h.userService.GetByID(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return "", false
	}

	token, err := h.authSvc.GenerateToken(c.Request.Context(), user)
	if err != nil {
		response.InternalError(c, "Failed to create chat session")
		return "", false
	}
	h.setOIDCSessionCookie(c, token, h.authSvc.GetAccessTokenExpiresIn())

	preference, err := h.resolveLaunchPreference(c.Request.Context(), subject.UserID, c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return "", false
	}

	target, err := h.chatSignInURLWithPreference(c.Request.Context(), preference, chatPageURL)
	if err != nil {
		slog.Warn("chat_launch_unavailable", "error", err)
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("CHAT_UNAVAILABLE", "Chat service is not configured or unavailable"))
		return "", false
	}
	if err := h.syncLobeUserConfig(c.Request.Context(), subject.UserID, target); err != nil {
		slog.Warn("lobe_user_config_sync_failed", "user_id", subject.UserID, "error", err)
	}

	return target, true
}

func (h *ChatHandler) syncLobeUserConfig(ctx context.Context, userID int64, chatSignInURL string) error {
	if h == nil || h.cfg == nil || userID <= 0 {
		return nil
	}
	secret := strings.TrimSpace(h.cfg.Lobe.InternalSharedSecret)
	if secret == "" {
		return nil
	}

	lobeBaseURL := strings.TrimSpace(h.cfg.Lobe.InternalURL)
	if lobeBaseURL == "" {
		lobeBaseURL = strings.TrimSpace(chatSignInURL)
	}
	parsed, err := url.Parse(lobeBaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("invalid lobe chat URL")
	}
	syncURL := (&url.URL{
		Scheme: parsed.Scheme,
		Host:   parsed.Host,
		Path:   "/api/internal/sync-user",
	}).String()
	payload, err := json.Marshal(map[string]string{"user_id": strconvFormatInt(userID)})
	if err != nil {
		return fmt.Errorf("encode lobe user sync request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, syncURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build lobe user sync request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")

	client := h.lobeSyncClient
	if client == nil {
		client = newLobeSyncHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request lobe user sync: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("lobe user sync returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (h *ChatHandler) chatPageSettings(ctx context.Context) (bool, string, error) {
	if h == nil || h.settingSvc == nil {
		return true, "", nil
	}
	settings, err := h.settingSvc.GetPublicSettings(ctx)
	if err != nil {
		return false, "", err
	}
	return settings.ChatPageEnabled, settings.ChatPageURL, nil
}

func (h *ChatHandler) resolveLaunchPreference(ctx context.Context, userID int64, c *gin.Context) (*chatLaunchPreference, error) {
	provider := strings.TrimSpace(firstNonEmpty(c.Query("provider"), c.PostForm("provider")))
	model := strings.TrimSpace(firstNonEmpty(c.Query("model"), c.Query("model_id"), c.PostForm("model"), c.PostForm("model_id")))
	if provider == "" && model == "" {
		return nil, nil
	}
	if provider == "" || model == "" {
		return nil, fmt.Errorf("both provider and model are required")
	}

	if h == nil || h.lobeConfigService == nil {
		return &chatLaunchPreference{Provider: provider, Model: model}, nil
	}

	cfg, err := h.lobeConfigService.GetUserConfig(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, p := range cfg.Providers {
		if p.ID != provider {
			continue
		}
		for _, m := range p.Models {
			if m.ID == model {
				return &chatLaunchPreference{Provider: provider, Model: model}, nil
			}
		}
		return nil, fmt.Errorf("selected model is not available in this group")
	}
	return nil, fmt.Errorf("selected group is not available")
}

func (h *ChatHandler) chatSignInURL(ctx context.Context, configuredChatURL ...string) (string, error) {
	return h.chatSignInURLWithPreference(ctx, nil, configuredChatURL...)
}

func (h *ChatHandler) chatSignInURLWithPreference(ctx context.Context, preference *chatLaunchPreference, configuredChatURL ...string) (string, error) {
	base := ""
	if len(configuredChatURL) > 0 {
		base = strings.TrimSpace(configuredChatURL[0])
	}
	if base == "" {
		enabled, chatPageURL, err := h.chatPageSettings(ctx)
		if err != nil {
			return "", err
		}
		if !enabled {
			return "", fmt.Errorf("chat page is disabled")
		}
		base = strings.TrimSpace(chatPageURL)
	}
	if base == "" && h != nil && h.cfg != nil {
		base = strings.TrimSpace(h.cfg.Lobe.ChatURL)
	}
	base = strings.TrimRight(base, "/")
	if base == "" && h != nil && h.cfg != nil {
		base = lobeOriginFromRedirectURIs(h.cfg.OIDCIssuer.RedirectURIs)
	}
	if base == "" && h != nil && h.cfg != nil {
		base = deriveLobeOriginFromFrontend(h.cfg.Server.FrontendURL)
	}
	if base == "" {
		return "", fmt.Errorf("lobe.chat_url is not configured")
	}

	u, err := url.Parse(base + "/signin")
	if err != nil {
		return "", fmt.Errorf("invalid lobe chat url")
	}
	q := u.Query()
	q.Set("source", "sub2api")
	// Build the callback URL the user lands on after OIDC completes.
	cb := url.URL{Path: "/agent/inbox"}
	cbq := cb.Query()
	if preference != nil && preference.Provider != "" && preference.Model != "" {
		cbq.Set("provider", preference.Provider)
		cbq.Set("modelId", preference.Model)
	}
	cb.RawQuery = cbq.Encode()
	q.Set("callbackUrl", cb.String())
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func lobeOriginFromRedirectURIs(redirectURIs []string) string {
	for _, raw := range redirectURIs {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		return u.Scheme + "://" + u.Host
	}
	return ""
}

func deriveLobeOriginFromFrontend(frontendURL string) string {
	u, err := url.Parse(strings.TrimSpace(frontendURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	if strings.HasPrefix(u.Host, "app.") {
		return u.Scheme + "://chat." + strings.TrimPrefix(u.Host, "app.")
	}
	return ""
}

func (h *ChatHandler) setOIDCSessionCookie(c *gin.Context, token string, maxAge int) {
	if h == nil || h.cfg == nil || strings.TrimSpace(token) == "" {
		return
	}
	name := strings.TrimSpace(h.cfg.OIDCIssuer.CookieName)
	if name == "" {
		name = "sub2api_access_token"
	}
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		Domain:   strings.TrimSpace(h.cfg.OIDCIssuer.CookieDomain),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
