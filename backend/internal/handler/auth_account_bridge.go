package handler

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/security"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

type shopAccountBridgeLookupRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type shopAccountBridgeCreateRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Username string `json:"username" binding:"omitempty,max=100"`
}

type shopAccountBridgeVerifyPasswordRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type shopAccountBridgeIdentityResponse struct {
	ID                int64  `json:"id"`
	Email             string `json:"email"`
	Username          string `json:"username"`
	Status            string `json:"status"`
	Requires2FA       bool   `json:"requires_2fa"`
	CredentialVersion uint64 `json:"credential_version"`
}

func bindShopAccountBridgeJSON(c *gin.Context, dst any) bool {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return false
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		response.BadRequest(c, "Invalid request")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		response.BadRequest(c, "Invalid request")
		return false
	}
	if err := binding.Validator.ValidateStruct(dst); err != nil {
		response.BadRequest(c, "Invalid request")
		return false
	}
	return true
}

func respondShopAccountBridgeIdentity(c *gin.Context, user *service.User) {
	if user == nil || user.ID <= 0 || user.CredentialVersion == 0 {
		response.InternalError(c, "Account bridge identity unavailable")
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, shopAccountBridgeIdentityResponse{
		ID:                user.ID,
		Email:             user.Email,
		Username:          user.Username,
		Status:            user.Status,
		Requires2FA:       user.TotpEnabled,
		CredentialVersion: user.CredentialVersion,
	})
}

// LookupShopAccountBridgeUser performs an exact email lookup. Email is sent in
// the authenticated JSON body, never in a URL or access-log query string.
func (h *AuthHandler) LookupShopAccountBridgeUser(c *gin.Context) {
	var req shopAccountBridgeLookupRequest
	if !bindShopAccountBridgeJSON(c, &req) {
		return
	}
	if h == nil || h.authService == nil {
		response.InternalError(c, "Account bridge unavailable")
		return
	}
	user, err := h.authService.LookupShopAccountBridgeUser(c.Request.Context(), security.NormalizeEmail(req.Email))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	respondShopAccountBridgeIdentity(c, user)
}

// CreateShopAccountBridgeUser creates only an ordinary user. The request DTO
// has no role, balance, group, quota, status, or admin-capability fields, and
// unknown fields are rejected.
func (h *AuthHandler) CreateShopAccountBridgeUser(c *gin.Context) {
	var req shopAccountBridgeCreateRequest
	if !bindShopAccountBridgeJSON(c, &req) {
		return
	}
	if len([]byte(req.Password)) > 72 {
		response.BadRequest(c, "Invalid request")
		return
	}
	if h == nil || h.authService == nil {
		response.InternalError(c, "Account bridge unavailable")
		return
	}
	user, err := h.authService.CreateShopAccountBridgeUser(
		c.Request.Context(),
		security.NormalizeEmail(req.Email),
		req.Password,
		strings.TrimSpace(req.Username),
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	respondShopAccountBridgeIdentity(c, user)
}

// VerifyPasswordForShopAccountBridge validates both stored password hashes
// without creating a login session, token, refresh token, or cookie.
func (h *AuthHandler) VerifyPasswordForShopAccountBridge(c *gin.Context) {
	var req shopAccountBridgeVerifyPasswordRequest
	if !bindShopAccountBridgeJSON(c, &req) {
		return
	}
	if len([]byte(req.Password)) > 72 {
		response.ErrorFrom(c, service.ErrInvalidCredentials)
		return
	}
	if h == nil || h.authService == nil {
		response.InternalError(c, "Account bridge unavailable")
		return
	}
	user, err := h.authService.VerifyShopAccountBridgePassword(
		c.Request.Context(),
		security.NormalizeEmail(req.Email),
		req.Password,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	respondShopAccountBridgeIdentity(c, user)
}
