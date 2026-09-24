package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/accountunification"
)

func TestMigrationTargetRequiresExplicitSafeLabel(t *testing.T) {
	t.Setenv("ACCOUNT_UNIFICATION_TARGET", "")
	if _, err := migrationTarget(); err == nil {
		t.Fatal("empty target was accepted")
	}
	t.Setenv("ACCOUNT_UNIFICATION_TARGET", "Staging")
	target, err := migrationTarget()
	if err != nil || target != "staging" {
		t.Fatalf("target=%q err=%v", target, err)
	}
	t.Setenv("ACCOUNT_UNIFICATION_TARGET", "staging;production")
	if _, err := migrationTarget(); err == nil {
		t.Fatal("unsafe target label was accepted")
	}
}

func TestShopOnlyApplyRequiresSeparateExplicitConfirmation(t *testing.T) {
	t.Setenv("ACCOUNT_UNIFICATION_TARGET", "staging")
	path := filepath.Join(t.TempDir(), "plan.json")
	digest, err := accountunification.WritePlan(path, &accountunification.Plan{
		Version: accountunification.PlanVersion, Target: "staging", GeneratedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--plan", path, "--plan-sha256", digest, "--cohort", "shop-only", "--confirm", "APPLY_MATCHED_ACCOUNTS_TO_STAGING"}
	err = runApply(args)
	if err == nil || !strings.Contains(err.Error(), "CREATE_SHOP_ONLY_MAIN_ACCOUNTS_IN_STAGING") {
		t.Fatalf("matched-account confirmation authorized Shop-only creation: %v", err)
	}
}
