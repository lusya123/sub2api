package middleware

import (
	"strings"

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
		if operatorRouteAllowed(c.Request.Method, c.FullPath(), c.Request.URL.Path) {
			c.Next()
			return
		}
		AbortWithError(c, 403, "FORBIDDEN", "Super admin access required")
	}
}

func operatorRouteAllowed(method, fullPath, rawPath string) bool {
	method = strings.ToUpper(strings.TrimSpace(method))
	path := fullPath
	if path == "" {
		path = rawPath
	}
	path = strings.TrimSpace(path)

	if strings.HasPrefix(path, "/api/v1/admin/dashboard") {
		if method == "GET" {
			return true
		}
		return method == "POST" && (strings.HasSuffix(path, "/users-usage") || strings.HasSuffix(path, "/api-keys-usage"))
	}

	if strings.HasPrefix(path, "/api/v1/admin/ops") {
		return method == "GET"
	}

	if strings.HasPrefix(path, "/api/v1/admin/usage") {
		return method == "GET"
	}

	if strings.HasPrefix(path, "/api/v1/admin/refund-inspection") {
		return method == "GET" || method == "POST"
	}

	if strings.HasPrefix(path, "/api/v1/admin/subscriptions") {
		switch path {
		case "/api/v1/admin/subscriptions":
			return method == "GET"
		case "/api/v1/admin/subscriptions/:id", "/api/v1/admin/subscriptions/:id/progress":
			return method == "GET" || (method == "DELETE" && path == "/api/v1/admin/subscriptions/:id")
		case "/api/v1/admin/subscriptions/assign", "/api/v1/admin/subscriptions/bulk-assign":
			return method == "POST"
		case "/api/v1/admin/subscriptions/:id/extend", "/api/v1/admin/subscriptions/:id/reset-quota":
			return method == "POST"
		}
	}
	if method == "GET" && strings.HasPrefix(path, "/api/v1/admin/groups/") && strings.HasSuffix(path, "/subscriptions") {
		return true
	}

	if strings.HasPrefix(path, "/api/v1/admin/users") {
		switch method {
		case "GET":
			switch path {
			case "/api/v1/admin/users",
				"/api/v1/admin/users/:id",
				"/api/v1/admin/users/:id/api-keys",
				"/api/v1/admin/users/:id/usage",
				"/api/v1/admin/users/:id/balance-history",
				"/api/v1/admin/users/:id/attributes",
				"/api/v1/admin/users/:id/subscriptions":
				return true
			}
		case "POST":
			switch path {
			case "/api/v1/admin/users",
				"/api/v1/admin/users/:id/balance",
				"/api/v1/admin/users/:id/replace-group":
				return true
			}
		case "PUT":
			return path == "/api/v1/admin/users/:id" || path == "/api/v1/admin/users/:id/attributes"
		case "DELETE":
			return path == "/api/v1/admin/users/:id"
		}
	}

	// The users table asks for attribute definitions and batch values. Operators may read
	// those, but definition writes remain super-admin-only by method gating.
	if strings.HasPrefix(path, "/api/v1/admin/user-attributes") {
		return method == "GET" || (method == "POST" && strings.HasSuffix(path, "/batch"))
	}

	return false
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
