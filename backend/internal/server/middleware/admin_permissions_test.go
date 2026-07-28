package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminPermissionGuardAllowsOperatorComplianceRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "status", method: http.MethodGet, path: "/api/v1/admin/compliance", wantStatus: http.StatusOK},
		{name: "accept", method: http.MethodPost, path: "/api/v1/admin/compliance/accept", wantStatus: http.StatusOK},
		{name: "unsupported method", method: http.MethodPut, path: "/api/v1/admin/compliance/accept", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyUserRole), service.RoleOperator)
				c.Next()
			})
			r.Use(AdminPermissionGuard())
			r.Handle(tt.method, tt.path, func(c *gin.Context) {
				c.String(http.StatusOK, "ok")
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			r.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestAdminAuditMiddlewareRecordsRedactedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &captureAdminAuditRepo{}
	auditService := service.NewAdminAuditService(repo)

	r := gin.New()
	r.Use(AdminAuditMiddleware(auditService))
	r.POST("/api/v1/admin/users/:id/balance", func(c *gin.Context) {
		_, _ = io.ReadAll(c.Request.Body)
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 42, Concurrency: 1})
		c.Set(string(ContextKeyUserRole), service.RoleOperator)
		c.Set(string(ContextKeyUserEmail), "operator@example.com")
		c.Status(http.StatusCreated)
	})

	body := bytes.NewBufferString(`{"balance":100,"password":"secret","nested":{"token":"abc"},"items":[{"api_key":"sk"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/123/balance?token=query-secret&plain=ok", body)
	req.Header.Set("User-Agent", "audit-test")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.NotNil(t, repo.input)
	require.Equal(t, int64(42), repo.input.ActorUserID)
	require.Equal(t, "operator@example.com", repo.input.ActorEmail)
	require.Equal(t, service.RoleOperator, repo.input.ActorRole)
	require.Equal(t, service.AuditAuthMethodJWT, repo.input.AuthMethod)
	require.Equal(t, "users", repo.input.Module)
	require.Equal(t, "write", repo.input.ActionType)
	require.Equal(t, "user", repo.input.TargetType)
	require.NotNil(t, repo.input.TargetID)
	require.Equal(t, int64(123), *repo.input.TargetID)
	require.Contains(t, repo.input.Summary, "调整用户余额")

	var bodyJSON map[string]any
	require.NoError(t, json.Unmarshal([]byte(repo.input.RequestBodyJSON), &bodyJSON))
	require.Equal(t, "[REDACTED]", bodyJSON["password"])
	require.Equal(t, float64(100), bodyJSON["balance"])
	nested, ok := bodyJSON["nested"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", nested["token"])
	items, ok := bodyJSON["items"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, items)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", item["api_key"])

	var queryJSON map[string]any
	require.NoError(t, json.Unmarshal([]byte(repo.input.QueryParamsJSON), &queryJSON))
	require.Equal(t, "[REDACTED]", queryJSON["token"])
	require.Equal(t, "ok", queryJSON["plain"])
}

func TestAdminAuditMiddlewareMarksSharedAdminAPIKeyAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &captureAdminAuditRepo{}
	auditService := service.NewAdminAuditService(repo)
	router := gin.New()
	router.Use(AdminAuditMiddleware(auditService))
	router.GET("/api/v1/admin/dashboard/stats", func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 1})
		c.Set(string(ContextKeyUserRole), service.RoleAdmin)
		c.Set(string(ContextKeyUserEmail), "first-admin@example.com")
		c.Set("auth_method", service.AuditAuthMethodAdminAPIKey)
		c.Status(http.StatusOK)
	})

	const key = "admin-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard/stats", nil)
	request.Header.Set("x-api-key", key)
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusOK, responseRecorder.Code)
	require.NotNil(t, repo.input)
	require.Equal(t, service.AuditAuthMethodAdminAPIKey, repo.input.AuthMethod)
	require.Equal(t, "x-api-key "+service.MaskAuditCredential(key), repo.input.CredentialMasked)
	require.NotContains(t, repo.input.CredentialMasked, key)
}

type countingAuditBody struct {
	data  *bytes.Reader
	reads int
}

func (b *countingAuditBody) Read(p []byte) (int, error) {
	b.reads++
	return b.data.Read(p)
}

func (b *countingAuditBody) Close() error { return nil }

func TestAdminAuditMiddlewareDoesNotReadBodyBeforeRejectedRequestNeedsIt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &captureAdminAuditRepo{}
	auditService := service.NewAdminAuditService(repo)
	router := gin.New()
	router.Use(AdminAuditMiddleware(auditService))
	router.POST("/api/v1/admin/users", func(c *gin.Context) {
		c.Status(http.StatusUnauthorized)
	})

	body := &countingAuditBody{data: bytes.NewReader(bytes.Repeat([]byte("x"), maxAuditBodyBytes*2))}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", nil)
	request.Body = body
	request.ContentLength = int64(maxAuditBodyBytes * 2)
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusUnauthorized, responseRecorder.Code)
	require.Zero(t, body.reads)
	require.NotNil(t, repo.input)
	require.JSONEq(t, `{}`, repo.input.RequestBodyJSON)
}

func TestAdminAuditMiddlewareOmitsCredentialBearingBodies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name   string
		route  string
		path   string
		body   string
		canary string
	}{
		{
			name:   "codex session import content",
			route:  "/api/v1/admin/accounts/import/codex-session",
			path:   "/api/v1/admin/accounts/import/codex-session",
			body:   `{"content":"{\"access_token\":\"codex-raw-canary\"}"}`,
			canary: "codex-raw-canary",
		},
		{
			name:   "oauth authorization code",
			route:  "/api/v1/admin/openai/exchange-code",
			path:   "/api/v1/admin/openai/exchange-code",
			body:   `{"session_id":"session-canary","code":"oauth-code-canary","state":"state-canary"}`,
			canary: "oauth-code-canary",
		},
		{
			name:   "cookie session key carried in code field",
			route:  "/api/v1/admin/accounts/cookie-auth",
			path:   "/api/v1/admin/accounts/cookie-auth",
			body:   `{"code":"cookie-session-canary"}`,
			canary: "cookie-session-canary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &captureAdminAuditRepo{}
			auditService := service.NewAdminAuditService(repo)
			router := gin.New()
			router.Use(AdminAuditMiddleware(auditService))
			router.POST(tt.route, func(c *gin.Context) {
				c.Set(string(ContextKeyUser), AuthSubject{UserID: 7})
				c.Set(string(ContextKeyUserRole), service.RoleAdmin)
				c.Status(http.StatusBadRequest)
			})

			request := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.body))
			request.Header.Set("Content-Type", "application/json")
			responseRecorder := httptest.NewRecorder()
			router.ServeHTTP(responseRecorder, request)

			require.Equal(t, http.StatusBadRequest, responseRecorder.Code)
			require.NotNil(t, repo.input)
			require.NotContains(t, repo.input.RequestBodyJSON, tt.canary)
			var recorded map[string]any
			require.NoError(t, json.Unmarshal([]byte(repo.input.RequestBodyJSON), &recorded))
			require.Equal(t, "credential-bearing body", recorded["_omitted"])
		})
	}
}

func TestAdminAuditRedactsGenericCodeFieldWithoutHidingErrorCode(t *testing.T) {
	redacted, ok := redactAuditBody([]byte(`{"code":"authorization-canary","error_code":"validation_failed"}`)).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", redacted["code"])
	require.Equal(t, "validation_failed", redacted["error_code"])
}

func TestAdminAuditSummaryHighlightsAdministratorRoleChanges(t *testing.T) {
	create := buildAuditSummary(
		http.MethodPost,
		"/api/v1/admin/users",
		"users",
		"users.write",
		"user",
		nil,
		`{"email":"new-admin@example.com","role":"admin"}`,
		http.StatusOK,
	)
	require.Contains(t, create, "角色=admin")

	targetID := int64(12)
	update := buildAuditSummary(
		http.MethodPut,
		"/api/v1/admin/users/:id",
		"users",
		"users.write",
		"user",
		&targetID,
		`{"role":"user"}`,
		http.StatusOK,
	)
	require.Contains(t, update, "角色=user")
}

func TestAdminAuditMiddlewareDoesNotPersistSensitivePathParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &captureAdminAuditRepo{}
	auditService := service.NewAdminAuditService(repo)
	router := gin.New()
	router.Use(AdminAuditMiddleware(auditService))
	router.GET("/api/v1/admin/refund-inspection/redeem-codes/:code", func(c *gin.Context) {
		c.Set(string(ContextKeyUser), AuthSubject{UserID: 9})
		c.Set(string(ContextKeyUserRole), service.RoleOperator)
		c.Status(http.StatusOK)
	})

	const canary = "REDEEM-CODE-PATH-CANARY"
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/refund-inspection/redeem-codes/"+canary, nil)
	responseRecorder := httptest.NewRecorder()
	router.ServeHTTP(responseRecorder, request)

	require.Equal(t, http.StatusOK, responseRecorder.Code)
	require.NotNil(t, repo.input)
	require.Equal(t, "/api/v1/admin/refund-inspection/redeem-codes/:code", repo.input.RouteTemplate)
	require.Equal(t, repo.input.RouteTemplate, repo.input.Path)
	require.NotContains(t, repo.input.Path, canary)
}

type captureAdminAuditRepo struct {
	input *service.AdminAuditLogInput
}

func (r *captureAdminAuditRepo) Insert(ctx context.Context, input *service.AdminAuditLogInput) error {
	clone := *input
	r.input = &clone
	return nil
}

func (r *captureAdminAuditRepo) List(ctx context.Context, filter *service.AdminAuditLogFilter) (*service.AdminAuditLogList, error) {
	return &service.AdminAuditLogList{}, nil
}

func (r *captureAdminAuditRepo) GetByID(ctx context.Context, id int64) (*service.AdminAuditLog, error) {
	return nil, service.ErrAdminAuditLogNotFound
}

func (r *captureAdminAuditRepo) BalanceSummary(ctx context.Context, filter *service.AdminAuditLogFilter) (*service.AdminAuditBalanceSummary, error) {
	return &service.AdminAuditBalanceSummary{}, nil
}
