package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type shopCredentialEventOutboxRepository struct {
	db *sql.DB
}

func NewShopCredentialEventOutboxRepository(db *sql.DB) service.ShopCredentialEventOutboxRepository {
	return &shopCredentialEventOutboxRepository{db: db}
}

func (r *shopCredentialEventOutboxRepository) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]service.ShopCredentialEvent, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil shop credential event outbox database")
	}
	if limit <= 0 {
		limit = 100
	}
	leaseSeconds := int64(lease / time.Second)
	if leaseSeconds < 1 {
		leaseSeconds = 30
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH candidates AS (
			SELECT id
			FROM shop_credential_event_outbox
			WHERE available_at <= NOW()
			  AND (claimed_at IS NULL OR claimed_at < NOW() - ($3 * INTERVAL '1 second'))
			ORDER BY id ASC
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE shop_credential_event_outbox AS o
		SET claimed_at = NOW(), claimed_by = $1
		FROM candidates AS c
		WHERE o.id = c.id
		RETURNING o.id, o.user_id, o.credential_version, o.attempts, o.occurred_at, o.created_at
	`, workerID, limit, leaseSeconds)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	events := make([]service.ShopCredentialEvent, 0, limit)
	for rows.Next() {
		var event service.ShopCredentialEvent
		if err := rows.Scan(
			&event.ID,
			&event.UserID,
			&event.CredentialVersion,
			&event.Attempts,
			&event.OccurredAt,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		if event.ID <= 0 || event.UserID <= 0 || event.CredentialVersion == 0 {
			return nil, fmt.Errorf("invalid shop credential event row %d", event.ID)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *shopCredentialEventOutboxRepository) DeleteClaimed(ctx context.Context, id int64, workerID string) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM shop_credential_event_outbox
		WHERE id = $1 AND claimed_by = $2
	`, id, workerID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("shop credential event claim %d is no longer owned by %s", id, workerID)
	}
	return nil
}

func (r *shopCredentialEventOutboxRepository) RetryClaimed(ctx context.Context, id int64, workerID string, availableAt time.Time, lastError string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE shop_credential_event_outbox
		SET attempts = attempts + 1,
			available_at = $3,
			last_error = $4,
			claimed_at = NULL,
			claimed_by = NULL
		WHERE id = $1 AND claimed_by = $2
	`, id, workerID, availableAt, lastError)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("shop credential event claim %d is no longer owned by %s", id, workerID)
	}
	return nil
}

func (r *shopCredentialEventOutboxRepository) Stats(ctx context.Context) (service.ShopCredentialEventOutboxStats, error) {
	var (
		stats     service.ShopCredentialEventOutboxStats
		oldest    sql.NullTime
		lastError sql.NullString
	)
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(created_at), COALESCE(MAX(attempts), 0),
			(SELECT last_error
			 FROM shop_credential_event_outbox
			 WHERE last_error IS NOT NULL
			 ORDER BY available_at DESC, id DESC
			 LIMIT 1)
		FROM shop_credential_event_outbox
	`).Scan(&stats.Pending, &oldest, &stats.MaxAttempts, &lastError)
	if err != nil {
		return stats, err
	}
	if oldest.Valid {
		value := oldest.Time
		stats.OldestCreatedAt = &value
	}
	if lastError.Valid {
		stats.LastError = lastError.String
	}
	return stats, nil
}
