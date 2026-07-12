//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAccountBridgeAuthHandler(t *testing.T, password string, requires2FA bool) *AuthHandler {
	t.Helper()
	user := &service.User{
		ID:          7102,
		Email:       "bridge-user@example.com",
		Username:    "bridge-user",
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		TotpEnabled: requires2FA,
	}
	require.NoError(t, user.SetPassword(password))
	repo := &userHandlerRepoStub{user: user}
	authService := service.NewAuthService(nil, repo, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil)
	return &AuthHandler{authService: authService}
}

func performAccountBridgeVerifyRequest(t *testing.T, handler *AuthHandler, payload string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/account-bridge/verify-password", bytes.NewBufferString(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	handler.VerifyPasswordForAccountBridge(c)
	return recorder
}

func TestVerifyPasswordForAccountBridgeReturnsMinimalIdentityWithoutSession(t *testing.T) {
	handler := newAccountBridgeAuthHandler(t, "CorrectPass123", true)
	recorder := performAccountBridgeVerifyRequest(t, handler, `{"email":"BRIDGE-USER@example.com","password":"CorrectPass123"}`)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Code int `json:"code"`
		Data struct {
			ID          int64  `json:"id"`
			Email       string `json:"email"`
			Username    string `json:"username"`
			Status      string `json:"status"`
			Requires2FA bool   `json:"requires_2fa"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Zero(t, response.Code)
	require.Equal(t, int64(7102), response.Data.ID)
	require.Equal(t, "bridge-user@example.com", response.Data.Email)
	require.Equal(t, "bridge-user", response.Data.Username)
	require.Equal(t, service.StatusActive, response.Data.Status)
	require.True(t, response.Data.Requires2FA)
}

func TestVerifyPasswordForAccountBridgeRejectsWrongPassword(t *testing.T) {
	handler := newAccountBridgeAuthHandler(t, "CorrectPass123", false)
	recorder := performAccountBridgeVerifyRequest(t, handler, `{"email":"bridge-user@example.com","password":"WrongPass123"}`)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "INVALID_CREDENTIALS", response.Reason)
}

func TestVerifyPasswordForAccountBridgeRejectsInvalidPayload(t *testing.T) {
	handler := newAccountBridgeAuthHandler(t, "CorrectPass123", false)
	recorder := performAccountBridgeVerifyRequest(t, handler, `{"email":"not-an-email","password":""}`)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
