package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

type accountBridgeVerifyPasswordRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type accountBridgeVerifyPasswordResponse struct {
	ID          int64  `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	Status      string `json:"status"`
	Requires2FA bool   `json:"requires_2fa"`
}

// VerifyPasswordForAccountBridge verifies credentials without creating a
// login session or triggering any cross-system password synchronization.
func (h *AuthHandler) VerifyPasswordForAccountBridge(c *gin.Context) {
	var req accountBridgeVerifyPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h == nil || h.authService == nil {
		response.InternalError(c, "Authentication service unavailable")
		return
	}

	user, err := h.authService.ValidatePasswordCredentials(
		c.Request.Context(),
		strings.TrimSpace(req.Email),
		req.Password,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, accountBridgeVerifyPasswordResponse{
		ID:          user.ID,
		Email:       user.Email,
		Username:    user.Username,
		Status:      user.Status,
		Requires2FA: user.TotpEnabled,
	})
}
