//go:build unit

package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type adminAPIKeySettingRepo struct {
	value string
}

func (r *adminAPIKeySettingRepo) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}
func (r *adminAPIKeySettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if key != SettingKeyAdminAPIKey {
		panic("unexpected setting key")
	}
	if r.value == "" {
		return "", ErrSettingNotFound
	}
	return r.value, nil
}
func (r *adminAPIKeySettingRepo) Set(_ context.Context, key, value string) error {
	if key != SettingKeyAdminAPIKey {
		panic("unexpected setting key")
	}
	r.value = value
	return nil
}
func (r *adminAPIKeySettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}
func (r *adminAPIKeySettingRepo) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}
func (r *adminAPIKeySettingRepo) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}
func (r *adminAPIKeySettingRepo) Delete(_ context.Context, key string) error {
	if key != SettingKeyAdminAPIKey {
		panic("unexpected setting key")
	}
	r.value = ""
	return nil
}

func TestAdminAPIKeyStoredAsOneWayDigest(t *testing.T) {
	repo := &adminAPIKeySettingRepo{}
	svc := NewSettingService(repo, &config.Config{})

	raw, err := svc.GenerateAdminAPIKey(context.Background())
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(raw, AdminAPIKeyPrefix))
	require.True(t, strings.HasPrefix(repo.value, adminAPIKeyHashRecordVersion+"$"))
	require.NotContains(t, repo.value, raw)

	valid, err := svc.ValidateAdminAPIKey(context.Background(), raw)
	require.NoError(t, err)
	require.True(t, valid)
	valid, err = svc.ValidateAdminAPIKey(context.Background(), raw+"wrong")
	require.NoError(t, err)
	require.False(t, valid)

	masked, exists, err := svc.GetAdminAPIKeyStatus(context.Background())
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, raw[:10]+"..."+raw[len(raw)-4:], masked)
}

func TestAdminAPIKeyLegacyPlaintextMigratesAfterSuccessfulValidation(t *testing.T) {
	const legacy = "admin-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	repo := &adminAPIKeySettingRepo{value: legacy}
	svc := NewSettingService(repo, &config.Config{})

	valid, err := svc.ValidateAdminAPIKey(context.Background(), legacy)
	require.NoError(t, err)
	require.True(t, valid)
	require.True(t, strings.HasPrefix(repo.value, adminAPIKeyHashRecordVersion+"$"))
	require.NotContains(t, repo.value, legacy)
}
