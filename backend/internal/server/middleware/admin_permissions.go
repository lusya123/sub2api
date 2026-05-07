package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/adminaccess"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AdminPermissionGuard limits operator accounts to operational admin surfaces.
// It must run after AdminAuthMiddleware.
func AdminPermissionGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRoleFromContext(c)
		if !ok {
			AbortWithError(c, 401, "UNAUTHORIZED", "User not found in context")
			return
		}
		if service.IsSuperAdminRole(role) {
			c.Next()
			return
		}
		if !service.IsOperatorRole(role) {
			AbortWithError(c, 403, "FORBIDDEN", "Admin access required")
			return
		}
		if adminaccess.OperatorRouteAllowed(c.Request.Method, c.FullPath(), c.Request.URL.Path) {
			c.Next()
			return
		}
		AbortWithError(c, 403, "FORBIDDEN", "Super admin access required")
	}
}

func SuperAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRoleFromContext(c)
		if !ok {
			AbortWithError(c, 401, "UNAUTHORIZED", "User not found in context")
			return
		}
		if !service.IsSuperAdminRole(role) {
			AbortWithError(c, 403, "FORBIDDEN", "Super admin access required")
			return
		}
		c.Next()
	}
}
