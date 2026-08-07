//go:build unit

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type accountBridgeRepoStub struct {
	*userHandlerRepoStub
	created     *service.User
	createErr   error
	createCalls int
	exists      bool
}

func (s *accountBridgeRepoStub) GetByEmail(_ context.Context, email string) (*service.User, error) {
	if s.userHandlerRepoStub == nil || s.user == nil || !strings.EqualFold(strings.TrimSpace(s.user.Email), strings.TrimSpace(email)) {
		return nil, service.ErrUserNotFound
	}
	cloned := *s.user
	return &cloned, nil
}

func (s *accountBridgeRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	return s.exists, nil
}

func (s *accountBridgeRepoStub) ExistsByEmailAlias(context.Context, string) (bool, error) {
	return s.exists, nil
}

func (s *accountBridgeRepoStub) CreateWithEmailAliasGuard(_ context.Context, user *service.User) error {
	s.createCalls++
	if s.createErr != nil {
		return s.createErr
	}
	cloned := *user
	cloned.ID = 8101
	cloned.CredentialVersion = 1
	s.created = &cloned
	s.user = &cloned
	*user = cloned
	return nil
}

func newShopAccountBridgeAuthHandler(repo service.UserRepository) *AuthHandler {
	authService := service.NewAuthService(nil, repo, nil, nil, &config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil)
	return &AuthHandler{authService: authService}
}

func performShopAccountBridgeRequest(t *testing.T, handler gin.HandlerFunc, path, payload string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(payload))
	c.Request.Header.Set("Content-Type", "application/json")
	handler(c)
	return recorder
}

func TestShopAccountBridgeVerifySupportsBothPasswordsWithoutSession(t *testing.T) {
	primary, err := bcrypt.GenerateFromPassword([]byte("MainPass123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	legacy, err := bcrypt.GenerateFromPassword([]byte("ShopPass123"), bcrypt.DefaultCost)
	require.NoError(t, err)
	legacyHash := string(legacy)
	repo := &accountBridgeRepoStub{userHandlerRepoStub: &userHandlerRepoStub{user: &service.User{
		ID:                     7102,
		Email:                  "bridge-user@example.com",
		Username:               "bridge-user",
		PasswordHash:           string(primary),
		LegacyShopPasswordHash: &legacyHash,
		Role:                   service.RoleUser,
		Status:                 service.StatusActive,
		CredentialVersion:      7,
	}}}
	handler := newShopAccountBridgeAuthHandler(repo)

	for _, password := range []string{"MainPass123", "ShopPass123"} {
		recorder := performShopAccountBridgeRequest(t, handler.VerifyPasswordForShopAccountBridge,
			"/api/v1/integrations/shop/account-bridge/verify-password",
			`{"email":"BRIDGE-USER@example.com","password":"`+password+`"}`,
		)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Empty(t, recorder.Header().Get("Set-Cookie"))
		require.Empty(t, recorder.Header().Get("Authorization"))
		var envelope map[string]any
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
		data := envelope["data"].(map[string]any)
		require.Equal(t, float64(7102), data["id"])
		require.Equal(t, float64(7), data["credential_version"])
		for _, forbidden := range []string{"role", "balance", "password", "password_hash", "legacy_shop_password_hash", "api_keys"} {
			_, present := data[forbidden]
			require.False(t, present, "response exposed %s", forbidden)
		}
	}
}

func TestShopAccountBridgeVerifyPreservesStatusAndTOTPPolicy(t *testing.T) {
	user := &service.User{
		ID: 7201, Email: "disabled@example.com", Username: "disabled", Role: service.RoleUser,
		Status: service.StatusDisabled, TotpEnabled: true, CredentialVersion: 9,
	}
	require.NoError(t, user.SetPassword("CorrectPass123"))
	repo := &accountBridgeRepoStub{userHandlerRepoStub: &userHandlerRepoStub{user: user}}
	handler := newShopAccountBridgeAuthHandler(repo)
	recorder := performShopAccountBridgeRequest(t, handler.VerifyPasswordForShopAccountBridge,
		"/api/v1/integrations/shop/account-bridge/verify-password",
		`{"email":"disabled@example.com","password":"CorrectPass123"}`,
	)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Data struct {
			Status      string `json:"status"`
			Requires2FA bool   `json:"requires_2fa"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, service.StatusDisabled, response.Data.Status)
	require.True(t, response.Data.Requires2FA)
}

func TestShopAccountBridgeVerifyRejectsWrongPassword(t *testing.T) {
	user := &service.User{ID: 7102, Email: "bridge-user@example.com", Role: service.RoleUser, Status: service.StatusActive}
	require.NoError(t, user.SetPassword("CorrectPass123"))
	handler := newShopAccountBridgeAuthHandler(&accountBridgeRepoStub{userHandlerRepoStub: &userHandlerRepoStub{user: user}})
	recorder := performShopAccountBridgeRequest(t, handler.VerifyPasswordForShopAccountBridge,
		"/api/v1/integrations/shop/account-bridge/verify-password",
		`{"email":"bridge-user@example.com","password":"WrongPass123"}`,
	)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "INVALID_CREDENTIALS", response.Reason)
}

func TestShopAccountBridgeNeverExposesOrVerifiesPrivilegedAccounts(t *testing.T) {
	for _, role := range []string{service.RoleAdmin, service.RoleOperator, "unknown-privileged"} {
		t.Run(role, func(t *testing.T) {
			user := &service.User{
				ID: 7199, Email: "privileged@example.com", Role: role,
				Status: service.StatusActive, CredentialVersion: 3,
			}
			require.NoError(t, user.SetPassword("PrivilegedPass123"))
			handler := newShopAccountBridgeAuthHandler(&accountBridgeRepoStub{
				userHandlerRepoStub: &userHandlerRepoStub{user: user},
			})

			lookup := performShopAccountBridgeRequest(t, handler.LookupShopAccountBridgeUser,
				"/api/v1/integrations/shop/account-bridge/lookup",
				`{"email":"privileged@example.com"}`,
			)
			require.Equal(t, http.StatusNotFound, lookup.Code)

			verify := performShopAccountBridgeRequest(t, handler.VerifyPasswordForShopAccountBridge,
				"/api/v1/integrations/shop/account-bridge/verify-password",
				`{"email":"privileged@example.com","password":"PrivilegedPass123"}`,
			)
			require.Equal(t, http.StatusUnauthorized, verify.Code)
			var response struct {
				Reason string `json:"reason"`
			}
			require.NoError(t, json.Unmarshal(verify.Body.Bytes(), &response))
			require.Equal(t, "INVALID_CREDENTIALS", response.Reason)
		})
	}
}

func TestShopAccountBridgeCreateCanOnlyCreateOrdinaryUser(t *testing.T) {
	repo := &accountBridgeRepoStub{userHandlerRepoStub: &userHandlerRepoStub{}}
	handler := newShopAccountBridgeAuthHandler(repo)
	recorder := performShopAccountBridgeRequest(t, handler.CreateShopAccountBridgeUser,
		"/api/v1/integrations/shop/account-bridge/users",
		`{"email":"new@example.com","password":"SafePassword123"}`,
	)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, repo.createCalls)
	require.NotNil(t, repo.created)
	require.Equal(t, service.RoleUser, repo.created.Role)
	require.Equal(t, service.StatusActive, repo.created.Status)
	require.Equal(t, "new@example.com", repo.created.Username)
	require.Zero(t, repo.created.Balance)
	require.Empty(t, repo.created.AllowedGroups)
	require.Empty(t, recorder.Header().Get("Set-Cookie"))

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	data := envelope["data"].(map[string]any)
	_, hasRole := data["role"]
	require.False(t, hasRole)
}

func TestShopAccountBridgeCreateRejectsPrivilegeFieldsAndOversizedPassword(t *testing.T) {
	tests := []string{
		`{"email":"new@example.com","password":"SafePassword123","role":"admin"}`,
		`{"email":"new@example.com","password":"SafePassword123","balance":1000000}`,
		`{"email":"new@example.com","password":"SafePassword123","status":"active"}`,
		`{"email":"new@example.com","password":"SafePassword123","allowed_groups":[1]}`,
		`{"email":"new@example.com","password":"` + strings.Repeat("p", 73) + `"}`,
	}
	for _, payload := range tests {
		repo := &accountBridgeRepoStub{userHandlerRepoStub: &userHandlerRepoStub{}}
		handler := newShopAccountBridgeAuthHandler(repo)
		recorder := performShopAccountBridgeRequest(t, handler.CreateShopAccountBridgeUser,
			"/api/v1/integrations/shop/account-bridge/users", payload)
		require.Equal(t, http.StatusBadRequest, recorder.Code)
		require.Zero(t, repo.createCalls)
	}
}

func TestShopAccountBridgeCreateReturnsConflictOnRace(t *testing.T) {
	repo := &accountBridgeRepoStub{
		userHandlerRepoStub: &userHandlerRepoStub{},
		createErr:           service.ErrEmailExists,
	}
	handler := newShopAccountBridgeAuthHandler(repo)
	recorder := performShopAccountBridgeRequest(t, handler.CreateShopAccountBridgeUser,
		"/api/v1/integrations/shop/account-bridge/users",
		`{"email":"race@example.com","password":"SafePassword123","username":"race"}`,
	)
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Equal(t, 1, repo.createCalls)
}
