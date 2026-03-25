//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type redeemTrialRepoStub struct {
	code         *RedeemCode
	updatedCode  *RedeemCode
	hasUsedTrial bool
	hasUsedErr   error
	useErr       error
	useCalls     int
	hasUsedCalls int
}

func (s *redeemTrialRepoStub) Create(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Create call")
}

func (s *redeemTrialRepoStub) CreateBatch(ctx context.Context, codes []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (s *redeemTrialRepoStub) GetByID(ctx context.Context, id int64) (*RedeemCode, error) {
	if s.updatedCode != nil {
		return s.updatedCode, nil
	}
	if s.code != nil && s.code.ID == id {
		return s.code, nil
	}
	return nil, ErrRedeemCodeNotFound
}

func (s *redeemTrialRepoStub) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	if s.code != nil && s.code.Code == code {
		return s.code, nil
	}
	return nil, ErrRedeemCodeNotFound
}

func (s *redeemTrialRepoStub) Update(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Update call")
}

func (s *redeemTrialRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *redeemTrialRepoStub) Use(ctx context.Context, id, userID int64) error {
	s.useCalls++
	if s.useErr != nil {
		return s.useErr
	}
	now := time.Now()
	s.updatedCode = &RedeemCode{
		ID:        id,
		Code:      s.code.Code,
		Type:      s.code.Type,
		Value:     s.code.Value,
		Status:    StatusUsed,
		UsedBy:    &userID,
		UsedAt:    &now,
		CreatedAt: s.code.CreatedAt,
	}
	return nil
}

func (s *redeemTrialRepoStub) List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *redeemTrialRepoStub) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *redeemTrialRepoStub) ListByUser(ctx context.Context, userID int64, limit int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (s *redeemTrialRepoStub) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (s *redeemTrialRepoStub) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

func (s *redeemTrialRepoStub) HasUsedTrialCodeByUser(ctx context.Context, userID int64) (bool, error) {
	s.hasUsedCalls++
	if s.hasUsedErr != nil {
		return false, s.hasUsedErr
	}
	return s.hasUsedTrial, nil
}

func newRedeemServiceTestEntClient(t *testing.T) *dbent.Client {
	t.Helper()

	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRedeemService_Redeem_RejectsRepeatedTrialCodeUsage(t *testing.T) {
	redeemRepo := &redeemTrialRepoStub{
		code: &RedeemCode{
			ID:        1,
			Code:      "TRIAL-ONLY",
			Type:      RedeemTypeBalance,
			Value:     5,
			Status:    StatusUnused,
			CreatedAt: time.Now(),
		},
		hasUsedTrial: true,
	}
	userRepo := &mockUserRepo{}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, newRedeemServiceTestEntClient(t), nil)

	_, err := svc.Redeem(context.Background(), 42, "TRIAL-ONLY")
	require.ErrorIs(t, err, ErrTrialRedeemUsed)
	require.Equal(t, 1, redeemRepo.hasUsedCalls)
	require.Zero(t, redeemRepo.useCalls)
}

func TestRedeemService_Redeem_NormalBalanceCodeNotBlockedByTrialHistory(t *testing.T) {
	redeemRepo := &redeemTrialRepoStub{
		code: &RedeemCode{
			ID:        2,
			Code:      "NORMAL-TOPUP",
			Type:      RedeemTypeBalance,
			Value:     20,
			Status:    StatusUnused,
			CreatedAt: time.Now(),
		},
		hasUsedTrial: true,
	}

	var balanceUpdates int
	userRepo := &mockUserRepo{
		updateBalanceFn: func(ctx context.Context, id int64, amount float64) error {
			balanceUpdates++
			require.Equal(t, int64(7), id)
			require.Equal(t, 20.0, amount)
			return nil
		},
	}

	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, newRedeemServiceTestEntClient(t), nil)

	got, err := svc.Redeem(context.Background(), 7, "NORMAL-TOPUP")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, StatusUsed, got.Status)
	require.Equal(t, 1, redeemRepo.useCalls)
	require.Zero(t, redeemRepo.hasUsedCalls)
	require.Equal(t, 1, balanceUpdates)
}
