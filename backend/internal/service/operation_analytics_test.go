package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type operationAnalyticsRepoStub struct {
	calls atomic.Int32
}

func (r *operationAnalyticsRepoStub) GetOperationAnalyticsSnapshot(ctx context.Context, filter OperationAnalyticsFilter) (*OperationAnalyticsSnapshot, error) {
	r.calls.Add(1)
	return &OperationAnalyticsSnapshot{
		StartTime:   filter.StartTime.Format(time.RFC3339),
		EndTime:     filter.EndTime.Format(time.RFC3339),
		Granularity: filter.Granularity,
		Timezone:    filter.Timezone,
		Core: OperationCoreMetrics{
			ActiveUsers:              20,
			PreviousActiveUsers:      40,
			ActiveUsersChangePercent: -50,
			NewUsers:                 10,
			FirstCallConversionRate:  0.2,
			BenefitConversionRate:    0.1,
			ExpiringSubscriptions:    12,
		},
	}, nil
}

func TestOperationAnalyticsService_UsesSnapshotCache(t *testing.T) {
	repo := &operationAnalyticsRepoStub{}
	svc := NewOperationAnalyticsService(repo)
	filter := OperationAnalyticsFilter{
		StartTime:   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 5, 3, 0, 0, 0, 0, time.UTC),
		Granularity: "day",
		Timezone:    "Asia/Shanghai",
	}

	first, err := svc.GetSnapshot(context.Background(), filter)
	require.NoError(t, err)
	second, err := svc.GetSnapshot(context.Background(), filter)
	require.NoError(t, err)

	require.Same(t, first, second)
	require.Equal(t, int32(1), repo.calls.Load())
	require.NotEmpty(t, first.Advice)
}

func TestBuildOperationAdvice_RiskRules(t *testing.T) {
	advice := BuildOperationAdvice(&OperationAnalyticsSnapshot{
		Core: OperationCoreMetrics{
			ActiveUsers:              50,
			ActiveUsersChangePercent: -30,
			NewUsers:                 20,
			FirstCallConversionRate:  0.1,
			BenefitConversionRate:    0.05,
			ExpiringSubscriptions:    11,
		},
	})

	require.Len(t, advice, 4)
	require.Equal(t, "活跃用户明显下降", advice[0].Title)
	require.Equal(t, "新用户首调用转化偏低", advice[1].Title)
	require.Equal(t, "权益转化有提升空间", advice[2].Title)
	require.Equal(t, "近期到期订阅较多", advice[3].Title)
}

func TestBuildOperationAdvice_ChurnRules(t *testing.T) {
	// 流失率 25% (>= 20% 警戒线) → 触发"流失率偏高"
	advice := BuildOperationAdvice(&OperationAnalyticsSnapshot{
		Churn: OperationChurnSnapshot{
			BaseUsers:          100,
			ChurnedUsers:       25,
			ChurnRate:          0.25,
			PreviousChurnRate:  0.20,
			ChurnRateChangePct: 25,
			HighValueAtRisk:    8,
			HighValueRevenue:   1234.5,
		},
	})

	titles := make([]string, 0, len(advice))
	for _, a := range advice {
		titles = append(titles, a.Title)
	}
	require.Contains(t, titles, "流失率偏高")
	require.Contains(t, titles, "高价值用户正在流失")
}

func TestBuildOperationAdvice_TrialRules(t *testing.T) {
	// 体验券领了不用 (use_rate < 40% 且 issued >= 20)
	advice := BuildOperationAdvice(&OperationAnalyticsSnapshot{
		TrialFunnel: OperationTrialFunnel{
			TrialUsersIssued: 100,
			TrialUsersUsed:   30,
			UseRate:          0.30,
		},
	})
	titles := make([]string, 0, len(advice))
	for _, a := range advice {
		titles = append(titles, a.Title)
	}
	require.Contains(t, titles, "体验券领了不用")

	// 体验后不付费 (conversion_rate < 5% 且 used >= 10)
	advice2 := BuildOperationAdvice(&OperationAnalyticsSnapshot{
		TrialFunnel: OperationTrialFunnel{
			TrialUsersIssued:    100,
			TrialUsersUsed:      80,
			TrialUsersConverted: 2,
			UseRate:             0.80,
			ConversionRate:      0.025,
		},
	})
	titles2 := make([]string, 0, len(advice2))
	for _, a := range advice2 {
		titles2 = append(titles2, a.Title)
	}
	require.Contains(t, titles2, "体验后不付费")
}

func TestBuildOperationAdvice_ChurnAcceleration(t *testing.T) {
	// 绝对流失率不高，但本周比上周陡增 → 触发"流失速度突然加快"
	advice := BuildOperationAdvice(&OperationAnalyticsSnapshot{
		Churn: OperationChurnSnapshot{
			BaseUsers:          200,
			ChurnedUsers:       20,
			ChurnRate:          0.10,
			PreviousChurnRate:  0.05,
			ChurnRateChangePct: 100,
		},
	})

	titles := make([]string, 0, len(advice))
	for _, a := range advice {
		titles = append(titles, a.Title)
	}
	require.Contains(t, titles, "流失速度突然加快")
}

func TestBuildOperationAdvice_DefaultHealthy(t *testing.T) {
	advice := BuildOperationAdvice(&OperationAnalyticsSnapshot{
		Core: OperationCoreMetrics{
			ActiveUsers:              5,
			ActiveUsersChangePercent: 5,
			NewUsers:                 0,
		},
	})

	require.Len(t, advice, 1)
	require.Equal(t, "success", advice[0].Level)
}
