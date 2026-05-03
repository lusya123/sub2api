package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOperatorRouteAllowed(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{name: "dashboard read", method: "GET", path: "/api/v1/admin/dashboard", want: true},
		{name: "operations read allowed", method: "GET", path: "/api/v1/admin/operations/snapshot", want: true},
		{name: "operations write denied", method: "POST", path: "/api/v1/admin/operations/snapshot", want: false},
		{name: "usage read", method: "GET", path: "/api/v1/admin/usage", want: true},
		{name: "usage cleanup write denied", method: "POST", path: "/api/v1/admin/usage/cleanup-tasks", want: false},
		{name: "user write allowed", method: "PUT", path: "/api/v1/admin/users/:id", want: true},
		{name: "user unknown write denied", method: "POST", path: "/api/v1/admin/users/export", want: false},
		{name: "role delegation denied", method: "PUT", path: "/api/v1/admin/users/:id/role", want: false},
		{name: "subscription assignment allowed", method: "POST", path: "/api/v1/admin/subscriptions/assign", want: true},
		{name: "subscription reset quota allowed", method: "POST", path: "/api/v1/admin/subscriptions/:id/reset-quota", want: true},
		{name: "subscription unknown write denied", method: "POST", path: "/api/v1/admin/subscriptions/import", want: false},
		{name: "group list read denied", method: "GET", path: "/api/v1/admin/groups", want: false},
		{name: "group all read allowed", method: "GET", path: "/api/v1/admin/groups/all", want: true},
		{name: "group detail read denied", method: "GET", path: "/api/v1/admin/groups/:id", want: false},
		{name: "group write denied", method: "PUT", path: "/api/v1/admin/groups/:id", want: false},
		{name: "ops retry denied", method: "POST", path: "/api/v1/admin/ops/errors/:id/retry", want: false},
		{name: "settings read denied", method: "GET", path: "/api/v1/admin/settings", want: false},
		{name: "settings write denied", method: "PUT", path: "/api/v1/admin/settings", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, operatorRouteAllowed(tt.method, tt.path, tt.path))
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
