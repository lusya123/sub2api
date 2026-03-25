//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// 测试订阅计费使用 ActualCost（倍率后的费用）而不是 TotalCost（倍率前的费用）
func TestGatewayServiceRecordUsage_SubscriptionBillingUsesActualCost(t *testing.T) {
	subRepo := &gatewayRecordUsageSubRepoStub{}
	userRepo := &openAIRecordUsageUserRepoStub{}
	usageRepo := &openAIRecordUsageLogRepoStub{}

	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.0

	svc := NewGatewayService(
		nil, nil, usageRepo, nil, userRepo, subRepo, nil, nil,
		cfg, nil, nil, NewBillingService(cfg, nil), nil,
		&BillingCacheService{}, nil, nil, &DeferredService{},
		nil, nil, nil, nil, nil,
	)

	groupID := int64(100)
	subscriptionID := int64(200)

	// 倍率 0.3，模拟月卡场景
	rateMultiplier := 0.3
	group := &Group{
		ID:               groupID,
		RateMultiplier:   rateMultiplier,
		SubscriptionType: SubscriptionTypeSubscription,
	}

	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "test_subscription_rate",
			Usage: ClaudeUsage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey: &APIKey{
			ID:      1,
			GroupID: &groupID,
			Group:   group,
		},
		User:    &User{ID: 1},
		Account: &Account{ID: 1},
		Subscription: &UserSubscription{
			ID:      subscriptionID,
			UserID:  1,
			GroupID: groupID,
		},
	})

	require.NoError(t, err)
	require.Equal(t, 1, subRepo.incrementCalls, "IncrementUsage should be called once")
	require.Equal(t, 0, userRepo.deductCalls, "DeductBalance should not be called for subscription billing")

	// 计算预期费用
	// claude-sonnet-4: input $3/MTok, output $15/MTok
	// TotalCost = 1000 * 3e-6 + 500 * 15e-6 = 0.003 + 0.0075 = 0.0105
	// ActualCost = TotalCost * 0.3 = 0.0105 * 0.3 = 0.00315
	expectedActualCost := 0.00315

	// 验证订阅计费使用的是 ActualCost（倍率后的费用）
	require.InDelta(t, expectedActualCost, subRepo.lastCost, 1e-6,
		"Subscription billing should use ActualCost (rate-multiplied cost), not TotalCost")
}

// 测试余额扣费使用 ActualCost（作为对比）
func TestGatewayServiceRecordUsage_BalanceBillingUsesActualCost(t *testing.T) {
	subRepo := &gatewayRecordUsageSubRepoStub{}
	userRepo := &openAIRecordUsageUserRepoStub{}
	usageRepo := &openAIRecordUsageLogRepoStub{}

	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.0

	svc := NewGatewayService(
		nil, nil, usageRepo, nil, userRepo, subRepo, nil, nil,
		cfg, nil, nil, NewBillingService(cfg, nil), nil,
		&BillingCacheService{}, nil, nil, &DeferredService{},
		nil, nil, nil, nil, nil,
	)

	groupID := int64(100)

	// 倍率 0.3
	rateMultiplier := 0.3
	group := &Group{
		ID:               groupID,
		RateMultiplier:   rateMultiplier,
		SubscriptionType: SubscriptionTypeSubscription,
	}

	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "test_balance_rate",
			Usage: ClaudeUsage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey: &APIKey{
			ID:      1,
			GroupID: &groupID,
			Group:   group,
		},
		User:         &User{ID: 1},
		Account:      &Account{ID: 1},
		Subscription: nil, // 无订阅，使用余额扣费
	})

	require.NoError(t, err)
	require.Equal(t, 0, subRepo.incrementCalls, "IncrementUsage should not be called for balance billing")
	require.Equal(t, 1, userRepo.deductCalls, "DeductBalance should be called once")

	// 计算预期费用
	expectedActualCost := 0.00315

	// 验证余额扣费使用的是 ActualCost（倍率后的费用）
	require.InDelta(t, expectedActualCost, userRepo.lastAmount, 1e-6,
		"Balance billing should use ActualCost (rate-multiplied cost)")
}

// 测试订阅计费和余额扣费使用相同的费用计算逻辑
func TestGatewayServiceRecordUsage_SubscriptionAndBalanceUseSameCostLogic(t *testing.T) {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.0

	rateMultiplier := 0.5
	groupID := int64(100)
	group := &Group{
		ID:               groupID,
		RateMultiplier:   rateMultiplier,
		SubscriptionType: SubscriptionTypeSubscription,
	}

	// 测试订阅计费
	subRepo1 := &gatewayRecordUsageSubRepoStub{}
	svc1 := NewGatewayService(
		nil, nil, &openAIRecordUsageLogRepoStub{}, nil,
		&openAIRecordUsageUserRepoStub{}, subRepo1, nil, nil,
		cfg, nil, nil, NewBillingService(cfg, nil), nil,
		&BillingCacheService{}, nil, nil, &DeferredService{},
		nil, nil, nil, nil, nil,
	)

	err := svc1.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "test_sub",
			Usage:     ClaudeUsage{InputTokens: 1000, OutputTokens: 500},
			Model:     "claude-sonnet-4",
			Duration:  time.Second,
		},
		APIKey:       &APIKey{ID: 1, GroupID: &groupID, Group: group},
		User:         &User{ID: 1},
		Account:      &Account{ID: 1},
		Subscription: &UserSubscription{ID: 200, UserID: 1, GroupID: groupID},
	})
	require.NoError(t, err)

	// 测试余额扣费
	userRepo2 := &openAIRecordUsageUserRepoStub{}
	svc2 := NewGatewayService(
		nil, nil, &openAIRecordUsageLogRepoStub{}, nil,
		userRepo2, &gatewayRecordUsageSubRepoStub{}, nil, nil,
		cfg, nil, nil, NewBillingService(cfg, nil), nil,
		&BillingCacheService{}, nil, nil, &DeferredService{},
		nil, nil, nil, nil, nil,
	)

	err = svc2.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "test_balance",
			Usage:     ClaudeUsage{InputTokens: 1000, OutputTokens: 500},
			Model:     "claude-sonnet-4",
			Duration:  time.Second,
		},
		APIKey:       &APIKey{ID: 1, GroupID: &groupID, Group: group},
		User:         &User{ID: 1},
		Account:      &Account{ID: 1},
		Subscription: nil,
	})
	require.NoError(t, err)

	// 验证订阅计费和余额扣费使用相同的费用
	require.InDelta(t, subRepo1.lastCost, userRepo2.lastAmount, 1e-10,
		"Subscription billing and balance billing should use the same cost (ActualCost)")
}

func TestGatewayServiceRecordUsage_UsesHiddenActualRateMultiplier(t *testing.T) {
	userRepo := &openAIRecordUsageUserRepoStub{}
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}

	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.0

	svc := NewGatewayService(
		nil, nil, usageRepo, nil, userRepo, &gatewayRecordUsageSubRepoStub{}, nil, nil,
		cfg, nil, nil, NewBillingService(cfg, nil), nil,
		&BillingCacheService{}, nil, nil, &DeferredService{},
		nil, nil, nil, nil, nil,
	)

	groupID := int64(100)
	actualRate := 0.5
	group := &Group{
		ID:                   groupID,
		RateMultiplier:       0.3,
		ActualRateMultiplier: &actualRate,
	}

	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{
			RequestID: "test_hidden_actual_rate",
			Usage: ClaudeUsage{
				InputTokens:  1000,
				OutputTokens: 500,
			},
			Model:    "claude-sonnet-4",
			Duration: time.Second,
		},
		APIKey:  &APIKey{ID: 1, GroupID: &groupID, Group: group},
		User:    &User{ID: 1},
		Account: &Account{ID: 1},
	})

	require.NoError(t, err)
	require.NotNil(t, usageRepo.lastLog)
	require.Equal(t, 0.3, usageRepo.lastLog.RateMultiplier, "user-visible rate should stay on display multiplier")
	require.NotNil(t, usageRepo.lastLog.ActualRateMultiplier)
	require.InDelta(t, 0.5, *usageRepo.lastLog.ActualRateMultiplier, 1e-12)

	expectedActualCost := 0.0105 * 0.5
	require.InDelta(t, expectedActualCost, usageRepo.lastLog.ActualCost, 1e-6)
	require.InDelta(t, expectedActualCost, userRepo.lastAmount, 1e-6)
}

// gatewayRecordUsageSubRepoStub 记录订阅计费的费用
type gatewayRecordUsageSubRepoStub struct {
	UserSubscriptionRepository

	incrementCalls int
	incrementErr   error
	lastCtxErr     error
	lastCost       float64 // 记录传入的费用
}

func (s *gatewayRecordUsageSubRepoStub) IncrementUsage(ctx context.Context, id int64, costUSD float64) error {
	s.incrementCalls++
	s.lastCost = costUSD
	s.lastCtxErr = ctx.Err()
	return s.incrementErr
}
