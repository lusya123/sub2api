//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCreateShopAccountBridgeUserDoesNotGrantMainSignupBenefits(t *testing.T) {
	ctx := context.Background()
	repo := &userRepoStub{nextID: 77}
	assigner := &defaultSubscriptionAssignerStub{}
	quotaRepo := &userPlatformQuotaRepoStub{}
	cfg := &config.Config{Default: config.DefaultConfig{UserBalance: 3.5, UserConcurrency: 2}}
	settings := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyDefaultBalance:                      "4.5",
		SettingKeyDefaultSubscriptions:                `[{"group_id":11,"validity_days":30}]`,
		SettingKeyDefaultPlatformQuotas:               `{"openai":{"weekly":12.34}}`,
		SettingKeyAuthSourceDefaultEmailBalance:       "8.5",
		SettingKeyAuthSourceDefaultEmailSubscriptions: `[{"group_id":21,"validity_days":14}]`,
		SettingKeyAuthSourceDefaultEmailGrantOnSignup: "true",
	}}, cfg)
	svc := NewAuthService(nil, repo, nil, nil, cfg, settings, nil, nil, nil, nil, assigner, nil, quotaRepo)

	user, err := svc.CreateShopAccountBridgeUser(ctx, "shop-only@example.com", "password123", "shop-user")
	require.NoError(t, err)
	require.NotNil(t, user)
	require.Equal(t, int64(77), user.ID)
	require.Zero(t, user.Balance, "Shop identity must not receive Main signup credit")
	require.Empty(t, assigner.calls, "Shop identity must not receive Main subscriptions")
	require.Empty(t, quotaRepo.bulkInsertCalls, "Shop identity must not receive Main platform quota grants")
}
