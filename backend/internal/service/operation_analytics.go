package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const operationAnalyticsCacheTTL = 60 * time.Second

type OperationAnalyticsRepository interface {
	GetOperationAnalyticsSnapshot(ctx context.Context, filter OperationAnalyticsFilter) (*OperationAnalyticsSnapshot, error)
}

type OperationAnalyticsFilter struct {
	StartTime   time.Time
	EndTime     time.Time
	Granularity string
	Timezone    string
	Modules     []string
	AllData     bool
}

type OperationAnalyticsSnapshot struct {
	GeneratedAt    string                        `json:"generated_at"`
	StartTime      string                        `json:"start_time"`
	EndTime        string                        `json:"end_time"`
	Granularity    string                        `json:"granularity"`
	Timezone       string                        `json:"timezone"`
	RevenueNote    string                        `json:"revenue_note"`
	Core           OperationCoreMetrics          `json:"core"`
	Trend          []OperationTrendPoint         `json:"trend"`
	Funnel         []OperationFunnelStep         `json:"funnel"`
	FunnelPrev     []OperationFunnelStep         `json:"funnel_previous"`
	TrialFunnel    OperationTrialFunnel          `json:"trial_funnel"`
	Cohorts        []OperationRetentionCohort    `json:"cohorts"`
	Lists          OperationUserLists            `json:"lists"`
	Distribution   OperationDistributionSnapshot `json:"distribution"`
	Churn          OperationChurnSnapshot        `json:"churn"`
	Baselines      OperationBaselines            `json:"baselines"`
	Pyramid        OperationUserPyramid          `json:"pyramid"`
	Financial      OperationFinancialCockpit     `json:"financial"`
	ProductMatrix  OperationProductMatrix        `json:"product_matrix"`
	ModuleStatuses map[string]string             `json:"module_statuses,omitempty"`
	Advice         []OperationAdvice             `json:"advice"`
}

// OperationFinancialCockpit 财务驾驶舱：余额沉淀 + 收入构成 + 现金流 + ARPU 曲线。
//
// 收入分类口径（按用户实际业务）：
//
//  1. 管理员充值（type='admin_balance'）
//     - AdminTopUpGross：value > 0 的总和（正向充值）
//     - AdminRefundAmount：abs(value < 0) 的总和（退款）
//     - AdminTopUpNet = Gross - Refund
//
//  2. 兑换码兑换（type IN ('balance', 'subscription')）
//     - 余额类卡密（type='balance'）
//     · RedeemBalanceReal：扣掉 5 元体验券的真实充值卡
//     · RedeemTrial：5 元体验券赠送（不算真收入）
//     - 订阅类卡密（type='subscription'）：兑换次数
//
// 注意：sub2api 没有独立的 orders/transactions 流水表，所有数据从 redeem_codes 反推，
// 展示的是"权益注入金额"而非真实到账。
type OperationFinancialCockpit struct {
	// 余额沉淀健康度
	TotalBalance         float64 `json:"total_balance"`
	DailyAvgCost         float64 `json:"daily_avg_cost"`
	BalanceMonthsCushion float64 `json:"balance_months_cushion"`
	BalanceHealth        string  `json:"balance_health"` // healthy / warning / danger / overloaded

	// 第 1 类：管理员充值（拆正负）
	AdminTopUpGross   float64 `json:"admin_topup_gross"`
	AdminRefundAmount float64 `json:"admin_refund_amount"`
	AdminRefundCount  int64   `json:"admin_refund_count"`
	AdminTopUpNet     float64 `json:"admin_topup_net"`

	// 第 2 类：兑换码兑换
	RedeemBalanceReal       float64 `json:"redeem_balance_real"`
	RedeemTrial             float64 `json:"redeem_trial"`
	RedeemSubscriptionCount int64   `json:"redeem_subscription_count"`
	NewSubscriptionsCount   int64   `json:"new_subscriptions_count"`

	// 现金流（inflow 已扣退款）
	InflowTotal  float64 `json:"inflow_total"`  // 净入账 = AdminTopUpNet + RedeemBalanceReal
	InflowGross  float64 `json:"inflow_gross"`  // 毛入账 = AdminTopUpGross + RedeemBalanceReal
	OutflowTotal float64 `json:"outflow_total"` // 实际消耗
	NetFlow      float64 `json:"net_flow"`      // InflowTotal - OutflowTotal
	RefundRate   float64 `json:"refund_rate"`   // AdminRefundAmount / InflowGross

	// ARPU 30 天历史
	ArpuHistory []OperationArpuPoint `json:"arpu_history"`
}

type OperationArpuPoint struct {
	Bucket     string  `json:"bucket"`
	Arpu       float64 `json:"arpu"`
	PayingArpu float64 `json:"paying_arpu"`
	Dau        int64   `json:"dau"`
}

// OperationProductMatrix 产品矩阵：套餐 BCG + 模型健康度。
type OperationProductMatrix struct {
	Plans  []OperationPlanMatrix  `json:"plans"`
	Models []OperationModelHealth `json:"models"`
}

type OperationPlanMatrix struct {
	GroupID     int64   `json:"group_id"`
	Name        string  `json:"name"`
	ActiveUsers int64   `json:"active_users"`
	Revenue     float64 `json:"revenue"`
	ARPU        float64 `json:"arpu"`
	Quadrant    string  `json:"quadrant"` // star / cash_cow / question / dog
}

type OperationModelHealth struct {
	Model              string  `json:"model"`
	Requests           int64   `json:"requests"`
	Users              int64   `json:"users"`
	Revenue            float64 `json:"revenue"`
	TrafficShare       float64 `json:"traffic_share"`
	UsersChangePercent float64 `json:"users_change_percent"`
}

// OperationChurnSnapshot 个性化流失分析。
//
// 核心思想：用每个用户最近 90 天调用间隔的 P50（中位数）当作"该用户的正常节奏"。
// 距上次调用 > P50 × N 时，按 N 的不同分级"异常静默 / 高危流失 / 确定流失"。
// 这样高频用户和低频用户都不会被一刀切误判。
type OperationChurnSnapshot struct {
	GlobalP50Days       float64                      `json:"global_p50_days"`
	Definition          string                       `json:"definition"`
	HealthyUsers        int64                        `json:"healthy_users"`
	AtRiskUsers         int64                        `json:"at_risk_users"`
	HighRiskUsers       int64                        `json:"high_risk_users"`
	ChurnedUsers        int64                        `json:"churned_users"`
	BaseUsers           int64                        `json:"base_users"`
	ChurnRate           float64                      `json:"churn_rate"`
	AtRiskRate          float64                      `json:"at_risk_rate"`
	PreviousChurnRate   float64                      `json:"previous_churn_rate"`
	ChurnRateChangePct  float64                      `json:"churn_rate_change_pct"`
	HighValueAtRisk     int64                        `json:"high_value_at_risk"`
	HighValueRevenue    float64                      `json:"high_value_revenue"`
	NewlyChurnedRevenue float64                      `json:"newly_churned_revenue"`
	Waterfall           OperationChurnWaterfall      `json:"waterfall"`
	History             []OperationChurnHistoryPoint `json:"history"`
}

// OperationChurnWaterfall 流失瀑布：本周比上周少了多少人，分四个流失原因。
type OperationChurnWaterfall struct {
	LastPeriodActive  int64 `json:"last_period_active"`
	StillActive       int64 `json:"still_active"`
	CompletelyGone    int64 `json:"completely_gone"`
	HalfActivity      int64 `json:"half_activity"`
	BalanceExhausted  int64 `json:"balance_exhausted"`
	SubscriptionEnded int64 `json:"subscription_ended"`
}

type OperationChurnHistoryPoint struct {
	Bucket    string  `json:"bucket"`
	ChurnRate float64 `json:"churn_rate"`
	Churned   int64   `json:"churned"`
}

// OperationBaselines 历史基线对比：环比 + 同比。
// YoY 数据不足 1 年时降级到 90 天均值对比，YoYAvailable=false。
type OperationBaselines struct {
	WindowDays                 int64    `json:"window_days"`
	YoYAvailable               bool     `json:"yoy_available"`
	ActualCostYoYPercent       *float64 `json:"actual_cost_yoy_percent,omitempty"`
	ActiveUsersYoYPercent      *float64 `json:"active_users_yoy_percent,omitempty"`
	NewUsersYoYPercent         *float64 `json:"new_users_yoy_percent,omitempty"`
	ActualCostVs90DAvgPercent  float64  `json:"actual_cost_vs_90d_avg_percent"`
	ActiveUsersVs90DAvgPercent float64  `json:"active_users_vs_90d_avg_percent"`
	NewUsersVs90DAvgPercent    float64  `json:"new_users_vs_90d_avg_percent"`
	HistoryAverageDailyCost    float64  `json:"history_average_daily_cost"`
	HistoryAverageDailyActive  float64  `json:"history_average_daily_active"`
	HistoryAverageDailyNew     float64  `json:"history_average_daily_new"`
}

// OperationUserPyramid 用户金字塔：按最近 30 天 actual_cost 分位数分 5 层。
type OperationUserPyramid struct {
	GeneratedAt  string                  `json:"generated_at"`
	WindowDays   int64                   `json:"window_days"`
	TotalUsers   int64                   `json:"total_users"`
	PaidUsers    int64                   `json:"paid_users"`
	PaidPercent  float64                 `json:"paid_percent"`
	TotalRevenue float64                 `json:"total_revenue"`
	Levels       []OperationPyramidLevel `json:"levels"`
}

type OperationPyramidLevel struct {
	Key               string  `json:"key"`
	Label             string  `json:"label"`
	Users             int64   `json:"users"`
	UserPercent       float64 `json:"user_percent"`
	Revenue           float64 `json:"revenue"`
	RevenuePercent    float64 `json:"revenue_percent"`
	AvgRevenuePerUser float64 `json:"avg_revenue_per_user"`
	MinRevenue        float64 `json:"min_revenue"`
}

type OperationCoreMetrics struct {
	ActiveUsers              int64   `json:"active_users"`
	AverageDAU               float64 `json:"average_dau"`
	NewUsers                 int64   `json:"new_users"`
	PayingUsers              int64   `json:"paying_users"`
	FirstCallConversionRate  float64 `json:"first_call_conversion_rate"`
	BenefitConversionRate    float64 `json:"benefit_conversion_rate"`
	RetentionD1              float64 `json:"retention_d1"`
	RetentionD7              float64 `json:"retention_d7"`
	RetentionD30             float64 `json:"retention_d30"`
	Requests                 int64   `json:"requests"`
	Tokens                   int64   `json:"tokens"`
	ActualCost               float64 `json:"actual_cost"`
	ActiveSubscriptions      int64   `json:"active_subscriptions"`
	ExpiringSubscriptions    int64   `json:"expiring_subscriptions"`
	ActiveAPIKeys            int64   `json:"active_api_keys"`
	ARPU                     float64 `json:"arpu"`
	RequestsPerActiveUser    float64 `json:"requests_per_active_user"`
	PreviousActiveUsers      int64   `json:"previous_active_users"`
	PreviousNewUsers         int64   `json:"previous_new_users"`
	PreviousPayingUsers      int64   `json:"previous_paying_users"`
	PreviousActualCost       float64 `json:"previous_actual_cost"`
	ActiveUsersChangePercent float64 `json:"active_users_change_percent"`
	NewUsersChangePercent    float64 `json:"new_users_change_percent"`
	PayingUsersChangePercent float64 `json:"paying_users_change_percent"`
	ActualCostChangePercent  float64 `json:"actual_cost_change_percent"`
}

type OperationTrendPoint struct {
	Bucket                  string  `json:"bucket"`
	NewUsers                int64   `json:"new_users"`
	ActiveUsers             int64   `json:"active_users"`
	Requests                int64   `json:"requests"`
	Tokens                  int64   `json:"tokens"`
	ActualCost              float64 `json:"actual_cost"`
	FirstCallUsers          int64   `json:"first_call_users"`
	BenefitUsers            int64   `json:"benefit_users"`
	FirstCallConversionRate float64 `json:"first_call_conversion_rate"`
	BenefitConversionRate   float64 `json:"benefit_conversion_rate"`
}

type OperationFunnelStep struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Count       int64   `json:"count"`
	Rate        float64 `json:"rate"`
	Description string  `json:"description"`
}

// OperationTrialFunnel 5 元体验券专属漏斗。
//
// 业务背景：sub2api 默认给注册用户的 5 元体验券走 redeem_codes（type='balance' AND value=5）。
// 这套漏斗回答："白嫖体验之后，到底有多少人愿意付钱？"
//
// 关键定义：
//   - "真实付费" = 排除 5 元体验券和 promo_codes 之外的余额获得（充值卡 / admin_balance / 套餐订阅）
//   - "用尽体验券" = 累计 actual_cost ≥ 4 元（即 5 元体验券的 80%）
//   - "领了券没调用" = 既包含"装不上客户端的"，也包含"调用失败的"——
//     usage_logs 不记失败调用，所以这两类合并展示。
type OperationTrialFunnel struct {
	TrialUsersIssued    int64   `json:"trial_users_issued"`
	TrialUsersUsed      int64   `json:"trial_users_used"`
	TrialUsersIdle      int64   `json:"trial_users_idle"`
	TrialUsersExhausted int64   `json:"trial_users_exhausted"`
	TrialUsersConverted int64   `json:"trial_users_converted"`
	NonTrialPaid        int64   `json:"non_trial_paid"`
	UseRate             float64 `json:"use_rate"`
	IdleRate            float64 `json:"idle_rate"`
	ExhaustionRate      float64 `json:"exhaustion_rate"`
	ConversionRate      float64 `json:"conversion_rate"`
	AvgConsumed         float64 `json:"avg_consumed"`
	TrialBalanceValue   float64 `json:"trial_balance_value"`
	ExhaustionThreshold float64 `json:"exhaustion_threshold"`
}

type OperationRetentionCohort struct {
	CohortDate string   `json:"cohort_date"`
	NewUsers   int64    `json:"new_users"`
	D0         *float64 `json:"d0"`
	D1         *float64 `json:"d1"`
	D7         *float64 `json:"d7"`
	D30        *float64 `json:"d30"`
}

type OperationUserLists struct {
	HighSpending    []OperationUserListItem `json:"high_spending"`
	SilentHighValue []OperationUserListItem `json:"silent_high_value"`
	BenefitIdle     []OperationUserListItem `json:"benefit_idle"`
	ExpiringSoon    []OperationUserListItem `json:"expiring_soon"`
	NewInactive     []OperationUserListItem `json:"new_inactive"`
}

type OperationUserListItem struct {
	UserID      int64   `json:"user_id"`
	Email       string  `json:"email"`
	Username    string  `json:"username"`
	Value       float64 `json:"value"`
	ValueLabel  string  `json:"value_label"`
	LastUsageAt *string `json:"last_usage_at,omitempty"`
	DaysSince   *int64  `json:"days_since,omitempty"`
	GroupName   string  `json:"group_name,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"`
}

type OperationDistributionSnapshot struct {
	Groups      []OperationDistributionItem `json:"groups"`
	Models      []OperationDistributionItem `json:"models"`
	APIKeys     []OperationDistributionItem `json:"api_keys"`
	Promos      []OperationDistributionItem `json:"promos"`
	RedeemTypes []OperationDistributionItem `json:"redeem_types"`
}

type OperationDistributionItem struct {
	Key        string  `json:"key"`
	Label      string  `json:"label"`
	Requests   int64   `json:"requests,omitempty"`
	Tokens     int64   `json:"tokens,omitempty"`
	ActualCost float64 `json:"actual_cost,omitempty"`
	Users      int64   `json:"users,omitempty"`
	Count      int64   `json:"count,omitempty"`
	Value      float64 `json:"value,omitempty"`
}

type OperationAdvice struct {
	Level  string `json:"level"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Action string `json:"action"`
}

type OperationAnalyticsService struct {
	repo  OperationAnalyticsRepository
	mu    sync.Mutex
	cache map[string]operationAnalyticsCacheEntry
}

type operationAnalyticsCacheEntry struct {
	expiresAt time.Time
	snapshot  *OperationAnalyticsSnapshot
}

func NewOperationAnalyticsService(repo OperationAnalyticsRepository) *OperationAnalyticsService {
	return &OperationAnalyticsService{
		repo:  repo,
		cache: make(map[string]operationAnalyticsCacheEntry),
	}
}

func (s *OperationAnalyticsService) GetSnapshot(ctx context.Context, filter OperationAnalyticsFilter) (*OperationAnalyticsSnapshot, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("operation analytics repository not available")
	}
	if filter.Granularity == "" {
		filter.Granularity = "day"
	}
	if filter.Timezone == "" {
		filter.Timezone = "Asia/Shanghai"
	}
	key := operationAnalyticsCacheKey(filter)

	now := time.Now()
	s.mu.Lock()
	if entry, ok := s.cache[key]; ok && now.Before(entry.expiresAt) && entry.snapshot != nil {
		s.mu.Unlock()
		return entry.snapshot, nil
	}
	s.mu.Unlock()

	snapshot, err := s.repo.GetOperationAnalyticsSnapshot(ctx, filter)
	if err != nil {
		return nil, err
	}
	snapshot.Advice = BuildOperationAdvice(snapshot)

	s.mu.Lock()
	s.cache[key] = operationAnalyticsCacheEntry{
		expiresAt: now.Add(operationAnalyticsCacheTTL),
		snapshot:  snapshot,
	}
	s.mu.Unlock()

	return snapshot, nil
}

func operationAnalyticsCacheKey(filter OperationAnalyticsFilter) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%t",
		filter.StartTime.UTC().Format(time.RFC3339),
		filter.EndTime.UTC().Format(time.RFC3339),
		filter.Granularity,
		filter.Timezone,
		operationAnalyticsModulesKey(filter.Modules),
		filter.AllData,
	)
}

func operationAnalyticsModulesKey(modules []string) string {
	if len(modules) == 0 {
		return "summary"
	}
	return fmt.Sprintf("%v", modules)
}

func BuildOperationAdvice(snapshot *OperationAnalyticsSnapshot) []OperationAdvice {
	if snapshot == nil {
		return nil
	}

	core := snapshot.Core
	churn := snapshot.Churn
	advice := make([]OperationAdvice, 0, 6)

	// 流失警报：流失率本身高 OR 比上周明显恶化
	if churn.BaseUsers > 0 && churn.ChurnRate >= 0.20 {
		advice = append(advice, OperationAdvice{
			Level: "warning",
			Title: "流失率偏高",
			Detail: fmt.Sprintf(
				"曾经活跃过的 %d 个用户里，已确定流失 %d 人 (%.1f%%)，比上周 %s。",
				churn.BaseUsers, churn.ChurnedUsers, churn.ChurnRate*100,
				deltaText(churn.ChurnRateChangePct, "上升", "下降", "持平"),
			),
			Action: "马上对「高危流失」人群发挽回券；同时检查上周是否有上游异常或体验劣化。",
		})
	} else if churn.PreviousChurnRate > 0 && churn.ChurnRateChangePct >= 30 {
		advice = append(advice, OperationAdvice{
			Level:  "warning",
			Title:  "流失速度突然加快",
			Detail: fmt.Sprintf("本周流失率比上周 +%.1f%%，绝对值 %.1f%%。", churn.ChurnRateChangePct, churn.ChurnRate*100),
			Action: "立刻看流失瀑布，定位是「完全消失」还是「余额耗尽」占主导，针对性发挽回。",
		})
	}

	// 高价值流失警报
	if churn.HighValueAtRisk >= 5 {
		advice = append(advice, OperationAdvice{
			Level: "warning",
			Title: "高价值用户正在流失",
			Detail: fmt.Sprintf(
				"%d 个历史付过钱的用户已进入异常静默或高危流失状态，他们历史共贡献 $%.2f。",
				churn.HighValueAtRisk, churn.HighValueRevenue,
			),
			Action: "打开「高危流失」名单，优先人工/邮件触达 ARPU 最高的前 20 人。",
		})
	}

	if core.ActiveUsersChangePercent <= -20 && core.ActiveUsers >= 5 {
		advice = append(advice, OperationAdvice{
			Level:  "warning",
			Title:  "活跃用户明显下降",
			Detail: fmt.Sprintf("本期活跃用户较上一周期下降 %.1f%%。", -core.ActiveUsersChangePercent),
			Action: "优先查看沉默高价值用户和最近未激活用户，做续费提醒或使用引导。",
		})
	}
	if core.NewUsers > 0 && core.FirstCallConversionRate < 0.35 {
		advice = append(advice, OperationAdvice{
			Level:  "warning",
			Title:  "新用户首调用转化偏低",
			Detail: fmt.Sprintf("新用户 7 天内首调用转化率为 %.1f%%。", core.FirstCallConversionRate*100),
			Action: "优化注册后的 API Key 引导、客户端安装入口和首单使用教程。",
		})
	}
	if core.NewUsers > 0 && core.BenefitConversionRate < 0.2 {
		advice = append(advice, OperationAdvice{
			Level:  "info",
			Title:  "权益转化有提升空间",
			Detail: fmt.Sprintf("新用户 7 天内获得余额或订阅权益的比例为 %.1f%%。", core.BenefitConversionRate*100),
			Action: "检查优惠码、订阅包和充值入口是否足够清晰，针对未转化新用户做触达。",
		})
	}

	// 体验券激活率 / 转化率
	tf := snapshot.TrialFunnel
	if tf.TrialUsersIssued >= 20 && tf.UseRate < 0.40 {
		advice = append(advice, OperationAdvice{
			Level: "warning",
			Title: "体验券领了不用",
			Detail: fmt.Sprintf(
				"%d 人拿到 5 元体验券，只有 %d 人 (%.1f%%) 真正发起调用，剩下的可能装不上客户端或调用失败。",
				tf.TrialUsersIssued, tf.TrialUsersUsed, tf.UseRate*100,
			),
			Action: "看注册引导页、首调用文档、客户端安装链接是不是有断点；考虑开发一个一键体验脚本。",
		})
	}
	// 退款率告警
	fc0 := snapshot.Financial
	if fc0.AdminRefundCount > 0 && fc0.InflowGross > 0 && fc0.RefundRate >= 0.20 {
		advice = append(advice, OperationAdvice{
			Level: "warning",
			Title: "退款率偏高",
			Detail: fmt.Sprintf(
				"本期管理员退款 %d 笔共 $%.2f，占毛入账 $%.2f 的 %.1f%%。",
				fc0.AdminRefundCount, fc0.AdminRefundAmount, fc0.InflowGross, fc0.RefundRate*100,
			),
			Action: "退款率超 20% 通常是产品体验或上游故障引发。检查最近的退款工单 + 上游告警。",
		})
	}

	// 余额沉淀健康度告警
	fc := snapshot.Financial
	if fc.TotalBalance > 0 && fc.BalanceMonthsCushion > 0 && fc.BalanceMonthsCushion < 1 {
		advice = append(advice, OperationAdvice{
			Level: "warning",
			Title: "余额池子快空了",
			Detail: fmt.Sprintf(
				"用户总余额 $%.2f，按近 30 天日均消耗算，只够再用 %.1f 个月（< 1 月）。",
				fc.TotalBalance, fc.BalanceMonthsCushion,
			),
			Action: "马上推充值券或续费活动；否则下个月会出现集中流失。",
		})
	} else if fc.BalanceMonthsCushion > 6 && fc.TotalBalance > 100 {
		advice = append(advice, OperationAdvice{
			Level: "info",
			Title: "余额沉淀过多",
			Detail: fmt.Sprintf(
				"余额池子 $%.2f，按当前消耗速度够用 %.1f 个月（> 6 月），钱躺着没动。",
				fc.TotalBalance, fc.BalanceMonthsCushion,
			),
			Action: "用户充了不用 = 体验出问题。考虑发限时使用券、推新模型刺激消耗。",
		})
	}

	if tf.TrialUsersUsed >= 10 && tf.ConversionRate < 0.05 {
		advice = append(advice, OperationAdvice{
			Level: "warning",
			Title: "体验后不付费",
			Detail: fmt.Sprintf(
				"%d 人用过体验券，但只有 %d 人 (%.1f%%) 真实付费。可能是定价、套餐或上游体验出了问题。",
				tf.TrialUsersUsed, tf.TrialUsersConverted, tf.ConversionRate*100,
			),
			Action: "在体验券快用尽时主动推送续费券；走访 5-10 个用尽体验券但没付费的用户看为什么。",
		})
	}
	if core.ExpiringSubscriptions >= 10 {
		advice = append(advice, OperationAdvice{
			Level:  "warning",
			Title:  "近期到期订阅较多",
			Detail: fmt.Sprintf("未来 7 天内有 %d 个订阅即将到期。", core.ExpiringSubscriptions),
			Action: "打开快到期名单，优先联系高消费或仍活跃的用户续订。",
		})
	}
	if len(advice) == 0 {
		advice = append(advice, OperationAdvice{
			Level:  "success",
			Title:  "运营指标暂未发现明显风险",
			Detail: "当前活跃、转化、订阅风险与流失率均在可观察范围内。",
			Action: "继续关注留存 cohort 和高价值用户名单，按周复盘。",
		})
	}
	return advice
}

func deltaText(pct float64, upWord, downWord, flatWord string) string {
	if pct > 1 {
		return fmt.Sprintf("%s %.1f%%", upWord, pct)
	}
	if pct < -1 {
		return fmt.Sprintf("%s %.1f%%", downWord, -pct)
	}
	return flatWord
}
