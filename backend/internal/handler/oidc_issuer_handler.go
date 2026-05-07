package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OIDCIssuerHandler serves Sub2API's OIDC Provider/Issuer endpoints.
type OIDCIssuerHandler struct {
	oidcIssuerService *service.OIDCIssuerService
}

func NewOIDCIssuerHandler(oidcIssuerService *service.OIDCIssuerService) *OIDCIssuerHandler {
	return &OIDCIssuerHandler{oidcIssuerService: oidcIssuerService}
}

func (h *OIDCIssuerHandler) Discovery(c *gin.Context) {
	if h.oidcIssuerService == nil || !h.oidcIssuerService.IsConfigured() {
		response.ErrorFrom(c, service.ErrOIDCIssuerNotConfigured)
		return
	}
	issuer := h.oidcIssuerService.Issuer()
	c.JSON(http.StatusOK, gin.H{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/oauth2/authorize",
		"token_endpoint":                        issuer + "/oauth2/token",
		"userinfo_endpoint":                     issuer + "/oauth2/userinfo",
		"jwks_uri":                              issuer + "/.well-known/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "email", "profile", "sub2api:chat"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"claims_supported":                      []string{"sub", "email", "email_verified", "name", "preferred_username"},
	})
}

func (h *OIDCIssuerHandler) JWKS(c *gin.Context) {
	if h.oidcIssuerService == nil || !h.oidcIssuerService.IsConfigured() {
		response.ErrorFrom(c, service.ErrOIDCIssuerNotConfigured)
		return
	}
	c.JSON(http.StatusOK, gin.H{"keys": []map[string]any{h.oidcIssuerService.JWK()}})
}

func (h *OIDCIssuerHandler) Authorize(c *gin.Context) {
	if h.oidcIssuerService == nil || !h.oidcIssuerService.IsConfigured() {
		response.ErrorFrom(c, service.ErrOIDCIssuerNotConfigured)
		return
	}

	clientID := strings.TrimSpace(c.Query("client_id"))
	redirectURI := strings.TrimSpace(c.Query("redirect_uri"))
	scope := strings.TrimSpace(c.Query("scope"))
	state := c.Query("state")
	nonce := c.Query("nonce")
	if !h.oidcIssuerService.ValidateRedirectURI(redirectURI) {
		response.BadRequest(c, "invalid_redirect_uri")
		return
	}
	if c.Query("response_type") != "code" || clientID != h.oidcIssuerService.ClientID() || !scopeContains(scope, "openid") {
		redirectOIDCError(c, redirectURI, state, "invalid_request")
		return
	}

	cookie, err := c.Cookie(h.oidcIssuerService.CookieName())
	if err != nil || strings.TrimSpace(cookie) == "" {
		redirectToLogin(c)
		return
	}
	user, err := h.oidcIssuerService.UserFromAccessToken(c.Request.Context(), cookie)
	if err != nil {
		redirectToLogin(c)
		return
	}

	code, err := h.oidcIssuerService.CreateCode(user.ID, clientID, redirectURI, scope, nonce)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	values := url.Values{"code": []string{code}}
	if state != "" {
		values.Set("state", state)
	}
	c.Redirect(http.StatusFound, service.BuildRedirectWithParams(redirectURI, values))
}

func (h *OIDCIssuerHandler) Token(c *gin.Context) {
	if h.oidcIssuerService == nil || !h.oidcIssuerService.IsConfigured() {
		response.ErrorFrom(c, service.ErrOIDCIssuerNotConfigured)
		return
	}
	if err := c.Request.ParseForm(); err != nil {
		response.BadRequest(c, "Invalid form")
		return
	}
	if c.PostForm("grant_type") != "authorization_code" {
		response.BadRequest(c, "Unsupported grant_type")
		return
	}

	clientID, clientSecret := clientCredentials(c)
	result, err := h.oidcIssuerService.ExchangeCode(
		c.Request.Context(),
		c.PostForm("code"),
		clientID,
		clientSecret,
		c.PostForm("redirect_uri"),
	)
	if err != nil {
		if service.IsOIDCIssuerInvalidAuth(err) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_grant"})
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": result.AccessToken,
		"id_token":     result.IDToken,
		"expires_in":   result.ExpiresIn,
		"scope":        "openid email profile sub2api:chat",
		"token_type":   "Bearer",
	})
}

func (h *OIDCIssuerHandler) UserInfo(c *gin.Context) {
	token := bearerToken(c.GetHeader("Authorization"))
	if token == "" {
		response.Unauthorized(c, "missing bearer token")
		return
	}
	user, err := h.oidcIssuerService.UserFromAccessToken(c.Request.Context(), token)
	if err != nil {
		response.Unauthorized(c, "invalid bearer token")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"sub":                strconvFormatInt(user.ID),
		"email":              user.Email,
		"email_verified":     true,
		"name":               userDisplayName(user),
		"preferred_username": user.Username,
	})
}

func clientCredentials(c *gin.Context) (string, string) {
	if id, secret, ok := c.Request.BasicAuth(); ok {
		return id, secret
	}
	return c.PostForm("client_id"), c.PostForm("client_secret")
}

func bearerToken(header string) string {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func scopeContains(scope string, needle string) bool {
	for _, item := range strings.Fields(scope) {
		if item == needle {
			return true
		}
	}
	return false
}

func redirectOIDCError(c *gin.Context, redirectURI, state, code string) {
	if redirectURI == "" {
		response.BadRequest(c, code)
		return
	}
	values := url.Values{"error": []string{code}}
	if state != "" {
		values.Set("state", state)
	}
	c.Redirect(http.StatusFound, service.BuildRedirectWithParams(redirectURI, values))
}

func redirectToLogin(c *gin.Context) {
	next := c.Request.URL.RequestURI()
	c.Redirect(http.StatusFound, "/login?redirect="+url.QueryEscape(next))
}

func userDisplayName(user *service.User) string {
	if user == nil {
		return ""
	}
	if strings.TrimSpace(user.Username) != "" {
		return user.Username
	}
	if at := strings.IndexByte(user.Email, '@'); at > 0 {
		return user.Email[:at]
	}
	return user.Email
}

func strconvFormatInt(id int64) string {
	return strconv.FormatInt(id, 10)
}
