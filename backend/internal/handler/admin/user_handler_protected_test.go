package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserHandlerProtectedAdminErrorsAreConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		serviceErr error
		reason     string
	}{
		{
			name:       "disable admin",
			method:     http.MethodPut,
			path:       "/api/v1/admin/users/1",
			body:       `{"status":"disabled"}`,
			serviceErr: service.ErrAdminUserDisableProtected,
			reason:     "ADMIN_USER_DISABLE_PROTECTED",
		},
		{
			name:       "delete admin",
			method:     http.MethodDelete,
			path:       "/api/v1/admin/users/1",
			serviceErr: service.ErrAdminUserDeleteProtected,
			reason:     "ADMIN_USER_DELETE_PROTECTED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newStubAdminService()
			svc.users[0].Role = service.RoleAdmin
			svc.updateUserErr = tt.serviceErr
			svc.deleteUserErr = tt.serviceErr
			h := NewUserHandler(svc, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.PUT("/api/v1/admin/users/:id", h.Update)
			router.DELETE("/api/v1/admin/users/:id", h.Delete)

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusConflict, rec.Code)
			var got response.Response
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			require.Equal(t, tt.reason, got.Reason)
			require.NotEqual(t, "internal error", got.Message)
		})
	}
}
