package accountunification

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	PlanVersion       = 1
	ActionApply       = "apply"
	ActionAlreadyDone = "already_applied"
	ActionManual      = "manual_review"
)

type Plan struct {
	Version     int            `json:"version"`
	Target      string         `json:"target"`
	GeneratedAt time.Time      `json:"generated_at"`
	Counts      map[string]int `json:"counts"`
	Items       []PlanItem     `json:"items"`
}

// PlanItem deliberately stores only one-way password-hash fingerprints. The
// reusable bcrypt verifiers are read again from both databases during apply.
type PlanItem struct {
	Email                   string   `json:"email"`
	Action                  string   `json:"action"`
	Reason                  string   `json:"reason"`
	MainUserIDs             []int64  `json:"main_user_ids,omitempty"`
	ShopUserIDs             []uint64 `json:"shop_user_ids,omitempty"`
	MainPasswordFingerprint string   `json:"main_password_fingerprint,omitempty"`
	ShopPasswordFingerprint string   `json:"shop_password_fingerprint,omitempty"`
	MainCredentialVersion   uint64   `json:"main_credential_version,omitempty"`
	ShopAuthorityVersion    uint64   `json:"shop_authority_version,omitempty"`
	ShopTokenVersion        uint64   `json:"shop_token_version,omitempty"`
	ShopAuthAuthority       string   `json:"shop_auth_authority,omitempty"`
	ShopBoundSub2APIUserID  int64    `json:"shop_bound_sub2api_user_id,omitempty"`
}

type ApplyResult struct {
	Email                 string `json:"email"`
	MainUserID            int64  `json:"main_user_id"`
	ShopUserID            uint64 `json:"shop_user_id"`
	CredentialVersion     uint64 `json:"credential_version"`
	MainLegacyAdded       bool   `json:"main_legacy_added"`
	ShopAuthorityPromoted bool   `json:"shop_authority_promoted"`
}

type mainUser struct {
	ID                int64
	Email             string
	PasswordHash      string
	LegacyShopHash    string
	CredentialVersion uint64
	Role              string
	Status            string
	TOTPEnabled       bool
}

type shopUser struct {
	ID                         uint64
	Email                      string
	PasswordHash               string
	LegacySub2APIHash          string
	AuthAuthority              string
	AuthorityCredentialVersion uint64
	Sub2APIUserID              int64
	TokenVersion               uint64
	Status                     string
	EmailVerified              bool
}

func BuildPlan(ctx context.Context, mainDB, shopDB *sql.DB, now time.Time) (*Plan, error) {
	if mainDB == nil || shopDB == nil {
		return nil, errors.New("both database connections are required")
	}
	mainUsers, err := loadMainUsers(ctx, mainDB)
	if err != nil {
		return nil, fmt.Errorf("load Main users (migrations 193-195 must already exist): %w", err)
	}
	shopUsers, err := loadShopUsers(ctx, shopDB)
	if err != nil {
		return nil, fmt.Errorf("load Shop users (account-unification columns must already exist): %w", err)
	}

	mainByEmail := make(map[string][]mainUser)
	shopByEmail := make(map[string][]shopUser)
	emails := make(map[string]struct{})
	for _, user := range mainUsers {
		email := normalizeEmail(user.Email)
		mainByEmail[email] = append(mainByEmail[email], user)
		emails[email] = struct{}{}
	}
	for _, user := range shopUsers {
		email := normalizeEmail(user.Email)
		shopByEmail[email] = append(shopByEmail[email], user)
		emails[email] = struct{}{}
	}

	orderedEmails := make([]string, 0, len(emails))
	for email := range emails {
		orderedEmails = append(orderedEmails, email)
	}
	sort.Strings(orderedEmails)

	plan := &Plan{
		Version:     PlanVersion,
		GeneratedAt: now.UTC(),
		Counts:      make(map[string]int),
		Items:       make([]PlanItem, 0, len(orderedEmails)),
	}
	for _, email := range orderedEmails {
		item := classify(email, mainByEmail[email], shopByEmail[email])
		plan.Items = append(plan.Items, item)
		plan.Counts[item.Action]++
		plan.Counts["reason:"+item.Reason]++
	}
	return plan, nil
}

func classify(email string, mains []mainUser, shops []shopUser) PlanItem {
	item := PlanItem{Email: email, Action: ActionManual}
	for _, user := range mains {
		item.MainUserIDs = append(item.MainUserIDs, user.ID)
	}
	for _, user := range shops {
		item.ShopUserIDs = append(item.ShopUserIDs, user.ID)
	}
	if email == "" {
		item.Reason = "empty_normalized_email"
		return item
	}
	if len(mains) == 0 {
		item.Reason = "shop_only"
		return item
	}
	if len(shops) == 0 {
		item.Reason = "main_only"
		return item
	}
	if len(mains) != 1 {
		item.Reason = "duplicate_main_email"
		return item
	}
	if len(shops) != 1 {
		item.Reason = "duplicate_shop_email"
		return item
	}

	main := mains[0]
	shop := shops[0]
	item.MainPasswordFingerprint = passwordFingerprint(main.PasswordHash)
	item.ShopPasswordFingerprint = passwordFingerprint(shop.PasswordHash)
	item.MainCredentialVersion = main.CredentialVersion
	item.ShopAuthorityVersion = shop.AuthorityCredentialVersion
	item.ShopTokenVersion = shop.TokenVersion
	item.ShopAuthAuthority = normalizedAuthority(shop.AuthAuthority)
	item.ShopBoundSub2APIUserID = shop.Sub2APIUserID

	switch {
	case !strings.EqualFold(strings.TrimSpace(main.Email), strings.TrimSpace(shop.Email)):
		item.Reason = "email_case_or_space_mismatch"
		return item
	case strings.ToLower(strings.TrimSpace(main.Role)) != "user":
		item.Reason = "privileged_main_account"
		return item
	case strings.ToLower(strings.TrimSpace(main.Status)) != "active":
		item.Reason = "inactive_main_account"
		return item
	case strings.ToLower(strings.TrimSpace(shop.Status)) != "active":
		item.Reason = "inactive_shop_account"
		return item
	case main.TOTPEnabled:
		item.Reason = "main_totp_enabled"
		return item
	case !shop.EmailVerified:
		item.Reason = "shop_email_unverified"
		return item
	case !isBcryptHash(main.PasswordHash):
		item.Reason = "unsupported_main_password_hash"
		return item
	case !isBcryptHash(shop.PasswordHash):
		item.Reason = "unsupported_shop_password_hash"
		return item
	}

	authority := normalizedAuthority(shop.AuthAuthority)
	if authority != "local" && authority != "sub2api" {
		item.Reason = "unknown_shop_auth_authority"
		return item
	}
	if shop.Sub2APIUserID != 0 && shop.Sub2APIUserID != main.ID {
		item.Reason = "shop_bound_to_different_main_user"
		return item
	}
	if main.LegacyShopHash != "" && main.LegacyShopHash != shop.PasswordHash && main.PasswordHash != shop.PasswordHash {
		item.Reason = "main_legacy_hash_conflict"
		return item
	}
	if shop.LegacySub2APIHash != "" && shop.LegacySub2APIHash != main.PasswordHash {
		item.Reason = "shop_legacy_hash_conflict"
		return item
	}
	if shop.AuthorityCredentialVersion > main.CredentialVersion {
		item.Reason = "shop_authority_version_ahead"
		return item
	}

	mainAcceptsShop := main.PasswordHash == shop.PasswordHash || main.LegacyShopHash == shop.PasswordHash
	shopMirrorReady := shop.LegacySub2APIHash == main.PasswordHash
	if authority == "sub2api" && shop.Sub2APIUserID == main.ID && mainAcceptsShop && shopMirrorReady && shop.AuthorityCredentialVersion == main.CredentialVersion {
		item.Action = ActionAlreadyDone
		item.Reason = "already_applied"
		return item
	}

	item.Action = ActionApply
	item.Reason = "matched_safe_pair"
	return item
}

func Apply(ctx context.Context, mainDB, shopDB *sql.DB, plan *Plan, maxUsers int, applyAll bool) ([]ApplyResult, error) {
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
		if item.Action != ActionApply {
			continue
		}
		if !applyAll && len(results) >= maxUsers {
			break
		}
		result, err := applyItem(ctx, mainDB, shopDB, item)
		if err != nil {
			return results, fmt.Errorf("apply %q: %w", item.Email, err)
		}
		results = append(results, result)
	}
	return results, nil
}

func applyItem(ctx context.Context, mainDB, shopDB *sql.DB, item PlanItem) (ApplyResult, error) {
	if len(item.MainUserIDs) != 1 || len(item.ShopUserIDs) != 1 {
		return ApplyResult{}, errors.New("plan item is not a unique pair")
	}
	shop, err := loadShopUserByID(ctx, shopDB, item.ShopUserIDs[0], false)
	if err != nil {
		return ApplyResult{}, err
	}
	if err := validateShopAgainstPlan(shop, item); err != nil {
		return ApplyResult{}, err
	}

	main, mainLegacyAdded, err := applyMainLegacy(ctx, mainDB, item, shop.PasswordHash)
	if err != nil {
		return ApplyResult{}, err
	}

	// Re-read Main after its transaction committed. A concurrent password or
	// policy change makes the plan stale and must stop Shop promotion.
	main, err = loadMainUserByID(ctx, mainDB, main.ID, false)
	if err != nil {
		return ApplyResult{}, err
	}
	if err := validateMainForPromotion(main, item, shop.PasswordHash); err != nil {
		return ApplyResult{}, err
	}

	promoted, err := applyShopAuthority(ctx, shopDB, item, main)
	if err != nil {
		// The Main-side addition is intentionally left in place. Re-running the
		// same plan is idempotent and can finish this second phase safely.
		return ApplyResult{}, fmt.Errorf("Main compatibility credential is present; Shop phase can be retried safely: %w", err)
	}
	return ApplyResult{
		Email:                 item.Email,
		MainUserID:            main.ID,
		ShopUserID:            shop.ID,
		CredentialVersion:     main.CredentialVersion,
		MainLegacyAdded:       mainLegacyAdded,
		ShopAuthorityPromoted: promoted,
	}, nil
}

func applyMainLegacy(ctx context.Context, db *sql.DB, item PlanItem, shopPasswordHash string) (mainUser, bool, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return mainUser{}, false, err
	}
	defer tx.Rollback()

	main, err := loadMainUserByID(ctx, tx, item.MainUserIDs[0], true)
	if err != nil {
		return mainUser{}, false, err
	}
	if err := validateMainForPromotion(main, item, shopPasswordHash); err != nil {
		return mainUser{}, false, err
	}
	if err := requireUniqueNormalizedEmail(ctx, tx, "users", main.ID, item.Email); err != nil {
		return mainUser{}, false, err
	}

	added := false
	if main.PasswordHash != shopPasswordHash && main.LegacyShopHash == "" {
		if err := tx.QueryRowContext(ctx, `
			UPDATE users
			SET legacy_shop_password_hash = $1
			WHERE id = $2 AND deleted_at IS NULL AND password_hash = $3
			RETURNING credential_version`, shopPasswordHash, main.ID, main.PasswordHash).Scan(&main.CredentialVersion); err != nil {
			return mainUser{}, false, fmt.Errorf("set Main legacy Shop password: %w", err)
		}
		main.LegacyShopHash = shopPasswordHash
		added = true
	}
	if err := tx.Commit(); err != nil {
		return mainUser{}, false, err
	}
	return main, added, nil
}

func applyShopAuthority(ctx context.Context, db *sql.DB, item PlanItem, main mainUser) (bool, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	shop, err := loadShopUserByID(ctx, tx, item.ShopUserIDs[0], true)
	if err != nil {
		return false, err
	}
	if err := validateShopAgainstPlan(shop, item); err != nil {
		return false, err
	}
	if err := requireUniqueNormalizedEmail(ctx, tx, "users", int64(shop.ID), item.Email); err != nil {
		return false, err
	}
	if shop.Sub2APIUserID != 0 && shop.Sub2APIUserID != main.ID {
		return false, errors.New("Shop user was concurrently bound to a different Main user")
	}
	var bindingCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE sub2_api_user_id = $1 AND id <> $2 AND deleted_at IS NULL`, main.ID, shop.ID).Scan(&bindingCount); err != nil {
		return false, err
	}
	if bindingCount != 0 {
		return false, errors.New("Main user ID is already bound to another Shop user")
	}
	if shop.LegacySub2APIHash != "" && shop.LegacySub2APIHash != main.PasswordHash {
		return false, errors.New("Shop legacy Main password changed after planning")
	}
	if shop.AuthorityCredentialVersion > main.CredentialVersion {
		return false, errors.New("Shop authority version is ahead of Main")
	}
	if shop.TokenVersion == math.MaxUint64 {
		return false, errors.New("Shop token version exhausted")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sub2api_credential_watermarks
			(sub2_api_user_id, credential_version, last_event_id, created_at, updated_at)
		VALUES ($1, 0, 0, NOW(), NOW())
		ON CONFLICT (sub2_api_user_id) DO NOTHING`, main.ID); err != nil {
		return false, fmt.Errorf("seed Shop credential watermark: %w", err)
	}
	var watermark uint64
	if err := tx.QueryRowContext(ctx, `
		SELECT credential_version
		FROM sub2api_credential_watermarks
		WHERE sub2_api_user_id = $1
		FOR UPDATE`, main.ID).Scan(&watermark); err != nil {
		return false, fmt.Errorf("lock Shop credential watermark: %w", err)
	}
	if watermark > main.CredentialVersion {
		return false, errors.New("Shop credential watermark is ahead of Main proof")
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sub2api_credential_watermarks
		SET credential_version = $1, updated_at = NOW()
		WHERE sub2_api_user_id = $2`, main.CredentialVersion, main.ID); err != nil {
		return false, fmt.Errorf("advance Shop credential watermark: %w", err)
	}

	changed := normalizedAuthority(shop.AuthAuthority) != "sub2api" ||
		shop.Sub2APIUserID != main.ID ||
		shop.LegacySub2APIHash != main.PasswordHash ||
		shop.AuthorityCredentialVersion != main.CredentialVersion
	if changed {
		result, err := tx.ExecContext(ctx, `
			UPDATE users
			SET legacy_sub2api_password_hash = $1,
				auth_authority = 'sub2api',
				sub2_api_user_id = $2,
				authority_credential_version = $3,
				token_version = token_version + 1,
				token_invalid_before = NOW(),
				updated_at = NOW()
			WHERE id = $4 AND deleted_at IS NULL AND password_hash = $5`,
			main.PasswordHash, main.ID, main.CredentialVersion, shop.ID, shop.PasswordHash)
		if err != nil {
			return false, fmt.Errorf("promote Shop authentication authority: %w", err)
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			return false, errors.New("Shop authority compare-and-swap did not update exactly one row")
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return changed, nil
}

func validateMainForPromotion(main mainUser, item PlanItem, shopPasswordHash string) error {
	if normalizeEmail(main.Email) != item.Email || passwordFingerprint(main.PasswordHash) != item.MainPasswordFingerprint {
		return errors.New("Main identity or primary password changed after planning")
	}
	if strings.ToLower(strings.TrimSpace(main.Role)) != "user" || strings.ToLower(strings.TrimSpace(main.Status)) != "active" || main.TOTPEnabled {
		return errors.New("Main role, status, or TOTP policy is no longer eligible")
	}
	if !isBcryptHash(main.PasswordHash) || !isBcryptHash(shopPasswordHash) {
		return errors.New("unsupported password hash")
	}
	if main.LegacyShopHash != "" && main.LegacyShopHash != shopPasswordHash && main.PasswordHash != shopPasswordHash {
		return errors.New("Main legacy Shop password conflicts with the planned Shop password")
	}
	return nil
}

func validateShopAgainstPlan(shop shopUser, item PlanItem) error {
	if normalizeEmail(shop.Email) != item.Email || passwordFingerprint(shop.PasswordHash) != item.ShopPasswordFingerprint {
		return errors.New("Shop identity or primary password changed after planning")
	}
	if strings.ToLower(strings.TrimSpace(shop.Status)) != "active" || !shop.EmailVerified {
		return errors.New("Shop status or email verification is no longer eligible")
	}
	authority := normalizedAuthority(shop.AuthAuthority)
	if authority != "local" && authority != "sub2api" {
		return errors.New("unknown Shop authentication authority")
	}
	return nil
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadMainUserByID(ctx context.Context, q rowQuerier, id int64, forUpdate bool) (mainUser, error) {
	query := `
		SELECT id, email, password_hash, COALESCE(legacy_shop_password_hash, ''),
			credential_version, role, status, COALESCE(totp_enabled, FALSE)
		FROM users WHERE id = $1 AND deleted_at IS NULL`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var user mainUser
	err := q.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.LegacyShopHash,
		&user.CredentialVersion, &user.Role, &user.Status, &user.TOTPEnabled,
	)
	if err != nil {
		return mainUser{}, fmt.Errorf("load Main user %d: %w", id, err)
	}
	return user, nil
}

func loadShopUserByID(ctx context.Context, q rowQuerier, id uint64, forUpdate bool) (shopUser, error) {
	query := `
		SELECT id, email, password_hash, COALESCE(legacy_sub2api_password_hash, ''),
			COALESCE(NULLIF(auth_authority, ''), 'local'), authority_credential_version,
			sub2_api_user_id, token_version, status, email_verified_at IS NOT NULL
		FROM users WHERE id = $1 AND deleted_at IS NULL`
	if forUpdate {
		query += " FOR UPDATE"
	}
	var user shopUser
	err := q.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.LegacySub2APIHash,
		&user.AuthAuthority, &user.AuthorityCredentialVersion, &user.Sub2APIUserID,
		&user.TokenVersion, &user.Status, &user.EmailVerified,
	)
	if err != nil {
		return shopUser{}, fmt.Errorf("load Shop user %d: %w", id, err)
	}
	return user, nil
}

func loadMainUsers(ctx context.Context, db *sql.DB) ([]mainUser, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, email, password_hash, COALESCE(legacy_shop_password_hash, ''),
			credential_version, role, status, COALESCE(totp_enabled, FALSE)
		FROM users WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []mainUser
	for rows.Next() {
		var user mainUser
		if err := rows.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.LegacyShopHash, &user.CredentialVersion, &user.Role, &user.Status, &user.TOTPEnabled); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func loadShopUsers(ctx context.Context, db *sql.DB) ([]shopUser, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, email, password_hash, COALESCE(legacy_sub2api_password_hash, ''),
			COALESCE(NULLIF(auth_authority, ''), 'local'), authority_credential_version,
			sub2_api_user_id, token_version, status, email_verified_at IS NOT NULL
		FROM users WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []shopUser
	for rows.Next() {
		var user shopUser
		if err := rows.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.LegacySub2APIHash, &user.AuthAuthority, &user.AuthorityCredentialVersion, &user.Sub2APIUserID, &user.TokenVersion, &user.Status, &user.EmailVerified); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func requireUniqueNormalizedEmail(ctx context.Context, q rowQuerier, table string, id int64, email string) error {
	if table != "users" {
		return errors.New("unsupported identity table")
	}
	var count int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE LOWER(TRIM(email)) = $1 AND deleted_at IS NULL`, email).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("normalized email no longer resolves uniquely for user %d", id)
	}
	return nil
}

func WritePlan(path string, plan *Plan) (string, error) {
	if strings.TrimSpace(path) == "" || plan == nil {
		return "", errors.New("plan output path and plan are required")
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := writePrivateFile(path, data); err != nil {
		return "", err
	}
	return bytesSHA256(data), nil
}

func ReadPlan(path string) (*Plan, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var plan Plan
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return nil, "", err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, "", errors.New("plan contains trailing JSON data")
	}
	return &plan, bytesSHA256(data), nil
}

func WriteResults(path string, results []ApplyResult) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return writePrivateFile(path, append(data, '\n'))
}

func writePrivateFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizedAuthority(authority string) string {
	authority = strings.ToLower(strings.TrimSpace(authority))
	if authority == "" {
		return "local"
	}
	return authority
}

func isBcryptHash(hash string) bool {
	if hash == "" {
		return false
	}
	_, err := bcrypt.Cost([]byte(hash))
	return err == nil
}

func passwordFingerprint(hash string) string {
	if hash == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(hash))
	return hex.EncodeToString(sum[:])
}

func bytesSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
