package admin

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

var (
	errOperatorTargetForbidden         = infraerrors.Forbidden("OPERATOR_TARGET_FORBIDDEN", "operators can only manage regular users")
	errOperatorRoleEscalationForbidden = infraerrors.Forbidden("OPERATOR_ROLE_ESCALATION_FORBIDDEN", "operators cannot create or promote administrator accounts")
)

func isOperatorRequest(c *gin.Context) bool {
	role, _ := middleware.GetUserRoleFromContext(c)
	return service.IsOperatorRole(role)
}

func ensureOperatorRoleAssignmentAllowed(c *gin.Context, role string) error {
	if isOperatorRequest(c) && role != "" && role != service.RoleUser {
		return errOperatorRoleEscalationForbidden
	}
	return nil
}

func ensureOperatorCanManageUser(ctx context.Context, c *gin.Context, adminService service.AdminService, userID int64) error {
	if !isOperatorRequest(c) {
		return nil
	}
	target, err := adminService.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	if target.Role != service.RoleUser {
		return errOperatorTargetForbidden
	}
	return nil
}

func ensureOperatorCanManageUserWithService(ctx context.Context, c *gin.Context, userService *service.UserService, userID int64) error {
	if !isOperatorRequest(c) {
		return nil
	}
	target, err := userService.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if target.Role != service.RoleUser {
		return errOperatorTargetForbidden
	}
	return nil
}
