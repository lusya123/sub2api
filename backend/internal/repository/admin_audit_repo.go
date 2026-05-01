package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type adminAuditRepository struct {
	db *sql.DB
}

func NewAdminAuditRepository(db *sql.DB) service.AdminAuditRepository {
	return &adminAuditRepository{db: db}
}

func (r *adminAuditRepository) Insert(ctx context.Context, input *service.AdminAuditLogInput) error {
	if r == nil || r.db == nil || input == nil {
		return nil
	}
	const q = `
INSERT INTO admin_audit_logs (
	created_at, actor_user_id, actor_email, actor_role, method, route_template, path,
	module, action, action_type, target_type, target_id, status_code, success,
	error_code, error_message, ip_address, user_agent, summary, query_params,
	request_body, duration_ms
) VALUES (
	$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20::jsonb,$21::jsonb,$22
)`
	_, err := r.db.ExecContext(ctx, q,
		input.CreatedAt,
		input.ActorUserID,
		input.ActorEmail,
		input.ActorRole,
		input.Method,
		input.RouteTemplate,
		input.Path,
		input.Module,
		input.Action,
		input.ActionType,
		input.TargetType,
		input.TargetID,
		input.StatusCode,
		input.Success,
		input.ErrorCode,
		input.ErrorMessage,
		input.IPAddress,
		input.UserAgent,
		input.Summary,
		input.QueryParamsJSON,
		input.RequestBodyJSON,
		input.DurationMS,
	)
	return err
}

func (r *adminAuditRepository) List(ctx context.Context, filter *service.AdminAuditLogFilter) (*service.AdminAuditLogList, error) {
	where, args := buildAdminAuditWhere(filter)
	countSQL := "SELECT COUNT(1) FROM admin_audit_logs l" + where
	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	page := filter.Page
	pageSize := filter.PageSize
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)
	querySQL := `
SELECT id, created_at, actor_user_id, actor_email, actor_role, method, route_template, path,
	module, action, action_type, target_type, target_id, status_code, success,
	error_code, error_message, ip_address, user_agent, summary,
	COALESCE(query_params, '{}'::jsonb)::text,
	COALESCE(request_body, '{}'::jsonb)::text,
	duration_ms
FROM admin_audit_logs l` + where + fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, querySQL, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	logs := make([]service.AdminAuditLog, 0, pageSize)
	for rows.Next() {
		item, scanErr := scanAdminAuditLog(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		logs = append(logs, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.hydrateAdminAuditUserRefs(ctx, logs); err != nil {
		return nil, err
	}
	return &service.AdminAuditLogList{Logs: logs, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *adminAuditRepository) GetByID(ctx context.Context, id int64) (*service.AdminAuditLog, error) {
	const q = `
SELECT id, created_at, actor_user_id, actor_email, actor_role, method, route_template, path,
	module, action, action_type, target_type, target_id, status_code, success,
	error_code, error_message, ip_address, user_agent, summary,
	COALESCE(query_params, '{}'::jsonb)::text,
	COALESCE(request_body, '{}'::jsonb)::text,
	duration_ms
FROM admin_audit_logs
	WHERE id = $1`
	row := r.db.QueryRowContext(ctx, q, id)
	item, err := scanAdminAuditLog(row)
	if err != nil {
		return nil, err
	}
	logs := []service.AdminAuditLog{*item}
	if err := r.hydrateAdminAuditUserRefs(ctx, logs); err != nil {
		return nil, err
	}
	return &logs[0], nil
}

type adminAuditScanner interface {
	Scan(dest ...any) error
}

func scanAdminAuditLog(scanner adminAuditScanner) (*service.AdminAuditLog, error) {
	var item service.AdminAuditLog
	var targetID sql.NullInt64
	var queryRaw string
	var bodyRaw string
	if err := scanner.Scan(
		&item.ID,
		&item.CreatedAt,
		&item.ActorUserID,
		&item.ActorEmail,
		&item.ActorRole,
		&item.Method,
		&item.RouteTemplate,
		&item.Path,
		&item.Module,
		&item.Action,
		&item.ActionType,
		&item.TargetType,
		&targetID,
		&item.StatusCode,
		&item.Success,
		&item.ErrorCode,
		&item.ErrorMessage,
		&item.IPAddress,
		&item.UserAgent,
		&item.Summary,
		&queryRaw,
		&bodyRaw,
		&item.DurationMS,
	); err != nil {
		return nil, translatePersistenceError(err, service.ErrAdminAuditLogNotFound, nil)
	}
	if targetID.Valid {
		v := targetID.Int64
		item.TargetID = &v
	}
	item.QueryParamsJSON = json.RawMessage(queryRaw)
	item.RequestBodyJSON = json.RawMessage(bodyRaw)
	return &item, nil
}

func (r *adminAuditRepository) hydrateAdminAuditUserRefs(ctx context.Context, logs []service.AdminAuditLog) error {
	if r == nil || r.db == nil || len(logs) == 0 {
		return nil
	}
	idsByLog := make([]map[int64]struct{}, len(logs))
	uniqueIDs := make(map[int64]struct{})
	for i := range logs {
		ids := collectAdminAuditUserIDs(&logs[i])
		if len(ids) == 0 {
			continue
		}
		idsByLog[i] = ids
		for id := range ids {
			uniqueIDs[id] = struct{}{}
		}
	}
	if len(uniqueIDs) == 0 {
		return nil
	}

	ids := make([]int64, 0, len(uniqueIDs))
	for id := range uniqueIDs {
		ids = append(ids, id)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, email FROM users WHERE id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	emailsByID := make(map[int64]string, len(ids))
	for rows.Next() {
		var id int64
		var email string
		if err := rows.Scan(&id, &email); err != nil {
			return err
		}
		emailsByID[id] = email
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for i := range logs {
		if len(idsByLog[i]) == 0 {
			continue
		}
		refs := make(map[int64]string, len(idsByLog[i]))
		for id := range idsByLog[i] {
			if email := strings.TrimSpace(emailsByID[id]); email != "" {
				refs[id] = email
			}
		}
		if len(refs) > 0 {
			logs[i].UserRefs = refs
		}
	}
	return nil
}

func collectAdminAuditUserIDs(log *service.AdminAuditLog) map[int64]struct{} {
	ids := make(map[int64]struct{})
	if log == nil {
		return ids
	}
	if log.TargetType == "user" && log.TargetID != nil {
		ids[*log.TargetID] = struct{}{}
	}
	collectAdminAuditUserIDsFromJSON(log.RequestBodyJSON, ids)
	collectAdminAuditUserIDsFromJSON(log.QueryParamsJSON, ids)
	return ids
}

func collectAdminAuditUserIDsFromJSON(raw json.RawMessage, ids map[int64]struct{}) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return
	}
	collectAdminAuditUserIDsFromValue(value, ids)
}

func collectAdminAuditUserIDsFromValue(value any, ids map[int64]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "user_id" || key == "user_ids" {
				addAdminAuditUserID(child, ids)
				continue
			}
			collectAdminAuditUserIDsFromValue(child, ids)
		}
	case []any:
		for _, child := range typed {
			collectAdminAuditUserIDsFromValue(child, ids)
		}
	}
}

func addAdminAuditUserID(value any, ids map[int64]struct{}) {
	switch typed := value.(type) {
	case json.Number:
		if id, err := typed.Int64(); err == nil && id > 0 {
			ids[id] = struct{}{}
		}
	case float64:
		id := int64(typed)
		if typed == float64(id) && id > 0 {
			ids[id] = struct{}{}
		}
	case int64:
		if typed > 0 {
			ids[typed] = struct{}{}
		}
	case string:
		if id, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64); err == nil && id > 0 {
			ids[id] = struct{}{}
		}
	case []any:
		for _, child := range typed {
			addAdminAuditUserID(child, ids)
		}
	}
}

func buildAdminAuditWhere(filter *service.AdminAuditLogFilter) (string, []any) {
	if filter == nil {
		filter = &service.AdminAuditLogFilter{}
	}
	clauses := make([]string, 0, 12)
	args := make([]any, 0, 12)
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}

	if filter.StartTime != nil {
		add("l.created_at >= $%d", filter.StartTime)
	}
	if filter.EndTime != nil {
		add("l.created_at <= $%d", filter.EndTime)
	}
	if filter.ActorUserID != nil {
		add("l.actor_user_id = $%d", *filter.ActorUserID)
	}
	if filter.ActorRole != "" {
		add("l.actor_role = $%d", filter.ActorRole)
	}
	if filter.Module != "" {
		add("l.module = $%d", filter.Module)
	}
	if filter.ActionType != "" {
		add("l.action_type = $%d", filter.ActionType)
	}
	if filter.ExcludeSuccessfulRead {
		clauses = append(clauses, "NOT (l.action_type = 'read' AND l.success = true)")
	}
	if filter.ExcludeActionType != "" {
		add("l.action_type <> $%d", filter.ExcludeActionType)
	}
	if filter.TargetType != "" {
		add("l.target_type = $%d", filter.TargetType)
	}
	if filter.TargetID != nil {
		add("l.target_id = $%d", *filter.TargetID)
	}
	if filter.Success != nil {
		add("l.success = $%d", *filter.Success)
	}
	if filter.StatusCode != nil {
		add("l.status_code = $%d", *filter.StatusCode)
	}
	if filter.Method != "" {
		add("l.method = $%d", filter.Method)
	}
	if filter.Route != "" {
		args = append(args, "%"+filter.Route+"%")
		idx := len(args)
		clauses = append(clauses, fmt.Sprintf("(l.route_template ILIKE $%d OR l.path ILIKE $%d)", idx, idx))
	}
	if filter.Query != "" {
		args = append(args, filter.Query)
		queryIdx := len(args)
		args = append(args, "%"+filter.Query+"%")
		likeIdx := len(args)
		clauses = append(clauses, fmt.Sprintf(`(
				to_tsvector('simple', COALESCE(l.summary,'') || ' ' || COALESCE(l.action,'') || ' ' || COALESCE(l.route_template,'') || ' ' || COALESCE(l.path,'') || ' ' || COALESCE(l.error_message,'') || ' ' || COALESCE(l.actor_email,'') || ' ' || COALESCE(l.actor_role,'') || ' ' || COALESCE(l.target_type,'') || ' ' || COALESCE(l.target_id::text,'') || ' ' || COALESCE(l.actor_user_id::text,'') || ' ' || COALESCE(l.request_body::text,'') || ' ' || COALESCE(l.query_params::text,'')) @@ plainto_tsquery('simple', $%d)
				OR l.summary ILIKE $%d
				OR l.action ILIKE $%d
				OR l.path ILIKE $%d
				OR l.error_message ILIKE $%d
				OR l.actor_email ILIKE $%d
				OR l.actor_role ILIKE $%d
				OR l.target_type ILIKE $%d
				OR l.target_id::text ILIKE $%d
				OR l.actor_user_id::text ILIKE $%d
			)`, queryIdx, likeIdx, likeIdx, likeIdx, likeIdx, likeIdx, likeIdx, likeIdx, likeIdx, likeIdx))
	}

	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
