package middleware

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSecurityAuditRouteMigrationMiddlewareContracts(t *testing.T) {
	const newClearRoute = "POST /api/v1/admin/security-audit-logs/clear"
	const legacyClearRoute = "POST /api/v1/admin/audit-logs/clear"

	require.Equal(t, service.AuditActionAuditLogClear, auditActionOverrides[newClearRoute])
	require.Equal(t, service.AuditActionAuditLogClear, auditActionOverrides[legacyClearRoute])
	require.True(t, isAPIPathGuardKnownFrontend("/admin/security-audit-logs"))
	require.True(t, isAPIPathGuardKnownFrontend("/admin/security-audit-logs/42"))
	require.Equal(t, "安全审计", auditModuleChineseName("security-audit-logs"))
}

func TestAdminAuditRedactionDoesNotPersistTOTPCode(t *testing.T) {
	redacted := mustMarshalAuditJSON(redactAuditBody([]byte(`{"totp_code":"123456"}`)))

	require.NotContains(t, redacted, "123456")
	require.True(t, strings.Contains(redacted, "[REDACTED]"))
}
