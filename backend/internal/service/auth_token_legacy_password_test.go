package service

import "testing"

func TestResolvedTokenVersionIncludesLegacyPasswordVerifier(t *testing.T) {
	legacyA := "$2a$10$legacy-a"
	legacyB := "$2a$10$legacy-b"
	base := &User{Email: "user@example.com", PasswordHash: "$2a$10$primary", TokenVersion: 7}

	withoutLegacy := resolvedTokenVersion(base)
	base.LegacyShopPasswordHash = &legacyA
	withLegacyA := resolvedTokenVersion(base)
	base.LegacyShopPasswordHash = &legacyB
	withLegacyB := resolvedTokenVersion(base)

	if withoutLegacy == withLegacyA || withLegacyA == withLegacyB || withoutLegacy == withLegacyB {
		t.Fatalf("credential fingerprint did not change across legacy verifier states")
	}
	base.TokenVersionResolved = true
	if got := resolvedTokenVersion(base); got != base.TokenVersion {
		t.Fatalf("pre-resolved token version changed: got %d want %d", got, base.TokenVersion)
	}
}
