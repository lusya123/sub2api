package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const maxAuditBodyBytes = 64 * 1024

func AdminAuditMiddleware(auditService *service.AdminAuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if auditService == nil {
			c.Next()
			return
		}

		start := time.Now()
		bodyRaw := readAndRestoreRequestBody(c)
		c.Next()

		subject, _ := GetAuthSubjectFromContext(c)
		role, _ := GetUserRoleFromContext(c)
		email, _ := GetUserEmailFromContext(c)
		status := c.Writer.Status()
		if status == 0 {
			status = 200
		}
		input := buildAuditInput(c, subject.UserID, email, role, bodyRaw, status, time.Since(start))
		auditCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		auditService.Record(auditCtx, input)
		cancel()
	}
}

func readAndRestoreRequestBody(c *gin.Context) []byte {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(nil))
		return nil
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if len(raw) > maxAuditBodyBytes {
		return raw[:maxAuditBodyBytes]
	}
	return raw
}

func buildAuditInput(c *gin.Context, actorID int64, actorEmail, actorRole string, bodyRaw []byte, status int, duration time.Duration) *service.AdminAuditLogInput {
	method := strings.ToUpper(c.Request.Method)
	route := c.FullPath()
	if route == "" {
		route = c.Request.URL.Path
	}
	module, action, actionType := classifyAdminAction(method, route)
	targetType, targetID := classifyAdminTarget(route, c)
	queryJSON := mustMarshalAuditJSON(redactAuditValue(queryParamsToMap(c)))
	bodyJSON := "{}"
	if len(bodyRaw) > 0 {
		bodyJSON = mustMarshalAuditJSON(redactAuditBody(bodyRaw))
	}
	summary := buildAuditSummary(method, route, module, action, targetType, targetID, bodyJSON, status)

	return &service.AdminAuditLogInput{
		CreatedAt:       time.Now().UTC(),
		ActorUserID:     actorID,
		ActorEmail:      actorEmail,
		ActorRole:       actorRole,
		Method:          method,
		RouteTemplate:   route,
		Path:            c.Request.URL.Path,
		Module:          module,
		Action:          action,
		ActionType:      actionType,
		TargetType:      targetType,
		TargetID:        targetID,
		StatusCode:      status,
		Success:         status >= 200 && status < 400,
		ErrorMessage:    auditErrorMessage(c, status),
		IPAddress:       c.ClientIP(),
		UserAgent:       c.Request.UserAgent(),
		Summary:         summary,
		QueryParamsJSON: queryJSON,
		RequestBodyJSON: bodyJSON,
		DurationMS:      duration.Milliseconds(),
	}
}

func classifyAdminAction(method, route string) (module, action, actionType string) {
	rel := strings.TrimPrefix(route, "/api/v1/admin/")
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		module = "admin"
	} else {
		module = parts[0]
	}
	switch method {
	case "GET":
		actionType = "read"
	case "POST", "PUT", "PATCH":
		actionType = "write"
	case "DELETE":
		actionType = "delete"
	default:
		actionType = strings.ToLower(method)
	}
	action = module + "." + actionType
	for _, part := range parts[1:] {
		if part == "" || strings.HasPrefix(part, ":") {
			continue
		}
		action += "." + strings.ReplaceAll(part, "-", "_")
	}
	return module, action, actionType
}

func classifyAdminTarget(route string, c *gin.Context) (string, *int64) {
	rel := strings.TrimPrefix(route, "/api/v1/admin/")
	parts := strings.Split(strings.Trim(rel, "/"), "/")
	if len(parts) == 0 {
		return "", nil
	}
	targetType := strings.TrimSuffix(parts[0], "s")
	if len(parts) >= 2 && parts[0] == "users" {
		targetType = "user"
	}
	if len(parts) >= 2 && parts[0] == "subscriptions" {
		targetType = "subscription"
	}
	if id := parseAuditID(c.Param("id")); id != nil {
		return targetType, id
	}
	if parts[0] == "subscriptions" && (strings.HasSuffix(route, "/assign") || strings.HasSuffix(route, "/bulk-assign")) {
		return "subscription", nil
	}
	return targetType, nil
}

func buildAuditSummary(method, route, module, action, targetType string, targetID *int64, bodyJSON string, status int) string {
	var body map[string]any
	_ = json.Unmarshal([]byte(bodyJSON), &body)
	statusText := auditStatusText(status)
	if route == "/api/v1/admin/users" && method == "POST" {
		return auditJoinParts([]string{
			statusText,
			"创建用户",
			"邮箱=" + auditMapString(body, "email"),
			"用户名=" + auditMapString(body, "username"),
			"初始余额=" + auditAnyString(body["balance"]),
			"并发=" + auditAnyString(body["concurrency"]),
			"可用分组=" + auditAnyString(body["allowed_groups"]),
		})
	}
	if route == "/api/v1/admin/users/:id" && method == "PUT" {
		return auditJoinParts([]string{
			statusText,
			"编辑用户",
			"用户ID=" + auditIDString(targetID),
			auditChangedFieldSummary(body),
		})
	}
	if route == "/api/v1/admin/users/:id" && method == "DELETE" {
		return auditJoinParts([]string{statusText, "删除用户", "用户ID=" + auditIDString(targetID)})
	}
	if strings.HasSuffix(route, "/role") && targetType == "user" {
		role := auditMapString(body, "role")
		verb := "撤销普通管理员"
		if role == service.RoleOperator {
			verb = "委派普通管理员"
		}
		return auditJoinParts([]string{statusText, verb, "用户ID=" + auditIDString(targetID), "目标角色=" + role})
	}
	if strings.HasSuffix(route, "/balance") && targetType == "user" {
		return auditJoinParts([]string{
			statusText,
			auditBalanceVerb(auditMapString(body, "operation")),
			"用户ID=" + auditIDString(targetID),
			"金额=" + auditAnyString(body["balance"]),
			"备注=" + auditMapString(body, "notes"),
		})
	}
	if strings.HasSuffix(route, "/replace-group") && targetType == "user" {
		return auditJoinParts([]string{
			statusText,
			"替换用户专属分组",
			"用户ID=" + auditIDString(targetID),
			"原分组ID=" + auditAnyString(body["old_group_id"]),
			"新分组ID=" + auditAnyString(body["new_group_id"]),
		})
	}
	if strings.HasSuffix(route, "/attributes") && targetType == "user" && method == "PUT" {
		return auditJoinParts([]string{statusText, "修改用户属性", "用户ID=" + auditIDString(targetID), "属性=" + auditAnyString(body["values"])})
	}
	if module == "subscriptions" && strings.HasSuffix(route, "/assign") {
		return auditJoinParts([]string{
			statusText,
			"分配订阅",
			"用户ID=" + auditAnyString(body["user_id"]),
			"订阅分组ID=" + auditAnyString(body["group_id"]),
			"有效天数=" + auditAnyString(body["validity_days"]),
			"备注=" + auditMapString(body, "notes"),
		})
	}
	if module == "subscriptions" && strings.HasSuffix(route, "/bulk-assign") {
		return auditJoinParts([]string{
			statusText,
			"批量分配订阅",
			"用户ID列表=" + auditAnyString(body["user_ids"]),
			"订阅分组ID=" + auditAnyString(body["group_id"]),
			"有效天数=" + auditAnyString(body["validity_days"]),
			"备注=" + auditMapString(body, "notes"),
		})
	}
	if module == "subscriptions" && strings.HasSuffix(route, "/extend") {
		return auditJoinParts([]string{
			statusText,
			auditSubscriptionDaysVerb(body["days"]),
			"订阅ID=" + auditIDString(targetID),
			"天数=" + auditAnyString(body["days"]),
		})
	}
	if module == "subscriptions" && strings.HasSuffix(route, "/reset-quota") {
		return auditJoinParts([]string{
			statusText,
			"重置订阅配额",
			"订阅ID=" + auditIDString(targetID),
			"日配额=" + auditAnyString(body["daily"]),
			"周配额=" + auditAnyString(body["weekly"]),
			"月配额=" + auditAnyString(body["monthly"]),
		})
	}
	if module == "subscriptions" && route == "/api/v1/admin/subscriptions/:id" && method == "DELETE" {
		return auditJoinParts([]string{statusText, "撤销订阅", "订阅ID=" + auditIDString(targetID)})
	}
	if route == "/api/v1/admin/usage/cleanup-tasks" && method == "POST" {
		return auditJoinParts([]string{
			statusText,
			"创建用量清理任务",
			"开始日期=" + auditMapString(body, "start_date"),
			"结束日期=" + auditMapString(body, "end_date"),
			"用户ID=" + auditAnyString(body["user_id"]),
			"API Key ID=" + auditAnyString(body["api_key_id"]),
			"账号ID=" + auditAnyString(body["account_id"]),
			"分组ID=" + auditAnyString(body["group_id"]),
			"模型=" + auditMapString(body, "model"),
			"请求类型=" + auditMapString(body, "request_type"),
		})
	}
	if route == "/api/v1/admin/usage/cleanup-tasks/:id/cancel" && method == "POST" {
		return auditJoinParts([]string{statusText, "取消用量清理任务", "任务ID=" + auditIDString(targetID)})
	}
	if method == "GET" {
		return auditJoinParts([]string{statusText, "查看" + auditModuleChineseName(module), "路由=" + route})
	}
	return auditJoinParts([]string{statusText, method + " " + route, "动作=" + action})
}

func auditStatusText(status int) string {
	if status >= 200 && status < 400 {
		return "成功"
	}
	return "失败(HTTP " + strconv.Itoa(status) + ")"
}

func auditBalanceVerb(operation string) string {
	switch operation {
	case "add":
		return "给用户充值"
	case "subtract":
		return "从用户余额扣款"
	case "set":
		return "设置用户余额"
	default:
		return "调整用户余额"
	}
}

func auditSubscriptionDaysVerb(v any) string {
	if n, ok := auditFloat64(v); ok && n < 0 {
		return "减少订阅天数"
	}
	return "增加订阅天数"
}

func auditChangedFieldSummary(body map[string]any) string {
	if len(body) == 0 {
		return ""
	}
	parts := make([]string, 0, 8)
	if v := auditMapString(body, "email"); v != "" {
		parts = append(parts, "邮箱="+v)
	}
	if v := auditMapString(body, "username"); v != "" {
		parts = append(parts, "用户名="+v)
	}
	if v := auditMapString(body, "status"); v != "" {
		parts = append(parts, "状态="+v)
	}
	if v := auditAnyString(body["balance"]); v != "" {
		parts = append(parts, "余额="+v)
	}
	if v := auditAnyString(body["concurrency"]); v != "" {
		parts = append(parts, "并发="+v)
	}
	if v := auditAnyString(body["allowed_groups"]); v != "" {
		parts = append(parts, "可用分组="+v)
	}
	if v := auditAnyString(body["group_rates"]); v != "" {
		parts = append(parts, "专属分组倍率="+v)
	}
	if _, ok := body["password"]; ok {
		parts = append(parts, "密码=已修改")
	}
	if v := auditMapString(body, "notes"); v != "" {
		parts = append(parts, "备注="+v)
	}
	return strings.Join(parts, "，")
}

func auditModuleChineseName(module string) string {
	switch module {
	case "dashboard":
		return "仪表盘"
	case "ops":
		return "运维监控"
	case "users":
		return "用户管理"
	case "subscriptions":
		return "订阅管理"
	case "usage":
		return "使用记录"
	case "settings":
		return "系统设置"
	case "audit-logs":
		return "操作审计"
	default:
		return module
	}
}

func auditJoinParts(parts []string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || strings.HasSuffix(part, "=") {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, "；")
}

func auditFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}

func auditErrorMessage(c *gin.Context, status int) string {
	if status < 400 {
		return ""
	}
	if len(c.Errors) > 0 {
		return c.Errors.String()
	}
	return "request failed"
}

func queryParamsToMap(c *gin.Context) map[string]any {
	out := make(map[string]any)
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return out
	}
	for k, v := range c.Request.URL.Query() {
		if len(v) == 1 {
			out[k] = v[0]
		} else {
			out[k] = v
		}
	}
	return out
}

func redactAuditBody(raw []byte) any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return map[string]any{"raw": "[non-json body]"}
	}
	return redactAuditValue(v)
}

func redactAuditValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if isSensitiveAuditKey(k) {
				out[k] = "[REDACTED]"
			} else {
				out[k] = redactAuditValue(val)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = redactAuditValue(x[i])
		}
		return out
	default:
		return v
	}
}

func isSensitiveAuditKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	sensitive := []string{"password", "token", "secret", "credential", "authorization", "cookie", "api_key", "apikey", "refresh", "access_token", "client_secret", "key"}
	for _, s := range sensitive {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func mustMarshalAuditJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil || len(b) == 0 {
		return "{}"
	}
	return string(b)
}

func parseAuditID(raw string) *int64 {
	if raw == "" {
		return nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return nil
	}
	return &id
}

func auditIDString(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}

func auditMapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return auditAnyString(m[key])
}

func auditAnyString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		if v == nil {
			return ""
		}
		b, _ := json.Marshal(v)
		return string(b)
	}
}
