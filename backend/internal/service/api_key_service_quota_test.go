//go:build unit

package service

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAPIKeyNameUsesCharacterLimit(t *testing.T) {
	name100 := strings.Repeat("界", MaxAPIKeyNameRunes)
	got, err := normalizeAPIKeyName(name100)
	require.NoError(t, err)
	require.True(t, utf8.ValidString(got))
	require.Equal(t, name100, got)

	_, err = normalizeAPIKeyName(name100 + "尾")
	require.ErrorIs(t, err, ErrInvalidAPIKeyName)

	_, err = normalizeAPIKeyName("")
	require.ErrorIs(t, err, ErrInvalidAPIKeyName)
}

type quotaStateRepoStub struct {
	quotaBaseAPIKeyRepoStub
	stateCalls int
	state      *APIKeyQuotaUsageState
	stateErr   error
}

func (s *quotaStateRepoStub) IncrementQuotaUsedAndGetState(ctx context.Context, id int64, amount float64) (*APIKeyQuotaUsageState, error) {
	s.stateCalls++
	if s.stateErr != nil {
		return nil, s.stateErr
	}
	if s.state == nil {
		return nil, nil
	}
	out := *s.state
	return &out, nil
}

type quotaStateCacheStub struct {
	deleteAuthKeys []string
}

func (s *quotaStateCacheStub) GetCreateAttemptCount(context.Context, int64) (int, error) {
	return 0, nil
}

func (s *quotaStateCacheStub) IncrementCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (s *quotaStateCacheStub) DeleteCreateAttemptCount(context.Context, int64) error {
	return nil
}

func (s *quotaStateCacheStub) IncrementDailyUsage(context.Context, string) error {
	return nil
}

func (s *quotaStateCacheStub) SetDailyUsageExpiry(context.Context, string, time.Duration) error {
	return nil
}

func (s *quotaStateCacheStub) GetAuthCache(context.Context, string) (*APIKeyAuthCacheEntry, error) {
	return nil, nil
}

func (s *quotaStateCacheStub) SetAuthCache(context.Context, string, *APIKeyAuthCacheEntry, time.Duration) error {
	return nil
}

func (s *quotaStateCacheStub) DeleteAuthCache(_ context.Context, key string) error {
	s.deleteAuthKeys = append(s.deleteAuthKeys, key)
	return nil
}

func (s *quotaStateCacheStub) PublishAuthCacheInvalidation(context.Context, string) error {
	return nil
}

func (s *quotaStateCacheStub) SubscribeAuthCacheInvalidation(context.Context, func(string)) error {
	return nil
}

type quotaBaseAPIKeyRepoStub struct {
	getByIDCalls int
}

func (s *quotaBaseAPIKeyRepoStub) Create(context.Context, *APIKey) error {
	panic("unexpected Create call")
}
func (s *quotaBaseAPIKeyRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	s.getByIDCalls++
	return nil, nil
}
func (s *quotaBaseAPIKeyRepoStub) GetKeyAndOwnerID(context.Context, int64) (string, int64, error) {
	panic("unexpected GetKeyAndOwnerID call")
}
func (s *quotaBaseAPIKeyRepoStub) GetByKey(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}
func (s *quotaBaseAPIKeyRepoStub) GetByKeyForAuth(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKeyForAuth call")
}
func (s *quotaBaseAPIKeyRepoStub) Update(context.Context, *APIKey) error {
	panic("unexpected Update call")
}
func (s *quotaBaseAPIKeyRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *quotaBaseAPIKeyRepoStub) DeleteWithAudit(context.Context, int64) error {
	panic("unexpected DeleteWithAudit call")
}
func (s *quotaBaseAPIKeyRepoStub) ListByUserID(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserID call")
}
func (s *quotaBaseAPIKeyRepoStub) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}
func (s *quotaBaseAPIKeyRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	panic("unexpected CountByUserID call")
}
func (s *quotaBaseAPIKeyRepoStub) ExistsByKey(context.Context, string) (bool, error) {
	panic("unexpected ExistsByKey call")
}
func (s *quotaBaseAPIKeyRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (s *quotaBaseAPIKeyRepoStub) SearchAPIKeys(context.Context, int64, string, int) ([]APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}
func (s *quotaBaseAPIKeyRepoStub) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}
func (s *quotaBaseAPIKeyRepoStub) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}
func (s *quotaBaseAPIKeyRepoStub) CountByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}
func (s *quotaBaseAPIKeyRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByUserID call")
}
func (s *quotaBaseAPIKeyRepoStub) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByGroupID call")
}
func (s *quotaBaseAPIKeyRepoStub) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}
func (s *quotaBaseAPIKeyRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	panic("unexpected UpdateLastUsed call")
}
func (s *quotaBaseAPIKeyRepoStub) IncrementRateLimitUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}
func (s *quotaBaseAPIKeyRepoStub) ResetRateLimitWindows(context.Context, int64) error {
	panic("unexpected ResetRateLimitWindows call")
}
func (s *quotaBaseAPIKeyRepoStub) GetRateLimitData(context.Context, int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

func TestAPIKeyService_UpdateQuotaUsed_UsesAtomicStatePath(t *testing.T) {
	repo := &quotaStateRepoStub{
		state: &APIKeyQuotaUsageState{
			QuotaUsed: 12,
			Quota:     10,
			Key:       "sk-test-quota",
			Status:    StatusAPIKeyQuotaExhausted,
		},
	}
	cache := &quotaStateCacheStub{}
	svc := &APIKeyService{
		apiKeyRepo: repo,
		cache:      cache,
	}

	err := svc.UpdateQuotaUsed(context.Background(), 101, 2)
	require.NoError(t, err)
	require.Equal(t, 1, repo.stateCalls)
	require.Equal(t, 0, repo.getByIDCalls, "fast path should not re-read API key by id")
	require.Equal(t, []string{svc.authCacheKey("sk-test-quota")}, cache.deleteAuthKeys)
}

func TestAPIKeyService_Update_ReactivatesQuotaExhaustedWhenQuotaUnlimited(t *testing.T) {
	repo := &apiKeyRepoStub{
		apiKey: &APIKey{
			ID:        10,
			UserID:    7,
			Key:       "sk-test-unlimited",
			Status:    StatusAPIKeyQuotaExhausted,
			Quota:     10,
			QuotaUsed: 12,
		},
	}
	svc := &APIKeyService{apiKeyRepo: repo}
	quota := 0.0

	updated, err := svc.Update(context.Background(), 10, 7, UpdateAPIKeyRequest{Quota: &quota})

	require.NoError(t, err)
	require.Equal(t, StatusActive, updated.Status)
	require.Equal(t, 0.0, updated.Quota)
	require.Len(t, repo.updatedKeys, 1)
	require.Equal(t, StatusActive, repo.updatedKeys[0].Status)
	require.Equal(t, 0.0, repo.updatedKeys[0].Quota)
}

func TestAPIKeyService_CreateRejectsInvalidLimitsBeforeRepositoryAccess(t *testing.T) {
	negative := -0.01
	zeroDays := 0
	tooManyDays := 3651
	tests := []struct {
		name string
		req  CreateAPIKeyRequest
		want error
	}{
		{name: "negative quota", req: CreateAPIKeyRequest{Quota: negative}, want: ErrInvalidAPIKeyQuota},
		{name: "NaN quota", req: CreateAPIKeyRequest{Quota: math.NaN()}, want: ErrInvalidAPIKeyQuota},
		{name: "infinite quota", req: CreateAPIKeyRequest{Quota: math.Inf(1)}, want: ErrInvalidAPIKeyQuota},
		{name: "negative 5h rate limit", req: CreateAPIKeyRequest{RateLimit5h: negative}, want: ErrInvalidRateLimit5h},
		{name: "negative daily rate limit", req: CreateAPIKeyRequest{RateLimit1d: negative}, want: ErrInvalidRateLimit1d},
		{name: "negative 7d rate limit", req: CreateAPIKeyRequest{RateLimit7d: negative}, want: ErrInvalidRateLimit7d},
		{name: "zero expiry days", req: CreateAPIKeyRequest{ExpiresInDays: &zeroDays}, want: ErrInvalidAPIKeyExpiry},
		{name: "expiry days too large", req: CreateAPIKeyRequest{ExpiresInDays: &tooManyDays}, want: ErrInvalidAPIKeyExpiry},
		{name: "name exceeds 100 characters", req: CreateAPIKeyRequest{Name: strings.Repeat("界", MaxAPIKeyNameRunes+1)}, want: ErrInvalidAPIKeyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (&APIKeyService{}).Create(context.Background(), 7, tt.req)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.want)
			require.True(t, infraerrors.IsBadRequest(err))
		})
	}
}

func TestAPIKeyService_UpdateRejectsInvalidLimitsBeforeRepositoryAccess(t *testing.T) {
	negative := -0.01
	longName := strings.Repeat("界", MaxAPIKeyNameRunes+1)
	tests := []struct {
		name string
		req  UpdateAPIKeyRequest
		want error
	}{
		{name: "negative quota", req: UpdateAPIKeyRequest{Quota: &negative}, want: ErrInvalidAPIKeyQuota},
		{name: "negative 5h rate limit", req: UpdateAPIKeyRequest{RateLimit5h: &negative}, want: ErrInvalidRateLimit5h},
		{name: "negative daily rate limit", req: UpdateAPIKeyRequest{RateLimit1d: &negative}, want: ErrInvalidRateLimit1d},
		{name: "negative 7d rate limit", req: UpdateAPIKeyRequest{RateLimit7d: &negative}, want: ErrInvalidRateLimit7d},
		{name: "name exceeds 100 characters", req: UpdateAPIKeyRequest{Name: &longName}, want: ErrInvalidAPIKeyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &quotaBaseAPIKeyRepoStub{}
			got, err := (&APIKeyService{apiKeyRepo: repo}).Update(context.Background(), 11, 7, tt.req)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.want)
			require.True(t, infraerrors.IsBadRequest(err))
			require.Zero(t, repo.getByIDCalls, "invalid limits must be rejected before repository access")
		})
	}
}

func TestAPIKeyLimitValidationPreservesZeroAsUnlimited(t *testing.T) {
	oneDay := 1
	maxDays := 3650
	require.NoError(t, validateCreateAPIKeyLimits(CreateAPIKeyRequest{}))
	require.NoError(t, validateCreateAPIKeyLimits(CreateAPIKeyRequest{ExpiresInDays: &oneDay}))
	require.NoError(t, validateCreateAPIKeyLimits(CreateAPIKeyRequest{ExpiresInDays: &maxDays}))
	zero := 0.0
	require.NoError(t, validateUpdateAPIKeyLimits(UpdateAPIKeyRequest{
		Quota:       &zero,
		RateLimit5h: &zero,
		RateLimit1d: &zero,
		RateLimit7d: &zero,
	}))
}
