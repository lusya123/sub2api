package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestShopCredentialEventOutboxRepository_ClaimUsesLeaseAndSkipLocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	occurred := time.Now().UTC().Add(-time.Second)
	created := time.Now().UTC()
	mock.ExpectQuery("(?s)claimed_at < NOW\\(\\) - .*FOR UPDATE SKIP LOCKED.*RETURNING").
		WithArgs("worker-a", 100, int64(30)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "credential_version", "attempts", "occurred_at", "created_at"}).
			AddRow(int64(9), int64(42), int64(7), 2, occurred, created))

	repo := NewShopCredentialEventOutboxRepository(db)
	events, err := repo.Claim(context.Background(), "worker-a", 100, 30*time.Second)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, int64(9), events[0].ID)
	require.Equal(t, int64(42), events[0].UserID)
	require.Equal(t, uint64(7), events[0].CredentialVersion)
	require.Equal(t, 2, events[0].Attempts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestShopCredentialEventOutboxRepository_RetryAndAckRequireClaimOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewShopCredentialEventOutboxRepository(db)

	retryAt := time.Now().UTC().Add(time.Minute)
	mock.ExpectExec("UPDATE shop_credential_event_outbox").
		WithArgs(int64(9), "worker", retryAt, "shop unavailable").
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.RetryClaimed(context.Background(), 9, "worker", retryAt, "shop unavailable"))

	mock.ExpectExec("DELETE FROM shop_credential_event_outbox").
		WithArgs(int64(9), "worker").
		WillReturnResult(sqlmock.NewResult(0, 0))
	err = repo.DeleteClaimed(context.Background(), 9, "worker")
	require.ErrorContains(t, err, "no longer owned")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestShopCredentialEventMigration_IsTransactionalVersionOnlyOutbox(t *testing.T) {
	content, err := migrations.FS.ReadFile("194_user_credential_version_shop_outbox.sql")
	require.NoError(t, err)
	sqlText := string(content)
	for _, required := range []string{
		"credential_version BIGINT NOT NULL DEFAULT 1",
		"NEW.credential_version := OLD.credential_version + 1",
		"OLD.password_hash IS DISTINCT FROM NEW.password_hash",
		"OLD.legacy_shop_password_hash IS DISTINCT FROM NEW.legacy_shop_password_hash",
		"OLD.email IS DISTINCT FROM NEW.email",
		"OLD.status IS DISTINCT FROM NEW.status",
		"OLD.totp_enabled IS DISTINCT FROM NEW.totp_enabled",
		"BEFORE UPDATE OF password_hash, legacy_shop_password_hash, email, status, totp_enabled, credential_version ON users",
		"AFTER UPDATE ON users",
		"shop_credential_event_outbox",
		"UNIQUE (user_id, credential_version)",
		"ON CONFLICT (user_id, credential_version) DO NOTHING",
		"claimed_at",
		"available_at",
	} {
		require.Contains(t, sqlText, required)
	}

	start := strings.Index(sqlText, "CREATE TABLE IF NOT EXISTS shop_credential_event_outbox")
	require.NotEqual(t, -1, start)
	end := strings.Index(sqlText[start:], ");")
	require.NotEqual(t, -1, end)
	outboxDDL := strings.ToLower(sqlText[start : start+end])
	require.NotContains(t, outboxDDL, "password")
	require.NotContains(t, outboxDDL, "hash")

	softDeleteFix, err := migrations.FS.ReadFile("195_user_credential_version_soft_delete.sql")
	require.NoError(t, err)
	softDeleteSQL := string(softDeleteFix)
	require.Contains(t, softDeleteSQL, "OLD.deleted_at IS DISTINCT FROM NEW.deleted_at")
	require.Contains(t, softDeleteSQL, "OLD.role IS DISTINCT FROM NEW.role")
	require.Contains(t, softDeleteSQL, "NEW.legacy_shop_password_hash := NULL")
	require.Contains(t, softDeleteSQL, "users_legacy_shop_password_ordinary_role")
	require.Contains(t, softDeleteSQL, "CHECK (role = 'user' OR legacy_shop_password_hash IS NULL)")
	require.Contains(t, softDeleteSQL, "BEFORE UPDATE OF password_hash, legacy_shop_password_hash, email, status, totp_enabled, deleted_at, role, credential_version ON users")
}
