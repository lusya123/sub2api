package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type modelMarketplaceMonitorRepository struct {
	db *sql.DB
}

func NewModelMarketplaceMonitorRepository(_ *dbent.Client, db *sql.DB) service.ModelMarketplaceMonitorRepository {
	return &modelMarketplaceMonitorRepository{db: db}
}

func (r *modelMarketplaceMonitorRepository) Create(ctx context.Context, m *service.ModelMarketplaceMonitor) error {
	extras, _ := json.Marshal(emptyModelMarketplaceSliceIfNil(m.ExtraModels))
	headers, _ := json.Marshal(emptyModelMarketplaceHeadersIfNil(m.ExtraHeaders))
	body, _ := json.Marshal(m.BodyOverride)
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO model_marketplace_monitors
			(name, provider, endpoint, api_key_encrypted, primary_model, extra_models, group_name, enabled, interval_seconds,
			 created_by, template_id, extra_headers, body_override_mode, body_override)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		RETURNING id, created_at, updated_at`,
		m.Name, m.Provider, m.Endpoint, m.APIKey, m.PrimaryModel, extras, m.GroupName, m.Enabled, m.IntervalSeconds,
		m.CreatedBy, m.TemplateID, headers, defaultModelMarketplaceBodyMode(m.BodyOverrideMode), nullableModelMarketplaceJSON(body, m.BodyOverride),
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	return err
}

func (r *modelMarketplaceMonitorRepository) GetByID(ctx context.Context, id int64) (*service.ModelMarketplaceMonitor, error) {
	rows, err := r.queryMonitors(ctx, `WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, service.ErrModelMarketplaceMonitorNotFound
	}
	return rows[0], nil
}

func (r *modelMarketplaceMonitorRepository) Update(ctx context.Context, m *service.ModelMarketplaceMonitor) error {
	extras, _ := json.Marshal(emptyModelMarketplaceSliceIfNil(m.ExtraModels))
	headers, _ := json.Marshal(emptyModelMarketplaceHeadersIfNil(m.ExtraHeaders))
	body, _ := json.Marshal(m.BodyOverride)
	err := r.db.QueryRowContext(ctx, `
		UPDATE model_marketplace_monitors
		SET name=$2, provider=$3, endpoint=$4, api_key_encrypted=$5, primary_model=$6, extra_models=$7,
		    group_name=$8, enabled=$9, interval_seconds=$10, template_id=$11, extra_headers=$12,
		    body_override_mode=$13, body_override=$14, updated_at=NOW()
		WHERE id=$1
		RETURNING updated_at`,
		m.ID, m.Name, m.Provider, m.Endpoint, m.APIKey, m.PrimaryModel, extras, m.GroupName, m.Enabled,
		m.IntervalSeconds, m.TemplateID, headers, defaultModelMarketplaceBodyMode(m.BodyOverrideMode), nullableModelMarketplaceJSON(body, m.BodyOverride),
	).Scan(&m.UpdatedAt)
	if err == sql.ErrNoRows {
		return service.ErrModelMarketplaceMonitorNotFound
	}
	return err
}

func (r *modelMarketplaceMonitorRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM model_marketplace_monitors WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrModelMarketplaceMonitorNotFound
	}
	return nil
}

func (r *modelMarketplaceMonitorRepository) List(ctx context.Context, params service.ModelMarketplaceMonitorListParams) ([]*service.ModelMarketplaceMonitor, int64, error) {
	where, args := buildMarketplaceMonitorWhere(params)
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_marketplace_monitors `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := params.Page
	if page <= 0 {
		page = 1
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.queryMonitors(ctx, where+fmt.Sprintf(" ORDER BY id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	return rows, total, err
}

func (r *modelMarketplaceMonitorRepository) ListEnabled(ctx context.Context) ([]*service.ModelMarketplaceMonitor, error) {
	return r.queryMonitors(ctx, `WHERE enabled = TRUE`)
}

func (r *modelMarketplaceMonitorRepository) MarkChecked(ctx context.Context, id int64, checkedAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE model_marketplace_monitors SET last_checked_at=$2, updated_at=NOW() WHERE id=$1`, id, checkedAt)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrModelMarketplaceMonitorNotFound
	}
	return nil
}

func (r *modelMarketplaceMonitorRepository) InsertHistoryBatch(ctx context.Context, rows []*service.ModelMarketplaceMonitorHistoryRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO model_marketplace_monitor_histories
			(monitor_id, model, status, latency_ms, ping_latency_ms, message, checked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()
	for _, row := range rows {
		if _, err := stmt.ExecContext(ctx, row.MonitorID, row.Model, row.Status, row.LatencyMs, row.PingLatencyMs, row.Message, row.CheckedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *modelMarketplaceMonitorRepository) DeleteHistoryBefore(ctx context.Context, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM model_marketplace_monitor_histories WHERE checked_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *modelMarketplaceMonitorRepository) ListHistory(ctx context.Context, monitorID int64, model string, limit int) ([]*service.ModelMarketplaceMonitorHistoryEntry, error) {
	args := []any{monitorID}
	where := `WHERE monitor_id = $1`
	if strings.TrimSpace(model) != "" {
		args = append(args, model)
		where += fmt.Sprintf(" AND model = $%d", len(args))
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, model, status, latency_ms, ping_latency_ms, message, checked_at
		FROM model_marketplace_monitor_histories `+where+fmt.Sprintf(" ORDER BY checked_at DESC LIMIT $%d", len(args)), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*service.ModelMarketplaceMonitorHistoryEntry{}
	for rows.Next() {
		e := &service.ModelMarketplaceMonitorHistoryEntry{}
		if err := rows.Scan(&e.ID, &e.Model, &e.Status, &e.LatencyMs, &e.PingLatencyMs, &e.Message, &e.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *modelMarketplaceMonitorRepository) ListLatestPerModel(ctx context.Context, monitorID int64) ([]*service.ModelMarketplaceMonitorLatest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (model) model, status, latency_ms, ping_latency_ms, checked_at
		FROM model_marketplace_monitor_histories
		WHERE monitor_id = $1
		ORDER BY model, checked_at DESC`, monitorID)
	return scanMarketplaceLatest(rows, err)
}

func (r *modelMarketplaceMonitorRepository) ComputeAvailability(ctx context.Context, monitorID int64, windowDays int) ([]*service.ModelMarketplaceMonitorAvailability, error) {
	rows, err := r.db.QueryContext(ctx, marketplaceAvailabilitySQL(`monitor_id = $1`, 2), monitorID, windowDays)
	return scanMarketplaceAvailability(rows, err, windowDays)
}

func (r *modelMarketplaceMonitorRepository) ListLatestForMonitorIDs(ctx context.Context, ids []int64) (map[int64][]*service.ModelMarketplaceMonitorLatest, error) {
	out := make(map[int64][]*service.ModelMarketplaceMonitorLatest, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (monitor_id, model) monitor_id, model, status, latency_ms, ping_latency_ms, checked_at
		FROM model_marketplace_monitor_histories
		WHERE monitor_id = ANY($1)
		ORDER BY monitor_id, model, checked_at DESC`, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		l := &service.ModelMarketplaceMonitorLatest{}
		if err := rows.Scan(&id, &l.Model, &l.Status, &l.LatencyMs, &l.PingLatencyMs, &l.CheckedAt); err != nil {
			return nil, err
		}
		out[id] = append(out[id], l)
	}
	return out, rows.Err()
}

func (r *modelMarketplaceMonitorRepository) ComputeAvailabilityForMonitors(ctx context.Context, ids []int64, windowDays int) (map[int64][]*service.ModelMarketplaceMonitorAvailability, error) {
	out := make(map[int64][]*service.ModelMarketplaceMonitorAvailability, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT monitor_id, model,
		       CASE WHEN COUNT(*) = 0 THEN 100 ELSE 100.0 * SUM(CASE WHEN status = 'operational' THEN 1 ELSE 0 END) / COUNT(*) END AS availability_pct,
		       AVG(latency_ms)::INT AS avg_latency_ms
		FROM model_marketplace_monitor_histories
		WHERE monitor_id = ANY($1) AND checked_at >= NOW() - ($2::INT * INTERVAL '1 day')
		GROUP BY monitor_id, model`, pq.Array(ids), windowDays)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		a := &service.ModelMarketplaceMonitorAvailability{WindowDays: windowDays}
		if err := rows.Scan(&id, &a.Model, &a.AvailabilityPct, &a.AvgLatencyMs); err != nil {
			return nil, err
		}
		out[id] = append(out[id], a)
	}
	return out, rows.Err()
}

func buildMarketplaceMonitorWhere(params service.ModelMarketplaceMonitorListParams) (string, []any) {
	parts := []string{}
	args := []any{}
	if params.Provider != "" {
		args = append(args, params.Provider)
		parts = append(parts, fmt.Sprintf("provider = $%d", len(args)))
	}
	if params.Enabled != nil {
		args = append(args, *params.Enabled)
		parts = append(parts, fmt.Sprintf("enabled = $%d", len(args)))
	}
	if s := strings.TrimSpace(params.Search); s != "" {
		args = append(args, "%"+s+"%")
		parts = append(parts, fmt.Sprintf("(name ILIKE $%d OR group_name ILIKE $%d OR primary_model ILIKE $%d)", len(args), len(args), len(args)))
	}
	if len(parts) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(parts, " AND "), args
}

func (r *modelMarketplaceMonitorRepository) queryMonitors(ctx context.Context, suffix string, args ...any) ([]*service.ModelMarketplaceMonitor, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, provider, endpoint, api_key_encrypted, primary_model, extra_models, group_name, enabled,
		       interval_seconds, last_checked_at, created_by, created_at, updated_at, template_id,
		       extra_headers, body_override_mode, body_override
		FROM model_marketplace_monitors `+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*service.ModelMarketplaceMonitor{}
	for rows.Next() {
		m := &service.ModelMarketplaceMonitor{}
		var extraRaw, headersRaw []byte
		var bodyRaw []byte
		if err := rows.Scan(&m.ID, &m.Name, &m.Provider, &m.Endpoint, &m.APIKey, &m.PrimaryModel, &extraRaw, &m.GroupName,
			&m.Enabled, &m.IntervalSeconds, &m.LastCheckedAt, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt, &m.TemplateID,
			&headersRaw, &m.BodyOverrideMode, &bodyRaw); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(extraRaw, &m.ExtraModels)
		_ = json.Unmarshal(headersRaw, &m.ExtraHeaders)
		if len(bodyRaw) > 0 {
			_ = json.Unmarshal(bodyRaw, &m.BodyOverride)
		}
		if m.ExtraModels == nil {
			m.ExtraModels = []string{}
		}
		if m.ExtraHeaders == nil {
			m.ExtraHeaders = map[string]string{}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanMarketplaceLatest(rows *sql.Rows, err error) ([]*service.ModelMarketplaceMonitorLatest, error) {
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*service.ModelMarketplaceMonitorLatest{}
	for rows.Next() {
		l := &service.ModelMarketplaceMonitorLatest{}
		if err := rows.Scan(&l.Model, &l.Status, &l.LatencyMs, &l.PingLatencyMs, &l.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func scanMarketplaceAvailability(rows *sql.Rows, err error, windowDays int) ([]*service.ModelMarketplaceMonitorAvailability, error) {
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*service.ModelMarketplaceMonitorAvailability{}
	for rows.Next() {
		a := &service.ModelMarketplaceMonitorAvailability{WindowDays: windowDays}
		if err := rows.Scan(&a.Model, &a.AvailabilityPct, &a.AvgLatencyMs); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func marketplaceAvailabilitySQL(where string, windowArg int) string {
	return fmt.Sprintf(`
		SELECT model,
		       CASE WHEN COUNT(*) = 0 THEN 100 ELSE 100.0 * SUM(CASE WHEN status = 'operational' THEN 1 ELSE 0 END) / COUNT(*) END AS availability_pct,
		       AVG(latency_ms)::INT AS avg_latency_ms
		FROM model_marketplace_monitor_histories
		WHERE %s AND checked_at >= NOW() - ($%d::INT * INTERVAL '1 day')
		GROUP BY model`, where, windowArg)
}

func emptyModelMarketplaceHeadersIfNil(h map[string]string) map[string]string {
	if h == nil {
		return map[string]string{}
	}
	return h
}

func defaultModelMarketplaceBodyMode(mode string) string {
	if mode == "" {
		return "off"
	}
	return mode
}

func emptyModelMarketplaceSliceIfNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func nullableModelMarketplaceJSON(raw []byte, v any) any {
	if v == nil {
		return nil
	}
	return raw
}
