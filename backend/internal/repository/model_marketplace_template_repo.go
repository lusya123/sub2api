package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type modelMarketplaceTemplateRepository struct {
	db *sql.DB
}

func NewModelMarketplaceTemplateRepository(_ *dbent.Client, db *sql.DB) service.ModelMarketplaceTemplateRepository {
	return &modelMarketplaceTemplateRepository{db: db}
}

func (r *modelMarketplaceTemplateRepository) Create(ctx context.Context, t *service.ModelMarketplaceTemplate) error {
	headers, _ := json.Marshal(emptyModelMarketplaceHeadersIfNil(t.ExtraHeaders))
	body, _ := json.Marshal(t.BodyOverride)
	return r.db.QueryRowContext(ctx, `
		INSERT INTO model_marketplace_request_templates
			(name, provider, description, extra_headers, body_override_mode, body_override)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, created_at, updated_at`,
		t.Name, t.Provider, t.Description, headers, defaultModelMarketplaceBodyMode(t.BodyOverrideMode), nullableModelMarketplaceJSON(body, t.BodyOverride),
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *modelMarketplaceTemplateRepository) GetByID(ctx context.Context, id int64) (*service.ModelMarketplaceTemplate, error) {
	rows, err := r.queryTemplates(ctx, `WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, service.ErrModelMarketplaceTemplateNotFound
	}
	return rows[0], nil
}

func (r *modelMarketplaceTemplateRepository) Update(ctx context.Context, t *service.ModelMarketplaceTemplate) error {
	headers, _ := json.Marshal(emptyModelMarketplaceHeadersIfNil(t.ExtraHeaders))
	body, _ := json.Marshal(t.BodyOverride)
	err := r.db.QueryRowContext(ctx, `
		UPDATE model_marketplace_request_templates
		SET name=$2, description=$3, extra_headers=$4, body_override_mode=$5, body_override=$6, updated_at=NOW()
		WHERE id=$1
		RETURNING updated_at`,
		t.ID, t.Name, t.Description, headers, defaultModelMarketplaceBodyMode(t.BodyOverrideMode), nullableModelMarketplaceJSON(body, t.BodyOverride),
	).Scan(&t.UpdatedAt)
	if err == sql.ErrNoRows {
		return service.ErrModelMarketplaceTemplateNotFound
	}
	return err
}

func (r *modelMarketplaceTemplateRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM model_marketplace_request_templates WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrModelMarketplaceTemplateNotFound
	}
	return nil
}

func (r *modelMarketplaceTemplateRepository) List(ctx context.Context, params service.ModelMarketplaceTemplateListParams) ([]*service.ModelMarketplaceTemplate, error) {
	if strings.TrimSpace(params.Provider) != "" {
		return r.queryTemplates(ctx, `WHERE provider = $1 ORDER BY id DESC`, params.Provider)
	}
	return r.queryTemplates(ctx, `ORDER BY id DESC`)
}

func (r *modelMarketplaceTemplateRepository) ApplyToMonitors(ctx context.Context, id int64, monitorIDs []int64) (int64, error) {
	tpl, err := r.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	headers, _ := json.Marshal(emptyModelMarketplaceHeadersIfNil(tpl.ExtraHeaders))
	body, _ := json.Marshal(tpl.BodyOverride)
	res, err := r.db.ExecContext(ctx, `
		UPDATE model_marketplace_monitors
		SET template_id=$1, extra_headers=$2, body_override_mode=$3, body_override=$4, updated_at=NOW()
		WHERE id = ANY($5) AND template_id=$1 AND provider=$6`,
		id, headers, defaultModelMarketplaceBodyMode(tpl.BodyOverrideMode), nullableModelMarketplaceJSON(body, tpl.BodyOverride), pq.Array(monitorIDs), tpl.Provider,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *modelMarketplaceTemplateRepository) CountAssociatedMonitors(ctx context.Context, id int64) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM model_marketplace_monitors WHERE template_id=$1`, id).Scan(&count)
	return count, err
}

func (r *modelMarketplaceTemplateRepository) ListAssociatedMonitors(ctx context.Context, id int64) ([]*service.ModelMarketplaceAssociatedMonitorBrief, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, provider, enabled
		FROM model_marketplace_monitors
		WHERE template_id=$1
		ORDER BY id DESC`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*service.ModelMarketplaceAssociatedMonitorBrief{}
	for rows.Next() {
		item := &service.ModelMarketplaceAssociatedMonitorBrief{}
		if err := rows.Scan(&item.ID, &item.Name, &item.Provider, &item.Enabled); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *modelMarketplaceTemplateRepository) queryTemplates(ctx context.Context, suffix string, args ...any) ([]*service.ModelMarketplaceTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, provider, description, extra_headers, body_override_mode, body_override, created_at, updated_at
		FROM model_marketplace_request_templates `+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*service.ModelMarketplaceTemplate{}
	for rows.Next() {
		t := &service.ModelMarketplaceTemplate{}
		var headersRaw, bodyRaw []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Provider, &t.Description, &headersRaw, &t.BodyOverrideMode, &bodyRaw, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(headersRaw, &t.ExtraHeaders)
		if len(bodyRaw) > 0 {
			_ = json.Unmarshal(bodyRaw, &t.BodyOverride)
		}
		if t.ExtraHeaders == nil {
			t.ExtraHeaders = map[string]string{}
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan model marketplace templates: %w", err)
	}
	return out, nil
}
