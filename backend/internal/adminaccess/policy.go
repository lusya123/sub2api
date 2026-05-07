package adminaccess

import "strings"

// OperatorRouteAllowed centralizes the custom operator API allowlist.
// Keep custom admin access changes in this package so upstream route churn
// usually only needs a small policy review during future merges.
func OperatorRouteAllowed(method, fullPath, rawPath string) bool {
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

	if strings.HasPrefix(path, "/api/v1/admin/operations") {
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
	if method == "GET" {
		switch path {
		case "/api/v1/admin/groups/all":
			return true
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
