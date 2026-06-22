package adminaccess

import (
	"testing"

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
		{name: "compliance status allowed", method: "GET", path: "/api/v1/admin/compliance", want: true},
		{name: "compliance accept allowed", method: "POST", path: "/api/v1/admin/compliance/accept", want: true},
		{name: "compliance unsupported write denied", method: "PUT", path: "/api/v1/admin/compliance/accept", want: false},
		{name: "compliance lookalike denied", method: "GET", path: "/api/v1/admin/compliance-settings", want: false},
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
		{name: "payment admin read denied", method: "GET", path: "/api/v1/admin/payment/dashboard", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, OperatorRouteAllowed(tt.method, tt.path, tt.path))
		})
	}
}
