package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAuditRouteContractsRegisterWithoutCollisions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Audit:    adminhandler.NewAuditHandler(nil),
		AuditLog: adminhandler.NewAuditLogHandler(nil, nil),
	}}
	admin := router.Group("/api/v1/admin")

	require.NotPanics(t, func() {
		registerAuditRoutes(admin, handlers)
		registerAuditLogRoutes(admin, handlers, nil)
	})

	type routeKey struct {
		method string
		path   string
	}
	routes := make(map[routeKey][]string)
	for _, route := range router.Routes() {
		key := routeKey{method: route.Method, path: route.Path}
		routes[key] = append(routes[key], route.Handler)
	}

	expected := map[routeKey]string{
		{method: http.MethodGet, path: "/api/v1/admin/audit-logs"}:                 "(*AuditHandler).List",
		{method: http.MethodGet, path: "/api/v1/admin/audit-logs/:id"}:             "(*AuditHandler).GetByID",
		{method: http.MethodGet, path: "/api/v1/admin/audit-logs/balance-summary"}: "(*AuditHandler).BalanceSummary",
		{method: http.MethodGet, path: "/api/v1/admin/security-audit-logs"}:        "(*AuditLogHandler).List",
		{method: http.MethodGet, path: "/api/v1/admin/security-audit-logs/:id"}:    "(*AuditLogHandler).Get",
		{method: http.MethodPost, path: "/api/v1/admin/security-audit-logs/clear"}: "(*AuditLogHandler).Clear",
		{method: http.MethodPost, path: "/api/v1/admin/audit-logs/clear"}:          "(*AuditLogHandler).Clear",
	}
	for key, handlerName := range expected {
		registered := routes[key]
		require.Lenf(t, registered, 1, "%s %s must have exactly one handler", key.method, key.path)
		require.Contains(t, registered[0], handlerName)
	}
}
