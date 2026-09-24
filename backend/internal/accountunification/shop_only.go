package accountunification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
)

// ApplyShopOnly is deliberately separate from Apply: an operator must select
// this cohort explicitly, and an old matched-pair command cannot create users.
func ApplyShopOnly(ctx context.Context, mainDB, shopDB *sql.DB, plan *Plan, maxUsers int, applyAll bool) ([]ApplyResult, error) {
	if mainDB == nil || shopDB == nil || plan == nil {
		return nil, errors.New("both database connections and a plan are required")
	}
	if plan.Version != PlanVersion {
		return nil, fmt.Errorf("unsupported plan version %d", plan.Version)
	}
	if !applyAll && maxUsers <= 0 {
		return nil, errors.New("max-users must be positive unless --all is set")
	}
	results := make([]ApplyResult, 0)
	for _, item := range plan.Items {
		if item.Action != ActionCreateMain {
			continue
		}
		if !applyAll && len(results) >= maxUsers {
			break
		}
		result, err := applyShopOnlyItem(ctx, mainDB, shopDB, item)
		if err != nil {
			return results, fmt.Errorf("create Main identity for %q: %w", item.Email, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func applyShopOnlyItem(ctx context.Context, mainDB, shopDB *sql.DB, item PlanItem) (ApplyResult, error) {
	if item.Action != ActionCreateMain || item.Reason != "shop_only_create_main" ||
		len(item.MainUserIDs) != 0 || len(item.ShopUserIDs) != 1 || item.Email == "" {
		return ApplyResult{}, errors.New("plan item is not a unique Shop-only candidate")
	}
	shop, err := loadShopUserByID(ctx, shopDB, item.ShopUserIDs[0], false)
	if err != nil {
		return ApplyResult{}, err
	}
	if normalizedAuthority(shop.AuthAuthority) == "local" {
		if err := validateShopAgainstPlan(shop, item); err != nil {
			return ApplyResult{}, err
		}
		if err := requireUniqueInbox(ctx, shopDB, item.Email, shop.ID); err != nil {
			return ApplyResult{}, fmt.Errorf("shop inbox collision: %w", err)
		}
	}

	main, created, err := createOrResumeMainMirror(ctx, mainDB, item, shop.PasswordHash)
	if err != nil {
		return ApplyResult{}, err
	}
	if normalizedAuthority(shop.AuthAuthority) == "sub2api" {
		if shop.Sub2APIUserID != main.ID || shop.LegacySub2APIHash != main.PasswordHash ||
			shop.AuthorityCredentialVersion == 0 || shop.AuthorityCredentialVersion > main.CredentialVersion {
			return ApplyResult{}, errors.New("shop authority does not match the resumable Main mirror")
		}
	} else {
		if main.Status != "disabled" {
			return ApplyResult{}, errors.New("main mirror is active before Shop authority promotion")
		}
		// The Main row stays disabled until Shop has atomically adopted its
		// stable ID. A stale Shop password or binding fails without making the
		// copied verifier usable at Main.
		if _, err := applyShopAuthority(ctx, shopDB, item, main); err != nil {
			return ApplyResult{}, fmt.Errorf("main mirror remains disabled; Shop promotion can be retried: %w", err)
		}
	}

	// Keep the Shop row locked while enabling Main. A concurrent disable or
	// password change must not slip between the final Shop proof and activation.
	gate, err := shopDB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return ApplyResult{}, err
	}
	defer func() { _ = gate.Rollback() }()
	currentShop, err := loadShopUserByID(ctx, gate, shop.ID, true)
	if err != nil {
		return ApplyResult{}, err
	}
	if normalizeEmail(currentShop.Email) != item.Email || normalizedAuthority(currentShop.AuthAuthority) != "sub2api" ||
		currentShop.Sub2APIUserID != main.ID || currentShop.LegacySub2APIHash != main.PasswordHash ||
		strings.ToLower(strings.TrimSpace(currentShop.Status)) != "active" || !currentShop.EmailVerified ||
		currentShop.PasswordSetupRequired || currentShop.AuthorityCredentialVersion == 0 ||
		currentShop.AuthorityCredentialVersion > main.CredentialVersion {
		return ApplyResult{}, errors.New("shop authority or status changed before Main activation")
	}
	if main.Status == "disabled" && (currentShop.TokenVersion != item.ShopTokenVersion+1 ||
		currentShop.AuthorityCredentialVersion != main.CredentialVersion) {
		return ApplyResult{}, errors.New("shop credential state changed before Main activation")
	}
	version, err := activateMainMirror(ctx, mainDB, main, item)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("shop is bound but Main mirror remains disabled; retry the same plan: %w", err)
	}
	if err := gate.Commit(); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{Email: item.Email, MainUserID: main.ID, ShopUserID: shop.ID,
		CredentialVersion: version, MainCreated: created, ShopAuthorityPromoted: shop.AuthAuthority != "sub2api"}, nil
}

type mirrorMain struct {
	mainUser
	Notes      string
	ZeroCredit bool
}

func createOrResumeMainMirror(ctx context.Context, db *sql.DB, item PlanItem, shopHash string) (mainUser, bool, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return mainUser{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	keys := []string{"users:normalized-email:" + item.Email, "users:email-alias-identity:" + inboxIdentity(item.Email)}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, mirrorLockHash(key)); err != nil {
			return mainUser{}, false, err
		}
	}
	rows, err := findInboxUsers(ctx, tx, item.Email)
	if err != nil {
		return mainUser{}, false, err
	}
	marker := fmt.Sprintf("shop-account-mirror:%d", item.ShopUserIDs[0])
	if len(rows) > 1 {
		return mainUser{}, false, errors.New("main inbox resolves to multiple users")
	}
	if len(rows) == 1 {
		row := rows[0]
		if normalizeEmail(row.Email) != item.Email || row.Notes != marker || row.PasswordHash != shopHash ||
			row.Role != "user" || (row.Status != "disabled" && row.Status != "active") ||
			(row.Status == "disabled" && (!row.ZeroCredit || row.CredentialVersion != 1)) {
			return mainUser{}, false, errors.New("main inbox is occupied by a different or changed account")
		}
		if err := ensureMirrorEmailIdentity(ctx, tx, row.ID, item.Email); err != nil {
			return mainUser{}, false, err
		}
		if err := tx.Commit(); err != nil {
			return mainUser{}, false, err
		}
		return row.mainUser, false, nil
	}
	if passwordFingerprint(shopHash) != item.ShopPasswordFingerprint || !isBcryptHash(shopHash) {
		return mainUser{}, false, errors.New("shop password changed after planning")
	}
	var main mainUser
	// Explicit zero entitlements override registration defaults. Inserting a
	// disabled user also prevents a usable stale password if phase two fails.
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (email, username, notes, password_hash, role, status, balance, rpm_limit)
		VALUES ($1, LEFT($4::text, 100), $2, $3, 'user', 'disabled', 0, 0)
		RETURNING id, credential_version`, item.Email, marker, shopHash, item.Email).Scan(&main.ID, &main.CredentialVersion)
	if err != nil {
		return mainUser{}, false, fmt.Errorf("insert disabled zero-credit Main mirror: %w", err)
	}
	if err := ensureMirrorEmailIdentity(ctx, tx, main.ID, item.Email); err != nil {
		return mainUser{}, false, err
	}
	main.Email, main.PasswordHash, main.Role, main.Status = item.Email, shopHash, "user", "disabled"
	if err := tx.Commit(); err != nil {
		return mainUser{}, false, err
	}
	return main, true, nil
}

// Direct SQL account creation must preserve the same email-identity invariant
// as the Main user repository. A retry also repairs a mirror created before
// this invariant was added, but refuses an identity owned by someone else.
func ensureMirrorEmailIdentity(ctx context.Context, tx *sql.Tx, userID int64, email string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO auth_identities (user_id, provider_type, provider_key, provider_subject, verified_at, metadata)
		VALUES ($1, 'email', 'email', $2, NOW(), '{"source":"shop_account_unification"}'::jsonb)
		ON CONFLICT (provider_type, provider_key, provider_subject) DO NOTHING`, userID, email)
	if err != nil {
		return fmt.Errorf("ensure Main email identity: %w", err)
	}
	var ownerID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT user_id FROM auth_identities
		WHERE provider_type = 'email' AND provider_key = 'email' AND provider_subject = $1`, email).Scan(&ownerID); err != nil {
		return fmt.Errorf("read Main email identity: %w", err)
	}
	if ownerID != userID {
		return errors.New("Main email identity is owned by another user")
	}
	return nil
}

func activateMainMirror(ctx context.Context, db *sql.DB, main mainUser, item PlanItem) (uint64, error) {
	if main.Status == "active" {
		return main.CredentialVersion, nil
	}
	if main.CredentialVersion != 1 {
		return 0, errors.New("disabled Main mirror is no longer at its initial credential version")
	}
	var version uint64
	err := db.QueryRowContext(ctx, `
		UPDATE users SET status = 'active', updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL AND status = 'disabled'
		  AND role = 'user' AND email = $2 AND password_hash = $3
		  AND notes = $4 AND credential_version = $5
		  AND balance = 0 AND concurrency > 0 AND rpm_limit = 0
		RETURNING credential_version`, main.ID, item.Email, main.PasswordHash,
		fmt.Sprintf("shop-account-mirror:%d", item.ShopUserIDs[0]), main.CredentialVersion).Scan(&version)
	if err != nil {
		return 0, err
	}
	return version, nil
}

func mirrorLockHash(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64())
}

// findInboxUsers uses a narrow dot-stripped candidate probe, then applies
// Main's inbox-equivalence rule in Go. The SQL may over-select but never
// authorizes an alias based on the broad probe alone.
func findInboxUsers(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, email string) ([]mirrorMain, error) {
	inbox := inboxIdentity(email)
	patterns, err := inboxLookupPatterns(email)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]struct{})
	var result []mirrorMain
	for _, pattern := range patterns {
		rows, err := q.QueryContext(ctx, `
			SELECT id, email, password_hash, notes, role, status, credential_version,
			       balance = 0 AND concurrency > 0 AND rpm_limit = 0
			FROM users WHERE deleted_at IS NULL
			  AND (REPLACE(LOWER(TRIM(email)), '.', '') = $1
			       OR REPLACE(LOWER(TRIM(email)), '.', '') LIKE $2 ESCAPE '')`, pattern.exact, pattern.plus)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var row mirrorMain
			if err := rows.Scan(&row.ID, &row.Email, &row.PasswordHash, &row.Notes, &row.Role,
				&row.Status, &row.CredentialVersion, &row.ZeroCredit); err != nil {
				_ = rows.Close()
				return nil, err
			}
			if inboxIdentity(row.Email) == inbox {
				if _, found := seen[row.ID]; !found {
					seen[row.ID] = struct{}{}
					result = append(result, row)
				}
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
	}
	return result, nil
}

func requireUniqueInbox(ctx context.Context, db *sql.DB, email string, shopID uint64) error {
	patterns, err := inboxLookupPatterns(email)
	if err != nil {
		return err
	}
	seen := make(map[uint64]struct{})
	for _, pattern := range patterns {
		rows, err := db.QueryContext(ctx, `
			SELECT id, email FROM users WHERE deleted_at IS NULL
			  AND (REPLACE(LOWER(TRIM(email)), '.', '') = $1
			       OR REPLACE(LOWER(TRIM(email)), '.', '') LIKE $2 ESCAPE '')`, pattern.exact, pattern.plus)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id uint64
			var stored string
			if err := rows.Scan(&id, &stored); err != nil {
				_ = rows.Close()
				return err
			}
			if inboxIdentity(stored) == inboxIdentity(email) {
				if id != shopID {
					_ = rows.Close()
					return errors.New("another Shop user has the same inbox")
				}
				seen[id] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		_ = rows.Close()
	}
	if len(seen) != 1 {
		return errors.New("shop inbox no longer resolves uniquely")
	}
	return nil
}

type inboxPattern struct{ exact, plus string }

func inboxLookupPatterns(email string) ([]inboxPattern, error) {
	local, domain, ok := strings.Cut(inboxIdentity(email), "@")
	if !ok || local == "" || domain == "" {
		return nil, errors.New("shop email has no valid inbox identity")
	}
	domains := []string{domain}
	if domain == "gmail.com" {
		domains = append(domains, "googlemail.com")
	}
	base := strings.ReplaceAll(local, ".", "")
	patterns := make([]inboxPattern, 0, len(domains))
	for _, candidateDomain := range domains {
		dotless := strings.ReplaceAll(candidateDomain, ".", "")
		patterns = append(patterns, inboxPattern{exact: base + "@" + dotless, plus: base + "+%@" + dotless})
	}
	return patterns, nil
}
