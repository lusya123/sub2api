package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserCheckPassword_DistinctLegacyAlwaysEvaluatesBothVerifiers(t *testing.T) {
	legacy := "legacy-hash"
	user := &User{PasswordHash: "primary-hash", LegacyShopPasswordHash: &legacy, Role: RoleUser}
	var compared []string
	compare := func(hash, _ []byte) error {
		compared = append(compared, string(hash))
		if string(hash) == "primary-hash" {
			return nil
		}
		return errors.New("mismatch")
	}

	require.True(t, user.checkPasswordWithComparator("password", compare))
	require.Equal(t, []string{"primary-hash", "legacy-hash"}, compared)
}

func TestUserCheckPassword_WithoutDistinctLegacyEvaluatesOneVerifier(t *testing.T) {
	user := &User{PasswordHash: "primary-hash", Role: RoleUser}
	var calls int
	compare := func(_, _ []byte) error {
		calls++
		return errors.New("mismatch")
	}

	require.False(t, user.checkPasswordWithComparator("password", compare))
	require.Equal(t, 1, calls)
}

func TestUserCheckPassword_NeverUsesLegacyVerifierForPrivilegedOrUnknownRole(t *testing.T) {
	legacy := "legacy-hash"
	for _, role := range []string{RoleAdmin, RoleOperator, "BillingAdmin", ""} {
		t.Run(role, func(t *testing.T) {
			user := &User{
				PasswordHash:           "primary-hash",
				LegacyShopPasswordHash: &legacy,
				Role:                   role,
			}
			var compared []string
			compare := func(hash, password []byte) error {
				compared = append(compared, string(hash))
				if string(hash) == string(password) {
					return nil
				}
				return errors.New("mismatch")
			}

			require.True(t, user.checkPasswordWithComparator("primary-hash", compare))
			require.Equal(t, []string{"primary-hash"}, compared)
			compared = nil
			require.False(t, user.checkPasswordWithComparator("legacy-hash", compare))
			require.Equal(t, []string{"primary-hash"}, compared)
		})
	}
}
