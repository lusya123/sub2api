<template>
  <AppLayout>
    <div class="space-y-8 pb-12">
      <!-- Header -->
      <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.operations.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.operations.description') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-3">
          <DateRangePicker
            v-model:start-date="startDate"
            v-model:end-date="endDate"
            include-half-year
            include-all
            default-preset="today"
            @change="handleDateRangeChange"
          />
          <div class="w-28">
            <Select v-model="granularity" :options="granularityOptions" @change="loadSnapshot" />
          </div>
          <button class="btn btn-primary" :disabled="loading" @click="loadSnapshot">
            {{ t('common.refresh') }}
          </button>
        </div>
      </div>

      <div
        v-if="snapshot?.revenue_note"
        class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-800/50 dark:bg-amber-900/20 dark:text-amber-200"
      >
        {{ snapshot.revenue_note }}
      </div>

      <div v-if="loading && !snapshot" class="flex items-center justify-center py-20">
        <LoadingSpinner size="lg" />
      </div>

      <template v-else-if="snapshot">
        <!-- ───── 第 1 屏：今日脉搏 ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.pulse.title')"
          :sub="t('admin.operations.v2.pulse.sub')"
        />
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
          <PulseCard
            :label="t('admin.operations.v2.pulse.revenue')"
            :value="`$${formatMoney(snapshot.core.actual_cost)}`"
            :wow="snapshot.core.actual_cost_change_percent"
            :yoy="snapshot.baselines.actual_cost_yoy_percent"
            :history="snapshot.baselines.actual_cost_vs_90d_avg_percent"
            :spark="trendSpark('actual_cost')"
            :hint="pulseHint('revenue', snapshot.core.actual_cost_change_percent)"
            tone="emerald"
          />
          <PulseCard
            :label="t('admin.operations.v2.pulse.activeUsers')"
            :value="formatNumber(snapshot.core.active_users)"
            :wow="snapshot.core.active_users_change_percent"
            :yoy="snapshot.baselines.active_users_yoy_percent"
            :history="snapshot.baselines.active_users_vs_90d_avg_percent"
            :spark="trendSpark('active_users')"
            :hint="pulseHint('active', snapshot.core.active_users_change_percent)"
            tone="sky"
          />
          <PulseCard
            :label="t('admin.operations.v2.pulse.newUsers')"
            :value="formatNumber(snapshot.core.new_users)"
            :wow="snapshot.core.new_users_change_percent"
            :yoy="snapshot.baselines.new_users_yoy_percent"
            :history="snapshot.baselines.new_users_vs_90d_avg_percent"
            :spark="trendSpark('new_users')"
            :hint="pulseHint('new', snapshot.core.new_users_change_percent)"
            tone="violet"
          />
          <PulseCard
            :label="t('admin.operations.v2.pulse.payingUsers')"
            :value="formatNumber(snapshot.core.paying_users)"
            :wow="snapshot.core.paying_users_change_percent"
            :history="0"
            :spark="trendSpark('actual_cost')"
            :hint="pulseHint('paying', snapshot.core.paying_users_change_percent)"
            tone="amber"
          />
        </div>

        <!-- ───── 第 2 屏：销售漏斗 ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.funnel.title')"
          :sub="t('admin.operations.v2.funnel.sub')"
        />
        <FunnelSection v-if="!isModuleLoading('funnel')" :current="snapshot.funnel" :previous="snapshot.funnel_previous" />
        <ModuleLoadingNotice v-else />

        <!-- ───── 第 2.5 屏：5 元体验券转化分析 ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.trial.title')"
          :sub="t('admin.operations.v2.trial.sub')"
        />
        <TrialFunnelSection v-if="!isModuleLoading('trial')" :trial="snapshot.trial_funnel" />
        <ModuleLoadingNotice v-else />

        <!-- ───── 第 3 屏：留存与流失 ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.churn.title')"
          :sub="t('admin.operations.v2.churn.sub')"
        />
        <ChurnSection
          v-if="!isModuleLoading('churn')"
          :churn="snapshot.churn"
          :retention-d1="snapshot.core.retention_d1"
          :retention-d7="snapshot.core.retention_d7"
          :retention-d30="snapshot.core.retention_d30"
        />
        <ModuleLoadingNotice v-else />

        <!-- ───── 第 4 屏：用户金字塔 ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.pyramid.title')"
          :sub="t('admin.operations.v2.pyramid.sub')"
        />
        <PyramidSection v-if="!isModuleLoading('pyramid')" :pyramid="snapshot.pyramid" />
        <ModuleLoadingNotice v-else />

        <!-- ───── 第 4.5 屏：产品矩阵 ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.product.title')"
          :sub="t('admin.operations.v2.product.sub')"
        />
        <ProductMatrixSection v-if="!isModuleLoading('product_matrix')" :matrix="snapshot.product_matrix" />
        <ModuleLoadingNotice v-else />

        <!-- ───── 第 4.7 屏：财务驾驶舱 ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.financial.title')"
          :sub="t('admin.operations.v2.financial.sub')"
        />
        <FinancialCockpitSection v-if="!isModuleLoading('financial')" :financial="snapshot.financial" />
        <ModuleLoadingNotice v-else />

        <!-- ───── 第 5 屏：行动汇总 ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.actions.title')"
          :sub="t('admin.operations.v2.actions.sub')"
        />
        <ActionSummary
          v-if="!isModuleLoading('churn') && !isModuleLoading('lists')"
          :churn="snapshot.churn"
          :core="snapshot.core"
          :lists="snapshot.lists"
          @open-list="openListModal"
        />
        <ModuleLoadingNotice v-else />

        <!-- ───── 详情面板（次要） ───── -->
        <SectionHeader
          :title="t('admin.operations.v2.details.title')"
          :sub="t('admin.operations.v2.details.sub')"
        />
        <div class="flex gap-2 overflow-x-auto border-b border-gray-200 dark:border-dark-700">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            class="whitespace-nowrap border-b-2 px-3 py-2 text-sm font-medium"
            :class="activeTab === tab.key ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-100'"
            @click="activeTab = tab.key"
          >
            {{ tab.label }}
          </button>
        </div>

        <section v-if="activeTab === 'cohort' && !isModuleLoading('cohorts')" class="card p-5">
          <ChartCaption
            :title="t('admin.operations.v2.cohort.title')"
            :hint="t('admin.operations.v2.cohort.hint')"
            :action="t('admin.operations.v2.cohort.action')"
          />
          <CohortHeatmap :cohorts="snapshot.cohorts" />
        </section>
        <ModuleLoadingNotice v-else-if="activeTab === 'cohort'" />

        <section v-else-if="activeTab === 'distribution' && !isModuleLoading('distribution')" class="grid grid-cols-1 gap-6 xl:grid-cols-2">
          <DistributionPanel :title="t('admin.operations.distribution.groups')" :items="snapshot.distribution.groups" kind="usage" />
          <DistributionPanel :title="t('admin.operations.distribution.models')" :items="snapshot.distribution.models" kind="usage" />
          <DistributionPanel :title="t('admin.operations.distribution.apiKeys')" :items="snapshot.distribution.api_keys" kind="usage" />
          <DistributionPanel :title="t('admin.operations.distribution.promos')" :items="[...snapshot.distribution.promos, ...snapshot.distribution.redeem_types]" kind="conversion" />
        </section>
        <ModuleLoadingNotice v-else-if="activeTab === 'distribution'" />

        <section v-else-if="activeTab === 'trend'" class="grid grid-cols-1 gap-6 xl:grid-cols-2">
          <TrendPanel
            :title="t('admin.operations.trend.activeRevenue')"
            :items="snapshot.trend"
            metric="actual_cost"
            :formatter="(value) => `$${formatMoney(value)}`"
          />
          <TrendPanel
            :title="t('admin.operations.trend.conversion')"
            :items="snapshot.trend"
            metric="first_call_conversion_rate"
            :formatter="formatPercent"
          />
        </section>

        <!-- 名单弹窗 -->
        <ListModal
          v-if="listModal.open"
          :title="listModal.title"
          :items="listModal.items"
          :hint="listModal.hint"
          @close="listModal.open = false"
          @open-usage="openUsage"
          @open-subscriptions="openSubscriptions"
        />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import { adminAPI } from '@/api/admin'
import type {
  OperationBaselines,
  OperationChurnSnapshot,
  OperationCoreMetrics,
  OperationDistributionSnapshot,
  OperationDistributionItem,
  OperationFinancialCockpit,
  OperationFunnelStep,
  OperationPlanMatrix,
  OperationProductMatrix,
  OperationPyramidLevel,
  OperationRetentionCohort,
  OperationSnapshot,
  OperationSnapshotParams,
  OperationTrendPoint,
  OperationTrialFunnel,
  OperationUserListItem,
  OperationUserLists,
  OperationUserPyramid
} from '@/api/admin/operations'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const snapshot = ref<OperationSnapshot | null>(null)
const loading = ref(false)
const moduleLoading = ref<Record<string, boolean>>({})
const loadRun = ref(0)
const activeTab = ref<'cohort' | 'distribution' | 'trend'>('cohort')
const granularity = ref<'day' | 'hour'>('day')
const datePreset = ref<string | null>('today')

const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const end = new Date()
const start = new Date(end)
const startDate = ref(formatLocalDate(start))
const endDate = ref(formatLocalDate(end))

const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  ...(datePreset.value === 'all' ? [] : [{ value: 'hour', label: t('admin.dashboard.hour') }])
])

const tabs = computed(() => [
  { key: 'cohort', label: t('admin.operations.v2.tabs.cohort') },
  { key: 'distribution', label: t('admin.operations.v2.tabs.distribution') },
  { key: 'trend', label: t('admin.operations.v2.tabs.trend') }
] as const)

interface ListModalState {
  open: boolean
  title: string
  items: OperationUserListItem[]
  hint: string
}
const listModal = reactive<ListModalState>({ open: false, title: '', items: [], hint: '' })

function openListModal(payload: { title: string; items: OperationUserListItem[]; hint: string }) {
  listModal.open = true
  listModal.title = payload.title
  listModal.items = payload.items
  listModal.hint = payload.hint
}

async function loadSnapshot() {
  const run = ++loadRun.value
  loading.value = true
  moduleLoading.value = {}
  try {
    if (datePreset.value === 'all') {
      granularity.value = 'day'
    }
    const params: OperationSnapshotParams = {
      granularity: granularity.value,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'Asia/Shanghai',
      modules: 'summary'
    }

    if (datePreset.value === 'all') {
      params.range = 'all'
    } else {
      params.start_date = startDate.value
      params.end_date = endDate.value
    }

    snapshot.value = normalizeSnapshot(await adminAPI.operations.getSnapshot(params))
    loading.value = false

    if (datePreset.value !== 'all') {
      void loadSupplementalModules(params, run)
    }
  } catch (error) {
    console.error('Failed to load operation dashboard', error)
    appStore.showError(t('admin.operations.failedToLoad'))
    loading.value = false
  }
}

async function loadSupplementalModules(baseParams: OperationSnapshotParams, run: number) {
  const groups = [
    ['funnel', 'trial'],
    ['baselines', 'lists', 'cohorts'],
    ['pyramid', 'distribution'],
    ['financial'],
    ['product_matrix'],
    ['churn']
  ]
  setModulesLoading([...new Set(groups.flat())], true)

  for (const modules of groups) {
    if (run !== loadRun.value) return
    try {
      const partial = normalizeSnapshot(await adminAPI.operations.getSnapshot({
        ...baseParams,
        modules: modules.join(',')
      }))
      if (run !== loadRun.value || !snapshot.value) return
      mergeSnapshotModules(snapshot.value, partial, modules)
    } catch (error) {
      console.error(`Failed to load operation dashboard modules: ${modules.join(',')}`, error)
      appStore.showError(t('admin.operations.failedToLoad'))
    } finally {
      setModulesLoading(modules, false)
    }
  }
}

function setModulesLoading(modules: string[], value: boolean) {
  moduleLoading.value = {
    ...moduleLoading.value,
    ...Object.fromEntries(modules.map((module) => [module, value]))
  }
}

function isModuleLoading(module: string): boolean {
  return Boolean(moduleLoading.value[module])
}

function mergeSnapshotModules(target: OperationSnapshot, partial: OperationSnapshot, modules: string[]) {
  target.generated_at = partial.generated_at || target.generated_at
  target.module_statuses = { ...(target.module_statuses || {}), ...(partial.module_statuses || {}) }
  for (const module of modules) {
    switch (module) {
      case 'core':
        target.core = partial.core
        break
      case 'trend':
        target.trend = partial.trend
        break
      case 'baselines':
        target.baselines = partial.baselines
        break
      case 'funnel':
        target.funnel = partial.funnel
        target.funnel_previous = partial.funnel_previous
        break
      case 'trial':
        target.trial_funnel = partial.trial_funnel
        break
      case 'lists':
        target.lists = partial.lists
        break
      case 'cohorts':
        target.cohorts = partial.cohorts
        break
      case 'distribution':
        target.distribution = partial.distribution
        break
      case 'churn':
        target.churn = partial.churn
        break
      case 'pyramid':
        target.pyramid = partial.pyramid
        break
      case 'financial':
        target.financial = partial.financial
        break
      case 'product_matrix':
        target.product_matrix = partial.product_matrix
        break
    }
  }
}

function normalizeSnapshot(input: OperationSnapshot): OperationSnapshot {
  const emptyChurnValue = emptyChurn()
  return {
    ...emptySnapshot(),
    ...input,
    core: { ...emptyCore(), ...(input?.core || {}) },
    trend: input?.trend || [],
    funnel: input?.funnel || [],
    funnel_previous: input?.funnel_previous || [],
    trial_funnel: { ...emptyTrialFunnel(), ...(input?.trial_funnel || {}) },
    cohorts: input?.cohorts || [],
    lists: { ...emptyLists(), ...(input?.lists || {}) },
    distribution: { ...emptyDistribution(), ...(input?.distribution || {}) },
    churn: {
      ...emptyChurnValue,
      ...(input?.churn || {}),
      waterfall: { ...emptyChurnValue.waterfall, ...(input?.churn?.waterfall || {}) },
      history: input?.churn?.history || []
    },
    baselines: { ...emptyBaselines(), ...(input?.baselines || {}) },
    pyramid: { ...emptyPyramid(), ...(input?.pyramid || {}) },
    financial: { ...emptyFinancial(), ...(input?.financial || {}) },
    product_matrix: { ...emptyProductMatrix(), ...(input?.product_matrix || {}) },
    advice: input?.advice || []
  }
}

function emptySnapshot(): OperationSnapshot {
  return {
    generated_at: '',
    start_time: '',
    end_time: '',
    granularity: 'day',
    timezone: 'Asia/Shanghai',
    revenue_note: '',
    core: emptyCore(),
    trend: [],
    funnel: [],
    funnel_previous: [],
    trial_funnel: emptyTrialFunnel(),
    cohorts: [],
    lists: emptyLists(),
    distribution: emptyDistribution(),
    churn: emptyChurn(),
    baselines: emptyBaselines(),
    pyramid: emptyPyramid(),
    financial: emptyFinancial(),
    product_matrix: emptyProductMatrix(),
    module_statuses: {},
    advice: []
  }
}

function emptyCore(): OperationCoreMetrics {
  return {
    active_users: 0,
    average_dau: 0,
    new_users: 0,
    paying_users: 0,
    first_call_conversion_rate: 0,
    benefit_conversion_rate: 0,
    retention_d1: 0,
    retention_d7: 0,
    retention_d30: 0,
    requests: 0,
    tokens: 0,
    actual_cost: 0,
    active_subscriptions: 0,
    expiring_subscriptions: 0,
    active_api_keys: 0,
    arpu: 0,
    requests_per_active_user: 0,
    previous_active_users: 0,
    previous_new_users: 0,
    previous_paying_users: 0,
    previous_actual_cost: 0,
    active_users_change_percent: 0,
    new_users_change_percent: 0,
    paying_users_change_percent: 0,
    actual_cost_change_percent: 0
  }
}

function emptyTrialFunnel(): OperationTrialFunnel {
  return {
    trial_users_issued: 0,
    trial_users_used: 0,
    trial_users_idle: 0,
    trial_users_exhausted: 0,
    trial_users_converted: 0,
    non_trial_paid: 0,
    use_rate: 0,
    idle_rate: 0,
    exhaustion_rate: 0,
    conversion_rate: 0,
    avg_consumed: 0,
    trial_balance_value: 5,
    exhaustion_threshold: 4
  }
}

function emptyLists(): OperationUserLists {
  return {
    high_spending: [],
    silent_high_value: [],
    benefit_idle: [],
    expiring_soon: [],
    new_inactive: []
  }
}

function emptyDistribution(): OperationDistributionSnapshot {
  return { groups: [], models: [], api_keys: [], promos: [], redeem_types: [] }
}

function emptyChurn(): OperationChurnSnapshot {
  return {
    global_p50_days: 0,
    definition: '',
    healthy_users: 0,
    at_risk_users: 0,
    high_risk_users: 0,
    churned_users: 0,
    base_users: 0,
    churn_rate: 0,
    at_risk_rate: 0,
    previous_churn_rate: 0,
    churn_rate_change_pct: 0,
    high_value_at_risk: 0,
    high_value_revenue: 0,
    newly_churned_revenue: 0,
    waterfall: {
      last_period_active: 0,
      still_active: 0,
      completely_gone: 0,
      half_activity: 0,
      balance_exhausted: 0,
      subscription_ended: 0
    },
    history: []
  }
}

function emptyBaselines(): OperationBaselines {
  return {
    window_days: 0,
    yoy_available: false,
    actual_cost_yoy_percent: null,
    active_users_yoy_percent: null,
    new_users_yoy_percent: null,
    actual_cost_vs_90d_avg_percent: 0,
    active_users_vs_90d_avg_percent: 0,
    new_users_vs_90d_avg_percent: 0,
    history_average_daily_cost: 0,
    history_average_daily_active: 0,
    history_average_daily_new: 0
  }
}

function emptyPyramid(): OperationUserPyramid {
  return {
    generated_at: '',
    window_days: 30,
    total_users: 0,
    paid_users: 0,
    paid_percent: 0,
    total_revenue: 0,
    levels: []
  }
}

function emptyFinancial(): OperationFinancialCockpit {
  return {
    total_balance: 0,
    daily_avg_cost: 0,
    balance_months_cushion: 0,
    balance_health: 'healthy',
    admin_topup_gross: 0,
    admin_refund_amount: 0,
    admin_refund_count: 0,
    admin_topup_net: 0,
    redeem_balance_real: 0,
    redeem_trial: 0,
    redeem_subscription_count: 0,
    new_subscriptions_count: 0,
    inflow_total: 0,
    inflow_gross: 0,
    outflow_total: 0,
    net_flow: 0,
    refund_rate: 0,
    arpu_history: []
  }
}

function emptyProductMatrix(): OperationProductMatrix {
  return { plans: [], models: [] }
}

function handleDateRangeChange(range: { startDate: string; endDate: string; preset: string | null }) {
  startDate.value = range.startDate
  endDate.value = range.endDate
  datePreset.value = range.preset
  if (range.preset === 'all') {
    granularity.value = 'day'
    void loadSnapshot()
    return
  }
  const diff = Math.abs(new Date(range.endDate).getTime() - new Date(range.startDate).getTime())
  granularity.value = diff <= 36 * 60 * 60 * 1000 ? 'hour' : 'day'
  void loadSnapshot()
}

function openUsage(item: OperationUserListItem) {
  const query: Record<string, string> = { user_id: String(item.user_id) }
  if (datePreset.value !== 'all') {
    query.start_date = startDate.value
    query.end_date = endDate.value
  }
  void router.push({
    path: '/admin/usage',
    query
  })
}

function openSubscriptions(item: OperationUserListItem) {
  void router.push({ path: '/admin/subscriptions', query: { user_id: String(item.user_id) } })
}

function trendSpark(metric: keyof OperationTrendPoint): number[] {
  if (!snapshot.value) return []
  return snapshot.value.trend.map((p) => Number(p[metric]) || 0)
}

function formatNumber(value: number | undefined): string {
  return Math.round(value || 0).toLocaleString()
}

function formatMoney(value: number | undefined): string {
  const v = value || 0
  if (v >= 1000) return `${(v / 1000).toFixed(2)}K`
  if (v >= 1) return v.toFixed(2)
  return v.toFixed(4)
}

function formatPercent(value: number | undefined): string {
  return `${((value || 0) * 100).toFixed(1)}%`
}

function pulseHint(kind: 'revenue' | 'active' | 'new' | 'paying', wow: number): string {
  const dir = wow > 1 ? 'up' : wow < -1 ? 'down' : 'flat'
  return t(`admin.operations.v2.pulse.hint.${kind}.${dir}`)
}

const MetricChange = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: Number, default: 0 },
    available: { type: Boolean, default: true }
  },
  setup(props) {
    return () => {
      if (!props.available) {
        return h('div', { class: 'flex items-baseline gap-1 text-xs text-gray-400 dark:text-gray-500' }, [
          h('span', props.label),
          h('span', '—')
        ])
      }
      const v = props.value || 0
      const cls =
        v > 1 ? 'text-emerald-600 dark:text-emerald-400'
        : v < -1 ? 'text-rose-600 dark:text-rose-400'
        : 'text-gray-500 dark:text-gray-400'
      const sign = v > 0 ? '+' : ''
      return h('div', { class: 'flex items-baseline gap-1 text-xs' }, [
        h('span', { class: 'text-gray-500 dark:text-gray-400' }, props.label),
        h('span', { class: `font-semibold ${cls}` }, `${sign}${v.toFixed(1)}%`)
      ])
    }
  }
})

const Sparkline = defineComponent({
  props: {
    values: { type: Array as PropType<number[]>, required: true },
    color: { type: String, default: '#10b981' }
  },
  setup(props) {
    return () => {
      const data = props.values.length ? props.values.map((v) => Number(v) || 0) : [0]
      const width = 240
      const height = 54
      const pad = 3
      const min = Math.min(...data)
      const max = Math.max(...data)
      const range = max - min || 1
      const step = data.length > 1 ? (width - pad * 2) / (data.length - 1) : 0
      const points = data.map((v, i) => {
        const x = pad + i * step
        const y = height - pad - ((v - min) / range) * (height - pad * 2)
        return `${x.toFixed(1)},${y.toFixed(1)}`
      })
      const area = `${pad},${height - pad} ${points.join(' ')} ${width - pad},${height - pad}`
      return h('div', { class: 'h-14 overflow-hidden rounded-md bg-gray-50/60 dark:bg-white/[0.03]' }, [
        h('svg', {
          class: 'h-full w-full',
          viewBox: `0 0 ${width} ${height}`,
          preserveAspectRatio: 'none',
          role: 'img',
          'aria-hidden': 'true'
        }, [
          h('polyline', {
            points: area,
            fill: props.color,
            opacity: '0.14',
            stroke: 'none'
          }),
          h('polyline', {
            points: points.join(' '),
            fill: 'none',
            stroke: props.color,
            'stroke-width': '2.5',
            'stroke-linecap': 'round',
            'stroke-linejoin': 'round',
            'vector-effect': 'non-scaling-stroke'
          })
        ])
      ])
    }
  }
})

const HorizontalBarChart = defineComponent({
  props: {
    items: {
      type: Array as PropType<Array<{ label: string; value: number; color: string }>>,
      required: true
    }
  },
  setup(props) {
    return () => {
      const max = Math.max(...props.items.map((item) => Number(item.value) || 0), 1)
      return h('div', { class: 'space-y-3 rounded-lg bg-gray-50/70 p-4 dark:bg-white/[0.03]' }, props.items.map((item) => {
        const value = Number(item.value) || 0
        const width = value > 0 ? Math.max(4, (value / max) * 100) : 0
        return h('div', { key: item.label, class: 'grid gap-2 md:grid-cols-[150px_minmax(0,1fr)_72px] md:items-center' }, [
          h('div', { class: 'truncate text-xs font-medium text-gray-600 dark:text-gray-300' }, item.label),
          h('div', { class: 'h-7 overflow-hidden rounded-md bg-white shadow-inner dark:bg-dark-800' }, [
            h('div', {
              class: 'h-full rounded-md transition-all',
              style: { width: `${width}%`, backgroundColor: item.color }
            })
          ]),
          h('div', { class: 'text-right text-sm font-semibold text-gray-900 dark:text-white' }, formatNumber(value))
        ])
      }))
    }
  }
})

const AreaLineChart = defineComponent({
  props: {
    labels: { type: Array as PropType<string[]>, required: true },
    values: { type: Array as PropType<number[]>, required: true },
    color: { type: String, default: '#0ea5e9' },
    suffix: { type: String, default: '' },
    height: { type: Number, default: 220 }
  },
  setup(props) {
    return () => {
      const data = props.values.length ? props.values.map((v) => Number(v) || 0) : [0]
      const width = 720
      const height = props.height
      const padX = 36
      const padY = 20
      const min = Math.min(0, ...data)
      const max = Math.max(...data, 1)
      const range = max - min || 1
      const step = data.length > 1 ? (width - padX * 2) / (data.length - 1) : 0
      const points = data.map((v, i) => {
        const x = padX + i * step
        const y = height - padY - ((v - min) / range) * (height - padY * 2)
        return { x, y, v }
      })
      const line = points.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
      const area = `${padX},${height - padY} ${line} ${width - padX},${height - padY}`
      const grid = [0, 0.5, 1].map((ratio) => {
        const y = padY + ratio * (height - padY * 2)
        return h('line', {
          key: ratio,
          x1: padX,
          x2: width - padX,
          y1: y,
          y2: y,
          stroke: 'currentColor',
          'stroke-width': '1',
          class: 'text-gray-200 dark:text-white/10'
        })
      })
      const last = points[points.length - 1]
      return h('div', { class: 'rounded-lg bg-gray-50/70 p-3 dark:bg-white/[0.03]' }, [
        h('svg', {
          class: 'h-full w-full overflow-visible',
          style: { height: `${height}px` },
          viewBox: `0 0 ${width} ${height}`,
          preserveAspectRatio: 'none',
          role: 'img',
          'aria-hidden': 'true'
        }, [
          ...grid,
          h('polyline', { points: area, fill: props.color, opacity: '0.14', stroke: 'none' }),
          h('polyline', {
            points: line,
            fill: 'none',
            stroke: props.color,
            'stroke-width': '2.5',
            'stroke-linecap': 'round',
            'stroke-linejoin': 'round',
            'vector-effect': 'non-scaling-stroke'
          }),
          last ? h('circle', { cx: last.x, cy: last.y, r: 4, fill: props.color }) : null
        ]),
        h('div', { class: 'mt-2 flex items-center justify-between text-[11px] text-gray-500 dark:text-gray-400' }, [
          h('span', props.labels[0] || ''),
          h('span', data.length ? `${data[data.length - 1].toFixed(1)}${props.suffix}` : `0${props.suffix}`),
          h('span', props.labels[props.labels.length - 1] || '')
        ])
      ])
    }
  }
})

const PulseCard = defineComponent({
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    wow: { type: Number, default: 0 },
    yoy: { type: Number as PropType<number | null | undefined>, default: null },
    history: { type: Number, default: 0 },
    spark: { type: Array as PropType<number[]>, required: true },
    hint: { type: String, required: true },
    tone: { type: String, default: 'emerald' }
  },
  setup(props) {
    const colorMap: Record<string, string> = {
      emerald: '#10b981',
      sky: '#0ea5e9',
      violet: '#8b5cf6',
      amber: '#f59e0b'
    }
    const color = colorMap[props.tone] || '#10b981'
    return () => h('div', { class: 'card p-5 space-y-3' }, [
      h('div', { class: 'flex items-baseline justify-between' }, [
        h('p', { class: 'text-xs font-medium text-gray-500 dark:text-gray-400' }, props.label),
        h('span', { class: 'text-[10px] text-gray-400 dark:text-gray-500' }, t('admin.operations.v2.pulse.spark30d'))
      ]),
      h('p', { class: 'text-3xl font-bold text-gray-900 dark:text-white' }, props.value),
      h(Sparkline, { values: props.spark, color }),
      h('div', { class: 'space-y-1.5 border-t border-gray-100 pt-3 dark:border-dark-700' }, [
        h(MetricChange, { label: t('admin.operations.v2.pulse.vsWow'), value: props.wow }),
        h(MetricChange, {
          label: t('admin.operations.v2.pulse.vsYoy'),
          value: props.yoy ?? 0,
          available: props.yoy !== null && props.yoy !== undefined
        }),
        h(MetricChange, { label: t('admin.operations.v2.pulse.vs90d'), value: props.history })
      ]),
      h('p', { class: 'text-xs italic text-gray-500 dark:text-gray-400' }, props.hint)
    ])
  }
})

const SectionHeader = defineComponent({
  props: {
    title: { type: String, required: true },
    sub: { type: String, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'flex items-end justify-between border-l-4 border-primary-500 pl-3' }, [
      h('div', null, [
        h('h2', { class: 'text-lg font-bold text-gray-900 dark:text-white' }, props.title),
        h('p', { class: 'mt-1 text-sm text-gray-500 dark:text-gray-400' }, props.sub)
      ])
    ])
  }
})

const ChartCaption = defineComponent({
  props: {
    title: { type: String, required: true },
    hint: { type: String, required: true },
    action: { type: String, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'mb-4 space-y-1' }, [
      h('h3', { class: 'text-base font-semibold text-gray-900 dark:text-white' }, props.title),
      h('p', { class: 'text-xs text-gray-600 dark:text-gray-300' }, [
        h('span', { class: 'font-medium text-gray-700 dark:text-gray-200' }, t('admin.operations.v2.captionPlain')),
        props.hint
      ]),
      h('p', { class: 'text-xs text-gray-600 dark:text-gray-300' }, [
        h('span', { class: 'font-medium text-emerald-600 dark:text-emerald-400' }, t('admin.operations.v2.captionAction')),
        props.action
      ])
    ])
  }
})

const ModuleLoadingNotice = defineComponent({
  setup() {
    return () => h('section', { class: 'card flex items-center justify-center gap-3 p-8 text-sm text-gray-500 dark:text-gray-400' }, [
      h(LoadingSpinner, { size: 'sm' }),
      h('span', t('common.loading'))
    ])
  }
})

// ─────────── 销售漏斗 ───────────
const FunnelSection = defineComponent({
  props: {
    current: { type: Array as PropType<OperationFunnelStep[]>, required: true },
    previous: { type: Array as PropType<OperationFunnelStep[]>, required: true }
  },
  setup(props) {
    return () => {
      const top = props.current[0]?.count || 1
      return h('section', { class: 'card p-5 space-y-4' }, [
        h(ChartCaption, {
          title: t('admin.operations.v2.funnel.chartTitle'),
          hint: t('admin.operations.v2.funnel.hint'),
          action: t('admin.operations.v2.funnel.action')
        }),
        h('div', { class: 'space-y-4' }, props.current.map((step, idx) => {
          const prev = props.previous[idx]
          const widthPct = Math.max(8, (step.count / top) * 100)
          const rateChange = prev ? (step.rate - prev.rate) * 100 : 0
          const rateCls = rateChange > 1 ? 'text-emerald-600 dark:text-emerald-400'
            : rateChange < -1 ? 'text-rose-600 dark:text-rose-400'
            : 'text-gray-500 dark:text-gray-400'
          return h('div', { key: step.key }, [
            h('div', { class: 'mb-1 flex items-center justify-between text-sm' }, [
              h('span', { class: 'font-medium text-gray-900 dark:text-white' }, [
                `${idx + 1}. ${step.label}`
              ]),
              h('span', { class: 'text-xs text-gray-500 dark:text-gray-400' }, [
                `${formatNumber(step.count)} 人 · ${formatPercent(step.rate)}`,
                prev
                  ? h('span', { class: `ml-2 font-semibold ${rateCls}` }, `(${rateChange >= 0 ? '+' : ''}${rateChange.toFixed(1)}pp vs 上周期)`)
                  : null
              ])
            ]),
            h('div', { class: 'relative h-7 w-full overflow-hidden rounded-md bg-gray-100 dark:bg-dark-700' }, [
              h('div', {
                class: 'h-full rounded-md bg-gradient-to-r from-primary-500 to-primary-400 transition-all',
                style: { width: `${widthPct}%` }
              })
            ]),
            h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-gray-400' }, step.description),
            idx < props.current.length - 1 && step.count > 0
              ? h('div', { class: 'mt-2 ml-3 flex items-center gap-2 text-xs' }, [
                  h('span', { class: 'text-gray-400' }, '↓'),
                  h('span', { class: 'text-gray-500 dark:text-gray-400' },
                    `${t('admin.operations.v2.funnel.dropTo')} ${props.current[idx + 1].label}：`),
                  h('span', { class: 'font-semibold text-gray-700 dark:text-gray-200' },
                    formatPercent(step.count > 0 ? props.current[idx + 1].count / step.count : 0))
                ])
              : null
          ])
        }))
      ])
    }
  }
})

// ─────────── 体验券子漏斗 ───────────
const TrialFunnelSection = defineComponent({
  props: { trial: { type: Object as PropType<OperationTrialFunnel>, required: true } },
  setup(props) {
    return () => {
      const tf = props.trial
      const cards: Array<{
        key: string
        tone: 'sky' | 'emerald' | 'amber' | 'rose' | 'violet'
        icon: string
        title: string
        value: string
        sub: string
      }> = [
        {
          key: 'issued',
          tone: 'sky',
          icon: '🎟️',
          title: t('admin.operations.v2.trial.cards.issued'),
          value: formatNumber(tf.trial_users_issued),
          sub: t('admin.operations.v2.trial.cards.issuedSub', { value: `$${tf.trial_balance_value.toFixed(0)}` })
        },
        {
          key: 'used',
          tone: 'emerald',
          icon: '✅',
          title: t('admin.operations.v2.trial.cards.used'),
          value: formatNumber(tf.trial_users_used),
          sub: t('admin.operations.v2.trial.cards.usedSub', { rate: formatPercent(tf.use_rate) })
        },
        {
          key: 'idle',
          tone: 'rose',
          icon: '🪦',
          title: t('admin.operations.v2.trial.cards.idle'),
          value: formatNumber(tf.trial_users_idle),
          sub: t('admin.operations.v2.trial.cards.idleSub', { rate: formatPercent(tf.idle_rate) })
        },
        {
          key: 'exhausted',
          tone: 'amber',
          icon: '🔥',
          title: t('admin.operations.v2.trial.cards.exhausted'),
          value: formatNumber(tf.trial_users_exhausted),
          sub: t('admin.operations.v2.trial.cards.exhaustedSub', {
            rate: formatPercent(tf.exhaustion_rate),
            threshold: `$${tf.exhaustion_threshold.toFixed(0)}`
          })
        },
        {
          key: 'converted',
          tone: 'violet',
          icon: '💰',
          title: t('admin.operations.v2.trial.cards.converted'),
          value: formatNumber(tf.trial_users_converted),
          sub: t('admin.operations.v2.trial.cards.convertedSub', { rate: formatPercent(tf.conversion_rate) })
        }
      ]
      const ringMap: Record<string, string> = {
        sky: 'ring-sky-200 dark:ring-sky-700',
        emerald: 'ring-emerald-200 dark:ring-emerald-700',
        amber: 'ring-amber-200 dark:ring-amber-700',
        rose: 'ring-rose-200 dark:ring-rose-700',
        violet: 'ring-violet-200 dark:ring-violet-700'
      }
      const textMap: Record<string, string> = {
        sky: 'text-sky-600 dark:text-sky-400',
        emerald: 'text-emerald-600 dark:text-emerald-400',
        amber: 'text-amber-600 dark:text-amber-400',
        rose: 'text-rose-600 dark:text-rose-400',
        violet: 'text-violet-600 dark:text-violet-400'
      }
      return h('section', { class: 'card p-5 space-y-5' }, [
        h(ChartCaption, {
          title: t('admin.operations.v2.trial.chartTitle'),
          hint: t('admin.operations.v2.trial.hint', {
            issued: tf.trial_users_issued,
            used: tf.trial_users_used,
            converted: tf.trial_users_converted
          }),
          action: t('admin.operations.v2.trial.action')
        }),
        h('div', { class: 'grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5' }, cards.map((c) =>
          h('div', { key: c.key, class: `card p-4 ring-1 ${ringMap[c.tone]} flex flex-col gap-2` }, [
            h('div', { class: 'flex items-center gap-1.5 text-xs font-medium text-gray-700 dark:text-gray-200' }, [
              h('span', c.icon),
              h('span', c.title)
            ]),
            h('p', { class: `text-3xl font-bold ${textMap[c.tone]}` }, c.value),
            h('p', { class: 'text-[11px] leading-snug text-gray-500 dark:text-gray-400' }, c.sub)
          ])
        )),
        h('div', { class: 'rounded-md bg-gray-50 p-3 text-xs leading-relaxed text-gray-600 dark:bg-dark-800 dark:text-gray-300' }, [
          h('p', null, [
            h('span', { class: 'font-semibold' }, t('admin.operations.v2.trial.summaryLabel')),
            t('admin.operations.v2.trial.summary', {
              avg: `$${tf.avg_consumed.toFixed(2)}`,
              nonTrial: tf.non_trial_paid
            })
          ]),
          h('p', { class: 'mt-1 text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.trial.caveat'))
        ])
      ])
    }
  }
})

// ─────────── 留存与流失 ───────────
const ChurnSection = defineComponent({
  props: {
    churn: { type: Object as PropType<OperationChurnSnapshot>, required: true },
    retentionD1: { type: Number, default: 0 },
    retentionD7: { type: Number, default: 0 },
    retentionD30: { type: Number, default: 0 }
  },
  setup(props) {
    return () => {
      const c = props.churn
      const totalLost = c.waterfall.completely_gone + c.waterfall.half_activity + c.waterfall.balance_exhausted + c.waterfall.subscription_ended

      // 三联流失率
      const churnTriple = h('div', { class: 'grid grid-cols-1 gap-4 md:grid-cols-3' }, [
        churnRateCard({
          title: t('admin.operations.v2.churn.totalRate'),
          rate: c.churn_rate,
          delta: c.churn_rate_change_pct,
          subtitle: t('admin.operations.v2.churn.totalRateSub', { p50: c.global_p50_days.toFixed(1) }),
          tone: c.churn_rate >= 0.3 ? 'rose' : c.churn_rate >= 0.15 ? 'amber' : 'emerald'
        }),
        churnRateCard({
          title: t('admin.operations.v2.churn.atRiskRate'),
          rate: c.at_risk_rate,
          subtitle: t('admin.operations.v2.churn.atRiskRateSub', { count: c.at_risk_users + c.high_risk_users }),
          tone: c.at_risk_rate >= 0.3 ? 'rose' : c.at_risk_rate >= 0.15 ? 'amber' : 'emerald'
        }),
        churnRateCard({
          title: t('admin.operations.v2.churn.highValueRisk'),
          rate: null,
          bigText: formatNumber(c.high_value_at_risk),
          subtitle: t('admin.operations.v2.churn.highValueRiskSub', { revenue: formatMoney(c.high_value_revenue) }),
          tone: c.high_value_at_risk >= 10 ? 'rose' : c.high_value_at_risk >= 3 ? 'amber' : 'emerald'
        })
      ])

      // 流失瀑布图
      const waterfall = h('section', { class: 'card p-5' }, [
        h(ChartCaption, {
          title: t('admin.operations.v2.churn.waterfallTitle'),
          hint: t('admin.operations.v2.churn.waterfallHint', {
            base: c.waterfall.last_period_active,
            still: c.waterfall.still_active,
            lost: totalLost
          }),
          action: t('admin.operations.v2.churn.waterfallAction')
        }),
        h(HorizontalBarChart, {
          items: [
            { label: t('admin.operations.v2.churn.lastWeekActive'), value: c.waterfall.last_period_active, color: '#0ea5e9' },
            { label: t('admin.operations.v2.churn.stillActive'), value: c.waterfall.still_active, color: '#10b981' },
            { label: t('admin.operations.v2.churn.completelyGone'), value: c.waterfall.completely_gone, color: '#ef4444' },
            { label: t('admin.operations.v2.churn.halfActivity'), value: c.waterfall.half_activity, color: '#f59e0b' },
            { label: t('admin.operations.v2.churn.balanceExhausted'), value: c.waterfall.balance_exhausted, color: '#a855f7' },
            { label: t('admin.operations.v2.churn.subscriptionEnded'), value: c.waterfall.subscription_ended, color: '#ec4899' }
          ]
        })
      ])

      // 流失率历史曲线
      const history = c.history && c.history.length
        ? h('section', { class: 'card p-5' }, [
            h(ChartCaption, {
              title: t('admin.operations.v2.churn.historyTitle'),
              hint: t('admin.operations.v2.churn.historyHint'),
              action: t('admin.operations.v2.churn.historyAction')
            }),
            h(AreaLineChart, {
              labels: [...c.history].reverse().map((p) => p.bucket),
              values: [...c.history].reverse().map((p) => +(p.churn_rate * 100).toFixed(2)),
              color: '#ef4444',
              suffix: '%',
              height: 220
            })
          ])
        : null

      // 留存层级
      const retention = h('section', { class: 'card p-5' }, [
        h(ChartCaption, {
          title: t('admin.operations.v2.churn.retentionTitle'),
          hint: t('admin.operations.v2.churn.retentionHint'),
          action: t('admin.operations.v2.churn.retentionAction')
        }),
        h('div', { class: 'grid grid-cols-3 gap-4' }, [
          retentionTile('D1', props.retentionD1),
          retentionTile('D7', props.retentionD7),
          retentionTile('D30', props.retentionD30)
        ])
      ])

      return h('div', { class: 'space-y-6' }, [
        h('p', { class: 'rounded-md bg-gray-50 px-3 py-2 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300' },
          [h('span', { class: 'font-semibold' }, t('admin.operations.v2.churn.definitionLabel')), c.definition]),
        churnTriple,
        waterfall,
        history,
        retention
      ])
    }
  }
})

interface ChurnRateCardOpts {
  title: string
  rate?: number | null
  delta?: number
  bigText?: string
  subtitle?: string
  tone?: 'emerald' | 'amber' | 'rose'
}
function churnRateCard(opts: ChurnRateCardOpts) {
  const toneMap = {
    emerald: { ring: 'ring-emerald-200 dark:ring-emerald-700', text: 'text-emerald-600 dark:text-emerald-400', icon: '🟢' },
    amber: { ring: 'ring-amber-200 dark:ring-amber-700', text: 'text-amber-600 dark:text-amber-400', icon: '🟡' },
    rose: { ring: 'ring-rose-200 dark:ring-rose-700', text: 'text-rose-600 dark:text-rose-400', icon: '🔴' }
  } as const
  const tone = toneMap[opts.tone || 'emerald']
  const valueText = opts.bigText ?? (opts.rate !== null && opts.rate !== undefined ? formatPercent(opts.rate) : '—')
  const children = [
    h('p', { class: 'text-xs font-medium text-gray-500 dark:text-gray-400' }, opts.title),
    h('p', { class: `mt-2 text-4xl font-bold ${tone.text}` }, [valueText, h('span', { class: 'ml-2 text-xl' }, tone.icon)]),
    opts.subtitle ? h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-gray-400' }, opts.subtitle) : null
  ]
  if (opts.delta !== undefined && Math.abs(opts.delta) > 0.5) {
    const dir = opts.delta > 0 ? 'text-rose-600 dark:text-rose-400' : 'text-emerald-600 dark:text-emerald-400'
    const sign = opts.delta > 0 ? '+' : ''
    children.push(
      h('p', { class: `mt-2 text-xs font-semibold ${dir}` },
        t('admin.operations.v2.churn.weeklyDelta', { value: `${sign}${opts.delta.toFixed(1)}%` }))
    )
  }
  return h('div', { class: `card p-5 ring-1 ${tone.ring}` }, children)
}

function retentionTile(label: string, rate: number) {
  const tone = rate >= 0.5 ? 'text-emerald-600 dark:text-emerald-400'
    : rate >= 0.2 ? 'text-amber-600 dark:text-amber-400'
    : 'text-rose-600 dark:text-rose-400'
  return h('div', { class: 'rounded-lg border border-gray-100 p-4 text-center dark:border-dark-700' }, [
    h('p', { class: 'text-xs text-gray-500 dark:text-gray-400' }, label),
    h('p', { class: `mt-2 text-2xl font-bold ${tone}` }, formatPercent(rate))
  ])
}

// ─────────── 用户金字塔 ───────────
const PyramidSection = defineComponent({
  props: { pyramid: { type: Object as PropType<OperationUserPyramid>, required: true } },
  setup(props) {
    const widthFor = (lvl: OperationPyramidLevel, max: number) => {
      const v = lvl.users === 0 ? 0 : Math.max(15, (lvl.users / max) * 100)
      return v
    }
    const colorFor = (key: string) => ({
      super: 'bg-gradient-to-r from-fuchsia-400 to-pink-500',
      gold: 'bg-gradient-to-r from-amber-300 to-amber-500',
      silver: 'bg-gradient-to-r from-slate-300 to-slate-500',
      bronze: 'bg-gradient-to-r from-orange-300 to-orange-500',
      free: 'bg-gradient-to-r from-gray-200 to-gray-400 dark:from-dark-700 dark:to-dark-600'
    } as Record<string, string>)[key] || 'bg-gray-300'

    return () => {
      const p = props.pyramid
      const max = Math.max(...p.levels.map((l) => l.users), 1)
      return h('section', { class: 'card p-5 space-y-5' }, [
        h(ChartCaption, {
          title: t('admin.operations.v2.pyramid.chartTitle'),
          hint: t('admin.operations.v2.pyramid.hint', {
            total: formatNumber(p.total_users),
            paid: formatPercent(p.paid_percent)
          }),
          action: t('admin.operations.v2.pyramid.action')
        }),
        h('div', { class: 'space-y-2' }, p.levels.map((lvl) =>
          h('div', { key: lvl.key, class: 'group' }, [
            h('div', { class: 'mb-1 flex items-center justify-between text-xs' }, [
              h('span', { class: 'font-medium text-gray-800 dark:text-gray-100' },
                `${lvl.label} · ${formatNumber(lvl.users)} 人 (${formatPercent(lvl.user_percent)})`),
              h('span', { class: 'text-gray-500 dark:text-gray-400' },
                lvl.key === 'free'
                  ? '不贡献收入'
                  : `贡献收入 ${formatPercent(lvl.revenue_percent)} · ARPU $${formatMoney(lvl.avg_revenue_per_user)}`)
            ]),
            h('div', { class: 'relative mx-auto flex h-8 items-center justify-center' }, [
              h('div', {
                class: `h-full rounded-md ${colorFor(lvl.key)} shadow-sm transition-all`,
                style: { width: `${widthFor(lvl, max)}%` }
              })
            ])
          ])
        )),
        h('div', { class: 'rounded-md bg-gray-50 p-3 text-xs text-gray-600 dark:bg-dark-800 dark:text-gray-300' }, [
          h('p', null, [
            h('span', { class: 'font-semibold' }, t('admin.operations.v2.pyramid.insightLabel')),
            t('admin.operations.v2.pyramid.insight', {
              top: formatPercent(topRevenueShare(p.levels)),
              users: formatPercent(topUserShare(p.levels))
            })
          ])
        ])
      ])
    }
  }
})

function topRevenueShare(levels: OperationPyramidLevel[]) {
  const top = levels.find((l) => l.key === 'super')
  const gold = levels.find((l) => l.key === 'gold')
  return (top?.revenue_percent || 0) + (gold?.revenue_percent || 0)
}
function topUserShare(levels: OperationPyramidLevel[]) {
  const top = levels.find((l) => l.key === 'super')
  const gold = levels.find((l) => l.key === 'gold')
  return (top?.user_percent || 0) + (gold?.user_percent || 0)
}

// ─────────── 产品矩阵（BCG + 模型健康度）───────────
const ProductMatrixSection = defineComponent({
  props: { matrix: { type: Object as PropType<OperationProductMatrix>, required: true } },
  setup(props) {
    return () => {
      const m = props.matrix
      return h('section', { class: 'card p-5 space-y-6' }, [
        h(ChartCaption, {
          title: t('admin.operations.v2.product.bcgTitle'),
          hint: t('admin.operations.v2.product.bcgHint'),
          action: t('admin.operations.v2.product.bcgAction')
        }),
        BcgMatrix({ plans: m.plans }),
        h('div', { class: 'border-t border-gray-100 pt-5 dark:border-dark-700' }, [
          h(ChartCaption, {
            title: t('admin.operations.v2.product.modelTitle'),
            hint: t('admin.operations.v2.product.modelHint'),
            action: t('admin.operations.v2.product.modelAction')
          }),
          m.models.length === 0
            ? h('p', { class: 'py-6 text-center text-sm text-gray-500 dark:text-gray-400' }, t('admin.operations.empty'))
            : h('div', { class: 'grid gap-3 md:grid-cols-2 xl:grid-cols-3' }, m.models.map((mh) =>
                h('div', { key: mh.model, class: 'rounded-lg border border-gray-100 p-4 dark:border-dark-700' }, [
                  h('p', { class: 'truncate text-sm font-semibold text-gray-900 dark:text-white' }, mh.model),
                  h('div', { class: 'mt-2 grid grid-cols-2 gap-2 text-xs' }, [
                    h('div', null, [
                      h('p', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.product.modelTrafficShare')),
                      h('p', { class: 'font-semibold text-gray-900 dark:text-white' }, formatPercent(mh.traffic_share))
                    ]),
                    h('div', null, [
                      h('p', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.product.modelRevenue')),
                      h('p', { class: 'font-semibold text-gray-900 dark:text-white' }, `$${formatMoney(mh.revenue)}`)
                    ]),
                    h('div', null, [
                      h('p', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.product.modelUsers')),
                      h('p', { class: 'font-semibold text-gray-900 dark:text-white' }, formatNumber(mh.users))
                    ]),
                    h('div', null, [
                      h('p', { class: 'text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.product.modelUsersChange')),
                      h('p', {
                        class: `font-semibold ${mh.users_change_percent > 1 ? 'text-emerald-600 dark:text-emerald-400'
                          : mh.users_change_percent < -1 ? 'text-rose-600 dark:text-rose-400'
                          : 'text-gray-700 dark:text-gray-200'}`
                      }, `${mh.users_change_percent > 0 ? '+' : ''}${mh.users_change_percent.toFixed(1)}%`)
                    ])
                  ])
                ])
              ))
        ])
      ])
    }
  }
})

function BcgMatrix(opts: { plans: OperationPlanMatrix[] }) {
  if (opts.plans.length === 0) {
    return h('p', { class: 'py-6 text-center text-sm text-gray-500 dark:text-gray-400' }, t('admin.operations.empty'))
  }
  const w = 720, h2 = 360, padX = 60, padY = 30
  const maxUsers = Math.max(...opts.plans.map((p) => p.active_users), 1)
  const maxArpu = Math.max(...opts.plans.map((p) => p.arpu), 0.01)
  const usersSorted = [...opts.plans].map((p) => p.active_users).sort((a, b) => a - b)
  const arpuSorted = [...opts.plans].map((p) => p.arpu).sort((a, b) => a - b)
  const userMid = usersSorted[Math.floor(usersSorted.length / 2)]
  const arpuMid = arpuSorted[Math.floor(arpuSorted.length / 2)]
  const xMid = padX + (userMid / maxUsers) * (w - padX * 2)
  const yMid = h2 - padY - (arpuMid / maxArpu) * (h2 - padY * 2)
  const colorMap: Record<string, string> = {
    star: '#a855f7', cash_cow: '#0ea5e9', question: '#f59e0b', dog: '#94a3b8'
  }
  return h('div', { class: 'rounded-lg bg-gray-50/70 p-3 dark:bg-white/[0.03]' }, [
    h('svg', {
      class: 'h-full w-full',
      viewBox: `0 0 ${w} ${h2}`,
      preserveAspectRatio: 'xMidYMid meet',
      role: 'img',
      'aria-hidden': 'true'
    }, [
      // 中位线
      h('line', { x1: xMid, x2: xMid, y1: padY, y2: h2 - padY, stroke: 'currentColor', 'stroke-width': '1', 'stroke-dasharray': '4 4', class: 'text-gray-300 dark:text-white/20' }),
      h('line', { x1: padX, x2: w - padX, y1: yMid, y2: yMid, stroke: 'currentColor', 'stroke-width': '1', 'stroke-dasharray': '4 4', class: 'text-gray-300 dark:text-white/20' }),
      // 象限标签
      h('text', { x: w - padX - 8, y: padY + 12, 'text-anchor': 'end', class: 'fill-fuchsia-500 text-[11px] font-semibold' }, '⭐ 明星'),
      h('text', { x: w - padX - 8, y: h2 - padY - 6, 'text-anchor': 'end', class: 'fill-sky-500 text-[11px] font-semibold' }, '🐮 现金牛'),
      h('text', { x: padX + 8, y: padY + 12, class: 'fill-amber-500 text-[11px] font-semibold' }, '❓ 问题'),
      h('text', { x: padX + 8, y: h2 - padY - 6, class: 'fill-slate-400 text-[11px] font-semibold' }, '☠️ 待砍'),
      // 散点
      ...opts.plans.map((p) => {
        const cx = padX + (p.active_users / maxUsers) * (w - padX * 2)
        const cy = h2 - padY - (p.arpu / maxArpu) * (h2 - padY * 2)
        const r = Math.max(6, Math.min(20, Math.sqrt(p.revenue) * 2))
        return h('g', { key: `${p.group_id}-${p.name}` }, [
          h('circle', { cx, cy, r, fill: colorMap[p.quadrant] || '#94a3b8', opacity: '0.65', stroke: 'white', 'stroke-width': '1.5' }),
          h('title', null, `${p.name}\n用户 ${p.active_users}\nARPU $${p.arpu.toFixed(4)}\n收入 $${formatMoney(p.revenue)}\n象限：${p.quadrant}`)
        ])
      }),
      // 坐标轴标签
      h('text', { x: w / 2, y: h2 - 4, 'text-anchor': 'middle', class: 'fill-gray-500 dark:fill-gray-400 text-[11px]' }, '→ 用户数'),
      h('text', { x: 8, y: h2 / 2, 'text-anchor': 'start', class: 'fill-gray-500 dark:fill-gray-400 text-[11px]' }, '↑ ARPU')
    ]),
    // 图例
    h('div', { class: 'mt-2 flex flex-wrap gap-3 text-[11px] text-gray-500 dark:text-gray-400' }, [
      h('span', null, [h('span', { class: 'mr-1', style: 'color:#a855f7' }, '●'), '明星：用户多 + ARPU 高，重点维护']),
      h('span', null, [h('span', { class: 'mr-1', style: 'color:#0ea5e9' }, '●'), '现金牛：用户多但 ARPU 低，薄利多销']),
      h('span', null, [h('span', { class: 'mr-1', style: 'color:#f59e0b' }, '●'), '问题：少人但客单高，需要规模']),
      h('span', null, [h('span', { class: 'mr-1', style: 'color:#94a3b8' }, '●'), '待砍：左下角，可考虑下线'])
    ])
  ])
}

// ─────────── 财务驾驶舱（余额沉淀 + 收入构成 + 现金流 + ARPU 曲线）───────────
const FinancialCockpitSection = defineComponent({
  props: { financial: { type: Object as PropType<OperationFinancialCockpit>, required: true } },
  setup(props) {
    return () => {
      const fc = props.financial
      const healthMap: Record<string, { tone: string; ring: string; text: string; label: string }> = {
        danger: { tone: 'rose', ring: 'ring-rose-200 dark:ring-rose-700', text: 'text-rose-600 dark:text-rose-400', label: t('admin.operations.v2.financial.healthDanger') },
        warning: { tone: 'amber', ring: 'ring-amber-200 dark:ring-amber-700', text: 'text-amber-600 dark:text-amber-400', label: t('admin.operations.v2.financial.healthWarning') },
        healthy: { tone: 'emerald', ring: 'ring-emerald-200 dark:ring-emerald-700', text: 'text-emerald-600 dark:text-emerald-400', label: t('admin.operations.v2.financial.healthHealthy') },
        overloaded: { tone: 'sky', ring: 'ring-sky-200 dark:ring-sky-700', text: 'text-sky-600 dark:text-sky-400', label: t('admin.operations.v2.financial.healthOverloaded') }
      }
      const health = healthMap[fc.balance_health] || healthMap.healthy

      const arpuLabels = fc.arpu_history.map((p) => p.bucket)
      const arpuValues = fc.arpu_history.map((p) => Number(p.paying_arpu) || 0)

      return h('section', { class: 'card p-5 space-y-6' }, [
        // ─── 余额沉淀仪表盘 ───
        h(ChartCaption, {
          title: t('admin.operations.v2.financial.cushionTitle'),
          hint: t('admin.operations.v2.financial.cushionHint', {
            balance: `$${formatMoney(fc.total_balance)}`,
            cost: `$${formatMoney(fc.daily_avg_cost)}`,
            months: fc.balance_months_cushion.toFixed(1)
          }),
          action: t(`admin.operations.v2.financial.cushionAction.${fc.balance_health}`) || t('admin.operations.v2.financial.cushionAction.healthy')
        }),
        h('div', { class: `grid gap-4 md:grid-cols-3` }, [
          h('div', { class: `card p-5 ring-1 ${health.ring} text-center md:col-span-1` }, [
            BalanceGauge({ months: fc.balance_months_cushion, color: health.text }),
            h('p', { class: `mt-2 text-2xl font-bold ${health.text}` },
              fc.balance_months_cushion >= 99 ? '∞' : `${fc.balance_months_cushion.toFixed(1)}${t('admin.operations.v2.financial.months')}`),
            h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-gray-400' }, health.label)
          ]),
          h('div', { class: 'card p-5 md:col-span-2' }, [
            h('p', { class: 'mb-3 text-xs font-medium text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.financial.cashflowTitle')),
            h('div', { class: 'grid grid-cols-3 gap-2 text-center' }, [
              h('div', null, [
                h('p', { class: 'text-[11px] text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.financial.inflow')),
                h('p', { class: 'mt-1 text-xl font-bold text-emerald-600 dark:text-emerald-400' }, `+$${formatMoney(fc.inflow_total)}`)
              ]),
              h('div', null, [
                h('p', { class: 'text-[11px] text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.financial.outflow')),
                h('p', { class: 'mt-1 text-xl font-bold text-rose-600 dark:text-rose-400' }, `-$${formatMoney(fc.outflow_total)}`)
              ]),
              h('div', null, [
                h('p', { class: 'text-[11px] text-gray-500 dark:text-gray-400' }, t('admin.operations.v2.financial.netFlow')),
                h('p', {
                  class: `mt-1 text-xl font-bold ${fc.net_flow >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'}`
                }, `${fc.net_flow >= 0 ? '+' : '-'}$${formatMoney(Math.abs(fc.net_flow))}`)
              ])
            ]),
            h('p', { class: 'mt-3 rounded bg-gray-50 px-3 py-2 text-[11px] leading-relaxed text-gray-600 dark:bg-dark-800 dark:text-gray-300' },
              t('admin.operations.v2.financial.cashflowNote'))
          ])
        ]),

        // ─── 收入构成（2 大类：管理员充值 + 兑换码兑换）───
        h('div', { class: 'border-t border-gray-100 pt-5 dark:border-dark-700' }, [
          h(ChartCaption, {
            title: t('admin.operations.v2.financial.revenueTitle'),
            hint: t('admin.operations.v2.financial.revenueHint'),
            action: t('admin.operations.v2.financial.revenueAction')
          }),
          h('div', { class: 'grid gap-4 md:grid-cols-2' }, [
            RevenueDonut({
              adminNet: fc.admin_topup_net,
              redeemReal: fc.redeem_balance_real,
              trial: fc.redeem_trial,
              refund: fc.admin_refund_amount
            }),
            h('div', { class: 'space-y-3' }, [
              // 第 1 类：管理员充值
              h('div', { class: 'rounded-lg border border-sky-100 bg-sky-50/40 p-3 dark:border-sky-900/40 dark:bg-sky-900/10' }, [
                h('p', { class: 'mb-2 text-xs font-semibold text-sky-700 dark:text-sky-300' },
                  `🛡️ ${t('admin.operations.v2.financial.adminGroup')}`),
                h('div', { class: 'space-y-1.5' }, [
                  miniRow(t('admin.operations.v2.financial.adminGross'), `+$${formatMoney(fc.admin_topup_gross)}`, '#10b981'),
                  miniRow(t('admin.operations.v2.financial.adminRefund', { count: fc.admin_refund_count }),
                    `-$${formatMoney(fc.admin_refund_amount)}`, '#ef4444'),
                  h('div', { class: 'mt-1 flex items-center justify-between border-t border-sky-200 pt-1.5 dark:border-sky-800' }, [
                    h('span', { class: 'text-xs font-bold text-sky-700 dark:text-sky-300' }, t('admin.operations.v2.financial.adminNet')),
                    h('span', { class: 'text-sm font-bold text-sky-700 dark:text-sky-300 tabular-nums' }, `$${formatMoney(fc.admin_topup_net)}`)
                  ])
                ])
              ]),
              // 第 2 类：兑换码兑换
              h('div', { class: 'rounded-lg border border-emerald-100 bg-emerald-50/40 p-3 dark:border-emerald-900/40 dark:bg-emerald-900/10' }, [
                h('p', { class: 'mb-2 text-xs font-semibold text-emerald-700 dark:text-emerald-300' },
                  `🎟️ ${t('admin.operations.v2.financial.redeemGroup')}`),
                h('div', { class: 'space-y-1.5' }, [
                  miniRow(t('admin.operations.v2.financial.redeemReal'), `$${formatMoney(fc.redeem_balance_real)}`, '#10b981'),
                  miniRow(`${t('admin.operations.v2.financial.redeemTrial')} · ${t('admin.operations.v2.financial.revTrialNote')}`,
                    `$${formatMoney(fc.redeem_trial)}`, '#94a3b8'),
                  miniRow(t('admin.operations.v2.financial.redeemSub'),
                    `${formatNumber(fc.redeem_subscription_count)} ${t('admin.operations.v2.financial.subUnit')}`, '#a855f7'),
                  miniRow(t('admin.operations.v2.financial.newSubs'),
                    `${formatNumber(fc.new_subscriptions_count)} ${t('admin.operations.v2.financial.subUnit')}`, '#8b5cf6')
                ])
              ]),
              fc.refund_rate > 0
                ? h('p', {
                  class: `rounded px-3 py-2 text-[11px] font-medium ${
                    fc.refund_rate >= 0.2
                      ? 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-300'
                      : 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
                  }`
                }, t('admin.operations.v2.financial.refundRateNote', {
                  rate: formatPercent(fc.refund_rate),
                  amount: `$${formatMoney(fc.admin_refund_amount)}`
                }))
                : null,
              h('p', { class: 'rounded bg-gray-50 px-3 py-2 text-[11px] leading-relaxed text-gray-600 dark:bg-dark-800 dark:text-gray-300' },
                t('admin.operations.v2.financial.revenueCaveat'))
            ])
          ])
        ]),

        // ─── ARPU 30 天曲线 ───
        h('div', { class: 'border-t border-gray-100 pt-5 dark:border-dark-700' }, [
          h(ChartCaption, {
            title: t('admin.operations.v2.financial.arpuTitle'),
            hint: t('admin.operations.v2.financial.arpuHint'),
            action: t('admin.operations.v2.financial.arpuAction')
          }),
          arpuLabels.length === 0
            ? h('p', { class: 'py-6 text-center text-sm text-gray-500 dark:text-gray-400' }, t('admin.operations.empty'))
            : h(AreaLineChart, { labels: arpuLabels, values: arpuValues, color: '#8b5cf6', suffix: '$', height: 200 })
        ])
      ])
    }
  }
})

function miniRow(label: string, valueText: string, color: string) {
  return h('div', { class: 'flex items-center justify-between' }, [
    h('span', { class: 'text-xs text-gray-700 dark:text-gray-200' }, label),
    h('span', { class: 'text-sm font-semibold tabular-nums', style: { color } }, valueText)
  ])
}

function BalanceGauge(opts: { months: number; color: string }) {
  // 半圆仪表盘：0-1月红 / 1-3月黄 / 3-6月绿 / 6+月蓝
  const w = 200, h2 = 110
  const cx = w / 2, cy = 100, r = 80
  const cap = Math.min(opts.months, 12)
  const angle = Math.PI - (cap / 12) * Math.PI
  const x = cx + r * Math.cos(angle)
  const y = cy - r * Math.sin(angle)
  // 4 个色段（红、黄、绿、蓝）
  const arc = (from: number, to: number, color: string) => {
    const a1 = Math.PI - (from / 12) * Math.PI
    const a2 = Math.PI - (to / 12) * Math.PI
    const x1 = cx + r * Math.cos(a1)
    const y1 = cy - r * Math.sin(a1)
    const x2 = cx + r * Math.cos(a2)
    const y2 = cy - r * Math.sin(a2)
    return h('path', {
      d: `M ${x1} ${y1} A ${r} ${r} 0 0 1 ${x2} ${y2}`,
      fill: 'none',
      stroke: color,
      'stroke-width': '14',
      'stroke-linecap': 'butt',
      opacity: '0.85'
    })
  }
  return h('svg', {
    class: 'mx-auto block',
    width: w,
    height: h2,
    viewBox: `0 0 ${w} ${h2}`
  }, [
    arc(0, 1, '#ef4444'),
    arc(1, 3, '#f59e0b'),
    arc(3, 6, '#10b981'),
    arc(6, 12, '#0ea5e9'),
    h('circle', { cx: x, cy: y, r: 7, fill: 'white', stroke: opts.color.includes('rose') ? '#ef4444' : opts.color.includes('amber') ? '#f59e0b' : opts.color.includes('emerald') ? '#10b981' : '#0ea5e9', 'stroke-width': '3' }),
    // 刻度
    h('text', { x: cx - r - 4, y: cy + 18, class: 'fill-gray-400 text-[10px]', 'text-anchor': 'middle' }, '0'),
    h('text', { x: cx, y: 18, class: 'fill-gray-400 text-[10px]', 'text-anchor': 'middle' }, '6'),
    h('text', { x: cx + r + 4, y: cy + 18, class: 'fill-gray-400 text-[10px]', 'text-anchor': 'middle' }, '12+')
  ])
}

function RevenueDonut(opts: { adminNet: number; redeemReal: number; trial: number; refund: number }) {
  const positive = Math.max(opts.adminNet, 0) + opts.redeemReal + opts.trial
  if (positive === 0) {
    return h('div', { class: 'flex items-center justify-center rounded-lg bg-gray-50/70 p-6 dark:bg-white/[0.03]' }, [
      h('p', { class: 'text-sm text-gray-500 dark:text-gray-400' }, t('admin.operations.empty'))
    ])
  }
  // 3 块正向 + 退款条
  const segments = [
    { v: Math.max(opts.adminNet, 0), color: '#0ea5e9', label: t('admin.operations.v2.financial.adminNet') },
    { v: opts.redeemReal, color: '#10b981', label: t('admin.operations.v2.financial.redeemReal') },
    { v: opts.trial, color: '#94a3b8', label: t('admin.operations.v2.financial.redeemTrial') }
  ]
  const w = 200, cx = w / 2, cy = w / 2, r = 78, rIn = 50
  let acc = -Math.PI / 2
  const paths = segments.filter((s) => s.v > 0).map((s, i) => {
    const fr = (s.v / positive) * Math.PI * 2
    const a1 = acc, a2 = acc + fr
    acc = a2
    const x1o = cx + r * Math.cos(a1), y1o = cy + r * Math.sin(a1)
    const x2o = cx + r * Math.cos(a2), y2o = cy + r * Math.sin(a2)
    const x1i = cx + rIn * Math.cos(a2), y1i = cy + rIn * Math.sin(a2)
    const x2i = cx + rIn * Math.cos(a1), y2i = cy + rIn * Math.sin(a1)
    const large = fr > Math.PI ? 1 : 0
    return h('path', {
      key: i,
      d: `M ${x1o} ${y1o} A ${r} ${r} 0 ${large} 1 ${x2o} ${y2o} L ${x1i} ${y1i} A ${rIn} ${rIn} 0 ${large} 0 ${x2i} ${y2i} Z`,
      fill: s.color
    })
  })
  return h('div', { class: 'flex flex-col items-center gap-2 rounded-lg bg-gray-50/70 p-3 dark:bg-white/[0.03]' }, [
    h('svg', { width: w, height: w, viewBox: `0 0 ${w} ${w}` }, [
      ...paths,
      h('text', { x: cx, y: cy - 6, 'text-anchor': 'middle', class: 'fill-gray-500 dark:fill-gray-400 text-[10px]' }, t('admin.operations.v2.financial.netInjected')),
      h('text', { x: cx, y: cy + 12, 'text-anchor': 'middle', class: 'fill-gray-900 dark:fill-white text-base font-bold' }, `$${formatMoney(positive)}`)
    ]),
    // 退款条
    opts.refund > 0
      ? h('div', { class: 'w-full rounded-md bg-rose-50 px-3 py-2 text-center dark:bg-rose-900/20' }, [
        h('p', { class: 'text-[11px] text-rose-700 dark:text-rose-300' }, t('admin.operations.v2.financial.refundBar')),
        h('p', { class: 'text-sm font-bold text-rose-700 dark:text-rose-300' }, `-$${formatMoney(opts.refund)}`)
      ])
      : null,
    // 图例
    h('div', { class: 'flex flex-wrap justify-center gap-2 text-[10px] text-gray-600 dark:text-gray-400' },
      segments.filter((s) => s.v > 0).map((s) =>
        h('span', { key: s.label, class: 'flex items-center gap-1' }, [
          h('span', { class: 'inline-block h-2 w-2 rounded-full', style: { backgroundColor: s.color } }),
          s.label
        ])
      )
    )
  ])
}

// ─────────── 行动汇总 ───────────
const ActionSummary = defineComponent({
  emits: ['openList'],
  props: {
    churn: { type: Object as PropType<OperationChurnSnapshot>, required: true },
    core: { type: Object as PropType<OperationCoreMetrics>, required: true },
    lists: { type: Object as PropType<OperationUserLists>, required: true }
  },
  setup(props, { emit }) {
    return () => {
      const cards = [
        {
          tone: 'rose',
          icon: '🔴',
          title: t('admin.operations.v2.actions.highRisk.title'),
          count: props.churn.high_risk_users + props.churn.churned_users,
          subtitle: t('admin.operations.v2.actions.highRisk.sub'),
          listKey: 'silent_high_value' as const,
          listTitle: t('admin.operations.lists.silentHighValue'),
          hint: t('admin.operations.v2.actions.highRisk.modal')
        },
        {
          tone: 'amber',
          icon: '🟡',
          title: t('admin.operations.v2.actions.atRisk.title'),
          count: props.churn.at_risk_users,
          subtitle: t('admin.operations.v2.actions.atRisk.sub'),
          listKey: 'silent_high_value' as const,
          listTitle: t('admin.operations.lists.silentHighValue'),
          hint: t('admin.operations.v2.actions.atRisk.modal')
        },
        {
          tone: 'sky',
          icon: '🟢',
          title: t('admin.operations.v2.actions.newInactive.title'),
          count: props.lists.new_inactive?.length || 0,
          subtitle: t('admin.operations.v2.actions.newInactive.sub'),
          listKey: 'new_inactive' as const,
          listTitle: t('admin.operations.lists.newInactive'),
          hint: t('admin.operations.v2.actions.newInactive.modal')
        },
        {
          tone: 'violet',
          icon: '🎁',
          title: t('admin.operations.v2.actions.expiring.title'),
          count: props.core.expiring_subscriptions,
          subtitle: t('admin.operations.v2.actions.expiring.sub'),
          listKey: 'expiring_soon' as const,
          listTitle: t('admin.operations.lists.expiringSoon'),
          hint: t('admin.operations.v2.actions.expiring.modal')
        }
      ]
      const ringMap: Record<string, string> = {
        rose: 'ring-rose-200 dark:ring-rose-700',
        amber: 'ring-amber-200 dark:ring-amber-700',
        sky: 'ring-sky-200 dark:ring-sky-700',
        violet: 'ring-violet-200 dark:ring-violet-700'
      }
      const textMap: Record<string, string> = {
        rose: 'text-rose-600 dark:text-rose-400',
        amber: 'text-amber-600 dark:text-amber-400',
        sky: 'text-sky-600 dark:text-sky-400',
        violet: 'text-violet-600 dark:text-violet-400'
      }
      return h('section', { class: 'space-y-4' }, [
        h(ChartCaption, {
          title: t('admin.operations.v2.actions.chartTitle'),
          hint: t('admin.operations.v2.actions.hint'),
          action: t('admin.operations.v2.actions.action')
        }),
        h('div', { class: 'grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4' }, cards.map((c) =>
          h('div', { key: c.title, class: `card p-5 ring-1 ${ringMap[c.tone]} flex flex-col gap-3` }, [
            h('div', { class: 'flex items-center gap-2 text-sm font-semibold text-gray-800 dark:text-gray-100' }, [
              h('span', c.icon),
              h('span', c.title)
            ]),
            h('p', { class: `text-4xl font-bold ${textMap[c.tone]}` }, formatNumber(c.count)),
            h('p', { class: 'text-xs text-gray-500 dark:text-gray-400' }, c.subtitle),
            h('button', {
              class: 'btn btn-sm btn-secondary mt-auto',
              onClick: () => emit('openList', { title: c.listTitle, items: props.lists[c.listKey] || [], hint: c.hint })
            }, t('admin.operations.v2.actions.viewList'))
          ])
        ))
      ])
    }
  }
})

// ─────────── Cohort 热力图 ───────────
const CohortHeatmap = defineComponent({
  props: { cohorts: { type: Array as PropType<OperationRetentionCohort[]>, required: true } },
  setup(props) {
    const cellColor = (v: number | null): string => {
      if (v === null) return 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-gray-500'
      const pct = v * 100
      if (pct >= 60) return 'bg-emerald-500/90 text-white'
      if (pct >= 40) return 'bg-emerald-400/80 text-white'
      if (pct >= 25) return 'bg-emerald-300/70 text-emerald-900'
      if (pct >= 10) return 'bg-emerald-200/60 text-emerald-900'
      if (pct > 0) return 'bg-emerald-100/60 text-emerald-900'
      return 'bg-gray-100 text-gray-400 dark:bg-dark-700 dark:text-gray-500'
    }
    return () => h('div', { class: 'overflow-x-auto' }, [
      h('table', { class: 'min-w-full text-xs' }, [
        h('thead', null, [
          h('tr', { class: 'text-gray-500 dark:text-gray-400' }, [
            h('th', { class: 'px-3 py-2 text-left' }, '注册日期'),
            h('th', { class: 'px-3 py-2 text-right' }, '人数'),
            h('th', { class: 'px-3 py-2 text-center' }, 'D0'),
            h('th', { class: 'px-3 py-2 text-center' }, 'D1'),
            h('th', { class: 'px-3 py-2 text-center' }, 'D7'),
            h('th', { class: 'px-3 py-2 text-center' }, 'D30')
          ])
        ]),
        h('tbody', null, props.cohorts.map((c) =>
          h('tr', { key: c.cohort_date, class: 'border-t border-gray-100 dark:border-dark-700' }, [
            h('td', { class: 'px-3 py-1.5 font-medium text-gray-800 dark:text-white' }, c.cohort_date),
            h('td', { class: 'px-3 py-1.5 text-right text-gray-600 dark:text-gray-300' }, formatNumber(c.new_users)),
            ...[c.d0, c.d1, c.d7, c.d30].map((v) =>
              h('td', { class: 'px-1 py-1.5 text-center' }, [
                h('div', { class: `mx-auto rounded px-2 py-1 text-[11px] font-semibold ${cellColor(v)}` },
                  v === null ? '—' : `${(v * 100).toFixed(0)}%`)
              ])
            )
          ])
        ))
      ])
    ])
  }
})

// ─────────── 趋势 / 分布（保留旧组件） ───────────
const TrendPanel = defineComponent({
  props: {
    title: { type: String, required: true },
    items: { type: Array as PropType<OperationTrendPoint[]>, required: true },
    metric: { type: String as PropType<keyof OperationTrendPoint>, required: true },
    formatter: { type: Function as PropType<(value: number) => string>, required: true }
  },
  setup(props) {
    return () => {
      const labels = props.items.map((p) => p.bucket)
      const data = props.items.map((p) => Number(p[props.metric]) || 0)
      return h('section', { class: 'card p-5' }, [
        h('h2', { class: 'mb-4 text-base font-semibold text-gray-900 dark:text-white' }, props.title),
        h(AreaLineChart, {
          labels,
          values: data,
          color: '#0ea5e9',
          suffix: '',
          height: 240
        })
      ])
    }
  }
})

const DistributionPanel = defineComponent({
  props: {
    title: { type: String, required: true },
    items: { type: Array as PropType<OperationDistributionItem[]>, required: true },
    kind: { type: String as PropType<'usage' | 'conversion'>, required: true }
  },
  setup(props) {
    return () => h('section', { class: 'card p-5' }, [
      h('h2', { class: 'mb-4 text-base font-semibold text-gray-900 dark:text-white' }, props.title),
      props.items.length
        ? h('div', { class: 'space-y-3' }, props.items.map((item) =>
            h('div', { key: `${item.key}-${item.label}`, class: 'flex items-center justify-between gap-3 border-b border-gray-100 pb-3 last:border-0 dark:border-dark-700' }, [
              h('div', { class: 'min-w-0' }, [
                h('p', { class: 'truncate text-sm font-medium text-gray-900 dark:text-white' }, item.label || item.key),
                h('p', { class: 'text-xs text-gray-500 dark:text-gray-400' }, props.kind === 'usage'
                  ? t('admin.operations.units.usageSummary', { requests: formatNumber(item.requests), tokens: formatNumber(item.tokens) })
                  : t('admin.operations.units.conversionSummary', { count: formatNumber(item.count), users: formatNumber(item.users) }))
              ]),
              h('div', { class: 'text-right text-sm font-semibold text-gray-900 dark:text-white' }, props.kind === 'usage'
                ? `$${formatMoney(item.actual_cost)}`
                : formatMoney(item.value))
            ])
          ))
        : h('p', { class: 'py-6 text-center text-sm text-gray-500 dark:text-gray-400' }, t('admin.operations.empty'))
    ])
  }
})

// ─────────── 名单弹窗 ───────────
const ListModal = defineComponent({
  emits: ['close', 'openUsage', 'openSubscriptions'],
  props: {
    title: { type: String, required: true },
    items: { type: Array as PropType<OperationUserListItem[]>, required: true },
    hint: { type: String, required: true }
  },
  setup(props, { emit }) {
    return () => h('div', {
      class: 'fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4',
      onClick: () => emit('close')
    }, [
      h('div', {
        class: 'relative max-h-[80vh] w-full max-w-3xl overflow-y-auto rounded-lg bg-white p-6 shadow-xl dark:bg-dark-800',
        onClick: (e: Event) => e.stopPropagation()
      }, [
        h('div', { class: 'mb-4 flex items-start justify-between' }, [
          h('div', null, [
            h('h2', { class: 'text-lg font-bold text-gray-900 dark:text-white' }, props.title),
            h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-gray-400' }, props.hint)
          ]),
          h('button', { class: 'btn btn-sm btn-secondary', onClick: () => emit('close') }, t('admin.operations.v2.modal.close'))
        ]),
        props.items.length === 0
          ? h('p', { class: 'py-12 text-center text-sm text-gray-500 dark:text-gray-400' }, t('admin.operations.empty'))
          : h('div', { class: 'space-y-2' }, props.items.map((item) =>
              h('div', { key: item.user_id, class: 'rounded-md border border-gray-100 p-3 dark:border-dark-700' }, [
                h('div', { class: 'flex items-start justify-between gap-3' }, [
                  h('div', { class: 'min-w-0' }, [
                    h('p', { class: 'truncate text-sm font-semibold text-gray-900 dark:text-white' }, item.username || item.email || `#${item.user_id}`),
                    h('p', { class: 'truncate text-xs text-gray-500 dark:text-gray-400' }, item.email || `#${item.user_id}`),
                    h('p', { class: 'mt-1 text-xs text-gray-500 dark:text-gray-400' }, listMeta(item))
                  ]),
                  h('div', { class: 'text-right' }, [
                    h('p', { class: 'text-sm font-semibold text-gray-900 dark:text-white' },
                      item.value ? `$${formatMoney(item.value)}` : item.value_label),
                    item.group_name ? h('p', { class: 'text-xs text-gray-500 dark:text-gray-400' }, item.group_name) : null
                  ])
                ]),
                h('div', { class: 'mt-2 flex gap-2' }, [
                  h('button', { class: 'btn btn-xs btn-secondary', onClick: () => emit('openUsage', item) }, t('admin.operations.actions.usage')),
                  h('button', { class: 'btn btn-xs btn-secondary', onClick: () => emit('openSubscriptions', item) }, t('admin.operations.actions.subscription'))
                ])
              ])
            ))
      ])
    ])
  }
})

function listMeta(item: OperationUserListItem): string {
  if (item.expires_at) return `${item.value_label} · ${new Date(item.expires_at).toLocaleDateString()}`
  if (item.days_since !== undefined) return `${item.value_label} · ${t('admin.operations.units.daysIdle', { value: item.days_since })}`
  if (item.last_usage_at) return `${item.value_label} · ${new Date(item.last_usage_at).toLocaleDateString()}`
  return item.value_label
}

onMounted(() => {
  void loadSnapshot()
})
</script>
