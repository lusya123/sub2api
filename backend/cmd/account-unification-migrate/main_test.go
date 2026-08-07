package main

import "testing"

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
