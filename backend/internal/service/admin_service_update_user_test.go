//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type userRepoStubForUpdateUser struct {
	userRepoStub
	updated []*User
}

func (s *userRepoStubForUpdateUser) Update(_ context.Context, user *User) error {
	clone := *user
	s.updated = append(s.updated, &clone)
	return nil
}

type userGroupRateRepoStubForUpdateUser struct {
	syncedUserID      int64
	syncedRates       map[int64]*float64
	getByUserIDCalled bool
	loadedRates       map[int64]float64
}

func (s *userGroupRateRepoStubForUpdateUser) GetByUserID(_ context.Context, userID int64) (map[int64]float64, error) {
	s.getByUserIDCalled = true
	if userID != s.syncedUserID {
		return nil, nil
	}
	return s.loadedRates, nil
}

func (s *userGroupRateRepoStubForUpdateUser) GetByUserAndGroup(context.Context, int64, int64) (*float64, error) {
	panic("unexpected GetByUserAndGroup call")
}

func (s *userGroupRateRepoStubForUpdateUser) GetByGroupID(context.Context, int64) ([]UserGroupRateEntry, error) {
	panic("unexpected GetByGroupID call")
}

func (s *userGroupRateRepoStubForUpdateUser) SyncUserGroupRates(_ context.Context, userID int64, rates map[int64]*float64) error {
	s.syncedUserID = userID
	s.syncedRates = rates
	return nil
}

func (s *userGroupRateRepoStubForUpdateUser) SyncGroupRateMultipliers(context.Context, int64, []GroupRateMultiplierInput) error {
	panic("unexpected SyncGroupRateMultipliers call")
}

func (s *userGroupRateRepoStubForUpdateUser) DeleteByGroupID(context.Context, int64) error {
	panic("unexpected DeleteByGroupID call")
}

func (s *userGroupRateRepoStubForUpdateUser) DeleteByUserID(context.Context, int64) error {
	panic("unexpected DeleteByUserID call")
}

func TestAdminService_UpdateUserRejectsInvalidGroupRates(t *testing.T) {
	negative := -0.1
	valid := 1.2
	tests := []struct {
		name  string
		rates map[int64]*float64
	}{
		{name: "negative rate", rates: map[int64]*float64{10: &negative}},
		{name: "invalid group id", rates: map[int64]*float64{0: &valid}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &userRepoStubForUpdateUser{
				userRepoStub: userRepoStub{user: &User{ID: 7, Email: "u@example.com", Role: RoleUser, Status: StatusActive}},
			}
			rateRepo := &userGroupRateRepoStubForUpdateUser{}
			svc := &adminServiceImpl{userRepo: userRepo, userGroupRateRepo: rateRepo}

			_, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{GroupRates: tt.rates})
			require.Error(t, err)
			require.True(t, infraerrors.IsBadRequest(err))
			require.Empty(t, userRepo.updated)
			require.Zero(t, rateRepo.syncedUserID)
			require.Nil(t, rateRepo.syncedRates)
		})
	}
}

func TestAdminService_UpdateUserAcceptsValidGroupRates(t *testing.T) {
	rate := 1.25
	userRepo := &userRepoStubForUpdateUser{
		userRepoStub: userRepoStub{user: &User{ID: 7, Email: "u@example.com", Role: RoleUser, Status: StatusActive}},
	}
	rateRepo := &userGroupRateRepoStubForUpdateUser{loadedRates: map[int64]float64{10: rate}}
	svc := &adminServiceImpl{userRepo: userRepo, userGroupRateRepo: rateRepo}

	user, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{
		GroupRates: map[int64]*float64{10: &rate},
	})
	require.NoError(t, err)
	require.Equal(t, int64(7), user.ID)
	require.Len(t, userRepo.updated, 1)
	require.Equal(t, int64(7), rateRepo.syncedUserID)
	require.Same(t, &rate, rateRepo.syncedRates[10])
	require.True(t, rateRepo.getByUserIDCalled)
	require.Equal(t, map[int64]float64{10: rate}, user.GroupRates)
}
