//go:build postgresintegration

package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestShopCredentialOutboxMigration_TriggerAtomicity runs only when an isolated
// PostgreSQL DSN is explicitly supplied. All objects are created inside one
// transaction and rolled back, so this test never needs a persistent database.
func TestShopCredentialOutboxMigration_TriggerAtomicity(t *testing.T) {
	dsn := os.Getenv("SUB2API_CREDENTIAL_OUTBOX_TEST_DSN")
	if dsn == "" {
		t.Skip("SUB2API_CREDENTIAL_OUTBOX_TEST_DSN is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, db.PingContext(ctx))
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	schemaName := fmt.Sprintf("shop_credential_outbox_%d", time.Now().UnixNano())
	quotedSchema := pq.QuoteIdentifier(schemaName)
	_, err = tx.ExecContext(ctx, "CREATE SCHEMA "+quotedSchema)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "SET LOCAL search_path TO "+quotedSchema+", public")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			legacy_shop_password_hash VARCHAR(255),
			role TEXT NOT NULL DEFAULT 'user',
			status TEXT NOT NULL DEFAULT 'active',
			totp_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			deleted_at TIMESTAMPTZ
		)`)
	require.NoError(t, err)
	migration, err := FS.ReadFile("194_user_credential_version_shop_outbox.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	softDeleteFix, err := FS.ReadFile("195_user_credential_version_soft_delete.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(softDeleteFix))
	require.NoError(t, err)

	_, err = tx.ExecContext(ctx, "INSERT INTO users (email, password_hash) VALUES ('before@example.com', 'primary-v1')")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 1, 0)

	_, err = tx.ExecContext(ctx, "UPDATE users SET password_hash = 'primary-v1' WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 1, 0)
	_, err = tx.ExecContext(ctx, "UPDATE users SET email = 'before@example.com' WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 1, 0)
	_, err = tx.ExecContext(ctx, "UPDATE users SET email = 'after@example.com' WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 2, 1)
	_, err = tx.ExecContext(ctx, "UPDATE users SET status = 'disabled' WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 3, 2)
	_, err = tx.ExecContext(ctx, "UPDATE users SET totp_enabled = TRUE WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 4, 3)

	_, err = tx.ExecContext(ctx, "UPDATE users SET password_hash = 'primary-v2' WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 5, 4)

	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET password_hash = 'primary-v3', legacy_shop_password_hash = 'legacy-v1'
		WHERE id = 1`)
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 6, 5)

	// Promotion is an authorization transition. It must revoke Shop sessions
	// and permanently retire the imported verifier in the same transaction.
	_, err = tx.ExecContext(ctx, "UPDATE users SET role = 'admin' WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 7, 6)
	assertRoleAndLegacyState(t, ctx, tx, "admin", false)
	// A direct attempt to attach a legacy verifier to a privileged row is
	// neutralized by the trigger and cannot advance the version as a fake event.
	_, err = tx.ExecContext(ctx, "UPDATE users SET legacy_shop_password_hash = 'forbidden' WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 7, 6)
	assertRoleAndLegacyState(t, ctx, tx, "admin", false)
	// Demotion advances authorization again, but the old verifier never
	// reappears.
	_, err = tx.ExecContext(ctx, "UPDATE users SET role = 'user' WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 8, 7)
	assertRoleAndLegacyState(t, ctx, tx, "user", false)

	deletedAt := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
	_, err = tx.ExecContext(ctx, "UPDATE users SET deleted_at = $1 WHERE id = 1", deletedAt)
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 9, 8)
	// Reapplying the same tombstone is a no-op, so a soft-delete path cannot
	// generate multiple revocation events for the same state transition.
	_, err = tx.ExecContext(ctx, "UPDATE users SET deleted_at = $1 WHERE id = 1", deletedAt)
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 9, 8)
	// Restoration is authorization-relevant too: existing Shop sessions must
	// not silently survive a delete/restore cycle.
	_, err = tx.ExecContext(ctx, "UPDATE users SET deleted_at = NULL WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 10, 9)
	_, err = tx.ExecContext(ctx, "UPDATE users SET deleted_at = NULL WHERE id = 1")
	require.NoError(t, err)
	assertCredentialState(t, ctx, tx, 10, 9)

	var eventID, userID, version int64
	err = tx.QueryRowContext(ctx, `
		SELECT id, user_id, credential_version
		FROM shop_credential_event_outbox
		ORDER BY id DESC LIMIT 1`).Scan(&eventID, &userID, &version)
	require.NoError(t, err)
	require.Positive(t, eventID)
	require.Equal(t, int64(1), userID)
	require.Equal(t, int64(10), version)

	_, err = tx.ExecContext(ctx, "SAVEPOINT manual_version")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE users SET credential_version = credential_version + 10 WHERE id = 1")
	require.Error(t, err)
	_, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT manual_version")
	require.NoError(t, rollbackErr)
	assertCredentialState(t, ctx, tx, 10, 9)

	// Force the outbox insert to fail and prove the password/version update is
	// rolled back with it rather than committing without a notification.
	_, err = tx.ExecContext(ctx, "SAVEPOINT outbox_failure")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE shop_credential_event_outbox
		ADD CONSTRAINT reject_new_outbox_rows CHECK (FALSE) NOT VALID`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE users SET legacy_shop_password_hash = 'legacy-v2' WHERE id = 1")
	require.Error(t, err)
	_, rollbackErr = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT outbox_failure")
	require.NoError(t, rollbackErr)
	assertCredentialState(t, ctx, tx, 10, 9)

	var sensitiveColumns int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = $1
		  AND table_name = 'shop_credential_event_outbox'
		  AND (column_name ILIKE '%password%' OR column_name ILIKE '%hash%')
	`, schemaName).Scan(&sensitiveColumns)
	require.NoError(t, err)
	require.Zero(t, sensitiveColumns)
}

func assertRoleAndLegacyState(t *testing.T, ctx context.Context, tx *sql.Tx, wantRole string, wantLegacy bool) {
	t.Helper()
	var role string
	var legacy sql.NullString
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT role, legacy_shop_password_hash FROM users WHERE id = 1").Scan(&role, &legacy))
	require.Equal(t, wantRole, role)
	require.Equal(t, wantLegacy, legacy.Valid)
}

func assertCredentialState(t *testing.T, ctx context.Context, tx *sql.Tx, wantVersion, wantEvents int64) {
	t.Helper()
	var version, events int64
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT credential_version FROM users WHERE id = 1").Scan(&version))
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM shop_credential_event_outbox").Scan(&events))
	require.Equal(t, wantVersion, version)
	require.Equal(t, wantEvents, events)
}
