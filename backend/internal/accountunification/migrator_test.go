package accountunification

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"golang.org/x/crypto/bcrypt"
)

func TestBuildPlanReadsBothDatabasesWithoutExportingVerifiers(t *testing.T) {
	mainDB, mainMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mainDB.Close()
	shopDB, shopMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer shopDB.Close()

	mainHash := mustTestHash(t, "MainPassword123")
	shopHash := mustTestHash(t, "ShopPassword456")
	mainMock.ExpectQuery("SELECT id, email, password_hash, COALESCE\\(legacy_shop_password_hash").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "legacy", "credential_version", "role", "status", "totp_enabled"}).
			AddRow(11, "user@example.com", mainHash, "", 4, "user", "active", false))
	shopMock.ExpectQuery("SELECT id, email, password_hash, COALESCE\\(legacy_sub2api_password_hash").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash", "legacy", "authority", "authority_version", "sub2api_user_id", "token_version", "status", "verified"}).
			AddRow(22, "user@example.com", shopHash, "", "local", 0, 0, 3, "active", true))

	plan, err := BuildPlan(context.Background(), mainDB, shopDB, time.Unix(123, 0))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Counts[ActionApply] != 1 || len(plan.Items) != 1 {
		t.Fatalf("unexpected plan counts/items: %#v %#v", plan.Counts, plan.Items)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), mainHash) || strings.Contains(string(encoded), shopHash) {
		t.Fatal("plan leaked a reusable password verifier")
	}
	if err := mainMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := shopMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClassifySafePairAndRedactHashes(t *testing.T) {
	mainHash := mustTestHash(t, "MainPassword123")
	shopHash := mustTestHash(t, "ShopPassword456")
	item := classify("user@example.com", []mainUser{{
		ID: 11, Email: "user@example.com", PasswordHash: mainHash,
		CredentialVersion: 4, Role: "user", Status: "active",
	}}, []shopUser{{
		ID: 22, Email: "user@example.com", PasswordHash: shopHash,
		AuthAuthority: "local", Status: "active", EmailVerified: true,
	}})
	if item.Action != ActionApply || item.Reason != "matched_safe_pair" {
		t.Fatalf("classification = %s/%s", item.Action, item.Reason)
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), mainHash) || strings.Contains(string(encoded), shopHash) {
		t.Fatal("plan leaked a reusable password verifier")
	}
}

func TestClassifyRejectsPrivilegedTOTPAndConflicts(t *testing.T) {
	mainHash := mustTestHash(t, "MainPassword123")
	shopHash := mustTestHash(t, "ShopPassword456")
	tests := []struct {
		name   string
		main   mainUser
		shop   shopUser
		reason string
	}{
		{
			name: "privileged", reason: "privileged_main_account",
			main: mainUser{ID: 1, Email: "u@example.com", PasswordHash: mainHash, Role: "admin", Status: "active"},
			shop: shopUser{ID: 2, Email: "u@example.com", PasswordHash: shopHash, Status: "active", EmailVerified: true},
		},
		{
			name: "totp", reason: "main_totp_enabled",
			main: mainUser{ID: 1, Email: "u@example.com", PasswordHash: mainHash, Role: "user", Status: "active", TOTPEnabled: true},
			shop: shopUser{ID: 2, Email: "u@example.com", PasswordHash: shopHash, Status: "active", EmailVerified: true},
		},
		{
			name: "binding conflict", reason: "shop_bound_to_different_main_user",
			main: mainUser{ID: 1, Email: "u@example.com", PasswordHash: mainHash, Role: "user", Status: "active"},
			shop: shopUser{ID: 2, Email: "u@example.com", PasswordHash: shopHash, Status: "active", EmailVerified: true, Sub2APIUserID: 999},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := classify("u@example.com", []mainUser{tt.main}, []shopUser{tt.shop})
			if item.Action != ActionManual || item.Reason != tt.reason {
				t.Fatalf("classification = %s/%s, want manual/%s", item.Action, item.Reason, tt.reason)
			}
		})
	}
}

func TestClassifyAlreadyApplied(t *testing.T) {
	mainHash := mustTestHash(t, "MainPassword123")
	shopHash := mustTestHash(t, "ShopPassword456")
	item := classify("u@example.com", []mainUser{{
		ID: 1, Email: "u@example.com", PasswordHash: mainHash, LegacyShopHash: shopHash,
		CredentialVersion: 9, Role: "user", Status: "active",
	}}, []shopUser{{
		ID: 2, Email: "u@example.com", PasswordHash: shopHash, LegacySub2APIHash: mainHash,
		AuthAuthority: "sub2api", AuthorityCredentialVersion: 9, Sub2APIUserID: 1,
		Status: "active", EmailVerified: true,
	}})
	if item.Action != ActionAlreadyDone {
		t.Fatalf("classification = %s/%s", item.Action, item.Reason)
	}
}

func TestApplyMatchedPairRunsMainThenShopAndCanBeRetried(t *testing.T) {
	mainDB, mainMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer mainDB.Close()
	shopDB, shopMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer shopDB.Close()

	mainHash := mustTestHash(t, "MainPassword123")
	shopHash := mustTestHash(t, "ShopPassword456")
	item := PlanItem{
		Email: "user@example.com", Action: ActionApply, Reason: "matched_safe_pair",
		MainUserIDs: []int64{11}, ShopUserIDs: []uint64{22},
		MainPasswordFingerprint: passwordFingerprint(mainHash),
		ShopPasswordFingerprint: passwordFingerprint(shopHash),
	}
	shopColumns := []string{"id", "email", "password_hash", "legacy", "authority", "authority_version", "sub2api_user_id", "token_version", "status", "verified"}
	mainColumns := []string{"id", "email", "password_hash", "legacy", "credential_version", "role", "status", "totp_enabled"}

	// Read Shop verifier before the Main-side transaction.
	shopMock.ExpectQuery("SELECT id, email, password_hash").WithArgs(uint64(22)).
		WillReturnRows(sqlmock.NewRows(shopColumns).AddRow(22, "user@example.com", shopHash, "", "local", 0, 0, 3, "active", true))

	mainMock.ExpectBegin()
	mainMock.ExpectQuery("SELECT id, email, password_hash").WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows(mainColumns).AddRow(11, "user@example.com", mainHash, "", 4, "user", "active", false))
	mainMock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE LOWER").WithArgs("user@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mainMock.ExpectQuery("UPDATE users").WithArgs(shopHash, int64(11), mainHash).
		WillReturnRows(sqlmock.NewRows([]string{"credential_version"}).AddRow(5))
	mainMock.ExpectCommit()
	// Re-read Main after commit before promoting Shop authority.
	mainMock.ExpectQuery("SELECT id, email, password_hash").WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows(mainColumns).AddRow(11, "user@example.com", mainHash, shopHash, 5, "user", "active", false))

	shopMock.ExpectBegin()
	shopMock.ExpectQuery("SELECT id, email, password_hash").WithArgs(uint64(22)).
		WillReturnRows(sqlmock.NewRows(shopColumns).AddRow(22, "user@example.com", shopHash, "", "local", 0, 0, 3, "active", true))
	shopMock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE LOWER").WithArgs("user@example.com").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	shopMock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users WHERE sub2_api_user_id").WithArgs(int64(11), uint64(22)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	shopMock.ExpectExec("INSERT INTO sub2api_credential_watermarks").WithArgs(int64(11)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	shopMock.ExpectQuery("SELECT credential_version").WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"credential_version"}).AddRow(0))
	shopMock.ExpectExec("UPDATE sub2api_credential_watermarks").WithArgs(uint64(5), int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	shopMock.ExpectExec("UPDATE users").
		WithArgs(mainHash, int64(11), uint64(5), uint64(22), shopHash).
		WillReturnResult(sqlmock.NewResult(0, 1))
	shopMock.ExpectCommit()

	results, err := Apply(context.Background(), mainDB, shopDB, &Plan{
		Version: PlanVersion,
		Items:   []PlanItem{item},
	}, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].MainLegacyAdded || !results[0].ShopAuthorityPromoted || results[0].CredentialVersion != 5 {
		t.Fatalf("unexpected results: %#v", results)
	}
	if err := mainMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := shopMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func mustTestHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcryptGenerate([]byte(password))
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

var bcryptGenerate = func(password []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(password, bcrypt.MinCost)
}
