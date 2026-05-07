package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ChatHandler handles browser entry points for the hosted chat experience.
type ChatHandler struct {
	cfg         *config.Config
	authSvc     *service.AuthService
	userService *service.UserService
}

type ChatLaunchResponse struct {
	URL string `json:"url"`
}

func NewChatHandler(cfg *config.Config, authService *service.AuthService, userService *service.UserService) *ChatHandler {
	return &ChatHandler{
		cfg:         cfg,
		authSvc:     authService,
		userService: userService,
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

	token, err := h.authSvc.GenerateToken(user)
	if err != nil {
		response.InternalError(c, "Failed to create chat session")
		return "", false
	}
	h.setOIDCSessionCookie(c, token, h.authSvc.GetAccessTokenExpiresIn())

	target, err := h.chatSignInURL()
	if err != nil {
		response.InternalError(c, err.Error())
		return "", false
	}

	return target, true
}

func (h *ChatHandler) chatSignInURL() (string, error) {
	base := strings.TrimRight(strings.TrimSpace(h.cfg.Lobe.ChatURL), "/")
	if base == "" {
		base = lobeOriginFromRedirectURIs(h.cfg.OIDC.RedirectURIs)
	}
	if base == "" {
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
	q.Set("callbackUrl", "/agent/inbox")
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
	name := strings.TrimSpace(h.cfg.OIDC.CookieName)
	if name == "" {
		name = "sub2api_access_token"
	}
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    token,
		Path:     "/",
		Domain:   strings.TrimSpace(h.cfg.OIDC.CookieDomain),
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}
