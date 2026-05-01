package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdminAuditRepositoryListHydratesUserRefs(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &adminAuditRepository{db: db}
	createdAt := time.Date(2026, 4, 30, 1, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT COUNT\(1\) FROM admin_audit_logs l`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	rows := sqlmock.NewRows([]string{
		"id", "created_at", "actor_user_id", "actor_email", "actor_role", "method", "route_template", "path",
		"module", "action", "action_type", "target_type", "target_id", "status_code", "success",
		"error_code", "error_message", "ip_address", "user_agent", "summary",
		"query_params", "request_body", "duration_ms",
	}).AddRow(
		int64(100), createdAt, int64(1), "admin@example.com", service.RoleAdmin, "POST",
		"/api/v1/admin/subscriptions/bulk-assign", "/api/v1/admin/subscriptions/bulk-assign",
		"subscriptions", "subscriptions.write.bulk_assign", "write", "user", int64(42), 200, true,
		"", "", "127.0.0.1", "test-agent", "bulk assign",
		`{"user_id":"46"}`, `{"user_id":43,"user_ids":[44,"45"],"nested":{"user_id":47}}`, int64(12),
	)
	mock.ExpectQuery(`SELECT id, created_at, actor_user_id`).
		WithArgs(20, 0).
		WillReturnRows(rows)

	mock.ExpectQuery(`SELECT id, email FROM users WHERE id = ANY`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).
			AddRow(int64(42), "target@example.com").
			AddRow(int64(43), "single@example.com").
			AddRow(int64(44), "bulk-a@example.com").
			AddRow(int64(45), "bulk-b@example.com").
			AddRow(int64(46), "query@example.com").
			AddRow(int64(47), "nested@example.com"))

	result, err := repo.List(context.Background(), &service.AdminAuditLogFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, result.Logs, 1)
	require.Equal(t, map[int64]string{
		42: "target@example.com",
		43: "single@example.com",
		44: "bulk-a@example.com",
		45: "bulk-b@example.com",
		46: "query@example.com",
		47: "nested@example.com",
	}, result.Logs[0].UserRefs)
	require.NoError(t, mock.ExpectationsWereMet())
}
