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
	 username TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '',
	 balance NUMERIC(20,8) NOT NULL DEFAULT 0, concurrency INT NOT NULL DEFAULT 5,
	 rpm_limit INT NOT NULL DEFAULT 0,
	 password_hash TEXT NOT NULL, legacy_shop_password_hash TEXT, role TEXT NOT NULL DEFAULT 'user',
	 status TEXT NOT NULL DEFAULT 'active', totp_enabled BOOLEAN DEFAULT FALSE,
	 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ)`)
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
	 balance NUMERIC(20,8) NOT NULL DEFAULT 0,
	 status TEXT NOT NULL DEFAULT 'active', email_verified_at TIMESTAMPTZ,
	 password_setup_required BOOLEAN NOT NULL DEFAULT FALSE, deleted_at TIMESTAMPTZ,
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

func TestPostgresShopOnlyCreatesIndependentZeroCreditMainIdentity(t *testing.T) {
	main, shop, _ := migrationFixture(t)
	ctx := context.Background()
	shopHash := mustTestHash(t, "ShopOnlyPassword123")
	_, err := shop.Exec(`INSERT INTO users (email, password_hash, email_verified_at, balance) VALUES ('shop-only@example.com', $1, NOW(), 42)`, shopHash)
	require.NoError(t, err)
	plan, err := BuildPlan(ctx, main, shop, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, plan.Counts[ActionCreateMain])

	results, err := ApplyShopOnly(ctx, main, shop, plan, 1, false)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].MainCreated)
	var mainHash, role, status string
	var balance string
	var concurrency, rpmLimit int
	err = main.QueryRow(`SELECT password_hash, role, status, balance, concurrency, rpm_limit FROM users WHERE email='shop-only@example.com'`).
		Scan(&mainHash, &role, &status, &balance, &concurrency, &rpmLimit)
	require.NoError(t, err)
	require.Equal(t, shopHash, mainHash)
	require.Equal(t, "user", role)
	require.Equal(t, "active", status)
	require.Equal(t, "0.00000000", balance)
	require.Equal(t, 5, concurrency)
	require.Zero(t, rpmLimit)
	var authority string
	var binding int64
	err = shop.QueryRow(`SELECT auth_authority, sub2_api_user_id FROM users WHERE email='shop-only@example.com'`).Scan(&authority, &binding)
	require.NoError(t, err)
	require.Equal(t, "sub2api", authority)
	require.Equal(t, results[0].MainUserID, binding)
	var shopBalance string
	require.NoError(t, shop.QueryRow(`SELECT balance FROM users WHERE email='shop-only@example.com'`).Scan(&shopBalance))
	require.Equal(t, "42.00000000", shopBalance)

	retried, err := ApplyShopOnly(ctx, main, shop, plan, 1, false)
	require.NoError(t, err)
	require.Len(t, retried, 1)
	require.False(t, retried[0].MainCreated)
	var mirrorCount int
	require.NoError(t, main.QueryRow(`SELECT COUNT(*) FROM users WHERE email='shop-only@example.com'`).Scan(&mirrorCount))
	require.Equal(t, 1, mirrorCount)
}

func TestPostgresShopOnlyRefusesAliasCreatedAfterPlanning(t *testing.T) {
	main, shop, _ := migrationFixture(t)
	ctx := context.Background()
	hash := mustTestHash(t, "ShopOnlyPassword123")
	_, err := shop.Exec(`INSERT INTO users (email, password_hash, email_verified_at) VALUES ('john.smith@gmail.com', $1, NOW())`, hash)
	require.NoError(t, err)
	plan, err := BuildPlan(ctx, main, shop, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, plan.Counts[ActionCreateMain])
	_, err = main.Exec(`INSERT INTO users (email, password_hash) VALUES ('johnsmith+new@googlemail.com', $1)`, hash)
	require.NoError(t, err)
	_, err = ApplyShopOnly(ctx, main, shop, plan, 1, false)
	require.ErrorContains(t, err, "occupied")
	var mirrorCount int
	require.NoError(t, main.QueryRow(`SELECT COUNT(*) FROM users WHERE email='john.smith@gmail.com'`).Scan(&mirrorCount))
	require.Zero(t, mirrorCount)
}

func TestPostgresShopOnlyResumesDisabledMainMirror(t *testing.T) {
	main, shop, _ := migrationFixture(t)
	ctx := context.Background()
	hash := mustTestHash(t, "ShopOnlyPassword123")
	_, err := shop.Exec(`INSERT INTO users (email, password_hash, email_verified_at) VALUES ('retry@example.com', $1, NOW())`, hash)
	require.NoError(t, err)
	plan, err := BuildPlan(ctx, main, shop, time.Now())
	require.NoError(t, err)
	var shopID uint64
	require.NoError(t, shop.QueryRow(`SELECT id FROM users WHERE email='retry@example.com'`).Scan(&shopID))
	_, err = main.Exec(`INSERT INTO users (email, password_hash, notes, status, balance) VALUES ('retry@example.com', $1, $2, 'disabled', 0)`, hash, fmt.Sprintf("shop-account-mirror:%d", shopID))
	require.NoError(t, err)
	result, err := ApplyShopOnly(ctx, main, shop, plan, 1, false)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.False(t, result[0].MainCreated)
	var status string
	require.NoError(t, main.QueryRow(`SELECT status FROM users WHERE email='retry@example.com'`).Scan(&status))
	require.Equal(t, "active", status)
}

func TestPostgresShopOnlyRetryNeverReenablesAdministrativelyDisabledMainUser(t *testing.T) {
	main, shop, _ := migrationFixture(t)
	ctx := context.Background()
	hash := mustTestHash(t, "ShopOnlyPassword123")
	_, err := shop.Exec(`INSERT INTO users (email, password_hash, email_verified_at) VALUES ('disabled@example.com', $1, NOW())`, hash)
	require.NoError(t, err)
	plan, err := BuildPlan(ctx, main, shop, time.Now())
	require.NoError(t, err)
	_, err = ApplyShopOnly(ctx, main, shop, plan, 1, false)
	require.NoError(t, err)
	_, err = main.Exec(`UPDATE users SET status='disabled' WHERE email='disabled@example.com'`)
	require.NoError(t, err)
	_, err = ApplyShopOnly(ctx, main, shop, plan, 1, false)
	require.Error(t, err)
	var status string
	require.NoError(t, main.QueryRow(`SELECT status FROM users WHERE email='disabled@example.com'`).Scan(&status))
	require.Equal(t, "disabled", status)
}

func TestPostgresShopOnlyRetryDoesNotActivateMainWhenShopWasDisabled(t *testing.T) {
	main, shop, _ := migrationFixture(t)
	ctx := context.Background()
	hash := mustTestHash(t, "ShopOnlyPassword123")
	_, err := shop.Exec(`INSERT INTO users (email, password_hash, email_verified_at) VALUES ('shop-disabled@example.com', $1, NOW())`, hash)
	require.NoError(t, err)
	plan, err := BuildPlan(ctx, main, shop, time.Now())
	require.NoError(t, err)
	var shopID uint64
	require.NoError(t, shop.QueryRow(`SELECT id FROM users WHERE email='shop-disabled@example.com'`).Scan(&shopID))
	var mainID int64
	err = main.QueryRow(`INSERT INTO users (email, password_hash, notes, status, balance) VALUES ('shop-disabled@example.com', $1, $2, 'disabled', 0) RETURNING id`, hash, fmt.Sprintf("shop-account-mirror:%d", shopID)).Scan(&mainID)
	require.NoError(t, err)
	_, err = shop.Exec(`UPDATE users SET auth_authority='sub2api', sub2_api_user_id=$1,
	 legacy_sub2api_password_hash=$2, authority_credential_version=1, status='disabled'
	 WHERE id=$3`, mainID, hash, shopID)
	require.NoError(t, err)
	_, err = ApplyShopOnly(ctx, main, shop, plan, 1, false)
	require.Error(t, err)
	var status string
	require.NoError(t, main.QueryRow(`SELECT status FROM users WHERE id=$1`, mainID).Scan(&status))
	require.Equal(t, "disabled", status)
}
