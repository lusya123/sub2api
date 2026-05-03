package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type operationAnalyticsRepository struct {
	db *sql.DB
}

func NewOperationAnalyticsRepository(db *sql.DB) service.OperationAnalyticsRepository {
	return &operationAnalyticsRepository{db: db}
}

func (r *operationAnalyticsRepository) GetOperationAnalyticsSnapshot(ctx context.Context, filter service.OperationAnalyticsFilter) (*service.OperationAnalyticsSnapshot, error) {
	if filter.Granularity != "hour" {
		filter.Granularity = "day"
	}
	if filter.Timezone == "" {
		filter.Timezone = "Asia/Shanghai"
	}
	modules := operationAnalyticsModuleSet(filter.Modules)

	snapshot := &service.OperationAnalyticsSnapshot{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		StartTime:      filter.StartTime.UTC().Format(time.RFC3339),
		EndTime:        filter.EndTime.UTC().Format(time.RFC3339),
		Granularity:    filter.Granularity,
		Timezone:       filter.Timezone,
		RevenueNote:    "运营口径：收入以 usage_logs.actual_cost 的消耗收入为准；余额/订阅兑换和后台分配只作为权益转化记录，不等同真实支付流水。",
		ModuleStatuses: make(map[string]string, len(modules)),
	}

	if modules["core"] {
		if filter.AllData {
			snapshot.RevenueNote += " 所有数据范围使用轻量累计口径，不计算留存、首调用转化和趋势重模块，避免拖慢生产库。"
		}
		if err := r.fillOperationCore(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["core"] = "ready"
	}
	if modules["trend"] {
		if err := r.fillOperationTrend(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["trend"] = "ready"
	}
	if modules["funnel"] {
		if err := r.fillOperationFunnel(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["funnel"] = "ready"
	}
	if modules["trial"] {
		if err := r.fillOperationTrialFunnel(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["trial"] = "ready"
	}
	if modules["cohorts"] {
		if err := r.fillOperationCohorts(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["cohorts"] = "ready"
	}
	if modules["lists"] {
		if err := r.fillOperationLists(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["lists"] = "ready"
	}
	if modules["distribution"] {
		if err := r.fillOperationDistribution(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["distribution"] = "ready"
	}
	if modules["churn"] {
		if err := r.fillOperationChurn(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["churn"] = "ready"
	}
	if modules["baselines"] {
		if err := r.fillOperationBaselines(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["baselines"] = "ready"
	}
	if modules["pyramid"] {
		if err := r.fillOperationPyramid(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["pyramid"] = "ready"
	}
	if modules["financial"] {
		if err := r.fillOperationFinancial(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["financial"] = "ready"
	}
	if modules["product_matrix"] {
		if err := r.fillOperationProductMatrix(ctx, filter, snapshot); err != nil {
			return nil, err
		}
		snapshot.ModuleStatuses["product_matrix"] = "ready"
	}
	if len(snapshot.ModuleStatuses) == 0 {
		snapshot.ModuleStatuses = nil
	}
	return snapshot, nil
}

func operationAnalyticsModuleSet(modules []string) map[string]bool {
	if len(modules) == 0 {
		modules = []string{"core", "trend"}
	}
	set := make(map[string]bool, len(modules))
	for _, module := range modules {
		switch module {
		case "core", "trend", "baselines", "funnel", "trial", "lists", "cohorts", "distribution", "churn", "pyramid", "financial", "product_matrix":
			set[module] = true
		}
	}
	return set
}

func (r *operationAnalyticsRepository) fillOperationCore(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	if filter.AllData {
		return r.fillOperationCoreAllData(ctx, filter, snapshot)
	}

	coreQuery := `
		WITH scoped_usage AS (
			SELECT user_id, api_key_id, created_at,
				input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens AS tokens,
				actual_cost
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2
		),
		real_paid_until_end AS (
			SELECT DISTINCT used_by AS user_id
			FROM redeem_codes
			WHERE used_by IS NOT NULL
			  AND used_at < $2
			  AND ((type = 'balance' AND value > 0 AND value <> 5) OR (type = 'admin_balance' AND value > 0) OR type = 'subscription')
			UNION
			SELECT DISTINCT user_id
			FROM user_subscriptions
			WHERE deleted_at IS NULL AND created_at < $2
		),
		previous_real_paid_until_end AS (
			SELECT DISTINCT used_by AS user_id
			FROM redeem_codes
			WHERE used_by IS NOT NULL
			  AND used_at < $4
			  AND ((type = 'balance' AND value > 0 AND value <> 5) OR (type = 'admin_balance' AND value > 0) OR type = 'subscription')
			UNION
			SELECT DISTINCT user_id
			FROM user_subscriptions
			WHERE deleted_at IS NULL AND created_at < $4
		),
		trial_users_until_end AS (
			SELECT DISTINCT used_by AS user_id
			FROM redeem_codes
			WHERE used_by IS NOT NULL AND type = 'balance' AND value = 5 AND used_at < $2
		),
		previous_trial_users_until_end AS (
			SELECT DISTINCT used_by AS user_id
			FROM redeem_codes
			WHERE used_by IS NOT NULL AND type = 'balance' AND value = 5 AND used_at < $4
		),
		daily_buckets AS (
			SELECT generate_series(
				($1::timestamptz AT TIME ZONE $5)::date,
				(($2::timestamptz - INTERVAL '1 second') AT TIME ZONE $5)::date,
				INTERVAL '1 day'
			)::date AS bucket
		),
		daily_active AS (
			SELECT COUNT(DISTINCT su.user_id)::float AS active_users
			FROM daily_buckets db
			LEFT JOIN scoped_usage su ON (su.created_at AT TIME ZONE $5)::date = db.bucket
			GROUP BY db.bucket
		),
		new_users AS (
			SELECT u.id, u.created_at
			FROM users u
			WHERE u.deleted_at IS NULL AND u.created_at >= $1 AND u.created_at < $2
		),
		new_user_flags AS (
			SELECT
				nu.id,
				EXISTS (
					SELECT 1 FROM usage_logs ul
					WHERE ul.user_id = nu.id
					  AND ul.created_at >= nu.created_at
					  AND ul.created_at < nu.created_at + INTERVAL '7 days'
				) AS first_call,
				EXISTS (
					SELECT 1 FROM usage_logs ul
					WHERE ul.user_id = nu.id
					  AND ul.actual_cost > 0
					  AND ul.created_at >= nu.created_at
					  AND ul.created_at < nu.created_at + INTERVAL '7 days'
				) AS consumed,
				EXISTS (
					SELECT 1 FROM redeem_codes rc
					WHERE rc.used_by = nu.id
					  AND rc.used_at >= nu.created_at
					  AND rc.used_at < nu.created_at + INTERVAL '7 days'
					  AND (
						(rc.type IN ('balance', 'admin_balance') AND rc.value > 0)
						OR rc.type = 'subscription'
					  )
				) OR EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = nu.id
					  AND us.deleted_at IS NULL
					  AND us.created_at >= nu.created_at
					  AND us.created_at < nu.created_at + INTERVAL '7 days'
				) AS benefited
			FROM new_users nu
		),
		retention_users AS (
			SELECT
				u.id,
				u.created_at,
				(($2::timestamptz AT TIME ZONE $5)::date >= ((u.created_at AT TIME ZONE $5)::date + 1)) AS mature_d1,
				(($2::timestamptz AT TIME ZONE $5)::date >= ((u.created_at AT TIME ZONE $5)::date + 7)) AS mature_d7,
				(($2::timestamptz AT TIME ZONE $5)::date >= ((u.created_at AT TIME ZONE $5)::date + 30)) AS mature_d30,
				EXISTS (
					SELECT 1 FROM usage_logs ul
					WHERE ul.user_id = u.id
					  AND (ul.created_at AT TIME ZONE $5)::date = ((u.created_at AT TIME ZONE $5)::date + 1)
				) AS d1,
				EXISTS (
					SELECT 1 FROM usage_logs ul
					WHERE ul.user_id = u.id
					  AND (ul.created_at AT TIME ZONE $5)::date = ((u.created_at AT TIME ZONE $5)::date + 7)
				) AS d7,
				EXISTS (
					SELECT 1 FROM usage_logs ul
					WHERE ul.user_id = u.id
					  AND (ul.created_at AT TIME ZONE $5)::date = ((u.created_at AT TIME ZONE $5)::date + 30)
				) AS d30
			FROM users u
			WHERE u.deleted_at IS NULL AND u.created_at >= $1 AND u.created_at < $2
		)
		SELECT
			COALESCE((SELECT COUNT(DISTINCT user_id) FROM scoped_usage), 0) AS active_users,
			COALESCE((SELECT AVG(active_users) FROM daily_active), 0) AS average_dau,
			COALESCE((SELECT COUNT(*) FROM new_users), 0) AS new_users,
			COALESCE((SELECT COUNT(*) FROM new_user_flags WHERE first_call), 0) AS first_call_users,
			COALESCE((SELECT COUNT(*) FROM new_user_flags WHERE benefited), 0) AS benefited_users,
			COALESCE((SELECT COUNT(*) FROM scoped_usage), 0) AS requests,
			COALESCE((SELECT SUM(tokens) FROM scoped_usage), 0) AS tokens,
			COALESCE((SELECT SUM(actual_cost) FROM scoped_usage), 0) AS actual_cost,
			COALESCE((
				SELECT COUNT(DISTINCT su.user_id)
				FROM scoped_usage su
				JOIN real_paid_until_end rp ON rp.user_id = su.user_id
				WHERE su.actual_cost > 0
			), 0) AS paying_users,
			COALESCE((SELECT COUNT(DISTINCT user_id) FROM scoped_usage WHERE actual_cost > 0), 0) AS consuming_users,
			COALESCE((
				SELECT COUNT(DISTINCT su.user_id)
				FROM scoped_usage su
				JOIN trial_users_until_end tu ON tu.user_id = su.user_id
				LEFT JOIN real_paid_until_end rp ON rp.user_id = su.user_id
				WHERE su.actual_cost > 0 AND rp.user_id IS NULL
			), 0) AS trial_consuming_users,
			COALESCE((SELECT COUNT(*) FROM user_subscriptions WHERE deleted_at IS NULL AND status = 'active' AND starts_at < $2 AND expires_at > $1), 0) AS active_subscriptions,
			COALESCE((SELECT COUNT(*) FROM user_subscriptions WHERE deleted_at IS NULL AND status = 'active' AND expires_at >= $2 AND expires_at < ($2::timestamptz + INTERVAL '7 days')), 0) AS expiring_subscriptions,
				COALESCE((SELECT COUNT(DISTINCT api_key_id) FROM scoped_usage WHERE api_key_id IS NOT NULL), 0) AS active_api_keys,
			COALESCE((SELECT COUNT(DISTINCT user_id) FROM usage_logs WHERE created_at >= $3 AND created_at < $4), 0) AS previous_active_users,
			COALESCE((
				SELECT COUNT(DISTINCT ul.user_id)
				FROM usage_logs ul
				JOIN previous_real_paid_until_end rp ON rp.user_id = ul.user_id
				WHERE ul.created_at >= $3 AND ul.created_at < $4 AND ul.actual_cost > 0
			), 0) AS previous_paying_users,
			COALESCE((SELECT COUNT(DISTINCT user_id) FROM usage_logs WHERE created_at >= $3 AND created_at < $4 AND actual_cost > 0), 0) AS previous_consuming_users,
			COALESCE((
				SELECT COUNT(DISTINCT ul.user_id)
				FROM usage_logs ul
				JOIN previous_trial_users_until_end tu ON tu.user_id = ul.user_id
				LEFT JOIN previous_real_paid_until_end rp ON rp.user_id = ul.user_id
				WHERE ul.created_at >= $3 AND ul.created_at < $4 AND ul.actual_cost > 0 AND rp.user_id IS NULL
			), 0) AS previous_trial_consuming_users,
			COALESCE((SELECT SUM(actual_cost) FROM usage_logs WHERE created_at >= $3 AND created_at < $4), 0) AS previous_actual_cost,
			COALESCE((SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= $3 AND created_at < $4), 0) AS previous_new_users,
			COALESCE((SELECT COUNT(*) FROM retention_users WHERE mature_d1), 0) AS eligible_d1,
			COALESCE((SELECT COUNT(*) FROM retention_users WHERE mature_d1 AND d1), 0) AS retained_d1,
			COALESCE((SELECT COUNT(*) FROM retention_users WHERE mature_d7), 0) AS eligible_d7,
			COALESCE((SELECT COUNT(*) FROM retention_users WHERE mature_d7 AND d7), 0) AS retained_d7,
			COALESCE((SELECT COUNT(*) FROM retention_users WHERE mature_d30), 0) AS eligible_d30,
			COALESCE((SELECT COUNT(*) FROM retention_users WHERE mature_d30 AND d30), 0) AS retained_d30
	`
	span := filter.EndTime.Sub(filter.StartTime)
	previousStart := filter.StartTime.Add(-span)
	previousEnd := filter.StartTime
	var firstCallUsers, benefitedUsers, eligibleD1, retainedD1, eligibleD7, retainedD7, eligibleD30, retainedD30 int64
	if err := r.db.QueryRowContext(ctx, coreQuery, filter.StartTime, filter.EndTime, previousStart, previousEnd, filter.Timezone).Scan(
		&snapshot.Core.ActiveUsers,
		&snapshot.Core.AverageDAU,
		&snapshot.Core.NewUsers,
		&firstCallUsers,
		&benefitedUsers,
		&snapshot.Core.Requests,
		&snapshot.Core.Tokens,
		&snapshot.Core.ActualCost,
		&snapshot.Core.PayingUsers,
		&snapshot.Core.ConsumingUsers,
		&snapshot.Core.TrialConsumingUsers,
		&snapshot.Core.ActiveSubscriptions,
		&snapshot.Core.ExpiringSubscriptions,
		&snapshot.Core.ActiveAPIKeys,
		&snapshot.Core.PreviousActiveUsers,
		&snapshot.Core.PreviousPayingUsers,
		&snapshot.Core.PreviousConsumingUsers,
		&snapshot.Core.PreviousTrialUsers,
		&snapshot.Core.PreviousActualCost,
		&snapshot.Core.PreviousNewUsers,
		&eligibleD1,
		&retainedD1,
		&eligibleD7,
		&retainedD7,
		&eligibleD30,
		&retainedD30,
	); err != nil {
		return fmt.Errorf("operation core metrics: %w", err)
	}

	snapshot.Core.FirstCallConversionRate = ratio(firstCallUsers, snapshot.Core.NewUsers)
	snapshot.Core.BenefitConversionRate = ratio(benefitedUsers, snapshot.Core.NewUsers)
	snapshot.Core.RetentionD1 = ratio(retainedD1, eligibleD1)
	snapshot.Core.RetentionD7 = ratio(retainedD7, eligibleD7)
	snapshot.Core.RetentionD30 = ratio(retainedD30, eligibleD30)
	if snapshot.Core.ActiveUsers > 0 {
		snapshot.Core.ARPU = snapshot.Core.ActualCost / float64(snapshot.Core.ActiveUsers)
		snapshot.Core.RequestsPerActiveUser = float64(snapshot.Core.Requests) / float64(snapshot.Core.ActiveUsers)
	}
	snapshot.Core.ActiveUsersChangePercent = changePercent(float64(snapshot.Core.ActiveUsers), float64(snapshot.Core.PreviousActiveUsers))
	snapshot.Core.NewUsersChangePercent = changePercent(float64(snapshot.Core.NewUsers), float64(snapshot.Core.PreviousNewUsers))
	snapshot.Core.PayingUsersChangePercent = changePercent(float64(snapshot.Core.PayingUsers), float64(snapshot.Core.PreviousPayingUsers))
	snapshot.Core.ConsumingChangePercent = changePercent(float64(snapshot.Core.ConsumingUsers), float64(snapshot.Core.PreviousConsumingUsers))
	snapshot.Core.TrialUsersChangePercent = changePercent(float64(snapshot.Core.TrialConsumingUsers), float64(snapshot.Core.PreviousTrialUsers))
	snapshot.Core.ActualCostChangePercent = changePercent(snapshot.Core.ActualCost, snapshot.Core.PreviousActualCost)
	return nil
}

func (r *operationAnalyticsRepository) fillOperationCoreAllData(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	query := `
		WITH all_usage AS (
			SELECT
				user_id,
				api_key_id,
				input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens AS tokens,
				actual_cost
			FROM usage_logs
		),
		real_payers AS (
			SELECT DISTINCT used_by AS user_id
			FROM redeem_codes
			WHERE used_by IS NOT NULL
			  AND ((type = 'balance' AND value > 0 AND value <> 5) OR (type = 'admin_balance' AND value > 0) OR type = 'subscription')
			UNION
			SELECT DISTINCT user_id
			FROM user_subscriptions
			WHERE deleted_at IS NULL
		),
		trial_only_users AS (
			SELECT DISTINCT rc.used_by AS user_id
			FROM redeem_codes rc
			LEFT JOIN real_payers rp ON rp.user_id = rc.used_by
			WHERE rc.used_by IS NOT NULL AND rc.type = 'balance' AND rc.value = 5 AND rp.user_id IS NULL
		)
		SELECT
			COALESCE((SELECT COUNT(DISTINCT user_id) FROM all_usage), 0) AS active_users,
			COALESCE((SELECT COUNT(*) FROM users WHERE deleted_at IS NULL), 0) AS new_users,
			COALESCE((SELECT COUNT(*) FROM all_usage), 0) AS requests,
			COALESCE((SELECT SUM(tokens) FROM all_usage), 0) AS tokens,
			COALESCE((SELECT SUM(actual_cost) FROM all_usage), 0) AS actual_cost,
			COALESCE((SELECT COUNT(*) FROM real_payers), 0) AS paying_users,
			COALESCE((SELECT COUNT(DISTINCT user_id) FROM all_usage WHERE actual_cost > 0), 0) AS consuming_users,
			COALESCE((
				SELECT COUNT(DISTINCT au.user_id)
				FROM all_usage au
				JOIN trial_only_users tu ON tu.user_id = au.user_id
				WHERE au.actual_cost > 0
			), 0) AS trial_consuming_users,
			COALESCE((SELECT COUNT(DISTINCT api_key_id) FROM all_usage WHERE api_key_id IS NOT NULL), 0) AS active_api_keys,
			COALESCE((SELECT COUNT(*) FROM user_subscriptions WHERE deleted_at IS NULL AND status = 'active' AND starts_at < $1 AND expires_at > $1), 0) AS active_subscriptions,
			COALESCE((SELECT COUNT(*) FROM user_subscriptions WHERE deleted_at IS NULL AND status = 'active' AND expires_at >= $1 AND expires_at < ($1::timestamptz + INTERVAL '7 days')), 0) AS expiring_subscriptions
	`
	if err := r.db.QueryRowContext(ctx, query, filter.EndTime).Scan(
		&snapshot.Core.ActiveUsers,
		&snapshot.Core.NewUsers,
		&snapshot.Core.Requests,
		&snapshot.Core.Tokens,
		&snapshot.Core.ActualCost,
		&snapshot.Core.PayingUsers,
		&snapshot.Core.ConsumingUsers,
		&snapshot.Core.TrialConsumingUsers,
		&snapshot.Core.ActiveAPIKeys,
		&snapshot.Core.ActiveSubscriptions,
		&snapshot.Core.ExpiringSubscriptions,
	); err != nil {
		return fmt.Errorf("operation core all data: %w", err)
	}
	if snapshot.Core.ActiveUsers > 0 {
		snapshot.Core.ARPU = snapshot.Core.ActualCost / float64(snapshot.Core.ActiveUsers)
		snapshot.Core.RequestsPerActiveUser = float64(snapshot.Core.Requests) / float64(snapshot.Core.ActiveUsers)
	}
	snapshot.Core.AverageDAU = float64(snapshot.Core.ActiveUsers)
	return nil
}

func (r *operationAnalyticsRepository) fillOperationTrend(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	format := "YYYY-MM-DD"
	if filter.Granularity == "hour" {
		format = "YYYY-MM-DD HH24:00"
	}
	points := map[string]*service.OperationTrendPoint{}

	usageQuery := fmt.Sprintf(`
		SELECT
			TO_CHAR(created_at AT TIME ZONE $3, '%s') AS bucket,
			COUNT(DISTINCT user_id) AS active_users,
			COUNT(*) AS requests,
			COALESCE(SUM(input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens), 0) AS tokens,
			COALESCE(SUM(actual_cost), 0) AS actual_cost
		FROM usage_logs
		WHERE created_at >= $1 AND created_at < $2
		GROUP BY bucket
		ORDER BY bucket
	`, format)
	rows, err := r.db.QueryContext(ctx, usageQuery, filter.StartTime, filter.EndTime, filter.Timezone)
	if err != nil {
		return fmt.Errorf("operation usage trend: %w", err)
	}
	for rows.Next() {
		var point service.OperationTrendPoint
		if err := rows.Scan(&point.Bucket, &point.ActiveUsers, &point.Requests, &point.Tokens, &point.ActualCost); err != nil {
			_ = rows.Close()
			return err
		}
		points[point.Bucket] = &point
	}
	if err := closeRows(rows); err != nil {
		return err
	}

	conversionQuery := fmt.Sprintf(`
		WITH new_users AS (
			SELECT
				u.id,
				u.created_at,
				TO_CHAR(u.created_at AT TIME ZONE $3, '%s') AS bucket
			FROM users u
			WHERE u.deleted_at IS NULL AND u.created_at >= $1 AND u.created_at < $2
		),
		flags AS (
			SELECT
				nu.bucket,
				nu.id,
				EXISTS (
					SELECT 1 FROM usage_logs ul
					WHERE ul.user_id = nu.id
					  AND ul.created_at >= nu.created_at
					  AND ul.created_at < nu.created_at + INTERVAL '7 days'
				) AS first_call,
				EXISTS (
					SELECT 1 FROM redeem_codes rc
					WHERE rc.used_by = nu.id
					  AND rc.used_at >= nu.created_at
					  AND rc.used_at < nu.created_at + INTERVAL '7 days'
					  AND ((rc.type IN ('balance', 'admin_balance') AND rc.value > 0) OR rc.type = 'subscription')
				) OR EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = nu.id
					  AND us.deleted_at IS NULL
					  AND us.created_at >= nu.created_at
					  AND us.created_at < nu.created_at + INTERVAL '7 days'
				) AS benefited
			FROM new_users nu
		)
		SELECT
			bucket,
			COUNT(*) AS new_users,
			COUNT(*) FILTER (WHERE first_call) AS first_call_users,
			COUNT(*) FILTER (WHERE benefited) AS benefit_users
		FROM flags
		GROUP BY bucket
		ORDER BY bucket
	`, format)
	rows, err = r.db.QueryContext(ctx, conversionQuery, filter.StartTime, filter.EndTime, filter.Timezone)
	if err != nil {
		return fmt.Errorf("operation conversion trend: %w", err)
	}
	for rows.Next() {
		var bucket string
		var newUsers, firstCallUsers, benefitUsers int64
		if err := rows.Scan(&bucket, &newUsers, &firstCallUsers, &benefitUsers); err != nil {
			_ = rows.Close()
			return err
		}
		point := points[bucket]
		if point == nil {
			point = &service.OperationTrendPoint{Bucket: bucket}
			points[bucket] = point
		}
		point.NewUsers = newUsers
		point.FirstCallUsers = firstCallUsers
		point.BenefitUsers = benefitUsers
		point.FirstCallConversionRate = ratio(firstCallUsers, newUsers)
		point.BenefitConversionRate = ratio(benefitUsers, newUsers)
	}
	if err := closeRows(rows); err != nil {
		return err
	}

	keys := make([]string, 0, len(points))
	for key := range points {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	snapshot.Trend = make([]service.OperationTrendPoint, 0, len(keys))
	for _, key := range keys {
		snapshot.Trend = append(snapshot.Trend, *points[key])
	}
	return nil
}

func (r *operationAnalyticsRepository) fillOperationFunnel(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	current, err := r.computeFunnelSteps(ctx, filter.StartTime, filter.EndTime)
	if err != nil {
		return err
	}
	snapshot.Funnel = current

	span := filter.EndTime.Sub(filter.StartTime)
	previous, err := r.computeFunnelSteps(ctx, filter.StartTime.Add(-span), filter.StartTime)
	if err != nil {
		return err
	}
	snapshot.FunnelPrev = previous
	return nil
}

func (r *operationAnalyticsRepository) computeFunnelSteps(ctx context.Context, start, end time.Time) ([]service.OperationFunnelStep, error) {
	// 业务定义：
	//   - 5 元体验券 = redeem_codes (type='balance' AND value=5)，对应 service.trialRedeemBalanceValue
	//   - "真实付费" 必须排除：5 元体验券 + promo_codes（这两类是白嫖/赠送）
	//   - 真实付费的来源：① 充值卡（type='balance' AND value <> 5）
	//                     ② 管理员充值（type='admin_balance'）
	//                     ③ 订阅卡密 / 订阅创建（type='subscription' / user_subscriptions）
	//   - "用尽体验券" = 注册 7 天内累计 actual_cost ≥ 4 元（5 元的 80%）
	//   - "复购" = 7 天内真实付费动作 ≥ 2 次
	query := `
		WITH new_users AS (
			SELECT id, created_at
			FROM users
			WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2
		),
		flags AS (
			SELECT
				nu.id,
				first_usage.first_at,
				cost_total.total AS cost_total,
				EXISTS (
					SELECT 1 FROM redeem_codes rc
					WHERE rc.used_by = nu.id
					  AND rc.used_at >= nu.created_at
					  AND rc.used_at < nu.created_at + INTERVAL '7 days'
					  AND rc.type = 'balance' AND rc.value = 5
				) AS got_trial,
				EXISTS (
					SELECT 1 FROM redeem_codes rc
					WHERE rc.used_by = nu.id
					  AND rc.used_at >= nu.created_at
					  AND rc.used_at < nu.created_at + INTERVAL '7 days'
					  AND (
						(rc.type = 'balance' AND rc.value <> 5 AND rc.value > 0)
						OR rc.type = 'admin_balance'
						OR rc.type = 'subscription'
					  )
				) OR EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = nu.id
					  AND us.deleted_at IS NULL
					  AND us.created_at >= nu.created_at
					  AND us.created_at < nu.created_at + INTERVAL '7 days'
				) AS real_paid,
				-- 注意：subscription 类卡密兑换时会同时写 redeem_codes 和 user_subscriptions，
				-- 为避免双重计数，redeem_codes 这侧只数 balance / admin_balance；
				-- 订阅类付费完全由 user_subscriptions 侧承担。
				(
					COALESCE((
						SELECT COUNT(*) FROM redeem_codes rc
						WHERE rc.used_by = nu.id
						  AND rc.used_at >= nu.created_at
						  AND rc.used_at < nu.created_at + INTERVAL '7 days'
						  AND (
							(rc.type = 'balance' AND rc.value <> 5 AND rc.value > 0)
							OR rc.type = 'admin_balance'
						  )
					), 0)
					+ COALESCE((
						SELECT COUNT(*) FROM user_subscriptions us
						WHERE us.user_id = nu.id
						  AND us.deleted_at IS NULL
						  AND us.created_at >= nu.created_at
						  AND us.created_at < nu.created_at + INTERVAL '7 days'
					), 0)
				) AS paid_count
			FROM new_users nu
			LEFT JOIN LATERAL (
				SELECT MIN(created_at) AS first_at
				FROM usage_logs ul
				WHERE ul.user_id = nu.id
				  AND ul.created_at >= nu.created_at
				  AND ul.created_at < nu.created_at + INTERVAL '7 days'
			) first_usage ON true
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(actual_cost), 0) AS total
				FROM usage_logs ul
				WHERE ul.user_id = nu.id
				  AND ul.created_at >= nu.created_at
				  AND ul.created_at < nu.created_at + INTERVAL '7 days'
			) cost_total ON true
		)
		SELECT
			COUNT(*)                                                  AS registered,
			COUNT(*) FILTER (WHERE got_trial)                         AS got_trial,
			COUNT(*) FILTER (WHERE first_at IS NOT NULL)              AS first_call,
			COUNT(*) FILTER (WHERE got_trial AND cost_total >= 4)     AS exhausted,
			COUNT(*) FILTER (WHERE real_paid)                         AS real_paid,
			COUNT(*) FILTER (WHERE real_paid AND paid_count >= 2)     AS repeat_paid
		FROM flags
	`
	var registered, gotTrial, firstCall, exhausted, realPaid, repeatPaid int64
	if err := r.db.QueryRowContext(ctx, query, start, end).Scan(
		&registered, &gotTrial, &firstCall, &exhausted, &realPaid, &repeatPaid,
	); err != nil {
		return nil, fmt.Errorf("operation funnel: %w", err)
	}
	return []service.OperationFunnelStep{
		{Key: "registered", Label: "注册", Count: registered, Rate: 1, Description: "选定周期内新注册的用户"},
		{Key: "got_trial", Label: "拿到体验券", Count: gotTrial, Rate: ratio(gotTrial, registered), Description: "兑换了 5 元体验券（自动或主动）"},
		{Key: "first_call", Label: "首次调用成功", Count: firstCall, Rate: ratio(firstCall, registered), Description: "注册后 7 天内有任何一次成功调用"},
		{Key: "exhausted", Label: "用尽体验券", Count: exhausted, Rate: ratio(exhausted, registered), Description: "7 天内累计消耗 ≥ 4 元，约用完 5 元体验券的 80%"},
		{Key: "real_paid", Label: "真实付费", Count: realPaid, Rate: ratio(realPaid, registered), Description: "充值卡 / 管理员充值 / 套餐订阅；明确不含 5 元体验券与 promo_codes"},
		{Key: "repeat_paid", Label: "复购", Count: repeatPaid, Rate: ratio(repeatPaid, registered), Description: "7 天内真实付费 ≥ 2 次"},
	}, nil
}

func (r *operationAnalyticsRepository) fillOperationCohorts(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	query := `
		WITH cohorts AS (
			SELECT
				(u.created_at AT TIME ZONE $2)::date AS cohort_date,
				u.id
			FROM users u
			WHERE u.deleted_at IS NULL
			  AND (u.created_at AT TIME ZONE $2)::date >= (($1::timestamptz AT TIME ZONE $2)::date - 13)
			  AND u.created_at < $1
		),
		flags AS (
			SELECT
				c.cohort_date,
				c.id,
				EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.user_id = c.id AND (ul.created_at AT TIME ZONE $2)::date = c.cohort_date) AS d0,
				EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.user_id = c.id AND (ul.created_at AT TIME ZONE $2)::date = c.cohort_date + 1) AS d1,
				EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.user_id = c.id AND (ul.created_at AT TIME ZONE $2)::date = c.cohort_date + 7) AS d7,
				EXISTS (SELECT 1 FROM usage_logs ul WHERE ul.user_id = c.id AND (ul.created_at AT TIME ZONE $2)::date = c.cohort_date + 30) AS d30
			FROM cohorts c
		)
		SELECT
			TO_CHAR(cohort_date, 'YYYY-MM-DD') AS cohort_date,
			COUNT(*) AS new_users,
			COUNT(*) FILTER (WHERE d0) AS d0,
			COUNT(*) FILTER (WHERE d1) AS d1,
			COUNT(*) FILTER (WHERE d7) AS d7,
			COUNT(*) FILTER (WHERE d30) AS d30
		FROM flags
		GROUP BY cohort_date
		ORDER BY cohort_date DESC
	`
	rows, err := r.db.QueryContext(ctx, query, filter.EndTime, filter.Timezone)
	if err != nil {
		return fmt.Errorf("operation cohorts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	endDate := localDate(filter.EndTime, filter.Timezone)
	for rows.Next() {
		var c service.OperationRetentionCohort
		var d0, d1, d7, d30 int64
		if err := rows.Scan(&c.CohortDate, &c.NewUsers, &d0, &d1, &d7, &d30); err != nil {
			return err
		}
		cohortDate, _ := time.Parse("2006-01-02", c.CohortDate)
		c.D0 = matureRatio(d0, c.NewUsers, cohortDate, endDate, 0)
		c.D1 = matureRatio(d1, c.NewUsers, cohortDate, endDate, 1)
		c.D7 = matureRatio(d7, c.NewUsers, cohortDate, endDate, 7)
		c.D30 = matureRatio(d30, c.NewUsers, cohortDate, endDate, 30)
		snapshot.Cohorts = append(snapshot.Cohorts, c)
	}
	return rows.Err()
}

func (r *operationAnalyticsRepository) fillOperationLists(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	var err error
	if snapshot.Lists.HighSpending, err = r.queryUserList(ctx, `
		SELECT u.id, COALESCE(u.email, ''), COALESCE(u.username, ''), SUM(ul.actual_cost) AS value, '区间消耗收入', MAX(ul.created_at), NULL::bigint, ''::text, NULL::timestamptz
			FROM usage_logs ul
			JOIN users u ON u.id = ul.user_id
			WHERE ul.created_at >= $1 AND ul.created_at < $2
			  AND u.deleted_at IS NULL
			GROUP BY u.id, u.email, u.username
		ORDER BY value DESC
		LIMIT 10
	`, filter.StartTime, filter.EndTime); err != nil {
		return err
	}
	if snapshot.Lists.SilentHighValue, err = r.queryUserList(ctx, `
		WITH stats AS (
			SELECT user_id, SUM(actual_cost) AS value, MAX(created_at) AS last_usage_at
			FROM usage_logs
			GROUP BY user_id
			HAVING SUM(actual_cost) > 0 AND MAX(created_at) < ($1::timestamptz - INTERVAL '7 days')
		)
		SELECT u.id, COALESCE(u.email, ''), COALESCE(u.username, ''), s.value, '历史消耗收入', s.last_usage_at,
			FLOOR(EXTRACT(EPOCH FROM ($1::timestamptz - s.last_usage_at)) / 86400)::bigint AS days_since,
			''::text, NULL::timestamptz
		FROM stats s
		JOIN users u ON u.id = s.user_id
		WHERE u.deleted_at IS NULL
		ORDER BY s.value DESC
		LIMIT 10
	`, filter.EndTime); err != nil {
		return err
	}
	if snapshot.Lists.BenefitIdle, err = r.queryUserList(ctx, `
		WITH last_usage AS (
			SELECT user_id, MAX(created_at) AS last_usage_at
			FROM usage_logs
			GROUP BY user_id
		),
		benefit AS (
			SELECT u.id AS user_id, u.balance AS value, lu.last_usage_at
			FROM users u
			LEFT JOIN last_usage lu ON lu.user_id = u.id
			WHERE u.deleted_at IS NULL
			  AND u.balance > 0
			  AND (lu.last_usage_at IS NULL OR lu.last_usage_at < ($1::timestamptz - INTERVAL '7 days'))
			UNION
			SELECT us.user_id, 0 AS value, lu.last_usage_at
			FROM user_subscriptions us
			LEFT JOIN last_usage lu ON lu.user_id = us.user_id
			WHERE us.deleted_at IS NULL AND us.status = 'active' AND us.expires_at > $1
				  AND (lu.last_usage_at IS NULL OR lu.last_usage_at < ($1::timestamptz - INTERVAL '7 days'))
		)
		SELECT u.id, COALESCE(u.email, ''), COALESCE(u.username, ''), MAX(b.value), '余额/订阅未使用', MAX(b.last_usage_at),
			CASE WHEN MAX(b.last_usage_at) IS NULL THEN NULL ELSE FLOOR(EXTRACT(EPOCH FROM ($1::timestamptz - MAX(b.last_usage_at))) / 86400)::bigint END AS days_since,
			''::text, NULL::timestamptz
			FROM benefit b
			JOIN users u ON u.id = b.user_id
			WHERE u.deleted_at IS NULL
			GROUP BY u.id, u.email, u.username
		ORDER BY MAX(b.value) DESC, u.id ASC
		LIMIT 10
	`, filter.EndTime); err != nil {
		return err
	}
	if snapshot.Lists.ExpiringSoon, err = r.queryUserList(ctx, `
		SELECT u.id, COALESCE(u.email, ''), COALESCE(u.username, ''), 0 AS value, '订阅即将到期', MAX(ul.created_at),
			CASE WHEN MAX(ul.created_at) IS NULL THEN NULL ELSE FLOOR(EXTRACT(EPOCH FROM ($1::timestamptz - MAX(ul.created_at))) / 86400)::bigint END AS days_since,
			COALESCE(g.name, ''), us.expires_at
			FROM user_subscriptions us
			JOIN users u ON u.id = us.user_id
			LEFT JOIN groups g ON g.id = us.group_id
			LEFT JOIN usage_logs ul ON ul.user_id = u.id
			WHERE u.deleted_at IS NULL
			  AND us.deleted_at IS NULL AND us.status = 'active'
		  AND us.expires_at >= $1 AND us.expires_at < ($1::timestamptz + INTERVAL '7 days')
		GROUP BY u.id, u.email, u.username, g.name, us.expires_at
		ORDER BY us.expires_at ASC
		LIMIT 10
	`, filter.EndTime); err != nil {
		return err
	}
	if snapshot.Lists.NewInactive, err = r.queryUserList(ctx, `
		SELECT u.id, COALESCE(u.email, ''), COALESCE(u.username, ''), 0 AS value, '新注册未首调用', NULL::timestamptz, NULL::bigint, ''::text, NULL::timestamptz
			FROM users u
			WHERE u.deleted_at IS NULL
			  AND u.created_at >= $1 AND u.created_at < $2
			  AND u.created_at < ($2::timestamptz - INTERVAL '7 days')
			  AND NOT EXISTS (
			SELECT 1 FROM usage_logs ul
			WHERE ul.user_id = u.id
			  AND ul.created_at >= u.created_at
			  AND ul.created_at < u.created_at + INTERVAL '7 days'
		  )
		ORDER BY u.created_at DESC
		LIMIT 10
	`, filter.StartTime, filter.EndTime); err != nil {
		return err
	}
	return nil
}

func (r *operationAnalyticsRepository) fillOperationDistribution(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	var err error
	if snapshot.Distribution.Groups, err = r.queryDistribution(ctx, `
		SELECT COALESCE(ul.group_id::text, 'none'), COALESCE(g.name, '未分组'), COUNT(*), COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0), COALESCE(SUM(ul.actual_cost), 0), COUNT(DISTINCT ul.user_id), 0::bigint, 0::numeric
		FROM usage_logs ul
		LEFT JOIN groups g ON g.id = ul.group_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		GROUP BY ul.group_id, g.name
		ORDER BY SUM(ul.actual_cost) DESC
		LIMIT 12
	`, filter.StartTime, filter.EndTime); err != nil {
		return err
	}
	if snapshot.Distribution.Models, err = r.queryDistribution(ctx, `
		SELECT COALESCE(NULLIF(TRIM(ul.requested_model), ''), ul.model), COALESCE(NULLIF(TRIM(ul.requested_model), ''), ul.model), COUNT(*), COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0), COALESCE(SUM(ul.actual_cost), 0), COUNT(DISTINCT ul.user_id), 0::bigint, 0::numeric
		FROM usage_logs ul
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		GROUP BY 1
		ORDER BY SUM(ul.actual_cost) DESC
		LIMIT 12
	`, filter.StartTime, filter.EndTime); err != nil {
		return err
	}
	if snapshot.Distribution.APIKeys, err = r.queryDistribution(ctx, `
		SELECT ul.api_key_id::text, COALESCE(k.name, 'API Key #' || ul.api_key_id::text), COUNT(*), COALESCE(SUM(ul.input_tokens + ul.output_tokens + ul.cache_creation_tokens + ul.cache_read_tokens), 0), COALESCE(SUM(ul.actual_cost), 0), COUNT(DISTINCT ul.user_id), 0::bigint, 0::numeric
		FROM usage_logs ul
		LEFT JOIN api_keys k ON k.id = ul.api_key_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		GROUP BY ul.api_key_id, k.name
		ORDER BY SUM(ul.actual_cost) DESC
		LIMIT 12
	`, filter.StartTime, filter.EndTime); err != nil {
		return err
	}
	if snapshot.Distribution.Promos, err = r.queryDistribution(ctx, `
		SELECT pc.code, pc.code, 0::bigint, 0::bigint, 0::numeric, COUNT(DISTINCT pcu.user_id), COUNT(*)::bigint, COALESCE(SUM(pcu.bonus_amount), 0)
		FROM promo_code_usages pcu
		JOIN promo_codes pc ON pc.id = pcu.promo_code_id
		WHERE pcu.used_at >= $1 AND pcu.used_at < $2
		GROUP BY pc.code
		ORDER BY COUNT(*) DESC
		LIMIT 12
	`, filter.StartTime, filter.EndTime); err != nil {
		return err
	}
	if snapshot.Distribution.RedeemTypes, err = r.queryDistribution(ctx, `
		SELECT rc.type, rc.type, 0::bigint, 0::bigint, 0::numeric, COUNT(DISTINCT rc.used_by), COUNT(*)::bigint, COALESCE(SUM(rc.value), 0)
		FROM redeem_codes rc
		WHERE rc.used_at >= $1 AND rc.used_at < $2 AND rc.used_by IS NOT NULL
		GROUP BY rc.type
		ORDER BY COUNT(*) DESC
		LIMIT 12
	`, filter.StartTime, filter.EndTime); err != nil {
		return err
	}
	return nil
}

// fillOperationTrialFunnel 5 元体验券子漏斗：领、用、闲置、用尽、转化付费。
//
// 与主漏斗不同：主漏斗按"7 天内动作"统计；体验券漏斗也用 7 天窗口，但分母换成"实际拿到体验券的人"，
// 这样转化率才有商业意义（"花了 5 元体验券钱获取了多少付费用户"）。
func (r *operationAnalyticsRepository) fillOperationTrialFunnel(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	const trialValue = 5.0
	const exhaustionThreshold = 4.0

	query := `
		WITH new_users AS (
			SELECT id, created_at
			FROM users
			WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2
		),
		trial AS (
			SELECT DISTINCT nu.id, nu.created_at
			FROM new_users nu
			JOIN redeem_codes rc ON rc.used_by = nu.id
			WHERE rc.type = 'balance' AND rc.value = 5
			  AND rc.used_at >= nu.created_at
			  AND rc.used_at < nu.created_at + INTERVAL '7 days'
		),
		flags AS (
			SELECT
				t.id,
				usage.first_at,
				COALESCE(usage.consumed, 0) AS consumed,
				EXISTS (
					SELECT 1 FROM redeem_codes rc
					WHERE rc.used_by = t.id
					  AND rc.used_at >= t.created_at
					  AND rc.used_at < t.created_at + INTERVAL '7 days'
					  AND (
						(rc.type = 'balance' AND rc.value <> 5 AND rc.value > 0)
						OR rc.type = 'admin_balance'
						OR rc.type = 'subscription'
					  )
				) OR EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = t.id AND us.deleted_at IS NULL
					  AND us.created_at >= t.created_at
					  AND us.created_at < t.created_at + INTERVAL '7 days'
				) AS converted
			FROM trial t
			LEFT JOIN LATERAL (
				SELECT MIN(created_at) AS first_at, COALESCE(SUM(actual_cost), 0) AS consumed
				FROM usage_logs ul
				WHERE ul.user_id = t.id
				  AND ul.created_at >= t.created_at
				  AND ul.created_at < t.created_at + INTERVAL '7 days'
			) usage ON true
		),
		non_trial_paid AS (
			SELECT COUNT(*) AS cnt
			FROM new_users nu
			WHERE NOT EXISTS (
				SELECT 1 FROM redeem_codes rc
				WHERE rc.used_by = nu.id AND rc.type = 'balance' AND rc.value = 5
				  AND rc.used_at >= nu.created_at
				  AND rc.used_at < nu.created_at + INTERVAL '7 days'
			)
			AND (
				EXISTS (
					SELECT 1 FROM redeem_codes rc
					WHERE rc.used_by = nu.id
					  AND rc.used_at >= nu.created_at
					  AND rc.used_at < nu.created_at + INTERVAL '7 days'
					  AND (
						(rc.type = 'balance' AND rc.value <> 5 AND rc.value > 0)
						OR rc.type = 'admin_balance'
						OR rc.type = 'subscription'
					  )
				) OR EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = nu.id AND us.deleted_at IS NULL
					  AND us.created_at >= nu.created_at
					  AND us.created_at < nu.created_at + INTERVAL '7 days'
				)
			)
		)
		SELECT
			(SELECT COUNT(*) FROM trial)                                  AS issued,
			COUNT(*) FILTER (WHERE first_at IS NOT NULL)                  AS used,
			COUNT(*) FILTER (WHERE consumed >= $3::float8)                AS exhausted,
			COUNT(*) FILTER (WHERE converted)                             AS converted,
			COALESCE(AVG(consumed) FILTER (WHERE first_at IS NOT NULL), 0) AS avg_consumed,
			(SELECT cnt FROM non_trial_paid)                              AS non_trial_paid
		FROM flags
	`
	tf := &snapshot.TrialFunnel
	tf.TrialBalanceValue = trialValue
	tf.ExhaustionThreshold = exhaustionThreshold
	if err := r.db.QueryRowContext(ctx, query, filter.StartTime, filter.EndTime, exhaustionThreshold).Scan(
		&tf.TrialUsersIssued,
		&tf.TrialUsersUsed,
		&tf.TrialUsersExhausted,
		&tf.TrialUsersConverted,
		&tf.AvgConsumed,
		&tf.NonTrialPaid,
	); err != nil {
		return fmt.Errorf("operation trial funnel: %w", err)
	}
	tf.TrialUsersIdle = tf.TrialUsersIssued - tf.TrialUsersUsed
	if tf.TrialUsersIssued > 0 {
		tf.UseRate = float64(tf.TrialUsersUsed) / float64(tf.TrialUsersIssued)
		tf.IdleRate = float64(tf.TrialUsersIdle) / float64(tf.TrialUsersIssued)
		tf.ExhaustionRate = float64(tf.TrialUsersExhausted) / float64(tf.TrialUsersIssued)
		tf.ConversionRate = float64(tf.TrialUsersConverted) / float64(tf.TrialUsersIssued)
	}
	return nil
}

// fillOperationChurn 计算个性化流失分析。
//
// 算法（人话版）：
//  1. 取每个用户最近 90 天所有调用，算调用间隔的中位数 P50；调用 < 5 次的用兜底 P50。
//  2. 阈值用"自己跟自己比"：
//     - 异常静默 At Risk: 距上次调用 > max(P50 × 2, 3 天)
//     - 高危流失 High Risk: 距上次调用 > max(P50 × 4, 14 天)
//     - 确定流失 Churned: 距上次调用 > max(P50 × 6, 30 天)
//  3. 加固条件（任一命中直接判流失）：余额=0 且无有效订阅 且 >14 天未调用；订阅过期 >14 天且未续。
//
// 这样高频用户和低频用户都不会被一刀切误判。
func (r *operationAnalyticsRepository) fillOperationChurn(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	endTime := filter.EndTime

	// Step 1: 全局 P50（兜底用）。注意：partition over user 的间隔窗口需要 ≥ 90d 数据。
	globalP50Query := `
		WITH calls AS (
			SELECT user_id, created_at,
				LAG(created_at) OVER (PARTITION BY user_id ORDER BY created_at) AS prev_at
			FROM usage_logs
			WHERE created_at >= ($1::timestamptz - INTERVAL '90 days')
			  AND created_at < $1
		),
		intervals AS (
			SELECT EXTRACT(EPOCH FROM (created_at - prev_at)) / 86400.0 AS gap_days
			FROM calls
			WHERE prev_at IS NOT NULL
		)
		SELECT COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY gap_days), 7.0)
		FROM intervals
	`
	var globalP50 float64
	if err := r.db.QueryRowContext(ctx, globalP50Query, endTime).Scan(&globalP50); err != nil {
		return fmt.Errorf("operation churn global p50: %w", err)
	}
	if globalP50 < 1 {
		globalP50 = 1
	}
	if globalP50 > 30 {
		globalP50 = 30
	}
	snapshot.Churn.GlobalP50Days = globalP50
	snapshot.Churn.Definition = fmt.Sprintf(
		"用每个用户自己最近 90 天的调用间隔中位数当尺子；调用次数不足时用全局中位数 %.1f 天兜底。"+
			"超过 P50×2 = 异常静默；P50×4 = 高危流失；P50×6 = 确定流失。",
		globalP50,
	)

	// Step 2: 按 P50 个性化分级。
	// 关键：分母是"曾经活跃过的用户"——也就是 90 天内至少有一次调用的人。
	// 这样流失率才有意义（你不能把从来没用过的免费注册算进流失）。
	churnQuery := `
		WITH end_ts AS (SELECT $1::timestamptz AS t),
		recent_users AS (
			SELECT DISTINCT user_id
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '90 days')
			  AND created_at < (SELECT t FROM end_ts)
		),
		last_calls AS (
			SELECT user_id, MAX(created_at) AS last_at
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '120 days')
			  AND created_at < (SELECT t FROM end_ts)
			GROUP BY user_id
		),
		gaps AS (
			SELECT user_id, created_at,
				LAG(created_at) OVER (PARTITION BY user_id ORDER BY created_at) AS prev_at
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '90 days')
			  AND created_at < (SELECT t FROM end_ts)
		),
		intervals AS (
			SELECT user_id,
				EXTRACT(EPOCH FROM (created_at - prev_at)) / 86400.0 AS gap_days
			FROM gaps
			WHERE prev_at IS NOT NULL
		),
		user_p50 AS (
			SELECT user_id,
				PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY gap_days) AS p50_gap,
				COUNT(*) AS sample
			FROM intervals
			GROUP BY user_id
		),
		hist_revenue AS (
			SELECT user_id, COALESCE(SUM(actual_cost), 0) AS revenue
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '90 days')
			  AND created_at < (SELECT t FROM end_ts)
			GROUP BY user_id
		),
		classified AS (
			SELECT
				ru.user_id,
				EXTRACT(EPOCH FROM ((SELECT t FROM end_ts) - lc.last_at)) / 86400.0 AS days_since,
				CASE
					WHEN up.sample >= 5 THEN GREATEST(LEAST(up.p50_gap, 60), 0.5)
					ELSE $2::float8
				END AS p50,
				COALESCE(hr.revenue, 0) AS revenue,
				COALESCE(u.balance, 0) AS balance,
				EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = ru.user_id AND us.deleted_at IS NULL
					  AND us.status = 'active' AND us.expires_at > (SELECT t FROM end_ts)
				) AS has_active_sub,
				EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = ru.user_id AND us.deleted_at IS NULL
					  AND us.expires_at < ((SELECT t FROM end_ts) - INTERVAL '14 days')
				) AS sub_expired_long
			FROM recent_users ru
			LEFT JOIN last_calls lc ON lc.user_id = ru.user_id
			LEFT JOIN user_p50 up ON up.user_id = ru.user_id
			LEFT JOIN hist_revenue hr ON hr.user_id = ru.user_id
			LEFT JOIN users u ON u.id = ru.user_id AND u.deleted_at IS NULL
		),
		bucketed AS (
			SELECT
				user_id,
				revenue,
				CASE
					WHEN days_since IS NULL THEN 'churned'
					-- 加固条件
					WHEN balance <= 0 AND NOT has_active_sub AND days_since > 14 THEN 'churned'
					WHEN sub_expired_long AND NOT has_active_sub THEN 'churned'
					-- 个性化分级
					WHEN days_since > GREATEST(p50 * 6, 30) THEN 'churned'
					WHEN days_since > GREATEST(p50 * 4, 14) THEN 'high_risk'
					WHEN days_since > GREATEST(p50 * 2, 3) THEN 'at_risk'
					ELSE 'healthy'
				END AS bucket
			FROM classified
			)
			SELECT
				COUNT(*) FILTER (WHERE bucket = 'healthy')   AS healthy,
				COUNT(*) FILTER (WHERE bucket = 'at_risk')   AS at_risk,
				COUNT(*) FILTER (WHERE bucket = 'high_risk') AS high_risk,
			COUNT(*) FILTER (WHERE bucket = 'churned')   AS churned,
				COUNT(*)                                      AS base,
				COUNT(*) FILTER (WHERE bucket IN ('at_risk','high_risk') AND revenue >= 1) AS hv_at_risk,
				COALESCE(SUM(revenue) FILTER (WHERE bucket IN ('at_risk','high_risk')), 0) AS hv_revenue,
				COALESCE(SUM(revenue) FILTER (WHERE bucket = 'churned'), 0)                AS churned_revenue
			FROM bucketed
		`
	if err := r.db.QueryRowContext(ctx, churnQuery, endTime, globalP50).Scan(
		&snapshot.Churn.HealthyUsers,
		&snapshot.Churn.AtRiskUsers,
		&snapshot.Churn.HighRiskUsers,
		&snapshot.Churn.ChurnedUsers,
		&snapshot.Churn.BaseUsers,
		&snapshot.Churn.HighValueAtRisk,
		&snapshot.Churn.HighValueRevenue,
		&snapshot.Churn.NewlyChurnedRevenue,
	); err != nil {
		return fmt.Errorf("operation churn classify: %w", err)
	}
	if snapshot.Churn.BaseUsers > 0 {
		snapshot.Churn.ChurnRate = float64(snapshot.Churn.ChurnedUsers) / float64(snapshot.Churn.BaseUsers)
		snapshot.Churn.AtRiskRate = float64(snapshot.Churn.AtRiskUsers+snapshot.Churn.HighRiskUsers) / float64(snapshot.Churn.BaseUsers)
	}

	// Step 3: 上一个等长周期的流失率（环比）。简化做法：用上周同一天为锚点重算。
	prevEnd := endTime.AddDate(0, 0, -7)
	prevQuery := `
		WITH end_ts AS (SELECT $1::timestamptz AS t),
		recent_users AS (
			SELECT DISTINCT user_id
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '90 days')
			  AND created_at < (SELECT t FROM end_ts)
		),
		last_calls AS (
			SELECT user_id, MAX(created_at) AS last_at
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '120 days')
			  AND created_at < (SELECT t FROM end_ts)
			GROUP BY user_id
		),
		classified AS (
			SELECT
				EXTRACT(EPOCH FROM ((SELECT t FROM end_ts) - lc.last_at)) / 86400.0 AS days_since
			FROM recent_users ru
			LEFT JOIN last_calls lc ON lc.user_id = ru.user_id
		)
		SELECT
			COUNT(*) AS base,
			COUNT(*) FILTER (WHERE days_since IS NULL OR days_since > GREATEST($2::float8 * 6, 30)) AS churned
		FROM classified
	`
	var prevBase, prevChurned int64
	if err := r.db.QueryRowContext(ctx, prevQuery, prevEnd, globalP50).Scan(&prevBase, &prevChurned); err != nil {
		return fmt.Errorf("operation churn previous: %w", err)
	}
	if prevBase > 0 {
		snapshot.Churn.PreviousChurnRate = float64(prevChurned) / float64(prevBase)
	}
	snapshot.Churn.ChurnRateChangePct = changePercent(snapshot.Churn.ChurnRate*100, snapshot.Churn.PreviousChurnRate*100)

	// Step 4: 流失瀑布——上周还活跃的人，本周怎么样了？
	//
	// 关键设计：5 个分类（fully_active / half / completely_gone / balance_exhausted /
	// subscription_ended）必须互斥且穷尽 last_period_active。这样瀑布图各条加起来
	// 等于上周活跃总数，用户不会被双重计数。
	waterfallQuery := `
		WITH end_ts AS (SELECT $1::timestamptz AS t),
		last_week AS (
			SELECT user_id, COUNT(*) AS calls
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '14 days')
			  AND created_at <  ((SELECT t FROM end_ts) - INTERVAL '7 days')
			GROUP BY user_id
		),
		this_week AS (
			SELECT user_id, COUNT(*) AS calls
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '7 days')
			  AND created_at <  (SELECT t FROM end_ts)
			GROUP BY user_id
		),
		joined AS (
			SELECT
				lw.user_id,
				lw.calls AS lw_calls,
				COALESCE(tw.calls, 0) AS tw_calls,
				COALESCE(u.balance, 0) AS balance,
				EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = lw.user_id AND us.deleted_at IS NULL
					  AND us.status = 'active' AND us.expires_at > (SELECT t FROM end_ts)
				) AS has_active_sub,
				EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = lw.user_id AND us.deleted_at IS NULL
					  AND us.expires_at >= ((SELECT t FROM end_ts) - INTERVAL '7 days')
					  AND us.expires_at <  (SELECT t FROM end_ts)
				) AS sub_just_expired
			FROM last_week lw
			LEFT JOIN this_week tw ON tw.user_id = lw.user_id
			LEFT JOIN users u ON u.id = lw.user_id AND u.deleted_at IS NULL
		),
		categorized AS (
			SELECT
				user_id,
				CASE
					-- 还在调用 + 调用量减半（更细的"流失风险"）
					WHEN tw_calls > 0 AND tw_calls * 2 < lw_calls THEN 'half'
					-- 这周完全没调用，且订阅刚到期 → 订阅断了导致的流失
					WHEN tw_calls = 0 AND sub_just_expired AND NOT has_active_sub THEN 'sub_ended'
					-- 这周完全没调用，且没钱也没订阅 → 余额耗尽导致的流失
					WHEN tw_calls = 0 AND balance <= 0 AND NOT has_active_sub THEN 'exhausted'
					-- 这周完全没调用，但还有钱/订阅 → 单纯消失（可能在忙）
					WHEN tw_calls = 0 THEN 'gone'
					-- 调用次数接近正常 → 充分活跃
					ELSE 'fully_active'
				END AS cat
			FROM joined
		)
		SELECT
			COUNT(*) AS last_active,
			COUNT(*) FILTER (WHERE cat = 'fully_active') AS still_active,
			COUNT(*) FILTER (WHERE cat = 'gone')         AS gone,
			COUNT(*) FILTER (WHERE cat = 'half')         AS half,
			COUNT(*) FILTER (WHERE cat = 'exhausted')    AS exhausted,
			COUNT(*) FILTER (WHERE cat = 'sub_ended')    AS sub_ended
		FROM categorized
	`
	if err := r.db.QueryRowContext(ctx, waterfallQuery, endTime).Scan(
		&snapshot.Churn.Waterfall.LastPeriodActive,
		&snapshot.Churn.Waterfall.StillActive,
		&snapshot.Churn.Waterfall.CompletelyGone,
		&snapshot.Churn.Waterfall.HalfActivity,
		&snapshot.Churn.Waterfall.BalanceExhausted,
		&snapshot.Churn.Waterfall.SubscriptionEnded,
	); err != nil {
		return fmt.Errorf("operation churn waterfall: %w", err)
	}

	// Step 5: 流失率历史曲线（最近 30 天，按天滚动 P50×6）。
	//
	// 性能版本：先一次性算出每个用户最近 120 天的 last_call_at，再 cross-join
	// 30 个时间锚点用 FILTER 聚合。复杂度 O(days × users)，远低于"30 anchor × 全表扫"。
	historyQuery := `
		WITH end_ts AS (SELECT $1::timestamptz AS t),
		threshold AS (SELECT (GREATEST($2::float8 * 6, 30) || ' days')::interval AS dt),
		last_calls AS (
			SELECT user_id, MAX(created_at) AS last_at
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '120 days')
			  AND created_at < (SELECT t FROM end_ts)
			GROUP BY user_id
		),
		anchors AS (
			SELECT
				offset_d,
				(SELECT t FROM end_ts) - (offset_d || ' days')::interval AS anchor_t
			FROM generate_series(0, 29) AS offset_d
		),
		per_day AS (
			SELECT
				a.offset_d,
				TO_CHAR((a.anchor_t AT TIME ZONE $3)::date, 'YYYY-MM-DD') AS bucket,
				COUNT(*) FILTER (
					WHERE lc.last_at >= a.anchor_t - INTERVAL '90 days'
					  AND lc.last_at <  a.anchor_t
				) AS base,
				COUNT(*) FILTER (
					WHERE lc.last_at >= a.anchor_t - INTERVAL '90 days'
					  AND lc.last_at <  a.anchor_t - (SELECT dt FROM threshold)
				) AS churned
			FROM anchors a
			CROSS JOIN last_calls lc
			GROUP BY a.offset_d, a.anchor_t
		)
		SELECT bucket, base, churned
		FROM per_day
		ORDER BY offset_d DESC
	`
	rows, err := r.db.QueryContext(ctx, historyQuery, endTime, globalP50, filter.Timezone)
	if err != nil {
		// 历史曲线失败不致命，只记日志。
		return nil
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var bucket string
		var base, churned int64
		if err := rows.Scan(&bucket, &base, &churned); err != nil {
			return nil
		}
		rate := 0.0
		if base > 0 {
			rate = float64(churned) / float64(base)
		}
		snapshot.Churn.History = append(snapshot.Churn.History, service.OperationChurnHistoryPoint{
			Bucket:    bucket,
			ChurnRate: rate,
			Churned:   churned,
		})
	}
	return nil
}

// fillOperationBaselines 计算同比 + 与历史 90 天日均的对比。
//
// YoY 数据不足 1 年时降级为 nil（前端隐藏），同时始终提供"vs 90 天日均"的对比。
func (r *operationAnalyticsRepository) fillOperationBaselines(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	span := filter.EndTime.Sub(filter.StartTime)
	windowDays := int64(span.Hours() / 24)
	if windowDays < 1 {
		windowDays = 1
	}
	snapshot.Baselines.WindowDays = windowDays

	// vs 90 天日均（始终可用）
	avgQuery := `
		WITH ninety_day AS (
			SELECT
				COUNT(DISTINCT user_id)::float8 / 90.0 AS daily_active,
				COALESCE(SUM(actual_cost), 0) / 90.0 AS daily_cost
			FROM usage_logs
			WHERE created_at >= ($1::timestamptz - INTERVAL '90 days')
			  AND created_at < $1
		),
		ninety_day_new AS (
			SELECT COUNT(*)::float8 / 90.0 AS daily_new
			FROM users
			WHERE deleted_at IS NULL
			  AND created_at >= ($1::timestamptz - INTERVAL '90 days')
			  AND created_at < $1
		)
		SELECT
			COALESCE((SELECT daily_active FROM ninety_day), 0),
			COALESCE((SELECT daily_cost FROM ninety_day), 0),
			COALESCE((SELECT daily_new FROM ninety_day_new), 0)
	`
	if err := r.db.QueryRowContext(ctx, avgQuery, filter.EndTime).Scan(
		&snapshot.Baselines.HistoryAverageDailyActive,
		&snapshot.Baselines.HistoryAverageDailyCost,
		&snapshot.Baselines.HistoryAverageDailyNew,
	); err != nil {
		return fmt.Errorf("operation baselines history avg: %w", err)
	}
	currDailyCost := snapshot.Core.ActualCost / float64(windowDays)
	currDailyActive := float64(snapshot.Core.ActiveUsers) / float64(windowDays)
	currDailyNew := float64(snapshot.Core.NewUsers) / float64(windowDays)
	snapshot.Baselines.ActualCostVs90DAvgPercent = changePercent(currDailyCost, snapshot.Baselines.HistoryAverageDailyCost)
	snapshot.Baselines.ActiveUsersVs90DAvgPercent = changePercent(currDailyActive, snapshot.Baselines.HistoryAverageDailyActive)
	snapshot.Baselines.NewUsersVs90DAvgPercent = changePercent(currDailyNew, snapshot.Baselines.HistoryAverageDailyNew)

	// 同比：去年同期等长窗口
	yoyStart := filter.StartTime.AddDate(-1, 0, 0)
	yoyEnd := filter.EndTime.AddDate(-1, 0, 0)
	yoyQuery := `
		SELECT
			(SELECT MIN(created_at) FROM usage_logs) AS earliest,
			COALESCE((SELECT COUNT(DISTINCT user_id) FROM usage_logs WHERE created_at >= $1 AND created_at < $2), 0),
			COALESCE((SELECT SUM(actual_cost) FROM usage_logs WHERE created_at >= $1 AND created_at < $2), 0),
			COALESCE((SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2), 0)
	`
	var earliest sql.NullTime
	var yoyActive, yoyNew int64
	var yoyCost float64
	if err := r.db.QueryRowContext(ctx, yoyQuery, yoyStart, yoyEnd).Scan(&earliest, &yoyActive, &yoyCost, &yoyNew); err != nil {
		return fmt.Errorf("operation baselines yoy: %w", err)
	}
	if earliest.Valid && earliest.Time.Before(yoyStart) {
		snapshot.Baselines.YoYAvailable = true
		costPct := changePercent(snapshot.Core.ActualCost, yoyCost)
		activePct := changePercent(float64(snapshot.Core.ActiveUsers), float64(yoyActive))
		newPct := changePercent(float64(snapshot.Core.NewUsers), float64(yoyNew))
		snapshot.Baselines.ActualCostYoYPercent = &costPct
		snapshot.Baselines.ActiveUsersYoYPercent = &activePct
		snapshot.Baselines.NewUsersYoYPercent = &newPct
	}
	return nil
}

// fillOperationPyramid 用最近 30 天的 actual_cost 给用户分 5 层。
//
// 分层规则（按花钱多少的累积分位数）：
//
//	super  → Top 1%（最赚钱的核心用户，往往贡献 30%+ 收入）
//	gold   → Top 5%
//	silver → Top 20%
//	bronze → 其余有付费的（用过钱但消费不大）
//	free   → 30 天内零消费
func (r *operationAnalyticsRepository) fillOperationPyramid(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	const windowDays = 30
	snapshot.Pyramid.WindowDays = windowDays
	snapshot.Pyramid.GeneratedAt = filter.EndTime.UTC().Format(time.RFC3339)

	query := `
		WITH end_ts AS (SELECT $1::timestamptz AS t),
		all_users AS (
			SELECT id FROM users WHERE deleted_at IS NULL
		),
		recent_cost AS (
			SELECT user_id, SUM(actual_cost) AS cost
			FROM usage_logs
			WHERE created_at >= ((SELECT t FROM end_ts) - INTERVAL '30 days')
			  AND created_at < (SELECT t FROM end_ts)
			GROUP BY user_id
			HAVING SUM(actual_cost) > 0
		),
		paid AS (
			SELECT
				rc.user_id,
				rc.cost,
				PERCENT_RANK() OVER (ORDER BY rc.cost) AS pct_rank
			FROM recent_cost rc
		),
		labeled AS (
			SELECT user_id, cost,
				CASE
					WHEN pct_rank >= 0.99 THEN 'super'
					WHEN pct_rank >= 0.95 THEN 'gold'
					WHEN pct_rank >= 0.80 THEN 'silver'
					ELSE 'bronze'
				END AS lvl
			FROM paid
		),
		summary AS (
			SELECT lvl, COUNT(*) AS users, SUM(cost) AS revenue, MIN(cost) AS min_cost
			FROM labeled
			GROUP BY lvl
			UNION ALL
			SELECT 'free', GREATEST(0, (SELECT COUNT(*) FROM all_users) - (SELECT COUNT(*) FROM paid))::bigint, 0, 0
		)
		SELECT lvl, users, COALESCE(revenue, 0), COALESCE(min_cost, 0)
		FROM summary
	`
	rows, err := r.db.QueryContext(ctx, query, filter.EndTime)
	if err != nil {
		return fmt.Errorf("operation pyramid: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type rawLevel struct {
		key     string
		users   int64
		revenue float64
		minRev  float64
	}
	raws := make(map[string]rawLevel)
	for rows.Next() {
		var rl rawLevel
		if err := rows.Scan(&rl.key, &rl.users, &rl.revenue, &rl.minRev); err != nil {
			return err
		}
		raws[rl.key] = rl
	}
	if err := rows.Err(); err != nil {
		return err
	}

	order := []struct {
		key   string
		label string
	}{
		{"super", "💎 超级用户"},
		{"gold", "🥇 金牌用户"},
		{"silver", "🥈 银牌用户"},
		{"bronze", "🥉 普通付费"},
		{"free", "🆓 免费试用"},
	}

	var totalUsers int64
	var totalRevenue, paidUsersF float64
	for _, lvl := range order {
		raw := raws[lvl.key]
		totalUsers += raw.users
		totalRevenue += raw.revenue
		if lvl.key != "free" {
			paidUsersF += float64(raw.users)
		}
	}
	snapshot.Pyramid.TotalUsers = totalUsers
	snapshot.Pyramid.PaidUsers = int64(paidUsersF)
	snapshot.Pyramid.TotalRevenue = totalRevenue
	if totalUsers > 0 {
		snapshot.Pyramid.PaidPercent = paidUsersF / float64(totalUsers)
	}

	for _, lvl := range order {
		raw := raws[lvl.key]
		level := service.OperationPyramidLevel{
			Key:        lvl.key,
			Label:      lvl.label,
			Users:      raw.users,
			Revenue:    raw.revenue,
			MinRevenue: raw.minRev,
		}
		if totalUsers > 0 {
			level.UserPercent = float64(raw.users) / float64(totalUsers)
		}
		if totalRevenue > 0 {
			level.RevenuePercent = raw.revenue / totalRevenue
		}
		if raw.users > 0 {
			level.AvgRevenuePerUser = raw.revenue / float64(raw.users)
		}
		snapshot.Pyramid.Levels = append(snapshot.Pyramid.Levels, level)
	}
	return nil
}

// fillOperationFinancial 财务驾驶舱：余额沉淀、收入构成、现金流、ARPU 30 天曲线。
//
// 关键说明：因为 sub2api 没有 orders/transactions 流水表，所有"收入"只能从 redeem_codes
// 和 user_subscriptions 反推。展示的是"权益注入金额"而不是"真实到账金额"。
//
//   - 充值收入 = redeem_codes (type='balance' AND value <> 5 AND value > 0) 的 SUM(value)
//   - 管理员充值 = redeem_codes (type='admin_balance') 的 SUM(value)
//   - 体验券赠送 = redeem_codes (type='balance' AND value = 5) 的 SUM(value)（不算真收入）
//   - 订阅次数 = redeem_codes (type='subscription') 兑换次数 + user_subscriptions 直接创建数
func (r *operationAnalyticsRepository) fillOperationFinancial(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	fc := &snapshot.Financial

	// 总余额 + 30 天日均消耗 + 沉淀月数
	balanceQuery := `
		SELECT
			COALESCE((SELECT SUM(balance) FROM users WHERE deleted_at IS NULL AND balance > 0), 0) AS total_balance,
			COALESCE((
				SELECT SUM(actual_cost) / 30.0
				FROM usage_logs
				WHERE created_at >= ($1::timestamptz - INTERVAL '30 days')
				  AND created_at < $1
			), 0) AS daily_avg_cost
	`
	if err := r.db.QueryRowContext(ctx, balanceQuery, filter.EndTime).Scan(&fc.TotalBalance, &fc.DailyAvgCost); err != nil {
		return fmt.Errorf("operation financial balance: %w", err)
	}
	if fc.DailyAvgCost > 0 {
		fc.BalanceMonthsCushion = fc.TotalBalance / (fc.DailyAvgCost * 30.0)
	} else if fc.TotalBalance > 0 {
		fc.BalanceMonthsCushion = 99
	}
	switch {
	case fc.BalanceMonthsCushion < 1:
		fc.BalanceHealth = "danger"
	case fc.BalanceMonthsCushion < 3:
		fc.BalanceHealth = "warning"
	case fc.BalanceMonthsCushion <= 6:
		fc.BalanceHealth = "healthy"
	default:
		fc.BalanceHealth = "overloaded"
	}

	// 收入构成（窗口期内）+ 现金流
	//
	// 关键拆分：
	//   - admin_balance 拆正负：value > 0 是充值；value < 0 是退款（用 ABS 取绝对值方便展示）
	//   - balance 类卡密：拆 5 元体验券 vs 真实充值卡
	revenueQuery := `
		SELECT
			COALESCE((
				SELECT SUM(value) FROM redeem_codes
				WHERE used_at >= $1 AND used_at < $2
				  AND type = 'admin_balance' AND value > 0
			), 0) AS admin_gross,
			COALESCE((
				SELECT ABS(SUM(value)) FROM redeem_codes
				WHERE used_at >= $1 AND used_at < $2
				  AND type = 'admin_balance' AND value < 0
			), 0) AS admin_refund,
			COALESCE((
				SELECT COUNT(*) FROM redeem_codes
				WHERE used_at >= $1 AND used_at < $2
				  AND type = 'admin_balance' AND value < 0
			), 0) AS admin_refund_count,
			COALESCE((
				SELECT SUM(value) FROM redeem_codes
				WHERE used_at >= $1 AND used_at < $2
				  AND type = 'balance' AND value <> 5 AND value > 0
			), 0) AS redeem_real,
			COALESCE((
				SELECT SUM(value) FROM redeem_codes
				WHERE used_at >= $1 AND used_at < $2
				  AND type = 'balance' AND value = 5
			), 0) AS redeem_trial,
			COALESCE((
				SELECT COUNT(*) FROM redeem_codes
				WHERE used_at >= $1 AND used_at < $2
				  AND type = 'subscription'
			), 0) AS redeem_sub_count,
			COALESCE((
				SELECT COUNT(*) FROM user_subscriptions
				WHERE deleted_at IS NULL AND created_at >= $1 AND created_at < $2
			), 0) AS new_subs,
			COALESCE((
				SELECT SUM(actual_cost) FROM usage_logs WHERE created_at >= $1 AND created_at < $2
			), 0) AS outflow
	`
	if err := r.db.QueryRowContext(ctx, revenueQuery, filter.StartTime, filter.EndTime).Scan(
		&fc.AdminTopUpGross,
		&fc.AdminRefundAmount,
		&fc.AdminRefundCount,
		&fc.RedeemBalanceReal,
		&fc.RedeemTrial,
		&fc.RedeemSubscriptionCount,
		&fc.NewSubscriptionsCount,
		&fc.OutflowTotal,
	); err != nil {
		return fmt.Errorf("operation financial revenue: %w", err)
	}
	fc.AdminTopUpNet = fc.AdminTopUpGross - fc.AdminRefundAmount
	fc.InflowGross = fc.AdminTopUpGross + fc.RedeemBalanceReal
	fc.InflowTotal = fc.AdminTopUpNet + fc.RedeemBalanceReal
	fc.NetFlow = fc.InflowTotal - fc.OutflowTotal
	if fc.InflowGross > 0 {
		fc.RefundRate = fc.AdminRefundAmount / fc.InflowGross
	}

	// ARPU 30 天曲线（按当地时区按天聚合）
	arpuQuery := `
		WITH days AS (
			SELECT generate_series(
				(($1::timestamptz - INTERVAL '30 days') AT TIME ZONE $2)::date,
				(($1::timestamptz - INTERVAL '1 day') AT TIME ZONE $2)::date,
				INTERVAL '1 day'
			)::date AS bucket
		),
		stats AS (
			SELECT
				(created_at AT TIME ZONE $2)::date AS d,
				SUM(actual_cost) AS cost,
				COUNT(DISTINCT user_id) AS dau,
				COUNT(DISTINCT user_id) FILTER (WHERE actual_cost > 0) AS paying
			FROM usage_logs
			WHERE created_at >= ($1::timestamptz - INTERVAL '30 days') AND created_at < $1
			GROUP BY 1
		)
		SELECT
			TO_CHAR(d.bucket, 'YYYY-MM-DD') AS bucket,
			COALESCE(s.cost, 0)             AS cost,
			COALESCE(s.dau, 0)              AS dau,
			COALESCE(s.paying, 0)           AS paying
		FROM days d
		LEFT JOIN stats s ON s.d = d.bucket
		ORDER BY d.bucket
	`
	rows, err := r.db.QueryContext(ctx, arpuQuery, filter.EndTime, filter.Timezone)
	if err != nil {
		return fmt.Errorf("operation financial arpu: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var p service.OperationArpuPoint
		var cost float64
		var paying int64
		if err := rows.Scan(&p.Bucket, &cost, &p.Dau, &paying); err != nil {
			return err
		}
		if p.Dau > 0 {
			p.Arpu = cost / float64(p.Dau)
		}
		if paying > 0 {
			p.PayingArpu = cost / float64(paying)
		}
		fc.ArpuHistory = append(fc.ArpuHistory, p)
	}
	return rows.Err()
}

// fillOperationProductMatrix 产品矩阵：套餐 BCG 散点 + 模型健康度。
//
// 套餐 BCG 象限规则（按全局中位数切）：
//   - star      用户多 + ARPU 高（明星套餐）
//   - cash_cow  用户多 + ARPU 低（薄利多销）
//   - question  用户少 + ARPU 高（高客单价但还没规模）
//   - dog       用户少 + ARPU 低（待砍）
func (r *operationAnalyticsRepository) fillOperationProductMatrix(ctx context.Context, filter service.OperationAnalyticsFilter, snapshot *service.OperationAnalyticsSnapshot) error {
	pm := &snapshot.ProductMatrix

	planQuery := `
		SELECT
			COALESCE(ul.group_id, 0) AS group_id,
			COALESCE(g.name, '未分组') AS name,
			COUNT(DISTINCT ul.user_id) AS active_users,
			COALESCE(SUM(ul.actual_cost), 0) AS revenue
		FROM usage_logs ul
		LEFT JOIN groups g ON g.id = ul.group_id
		WHERE ul.created_at >= $1 AND ul.created_at < $2
		GROUP BY ul.group_id, g.name
		HAVING COUNT(DISTINCT ul.user_id) > 0
		ORDER BY SUM(ul.actual_cost) DESC
		LIMIT 50
	`
	rows, err := r.db.QueryContext(ctx, planQuery, filter.StartTime, filter.EndTime)
	if err != nil {
		return fmt.Errorf("operation product plans: %w", err)
	}
	plans := make([]service.OperationPlanMatrix, 0)
	for rows.Next() {
		var p service.OperationPlanMatrix
		if err := rows.Scan(&p.GroupID, &p.Name, &p.ActiveUsers, &p.Revenue); err != nil {
			_ = rows.Close()
			return err
		}
		if p.ActiveUsers > 0 {
			p.ARPU = p.Revenue / float64(p.ActiveUsers)
		}
		plans = append(plans, p)
	}
	if err := closeRows(rows); err != nil {
		return err
	}

	// 自动象限：用全局中位数切
	if len(plans) > 0 {
		usersSorted := make([]int64, len(plans))
		arpuSorted := make([]float64, len(plans))
		for i, p := range plans {
			usersSorted[i] = p.ActiveUsers
			arpuSorted[i] = p.ARPU
		}
		sort.Slice(usersSorted, func(i, j int) bool { return usersSorted[i] < usersSorted[j] })
		sort.Float64s(arpuSorted)
		userMedian := usersSorted[len(usersSorted)/2]
		arpuMedian := arpuSorted[len(arpuSorted)/2]
		for i, p := range plans {
			highUsers := p.ActiveUsers >= userMedian
			highArpu := p.ARPU >= arpuMedian
			switch {
			case highUsers && highArpu:
				plans[i].Quadrant = "star"
			case highUsers && !highArpu:
				plans[i].Quadrant = "cash_cow"
			case !highUsers && highArpu:
				plans[i].Quadrant = "question"
			default:
				plans[i].Quadrant = "dog"
			}
		}
	}
	pm.Plans = plans

	// 模型健康度（含环比用户数变化）
	span := filter.EndTime.Sub(filter.StartTime)
	prevStart := filter.StartTime.Add(-span)
	modelQuery := `
		WITH curr AS (
			SELECT
				COALESCE(NULLIF(TRIM(requested_model), ''), model) AS model,
				COUNT(*) AS requests,
				COUNT(DISTINCT user_id) AS users,
				COALESCE(SUM(actual_cost), 0) AS revenue
			FROM usage_logs
			WHERE created_at >= $1 AND created_at < $2
			GROUP BY 1
		),
		prev AS (
			SELECT
				COALESCE(NULLIF(TRIM(requested_model), ''), model) AS model,
				COUNT(DISTINCT user_id) AS users
			FROM usage_logs
			WHERE created_at >= $3 AND created_at < $1
			GROUP BY 1
		),
		total AS (SELECT SUM(requests) AS total FROM curr)
		SELECT
			c.model, c.requests, c.users, c.revenue,
			COALESCE(c.requests::float / NULLIF((SELECT total FROM total), 0), 0) AS share,
			COALESCE(p.users, 0) AS prev_users
		FROM curr c
		LEFT JOIN prev p ON p.model = c.model
		ORDER BY c.revenue DESC
		LIMIT 12
	`
	rows, err = r.db.QueryContext(ctx, modelQuery, filter.StartTime, filter.EndTime, prevStart)
	if err != nil {
		return fmt.Errorf("operation product models: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var m service.OperationModelHealth
		var prevUsers int64
		if err := rows.Scan(&m.Model, &m.Requests, &m.Users, &m.Revenue, &m.TrafficShare, &prevUsers); err != nil {
			return err
		}
		m.UsersChangePercent = changePercent(float64(m.Users), float64(prevUsers))
		pm.Models = append(pm.Models, m)
	}
	return rows.Err()
}

func (r *operationAnalyticsRepository) queryUserList(ctx context.Context, query string, args ...any) ([]service.OperationUserListItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.OperationUserListItem, 0)
	for rows.Next() {
		var item service.OperationUserListItem
		var lastUsage sql.NullTime
		var daysSince sql.NullInt64
		var expiresAt sql.NullTime
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.Value, &item.ValueLabel, &lastUsage, &daysSince, &item.GroupName, &expiresAt); err != nil {
			return nil, err
		}
		if lastUsage.Valid {
			v := lastUsage.Time.UTC().Format(time.RFC3339)
			item.LastUsageAt = &v
		}
		if daysSince.Valid {
			v := daysSince.Int64
			item.DaysSince = &v
		}
		if expiresAt.Valid {
			v := expiresAt.Time.UTC().Format(time.RFC3339)
			item.ExpiresAt = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *operationAnalyticsRepository) queryDistribution(ctx context.Context, query string, args ...any) ([]service.OperationDistributionItem, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.OperationDistributionItem, 0)
	for rows.Next() {
		var item service.OperationDistributionItem
		if err := rows.Scan(&item.Key, &item.Label, &item.Requests, &item.Tokens, &item.ActualCost, &item.Users, &item.Count, &item.Value); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func ratio(numerator, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func changePercent(current, previous float64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return (current - previous) / previous * 100
}

func matureRatio(count, total int64, cohortDate, endDate time.Time, days int) *float64 {
	if cohortDate.AddDate(0, 0, days).After(endDate) {
		return nil
	}
	v := ratio(count, total)
	return &v
}

func localDate(t time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func closeRows(rows *sql.Rows) error {
	if err := rows.Close(); err != nil {
		return err
	}
	return rows.Err()
}
