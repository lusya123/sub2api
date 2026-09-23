//go:build postgresintegration

package accountunification

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Uses only disposable, explicitly supplied loopback PostgreSQL fixtures.
func migrationFixture(t *testing.T) (*sql.DB, *sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("ACCOUNT_UNIFICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("ACCOUNT_UNIFICATION_TEST_DSN is not set")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Contains(t, []string{"127.0.0.1", "localhost", "::1"}, u.Hostname())
	root, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = root.Close() })
	makeDB := func(suffix string) (*sql.DB, string) {
		schema := fmt.Sprintf("au_%d_%s", time.Now().UnixNano(), suffix)
		_, err := root.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema))
		require.NoError(t, err)
		v := *u
		q := v.Query()
		q.Set("search_path", schema)
		q.Set("application_name", schema)
		v.RawQuery = q.Encode()
		db, err := sql.Open("postgres", v.String())
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close(); _, _ = root.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE") })
		return db, schema
	}
	main, _ := makeDB("main")
	shop, name := makeDB("shop")
	_, err = main.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY, email TEXT NOT NULL,
	 password_hash TEXT NOT NULL, legacy_shop_password_hash TEXT, role TEXT NOT NULL DEFAULT 'user',
	 status TEXT NOT NULL DEFAULT 'active', totp_enabled BOOLEAN DEFAULT FALSE, deleted_at TIMESTAMPTZ)`)
	require.NoError(t, err)
	for _, name := range []string{"194_user_credential_version_shop_outbox.sql", "195_user_credential_version_soft_delete.sql"} {
		migration, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		_, err = main.Exec(string(migration))
		require.NoError(t, err)
	}
	_, err = shop.Exec(`CREATE TABLE users (id BIGSERIAL PRIMARY KEY, email TEXT NOT NULL,
	 password_hash TEXT NOT NULL, legacy_sub2api_password_hash TEXT NOT NULL DEFAULT '',
	 auth_authority TEXT NOT NULL DEFAULT 'local', authority_credential_version BIGINT NOT NULL DEFAULT 0,
	 sub2_api_user_id BIGINT NOT NULL DEFAULT 0, token_version BIGINT NOT NULL DEFAULT 0,
	 status TEXT NOT NULL DEFAULT 'active', email_verified_at TIMESTAMPTZ, deleted_at TIMESTAMPTZ,
	 token_invalid_before TIMESTAMPTZ, updated_at TIMESTAMPTZ);
	 CREATE TABLE sub2api_credential_watermarks (sub2_api_user_id BIGINT PRIMARY KEY,
	 credential_version BIGINT NOT NULL, last_event_id BIGINT NOT NULL, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ)`)
	require.NoError(t, err)
	mainHash, shopHash := mustTestHash(t, "MainPassword123"), mustTestHash(t, "ShopPassword456")
	_, err = main.Exec("INSERT INTO users (email, password_hash) VALUES ('fixture@example.com', $1)", mainHash)
	require.NoError(t, err)
	_, err = shop.Exec("INSERT INTO users (email, password_hash, email_verified_at) VALUES ('fixture@example.com', $1, NOW())", shopHash)
	require.NoError(t, err)
	return main, shop, name
}

func TestPostgresMigrationApplyRetryAndPolicyDrift(t *testing.T) {
	main, shop, _ := migrationFixture(t)
	ctx := context.Background()
	plan, err := BuildPlan(ctx, main, shop, time.Now())
	require.NoError(t, err)
	result, err := Apply(ctx, main, shop, plan, 1, false)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, uint64(2), result[0].CredentialVersion)
	require.True(t, result[0].MainLegacyAdded)
	require.True(t, result[0].ShopAuthorityPromoted)
	result, err = Apply(ctx, main, shop, plan, 1, false)
	require.NoError(t, err)
	require.False(t, result[0].MainLegacyAdded)
	require.False(t, result[0].ShopAuthorityPromoted)
	var tokenVersion int
	require.NoError(t, shop.QueryRow("SELECT token_version FROM users WHERE id=1").Scan(&tokenVersion))
	require.Equal(t, 1, tokenVersion)
	_, err = main.Exec("UPDATE users SET status='disabled'; UPDATE users SET status='active'")
	require.NoError(t, err)
	_, err = Apply(ctx, main, shop, plan, 1, false)
	require.ErrorContains(t, err, "changed after planning")
}

// Credential events lock watermark -> user. A migration waiting for that
// watermark must not hold the user lock and deadlock the event transaction.
func TestPostgresMigrationUsesEventLockOrder(t *testing.T) {
	mainDB, shopDB, application := migrationFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	plan, err := BuildPlan(ctx, mainDB, shopDB, time.Now())
	require.NoError(t, err)
	main, err := loadMainUserByID(ctx, mainDB, 1, false)
	require.NoError(t, err)
	_, err = shopDB.Exec("INSERT INTO sub2api_credential_watermarks VALUES (1,0,0,NOW(),NOW())")
	require.NoError(t, err)
	event, err := shopDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer event.Rollback()
	_, err = event.ExecContext(ctx, "UPDATE sub2api_credential_watermarks SET credential_version=1 WHERE sub2_api_user_id=1")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := applyShopAuthority(ctx, shopDB, plan.Items[0], main); done <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := shopDB.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')", application).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	_, err = event.ExecContext(ctx, "SET LOCAL lock_timeout = '500ms'")
	require.NoError(t, err)
	_, lockErr := event.ExecContext(ctx, "UPDATE users SET updated_at=NOW() WHERE id=1")
	_ = event.Rollback()
	migrationErr := <-done
	require.NoError(t, lockErr, "migration held user lock while waiting for event watermark")
	require.NoError(t, migrationErr)
}
