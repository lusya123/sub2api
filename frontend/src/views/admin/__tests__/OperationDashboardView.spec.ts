import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import OperationDashboardView from '../OperationDashboardView.vue'

const { getSnapshot, push } = vi.hoisted(() => ({
  getSnapshot: vi.fn(),
  push: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    operations: {
      getSnapshot
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) =>
        params?.value !== undefined ? `${key}:${params.value}` : key
    })
  }
})

// Stub vue-chartjs so we don't need a real canvas during unit tests.
vi.mock('vue-chartjs', () => ({
  Line: { name: 'StubLine', render: () => null },
  Bar: { name: 'StubBar', render: () => null }
}))

// Avoid importing the heavy chart.js module graph in jsdom.
vi.mock('chart.js', () => ({
  Chart: { register: () => undefined },
  CategoryScale: {},
  LinearScale: {},
  BarElement: {},
  PointElement: {},
  LineElement: {},
  Tooltip: {},
  Filler: {},
  Legend: {}
}))

const snapshot = {
  generated_at: '2026-05-03T00:00:00Z',
  start_time: '2026-04-20T00:00:00Z',
  end_time: '2026-05-03T00:00:00Z',
  granularity: 'day',
  timezone: 'Asia/Shanghai',
  revenue_note: 'note',
  core: {
    active_users: 12,
    average_dau: 3,
    new_users: 4,
    paying_users: 2,
    first_call_conversion_rate: 0.5,
    benefit_conversion_rate: 0.25,
    retention_d1: 0.4,
    retention_d7: 0.3,
    retention_d30: 0,
    requests: 100,
    tokens: 2000,
    actual_cost: 12.34,
    active_subscriptions: 8,
    expiring_subscriptions: 2,
    active_api_keys: 5,
    arpu: 1,
    requests_per_active_user: 8,
    previous_active_users: 10,
    previous_new_users: 3,
    previous_paying_users: 1,
    previous_actual_cost: 11.0,
    active_users_change_percent: 20,
    new_users_change_percent: 33.3,
    paying_users_change_percent: 100,
    actual_cost_change_percent: 10
  },
  trend: [{
    bucket: '2026-05-03',
    new_users: 4,
    active_users: 12,
    requests: 100,
    tokens: 2000,
    actual_cost: 12.34,
    first_call_users: 2,
    benefit_users: 1,
    first_call_conversion_rate: 0.5,
    benefit_conversion_rate: 0.25
  }],
  funnel: [
    { key: 'registered', label: 'registered', count: 4, rate: 1, description: 'desc' },
    { key: 'first_call', label: 'first_call', count: 2, rate: 0.5, description: 'desc2' }
  ],
  funnel_previous: [
    { key: 'registered', label: 'registered', count: 3, rate: 1, description: 'desc' },
    { key: 'first_call', label: 'first_call', count: 2, rate: 0.66, description: 'desc2' }
  ],
  trial_funnel: {
    trial_users_issued: 50,
    trial_users_used: 30,
    trial_users_idle: 20,
    trial_users_exhausted: 8,
    trial_users_converted: 4,
    non_trial_paid: 2,
    use_rate: 0.6,
    idle_rate: 0.4,
    exhaustion_rate: 0.16,
    conversion_rate: 0.08,
    avg_consumed: 2.35,
    trial_balance_value: 5,
    exhaustion_threshold: 4
  },
  cohorts: [{ cohort_date: '2026-05-03', new_users: 4, d0: 0.5, d1: null, d7: null, d30: null }],
  lists: {
    high_spending: [{ user_id: 7, email: 'u@test.dev', username: '', value: 12, value_label: 'spend' }],
    silent_high_value: [{ user_id: 9, email: 'silent@test.dev', username: '', value: 99, value_label: 'spend' }],
    benefit_idle: [],
    expiring_soon: [],
    new_inactive: []
  },
  distribution: {
    groups: [],
    models: [],
    api_keys: [],
    promos: [],
    redeem_types: []
  },
  churn: {
    global_p50_days: 5.4,
    definition: 'churn-def',
    healthy_users: 50,
    at_risk_users: 8,
    high_risk_users: 4,
    churned_users: 6,
    base_users: 68,
    churn_rate: 0.088,
    at_risk_rate: 0.176,
    previous_churn_rate: 0.05,
    churn_rate_change_pct: 76,
    high_value_at_risk: 5,
    high_value_revenue: 250.5,
    newly_churned_revenue: 88.0,
    waterfall: {
      last_period_active: 100,
      still_active: 80,
      completely_gone: 10,
      half_activity: 5,
      balance_exhausted: 3,
      subscription_ended: 2
    },
    history: [
      { bucket: '2026-05-01', churn_rate: 0.05, churned: 5 },
      { bucket: '2026-05-02', churn_rate: 0.06, churned: 6 }
    ]
  },
  baselines: {
    window_days: 14,
    yoy_available: false,
    actual_cost_yoy_percent: null,
    active_users_yoy_percent: null,
    new_users_yoy_percent: null,
    actual_cost_vs_90d_avg_percent: 5,
    active_users_vs_90d_avg_percent: 8,
    new_users_vs_90d_avg_percent: -2,
    history_average_daily_cost: 1.0,
    history_average_daily_active: 8,
    history_average_daily_new: 2
  },
  pyramid: {
    generated_at: '2026-05-03T00:00:00Z',
    window_days: 30,
    total_users: 100,
    paid_users: 30,
    paid_percent: 0.3,
    total_revenue: 1000,
    levels: [
      { key: 'super', label: '💎', users: 1, user_percent: 0.01, revenue: 380, revenue_percent: 0.38, avg_revenue_per_user: 380, min_revenue: 200 },
      { key: 'gold', label: '🥇', users: 5, user_percent: 0.05, revenue: 320, revenue_percent: 0.32, avg_revenue_per_user: 64, min_revenue: 50 },
      { key: 'silver', label: '🥈', users: 9, user_percent: 0.09, revenue: 200, revenue_percent: 0.20, avg_revenue_per_user: 22, min_revenue: 10 },
      { key: 'bronze', label: '🥉', users: 15, user_percent: 0.15, revenue: 100, revenue_percent: 0.10, avg_revenue_per_user: 6, min_revenue: 1 },
      { key: 'free', label: '🆓', users: 70, user_percent: 0.70, revenue: 0, revenue_percent: 0, avg_revenue_per_user: 0, min_revenue: 0 }
    ]
  },
  financial: {
    total_balance: 4500,
    daily_avg_cost: 50,
    balance_months_cushion: 3.0,
    balance_health: 'healthy',
    admin_topup_gross: 250,
    admin_refund_amount: 50,
    admin_refund_count: 2,
    admin_topup_net: 200,
    redeem_balance_real: 800,
    redeem_trial: 350,
    redeem_subscription_count: 4,
    new_subscriptions_count: 5,
    inflow_total: 1000,
    inflow_gross: 1050,
    outflow_total: 700,
    net_flow: 300,
    refund_rate: 0.0476,
    arpu_history: [
      { bucket: '2026-04-01', arpu: 0.5, paying_arpu: 2.0, dau: 10 },
      { bucket: '2026-04-02', arpu: 0.6, paying_arpu: 2.5, dau: 12 }
    ]
  },
  product_matrix: {
    plans: [
      { group_id: 1, name: '金卡', active_users: 20, revenue: 400, arpu: 20, quadrant: 'star' },
      { group_id: 2, name: '银卡', active_users: 50, revenue: 100, arpu: 2, quadrant: 'cash_cow' },
      { group_id: 3, name: '试用', active_users: 5, revenue: 60, arpu: 12, quadrant: 'question' },
      { group_id: 4, name: '免费', active_users: 5, revenue: 5, arpu: 1, quadrant: 'dog' }
    ],
    models: [
      { model: 'claude-opus-4', requests: 1000, users: 20, revenue: 400, traffic_share: 0.5, users_change_percent: 10 },
      { model: 'claude-sonnet-4', requests: 600, users: 30, revenue: 100, traffic_share: 0.3, users_change_percent: -5 }
    ]
  },
  advice: [{ level: 'warning', title: 'advice-title', detail: 'advice-detail', action: 'advice-action' }]
}

const stubs = {
  AppLayout: { template: '<div><slot /></div>' },
  DateRangePicker: true,
  LoadingSpinner: true,
  Select: true,
  StubLine: true,
  StubBar: true
}

describe('OperationDashboardView', () => {
  beforeEach(() => {
    getSnapshot.mockReset()
    push.mockReset()
    getSnapshot.mockResolvedValue(snapshot)
  })

  it('renders the new pulse, churn and pyramid sections without advice banner', async () => {
    const wrapper = mount(OperationDashboardView, { global: { stubs } })
    await flushPromises()

    expect(getSnapshot).toHaveBeenCalled()
    const today = new Date()
    const todayText = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`
    expect(getSnapshot).toHaveBeenCalledWith(expect.objectContaining({
      start_date: todayText,
      end_date: todayText,
      modules: 'summary'
    }))
    const text = wrapper.text()

    // Header + revenue card
    expect(text).toContain('admin.operations.title')
    expect(text).toContain('$12.34')

    // Action advice was removed from the operations dashboard surface.
    expect(text).not.toContain('advice-title')
    expect(text).not.toContain('advice-action')

    // Churn definition surfaced as inline explanation
    expect(text).toContain('churn-def')

    // Pyramid total + paid percent
    expect(text).toContain('100')
  })

  it('opens the silent high-value list via the action card and links to usage details', async () => {
    const wrapper = mount(OperationDashboardView, { global: { stubs } })
    await flushPromises()

    // Click the "view list" button on one of the action summary cards
    const viewListButtons = wrapper.findAll('button').filter((btn) => btn.text() === 'admin.operations.v2.actions.viewList')
    expect(viewListButtons.length).toBeGreaterThan(0)
    await viewListButtons[0].trigger('click')
    await flushPromises()

    // Modal renders the list item
    expect(wrapper.text()).toContain('silent@test.dev')

    // Click "usage" inside the modal
    const usageButton = wrapper.findAll('button').find((btn) => btn.text() === 'admin.operations.actions.usage')
    expect(usageButton).toBeDefined()
    await usageButton?.trigger('click')

    expect(push).toHaveBeenCalledWith(expect.objectContaining({
      path: '/admin/usage',
      query: expect.objectContaining({ user_id: '9' })
    }))
  })

  it('requests all-data snapshots without start and end dates', async () => {
    const wrapper = mount(OperationDashboardView, { global: { stubs } })
    await flushPromises()
    getSnapshot.mockClear()

    const picker = wrapper.findComponent({ name: 'DateRangePicker' })
    await picker.vm.$emit('change', { startDate: '', endDate: '', preset: 'all' })
    await flushPromises()

    expect(getSnapshot).toHaveBeenCalledWith(expect.objectContaining({
      range: 'all',
      granularity: 'day',
      modules: 'summary'
    }))
    expect(getSnapshot.mock.calls[0][0]).not.toHaveProperty('start_date')
    expect(getSnapshot.mock.calls[0][0]).not.toHaveProperty('end_date')
  })

  it('requests custom long ranges with concrete dates', async () => {
    const wrapper = mount(OperationDashboardView, { global: { stubs } })
    await flushPromises()
    getSnapshot.mockClear()

    const picker = wrapper.findComponent({ name: 'DateRangePicker' })
    await picker.vm.$emit('change', { startDate: '2026-01-01', endDate: '2026-06-29', preset: '180days' })
    await flushPromises()

    expect(getSnapshot).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-01-01',
      end_date: '2026-06-29',
      granularity: 'day',
      modules: 'summary'
    }))
  })

  it('does not crash when a cached old snapshot is missing new module objects', async () => {
    const legacySnapshot = { ...snapshot }
    delete (legacySnapshot as Record<string, unknown>).financial
    delete (legacySnapshot as Record<string, unknown>).product_matrix
    getSnapshot.mockResolvedValueOnce(legacySnapshot)

    const wrapper = mount(OperationDashboardView, { global: { stubs } })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.operations.title')
    expect(wrapper.text()).toContain('$12.34')
  })
})
