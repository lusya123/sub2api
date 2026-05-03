import { apiClient } from '../client'

export interface OperationSnapshotParams {
  start_date?: string
  end_date?: string
  granularity?: 'day' | 'hour'
  timezone?: string
  range?: 'all'
  modules?: string
}

export interface OperationCoreMetrics {
  active_users: number
  average_dau: number
  new_users: number
  paying_users: number
  consuming_users: number
  trial_consuming_users: number
  first_call_conversion_rate: number
  benefit_conversion_rate: number
  retention_d1: number
  retention_d7: number
  retention_d30: number
  requests: number
  tokens: number
  actual_cost: number
  active_subscriptions: number
  expiring_subscriptions: number
  active_api_keys: number
  arpu: number
  requests_per_active_user: number
  previous_active_users: number
  previous_new_users: number
  previous_paying_users: number
  previous_consuming_users: number
  previous_trial_consuming_users: number
  previous_actual_cost: number
  active_users_change_percent: number
  new_users_change_percent: number
  paying_users_change_percent: number
  consuming_users_change_percent: number
  trial_consuming_users_change_percent: number
  actual_cost_change_percent: number
}

export interface OperationChurnWaterfall {
  last_period_active: number
  still_active: number
  completely_gone: number
  half_activity: number
  balance_exhausted: number
  subscription_ended: number
}

export interface OperationChurnHistoryPoint {
  bucket: string
  churn_rate: number
  churned: number
}

export interface OperationChurnSnapshot {
  global_p50_days: number
  definition: string
  healthy_users: number
  at_risk_users: number
  high_risk_users: number
  churned_users: number
  base_users: number
  churn_rate: number
  at_risk_rate: number
  previous_churn_rate: number
  churn_rate_change_pct: number
  high_value_at_risk: number
  high_value_revenue: number
  newly_churned_revenue: number
  waterfall: OperationChurnWaterfall
  history: OperationChurnHistoryPoint[]
}

export interface OperationBaselines {
  window_days: number
  yoy_available: boolean
  actual_cost_yoy_percent?: number | null
  active_users_yoy_percent?: number | null
  new_users_yoy_percent?: number | null
  actual_cost_vs_90d_avg_percent: number
  active_users_vs_90d_avg_percent: number
  new_users_vs_90d_avg_percent: number
  history_average_daily_cost: number
  history_average_daily_active: number
  history_average_daily_new: number
}

export interface OperationPyramidLevel {
  key: 'super' | 'gold' | 'silver' | 'bronze' | 'free'
  label: string
  users: number
  user_percent: number
  revenue: number
  revenue_percent: number
  avg_revenue_per_user: number
  min_revenue: number
}

export interface OperationUserPyramid {
  generated_at: string
  window_days: number
  total_users: number
  paid_users: number
  paid_percent: number
  total_revenue: number
  levels: OperationPyramidLevel[]
}

export interface OperationTrendPoint {
  bucket: string
  new_users: number
  active_users: number
  requests: number
  tokens: number
  actual_cost: number
  first_call_users: number
  benefit_users: number
  first_call_conversion_rate: number
  benefit_conversion_rate: number
}

export interface OperationFunnelStep {
  key: string
  label: string
  count: number
  rate: number
  description: string
}

export interface OperationArpuPoint {
  bucket: string
  arpu: number
  paying_arpu: number
  dau: number
}

export interface OperationFinancialCockpit {
  total_balance: number
  daily_avg_cost: number
  balance_months_cushion: number
  balance_health: 'healthy' | 'warning' | 'danger' | 'overloaded' | string
  // 第 1 类：管理员充值（拆正负）
  admin_topup_gross: number
  admin_refund_amount: number
  admin_refund_count: number
  admin_topup_net: number
  // 第 2 类：兑换码兑换
  redeem_balance_real: number
  redeem_trial: number
  redeem_subscription_count: number
  new_subscriptions_count: number
  // 现金流
  inflow_total: number
  inflow_gross: number
  outflow_total: number
  net_flow: number
  refund_rate: number
  arpu_history: OperationArpuPoint[]
}

export interface OperationPlanMatrix {
  group_id: number
  name: string
  active_users: number
  revenue: number
  arpu: number
  quadrant: 'star' | 'cash_cow' | 'question' | 'dog' | string
}

export interface OperationModelHealth {
  model: string
  requests: number
  users: number
  revenue: number
  traffic_share: number
  users_change_percent: number
}

export interface OperationProductMatrix {
  plans: OperationPlanMatrix[]
  models: OperationModelHealth[]
}

export interface OperationTrialFunnel {
  trial_users_issued: number
  trial_users_used: number
  trial_users_idle: number
  trial_users_exhausted: number
  trial_users_converted: number
  non_trial_paid: number
  use_rate: number
  idle_rate: number
  exhaustion_rate: number
  conversion_rate: number
  avg_consumed: number
  trial_balance_value: number
  exhaustion_threshold: number
}

export interface OperationRetentionCohort {
  cohort_date: string
  new_users: number
  d0: number | null
  d1: number | null
  d7: number | null
  d30: number | null
}

export interface OperationUserListItem {
  user_id: number
  email: string
  username: string
  value: number
  value_label: string
  last_usage_at?: string
  days_since?: number
  group_name?: string
  expires_at?: string
}

export interface OperationUserLists {
  high_spending: OperationUserListItem[]
  silent_high_value: OperationUserListItem[]
  benefit_idle: OperationUserListItem[]
  expiring_soon: OperationUserListItem[]
  new_inactive: OperationUserListItem[]
}

export interface OperationDistributionItem {
  key: string
  label: string
  requests?: number
  tokens?: number
  actual_cost?: number
  users?: number
  count?: number
  value?: number
}

export interface OperationDistributionSnapshot {
  groups: OperationDistributionItem[]
  models: OperationDistributionItem[]
  api_keys: OperationDistributionItem[]
  promos: OperationDistributionItem[]
  redeem_types: OperationDistributionItem[]
}

export interface OperationAdvice {
  level: 'success' | 'info' | 'warning' | string
  title: string
  detail: string
  action: string
}

export interface OperationSnapshot {
  generated_at: string
  start_time: string
  end_time: string
  granularity: 'day' | 'hour'
  timezone: string
  revenue_note: string
  core: OperationCoreMetrics
  trend: OperationTrendPoint[]
  funnel: OperationFunnelStep[]
  funnel_previous: OperationFunnelStep[]
  trial_funnel: OperationTrialFunnel
  cohorts: OperationRetentionCohort[]
  lists: OperationUserLists
  distribution: OperationDistributionSnapshot
  churn: OperationChurnSnapshot
  baselines: OperationBaselines
  pyramid: OperationUserPyramid
  financial: OperationFinancialCockpit
  product_matrix: OperationProductMatrix
  module_statuses?: Record<string, string>
  advice: OperationAdvice[]
}

export async function getSnapshot(params?: OperationSnapshotParams): Promise<OperationSnapshot> {
  const { data } = await apiClient.get<OperationSnapshot>('/admin/operations/snapshot', {
    params,
    timeout: 300000
  })
  return data
}

export const operationsAPI = {
  getSnapshot
}

export default operationsAPI
