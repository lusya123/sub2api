<template>
  <AppLayout>
    <div class="mx-auto max-w-[1680px] space-y-5">
      <nav class="flex flex-wrap items-center gap-2 rounded-xl border border-gray-200/80 bg-white/80 p-1 shadow-sm backdrop-blur dark:border-dark-700/70 dark:bg-dark-800/70">
        <router-link
          v-for="item in topNavItems"
          :key="item.path"
          :to="item.path"
          class="inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold transition-colors"
          :class="item.active ? 'bg-gray-950 text-white shadow-sm dark:bg-white dark:text-gray-950' : 'text-gray-600 hover:bg-gray-100 hover:text-gray-950 dark:text-gray-300 dark:hover:bg-dark-700 dark:hover:text-white'"
        >
          <Icon :name="item.icon" size="sm" />
          {{ item.label }}
        </router-link>
      </nav>

      <section class="rounded-xl border border-gray-200/80 bg-white/85 p-4 shadow-sm backdrop-blur dark:border-dark-700/70 dark:bg-dark-800/75">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2 text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
              <span>{{ t('modelMarketplaceStatus.title') }}</span>
              <span class="h-1 w-1 rounded-full bg-gray-300 dark:bg-dark-600"></span>
              <span>{{ t('modelMarketplaceStatus.totalModels', { count: modelCards.length }, `${modelCards.length} models`) }}</span>
            </div>
            <h2 class="mt-1 text-2xl font-bold text-gray-950 dark:text-white">
              {{ t('modelMarketplaceStatus.squareHeading', 'Model Square') }}
            </h2>
          </div>

          <div class="relative w-full xl:w-[560px]">
            <Icon
              name="search"
              size="md"
              class="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
            />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('modelMarketplaceStatus.searchPlaceholderFull', 'Search model name, provider, group, endpoint or tag...')"
              class="input h-12 rounded-xl pl-11 pr-16"
            />
            <kbd class="pointer-events-none absolute right-4 top-1/2 hidden -translate-y-1/2 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 text-[11px] font-semibold text-gray-400 dark:border-dark-700 dark:bg-dark-900/70 sm:inline-flex">
              ⌘K
            </kbd>
          </div>
        </div>

        <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <div
            v-for="stat in summaryStats"
            :key="stat.label"
            class="rounded-lg border border-gray-100 bg-gray-50/70 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/35"
          >
            <div class="text-xs text-gray-500 dark:text-gray-400">{{ stat.label }}</div>
            <div class="mt-1 flex items-end justify-between gap-3">
              <div class="font-mono text-xl font-bold text-gray-950 dark:text-white">{{ stat.value }}</div>
              <span class="text-xs font-medium" :class="stat.tone">{{ stat.hint }}</span>
            </div>
          </div>
        </div>
      </section>

      <div class="grid gap-5 lg:grid-cols-[300px_minmax(0,1fr)]">
        <aside class="space-y-4 rounded-xl border border-gray-200/80 bg-white/85 p-4 shadow-sm dark:border-dark-700/70 dark:bg-dark-800/75 lg:sticky lg:top-24 lg:max-h-[calc(100vh-7rem)] lg:overflow-y-auto">
          <div class="flex items-start justify-between gap-3">
            <div>
              <h3 class="text-base font-bold text-gray-950 dark:text-white">
                {{ t('modelMarketplaceStatus.filters.title', 'Filters') }}
              </h3>
              <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
                {{ t('modelMarketplaceStatus.filters.description', 'Filter by provider, group, status and model tags.') }}
              </p>
            </div>
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-semibold text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-white"
              @click="resetFilters"
            >
              <Icon name="refresh" size="xs" />
              {{ t('modelMarketplaceStatus.filters.reset', 'Reset') }}
            </button>
          </div>

          <FilterSection
            :title="t('modelMarketplaceStatus.filters.groups', 'Groups')"
            :options="groupOptions"
            :active="groupFilter"
            @select="groupFilter = $event"
          />
          <FilterSection
            :title="t('modelMarketplaceStatus.filters.providers', 'Providers')"
            :options="vendorOptions"
            :active="vendorFilter"
            @select="vendorFilter = $event"
          />
          <FilterSection
            :title="t('modelMarketplaceStatus.filters.tags', 'Model Tags')"
            :options="tagOptions"
            :active="tagFilter"
            @select="tagFilter = $event"
          />
          <FilterSection
            :title="t('modelMarketplaceStatus.filters.billing', 'Billing')"
            :options="billingOptions"
            :active="billingFilter"
            @select="billingFilter = $event"
          />
          <FilterSection
            :title="t('modelMarketplaceStatus.filters.endpoints', 'Endpoint Types')"
            :options="endpointOptions"
            :active="endpointFilter"
            @select="endpointFilter = $event"
          />
          <FilterSection
            :title="t('modelMarketplaceStatus.filters.health', 'Health')"
            :options="healthOptions"
            :active="healthFilter"
            @select="healthFilter = $event"
          />
        </aside>

        <section class="min-w-0 space-y-4">
          <div class="flex flex-col gap-3 rounded-xl border border-gray-200/80 bg-white/85 p-4 shadow-sm dark:border-dark-700/70 dark:bg-dark-800/75 2xl:flex-row 2xl:items-center 2xl:justify-between">
            <div class="flex min-w-0 items-center gap-3">
              <div class="grid h-10 w-10 place-items-center rounded-lg bg-gray-950 text-white dark:bg-white dark:text-gray-950">
                {{ filteredModelCards.length }}
              </div>
              <div class="min-w-0">
                <div class="font-semibold text-gray-950 dark:text-white">
                  {{ t('modelMarketplaceStatus.visibleModels', { count: filteredModelCards.length }, `${filteredModelCards.length} models`) }}
                </div>
                <div class="truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ activeFilterSummary }}
                </div>
              </div>
            </div>

            <div class="flex flex-wrap items-center gap-2">
              <div class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-700 dark:bg-dark-900/60">
                <button
                  v-for="mode in priceUnitOptions"
                  :key="mode.value"
                  type="button"
                  class="rounded-md px-3 py-1.5 text-xs font-semibold transition-colors"
                  :class="priceUnit === mode.value ? 'bg-gray-950 text-white dark:bg-white dark:text-gray-950' : 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white'"
                  @click="priceUnit = mode.value"
                >
                  {{ mode.label }}
                </button>
              </div>

              <button
                type="button"
                class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-semibold text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
                @click="toggleSort"
              >
                <Icon name="sort" size="sm" />
                {{ sortLabel }}
              </button>

              <button
                type="button"
                class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-semibold text-gray-600 hover:bg-gray-50 disabled:opacity-60 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
                :disabled="loading"
                @click="manualReload"
              >
                <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
                {{ t('common.refresh', 'Refresh') }}
              </button>
            </div>
          </div>

          <div
            v-if="loading && modelCards.length === 0"
            class="grid grid-cols-1 gap-4 xl:grid-cols-2 2xl:grid-cols-3"
          >
            <div
              v-for="i in 9"
              :key="i"
              class="h-[210px] animate-pulse rounded-xl border border-gray-200/80 bg-white/70 p-5 dark:border-dark-700/70 dark:bg-dark-800/60"
            >
              <div class="flex items-start gap-3">
                <div class="h-12 w-12 rounded-xl bg-gray-200 dark:bg-dark-700"></div>
                <div class="flex-1 space-y-2">
                  <div class="h-4 w-2/3 rounded bg-gray-200 dark:bg-dark-700"></div>
                  <div class="h-3 w-1/2 rounded bg-gray-100 dark:bg-dark-700"></div>
                </div>
              </div>
              <div class="mt-6 h-10 rounded-lg bg-gray-100 dark:bg-dark-900/40"></div>
              <div class="mt-5 flex gap-2">
                <div class="h-6 w-20 rounded bg-gray-100 dark:bg-dark-900/40"></div>
                <div class="h-6 w-24 rounded bg-gray-100 dark:bg-dark-900/40"></div>
              </div>
            </div>
          </div>

          <EmptyState
            v-else-if="filteredModelCards.length === 0"
            :title="t('modelMarketplaceStatus.empty.title')"
            :description="t('modelMarketplaceStatus.empty.description')"
          />

          <div v-else class="grid grid-cols-1 gap-4 xl:grid-cols-2 2xl:grid-cols-3">
            <article
              v-for="card in pagedModelCards"
              :key="card.key"
              class="group flex min-h-[210px] min-w-0 flex-col rounded-xl border border-gray-200/80 bg-white/90 p-5 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-card-hover dark:border-dark-700/70 dark:bg-dark-800/75 dark:hover:border-primary-500/30"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="flex min-w-0 items-start gap-3">
                  <span
                    class="grid h-12 w-12 flex-shrink-0 place-items-center rounded-xl ring-1 ring-black/5 dark:ring-white/10"
                    :class="[providerGradient(card.provider), providerTintClass(card.provider)]"
                  >
                    <ProviderIcon :provider="card.provider" :size="22" />
                  </span>

                  <div class="min-w-0">
                    <h3 class="truncate font-mono text-lg font-bold text-gray-950 dark:text-white">
                      {{ card.model }}
                    </h3>
                    <div class="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                      <span>{{ priceLine(card) }}</span>
                      <span class="text-gray-300 dark:text-dark-600">|</span>
                      <span>{{ card.monitor.name }}</span>
                    </div>
                  </div>
                </div>

                <div class="flex flex-shrink-0 items-center gap-2">
                  <button
                    type="button"
                    class="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-semibold text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-dark-700"
                    @click="openDetail(card.monitor)"
                  >
                    {{ t('modelMarketplaceStatus.detailButton', 'Details') }}
                    <Icon name="chevronRight" size="xs" />
                  </button>
                  <button
                    type="button"
                    class="grid h-8 w-8 place-items-center rounded-lg border border-gray-200 text-gray-500 hover:bg-gray-50 hover:text-gray-900 dark:border-dark-700 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-white"
                    :title="t('common.copy', 'Copy')"
                    @click="copyModel(card.model)"
                  >
                    <Icon name="copy" size="sm" />
                  </button>
                </div>
              </div>

              <p class="mt-4 line-clamp-2 min-h-[40px] text-sm leading-5 text-gray-600 dark:text-gray-300">
                {{ card.description }}
              </p>

              <div class="mt-4 flex flex-wrap items-center gap-2">
                <span class="rounded-md px-2 py-1 text-xs font-semibold" :class="statusBadgeClass(card.status)">
                  {{ statusLabel(card.status) }}
                </span>
                <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                  {{ card.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped') }}
                </span>
                <span class="rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                  {{ billingLabel(card.billingType) }}
                </span>
                <span
                  v-for="tag in card.tags.slice(0, 3)"
                  :key="`${card.key}:${tag}`"
                  class="rounded-md bg-gray-50 px-2 py-1 text-xs font-medium text-gray-500 ring-1 ring-gray-200 dark:bg-dark-900/40 dark:text-gray-400 dark:ring-dark-700"
                >
                  {{ tag }}
                </span>
              </div>

              <div class="mt-auto grid grid-cols-3 gap-2 pt-4">
                <MetricCell
                  :label="t('monitorCommon.dialogLatency')"
                  :value="formatLatency(card.latencyMs)"
                  :suffix="card.latencyMs == null ? '' : 'ms'"
                />
                <MetricCell
                  :label="t('monitorCommon.endpointPing')"
                  :value="formatLatency(card.pingLatencyMs)"
                  :suffix="card.pingLatencyMs == null ? '' : 'ms'"
                />
                <MetricCell
                  :label="t('modelMarketplaceStatus.availability7d', '7d Availability')"
                  :value="formatPercent(card.availability7d)"
                />
              </div>
            </article>
          </div>

          <div
            v-if="filteredModelCards.length > 0"
            class="flex flex-col gap-3 rounded-xl border border-gray-200/80 bg-white/85 p-4 shadow-sm dark:border-dark-700/70 dark:bg-dark-800/75 sm:flex-row sm:items-center sm:justify-between"
          >
            <div class="text-sm text-gray-500 dark:text-gray-400">
              {{ paginationSummary }}
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <select v-model.number="pageSize" class="input h-9 w-28 rounded-lg py-1 text-sm">
                <option :value="12">12 / page</option>
                <option :value="24">24 / page</option>
                <option :value="48">48 / page</option>
                <option :value="96">96 / page</option>
              </select>
              <button
                type="button"
                class="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-3 py-2 text-sm font-semibold text-gray-600 hover:bg-gray-50 disabled:opacity-50 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-dark-700"
                :disabled="safeCurrentPage <= 1"
                @click="currentPage = safeCurrentPage - 1"
              >
                <Icon name="chevronLeft" size="xs" />
                {{ t('common.previous', 'Previous') }}
              </button>
              <span class="rounded-lg bg-gray-100 px-3 py-2 font-mono text-sm font-semibold text-gray-700 dark:bg-dark-700 dark:text-gray-200">
                {{ safeCurrentPage }} / {{ totalPages }}
              </span>
              <button
                type="button"
                class="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-3 py-2 text-sm font-semibold text-gray-600 hover:bg-gray-50 disabled:opacity-50 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-dark-700"
                :disabled="safeCurrentPage >= totalPages"
                @click="currentPage = safeCurrentPage + 1"
              >
                {{ t('common.next', 'Next') }}
                <Icon name="chevronRight" size="xs" />
              </button>
            </div>
          </div>
        </section>
      </div>
    </div>

    <ModelMarketplaceDetailDialog
      :show="showDetail"
      :monitor-id="detailTarget?.id ?? null"
      :title="detailTitle"
      @close="closeDetail"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { PropType } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  list as listModelMarketplaceViews,
  type UserModelMarketplaceView,
  type UserModelMarketplaceExtraModel,
} from '@/api/modelMarketplace'
import type { Provider, MonitorStatus } from '@/api/admin/modelMarketplaceMonitor'
import type { BillingMode } from '@/constants/channel'
import type { UserSupportedModelPricing } from '@/api/channels'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelMarketplaceDetailDialog from '@/components/user/ModelMarketplaceDetailDialog.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  DEFAULT_INTERVAL_SECONDS,
  STATUS_DEGRADED,
  STATUS_ERROR,
  STATUS_FAILED,
  STATUS_OPERATIONAL,
} from '@/constants/modelMarketplaceMonitor'
import { BILLING_MODE_IMAGE, BILLING_MODE_PER_REQUEST, BILLING_MODE_TOKEN } from '@/constants/channel'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { useModelMarketplaceMonitorFormat, providerGradient } from '@/composables/useModelMarketplaceMonitorFormat'
import { formatScaled } from '@/utils/pricing'

type FilterKey = 'group' | 'vendor' | 'tag' | 'billing' | 'endpoint' | 'health'
type FilterValue = string
type PriceUnit = '1M' | '1K'
type SortMode = 'name' | 'health'

interface FilterOption {
  value: FilterValue
  label: string
  count?: number | string
  icon?: string
}

interface MarketplaceModelCard {
  key: string
  monitor: UserModelMarketplaceView
  monitorId: number
  model: string
  provider: Provider
  vendor: string
  groupName: string
  status: MonitorStatus | ''
  health: 'available' | 'degraded' | 'unavailable' | 'unknown'
  latencyMs: number | null
  pingLatencyMs: number | null
  availability7d: number | null
  pricing: UserSupportedModelPricing | null
  billingType: BillingMode | 'unknown'
  endpointTypes: string[]
  tags: string[]
  description: string
  isPrimary: boolean
}

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const {
  statusLabel,
  statusBadgeClass,
  providerLabel,
  formatLatency,
  formatPercent,
} = useModelMarketplaceMonitorFormat()

const items = ref<UserModelMarketplaceView[]>([])
const loading = ref(false)
const showDetail = ref(false)
const detailTarget = ref<UserModelMarketplaceView | null>(null)
const searchQuery = ref('')
const groupFilter = ref('all')
const vendorFilter = ref('all')
const tagFilter = ref('all')
const billingFilter = ref('all')
const endpointFilter = ref('all')
const healthFilter = ref('all')
const priceUnit = ref<PriceUnit>('1M')
const sortMode = ref<SortMode>('name')
const currentPage = ref(1)
const pageSize = ref(12)

let abortController: AbortController | null = null

const autoRefresh = useAutoRefresh({
  storageKey: 'model-marketplace-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: DEFAULT_INTERVAL_SECONDS,
  onRefresh: () => reload(true),
  shouldPause: () => document.hidden || loading.value,
})
const countdown = autoRefresh.countdown

const PROVIDER_TINT: Record<string, string> = {
  openai: 'text-emerald-600 dark:text-emerald-300',
  anthropic: 'text-orange-600 dark:text-orange-300',
  gemini: 'text-sky-600 dark:text-sky-300',
}

const topNavItems = computed(() => [
  {
    path: '/model-marketplace',
    label: t('nav.modelMarketplace'),
    icon: 'grid' as const,
    active: route.path === '/model-marketplace',
  },
  {
    path: '/status',
    label: t('nav.modelStatus'),
    icon: 'chart' as const,
    active: route.path === '/status',
  },
  {
    path: '/available-channels',
    label: t('nav.availableChannels'),
    icon: 'server' as const,
    active: route.path === '/available-channels',
  },
])

const modelCards = computed<MarketplaceModelCard[]>(() => {
  return items.value.flatMap((monitor) => {
    const cards: MarketplaceModelCard[] = [
      buildModelCard({
        monitor,
        model: monitor.primary_model,
        status: monitor.primary_status,
        latencyMs: monitor.primary_latency_ms,
        pingLatencyMs: monitor.primary_ping_latency_ms,
        availability7d: monitor.availability_7d,
        pricing: monitor.primary_pricing,
        isPrimary: true,
      }),
    ]
    for (const extra of monitor.extra_models || []) {
      cards.push(extraToCard(monitor, extra))
    }
    return cards
  })
})

const filteredModelCards = computed(() => filterCards([]))

const sortedModelCards = computed(() => {
  const order = [...filteredModelCards.value]
  if (sortMode.value === 'health') {
    return order.sort((a, b) => healthRank(a) - healthRank(b) || a.model.localeCompare(b.model))
  }
  return order.sort((a, b) => a.model.localeCompare(b.model))
})

const totalPages = computed(() => Math.max(1, Math.ceil(sortedModelCards.value.length / pageSize.value)))

const safeCurrentPage = computed(() => Math.min(Math.max(1, currentPage.value), totalPages.value))

const pagedModelCards = computed(() => {
  const start = (safeCurrentPage.value - 1) * pageSize.value
  return sortedModelCards.value.slice(start, start + pageSize.value)
})

const paginationSummary = computed(() => {
  if (sortedModelCards.value.length === 0) return t('modelMarketplaceStatus.visibleModels', { count: 0 })
  const start = (safeCurrentPage.value - 1) * pageSize.value + 1
  const end = Math.min(sortedModelCards.value.length, start + pageSize.value - 1)
  return t(
    'modelMarketplaceStatus.paginationSummary',
    { start, end, total: sortedModelCards.value.length },
    `Showing ${start}-${end} of ${sortedModelCards.value.length}`,
  )
})

watch(
  [searchQuery, groupFilter, vendorFilter, tagFilter, billingFilter, endpointFilter, healthFilter, pageSize],
  () => {
    currentPage.value = 1
  },
)

const summaryStats = computed(() => {
  const available = modelCards.value.filter((m) => m.health === 'available').length
  const degraded = modelCards.value.filter((m) => m.health === 'degraded').length
  const unavailable = modelCards.value.filter((m) => m.health === 'unavailable').length
  const providerCount = new Set(modelCards.value.map((m) => m.vendor)).size
  return [
    {
      label: t('modelMarketplaceStatus.stats.total', 'Total models'),
      value: modelCards.value.length,
      hint: `${providerCount} ${t('modelMarketplaceStatus.stats.providers', 'providers')}`,
      tone: 'text-gray-500 dark:text-gray-400',
    },
    {
      label: t('modelMarketplaceStatus.stats.available', 'Available'),
      value: available,
      hint: modelCards.value.length ? `${Math.round((available / modelCards.value.length) * 100)}%` : '-',
      tone: 'text-emerald-600 dark:text-emerald-300',
    },
    {
      label: t('modelMarketplaceStatus.stats.degraded', 'Degraded'),
      value: degraded,
      hint: t('monitorCommon.status.degraded'),
      tone: 'text-amber-600 dark:text-amber-300',
    },
    {
      label: t('modelMarketplaceStatus.stats.unavailable', 'Unavailable'),
      value: unavailable,
      hint: t('monitorCommon.status.failed'),
      tone: 'text-red-600 dark:text-red-300',
    },
  ]
})

const activeFilterSummary = computed(() => {
  const parts = [
    labelForActive(groupOptions.value, groupFilter.value),
    labelForActive(vendorOptions.value, vendorFilter.value),
    labelForActive(tagOptions.value, tagFilter.value),
    labelForActive(billingOptions.value, billingFilter.value),
    labelForActive(endpointOptions.value, endpointFilter.value),
    labelForActive(healthOptions.value, healthFilter.value),
  ].filter(Boolean)
  if (parts.length === 0 && !searchQuery.value.trim()) {
    return t('modelMarketplaceStatus.allFilters', 'All configured models from admin marketplace monitors')
  }
  return parts.join(' / ')
})

const groupOptions = computed<FilterOption[]>(() => {
  const values = uniqueSorted(modelCards.value.map((m) => m.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped')))
  return [
    { value: 'all', label: t('modelMarketplaceStatus.filters.allGroups', 'All Groups'), count: filterCards(['group']).length },
    ...values.map((value) => ({
      value,
      label: value,
      count: filterCards(['group']).filter((m) => (m.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped')) === value).length,
    })),
  ]
})

const vendorOptions = computed<FilterOption[]>(() => {
  const values = uniqueSorted(modelCards.value.map((m) => m.vendor))
  return [
    { value: 'all', label: t('modelMarketplaceStatus.filters.allProviders', 'All Providers'), count: filterCards(['vendor']).length },
    ...values.map((value) => ({
      value,
      label: value,
      count: filterCards(['vendor']).filter((m) => m.vendor === value).length,
    })),
  ]
})

const tagOptions = computed<FilterOption[]>(() => {
  const values = uniqueSorted(modelCards.value.flatMap((m) => m.tags))
  return [
    { value: 'all', label: t('modelMarketplaceStatus.filters.allTags', 'All Tags'), count: filterCards(['tag']).length },
    ...values.map((value) => ({
      value,
      label: value,
      count: filterCards(['tag']).filter((m) => m.tags.includes(value)).length,
    })),
  ]
})

const billingOptions = computed<FilterOption[]>(() => {
  const options = [
    { value: 'all', label: t('modelMarketplaceStatus.filters.allBilling', 'All Billing') },
    { value: BILLING_MODE_TOKEN, label: t('availableChannels.pricing.billingModeToken') },
    { value: BILLING_MODE_PER_REQUEST, label: t('availableChannels.pricing.billingModePerRequest') },
    { value: BILLING_MODE_IMAGE, label: t('availableChannels.pricing.billingModeImage') },
    { value: 'unknown', label: t('availableChannels.noPricing') },
  ]
  return options.map((option) => ({
    ...option,
    count: option.value === 'all'
      ? filterCards(['billing']).length
      : filterCards(['billing']).filter((m) => m.billingType === option.value).length,
  }))
})

const endpointOptions = computed<FilterOption[]>(() => {
  const values = uniqueSorted(modelCards.value.flatMap((m) => m.endpointTypes))
  return [
    { value: 'all', label: t('modelMarketplaceStatus.filters.allEndpoints', 'All Endpoints'), count: filterCards(['endpoint']).length },
    ...values.map((value) => ({
      value,
      label: value,
      count: filterCards(['endpoint']).filter((m) => m.endpointTypes.includes(value)).length,
    })),
  ]
})

const healthOptions = computed<FilterOption[]>(() => {
  const options = [
    { value: 'all', label: t('modelMarketplaceStatus.filters.allHealth', 'All Health') },
    { value: 'available', label: t('modelMarketplaceStatus.health.available', 'Available') },
    { value: 'degraded', label: t('modelMarketplaceStatus.health.degraded', 'Degraded') },
    { value: 'unavailable', label: t('modelMarketplaceStatus.health.unavailable', 'Unavailable') },
    { value: 'unknown', label: t('monitorCommon.status.unknown') },
  ]
  return options.map((option) => ({
    ...option,
    count: option.value === 'all'
      ? filterCards(['health']).length
      : filterCards(['health']).filter((m) => m.health === option.value).length,
  }))
})

const priceUnitOptions = computed(() => [
  { value: '1M' as const, label: '/1M' },
  { value: '1K' as const, label: '/1K' },
])

const sortLabel = computed(() => {
  return sortMode.value === 'name'
    ? t('modelMarketplaceStatus.sort.name', 'Name')
    : t('modelMarketplaceStatus.sort.health', 'Health')
})

const detailTitle = computed(() => {
  return detailTarget.value?.name || t('modelMarketplaceStatus.detailTitle')
})

function buildModelCard(input: {
  monitor: UserModelMarketplaceView
  model: string
  status: MonitorStatus | ''
  latencyMs: number | null
  pingLatencyMs: number | null
  availability7d: number | null
  pricing: UserSupportedModelPricing | null
  isPrimary: boolean
}): MarketplaceModelCard {
  const billingType = input.pricing?.billing_mode ?? 'unknown'
  const endpointTypes = inferEndpointTypes(input.monitor.provider, input.model, billingType)
  const tags = inferTags(input.model, input.status, billingType, endpointTypes, input.isPrimary)
  return {
    key: `${input.monitor.id}:${input.model}:${input.isPrimary ? 'primary' : 'extra'}`,
    monitor: input.monitor,
    monitorId: input.monitor.id,
    model: input.model,
    provider: input.monitor.provider,
    vendor: providerLabel(input.monitor.provider),
    groupName: input.monitor.group_name,
    status: input.status,
    health: resolveHealth(input.status),
    latencyMs: input.latencyMs,
    pingLatencyMs: input.pingLatencyMs,
    availability7d: input.availability7d,
    pricing: input.pricing,
    billingType,
    endpointTypes,
    tags,
    description: buildDescription(input.monitor, input.model, input.status, input.isPrimary),
    isPrimary: input.isPrimary,
  }
}

function extraToCard(monitor: UserModelMarketplaceView, extra: UserModelMarketplaceExtraModel): MarketplaceModelCard {
  return buildModelCard({
    monitor,
    model: extra.model,
    status: extra.status,
    latencyMs: extra.latency_ms,
    pingLatencyMs: extra.ping_latency_ms,
    availability7d: extra.availability_7d,
    pricing: extra.pricing,
    isPrimary: false,
  })
}

function filterCards(ignore: FilterKey[]): MarketplaceModelCard[] {
  const q = searchQuery.value.trim().toLowerCase()
  return modelCards.value.filter((card) => {
    if (!ignore.includes('group') && groupFilter.value !== 'all') {
      const group = card.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped')
      if (group !== groupFilter.value) return false
    }
    if (!ignore.includes('vendor') && vendorFilter.value !== 'all' && card.vendor !== vendorFilter.value) return false
    if (!ignore.includes('tag') && tagFilter.value !== 'all' && !card.tags.includes(tagFilter.value)) return false
    if (!ignore.includes('billing') && billingFilter.value !== 'all' && card.billingType !== billingFilter.value) return false
    if (!ignore.includes('endpoint') && endpointFilter.value !== 'all' && !card.endpointTypes.includes(endpointFilter.value)) return false
    if (!ignore.includes('health') && healthFilter.value !== 'all' && card.health !== healthFilter.value) return false
    if (!q) return true
    return [
      card.model,
      card.monitor.name,
      card.groupName,
      card.vendor,
      card.provider,
      card.billingType,
      ...card.tags,
      ...card.endpointTypes,
    ].some((value) => String(value || '').toLowerCase().includes(q))
  })
}

function resetFilters() {
  searchQuery.value = ''
  groupFilter.value = 'all'
  vendorFilter.value = 'all'
  tagFilter.value = 'all'
  billingFilter.value = 'all'
  endpointFilter.value = 'all'
  healthFilter.value = 'all'
  currentPage.value = 1
}

function toggleSort() {
  sortMode.value = sortMode.value === 'name' ? 'health' : 'name'
  currentPage.value = 1
}

function labelForActive(options: FilterOption[], value: string): string {
  if (value === 'all') return ''
  return options.find((option) => option.value === value)?.label || value
}

function uniqueSorted(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b))
}

function resolveHealth(status: MonitorStatus | ''): MarketplaceModelCard['health'] {
  if (status === STATUS_OPERATIONAL) return 'available'
  if (status === STATUS_DEGRADED) return 'degraded'
  if (status === STATUS_FAILED || status === STATUS_ERROR) return 'unavailable'
  return 'unknown'
}

function healthRank(card: MarketplaceModelCard): number {
  if (card.health === 'unavailable') return 0
  if (card.health === 'degraded') return 1
  if (card.health === 'unknown') return 2
  return 3
}

function providerTintClass(provider: string): string {
  return PROVIDER_TINT[provider] ?? 'text-gray-500 dark:text-gray-300'
}

function inferEndpointTypes(provider: Provider, model: string, billingType: BillingMode | 'unknown'): string[] {
  const lower = model.toLowerCase()
  const endpoints = new Set<string>()
  if (lower.includes('embed') || lower.includes('bge') || lower.includes('ada')) endpoints.add('Embedding')
  if (billingType === BILLING_MODE_IMAGE || lower.includes('image') || lower.includes('flux') || lower.includes('dall')) endpoints.add('Image')
  if (provider === 'anthropic') endpoints.add('Anthropic')
  if (provider === 'gemini') endpoints.add('Gemini')
  if (provider === 'openai') {
    endpoints.add('Chat')
    endpoints.add('Response')
  }
  return Array.from(endpoints)
}

function inferTags(
  model: string,
  status: MonitorStatus | '',
  billingType: BillingMode | 'unknown',
  endpoints: string[],
  isPrimary: boolean,
): string[] {
  const lower = model.toLowerCase()
  const tags = new Set<string>()
  if (isPrimary) tags.add('primary')
  if (status === STATUS_OPERATIONAL || status === STATUS_DEGRADED) tags.add('available')
  if (status === STATUS_DEGRADED) tags.add('degraded')
  if (status === STATUS_FAILED || status === STATUS_ERROR) tags.add('unavailable')
  if (lower.includes('audio') || lower.includes('whisper') || lower.includes('tts')) tags.add('audio')
  if (lower.includes('vision') || lower.includes('4o') || lower.includes('gemini')) tags.add('vision')
  if (lower.includes('reason') || lower.includes('thinking') || lower.includes('o1') || lower.includes('o3') || lower.includes('o4') || lower.includes('r1')) tags.add('reasoning')
  if (endpoints.includes('Embedding')) tags.add('embedding')
  if (endpoints.includes('Image')) tags.add('image')
  if (billingType === BILLING_MODE_PER_REQUEST) tags.add('per-request')
  if (billingType === BILLING_MODE_TOKEN) tags.add('token')
  return Array.from(tags)
}

function buildDescription(monitor: UserModelMarketplaceView, model: string, status: MonitorStatus | '', isPrimary: boolean): string {
  const role = isPrimary
    ? t('modelMarketplaceStatus.primaryModel')
    : t('modelMarketplaceStatus.extraModel', 'Additional model')
  const statusText = statusLabel(status)
  return t(
    'modelMarketplaceStatus.cardDescription',
    {
      provider: providerLabel(monitor.provider),
      group: monitor.group_name || t('modelMarketplaceStatus.ungrouped', 'Ungrouped'),
      role,
      model,
      status: statusText,
    },
    `${providerLabel(monitor.provider)} ${role}: ${model}. Group ${monitor.group_name || '-'}, current health ${statusText}.`,
  )
}

function billingLabel(mode: BillingMode | 'unknown'): string {
  if (mode === BILLING_MODE_TOKEN) return t('availableChannels.pricing.billingModeToken')
  if (mode === BILLING_MODE_PER_REQUEST) return t('availableChannels.pricing.billingModePerRequest')
  if (mode === BILLING_MODE_IMAGE) return t('availableChannels.pricing.billingModeImage')
  return t('availableChannels.noPricing')
}

function priceLine(card: MarketplaceModelCard): string {
  const pieces = pricePieces(card)
  if (pieces.length === 0) return t('modelMarketplaceStatus.priceUnavailable')
  return pieces.join('  ')
}

function pricePieces(card: MarketplaceModelCard): string[] {
  const pricing = card.pricing
  if (!pricing) return []
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST && pricing.per_request_price != null) {
    return [`${t('availableChannels.pricing.perRequestPrice')} ${formatScaled(pricing.per_request_price, 1)}${t('availableChannels.pricing.unitPerRequest')}`]
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE && pricing.image_output_price != null) {
    return [`${t('availableChannels.pricing.imageOutputPrice')} ${formatScaled(pricing.image_output_price, 1)}${t('availableChannels.pricing.unitPerRequest')}`]
  }
  if (pricing.billing_mode !== BILLING_MODE_TOKEN) return []
  const scale = priceUnit.value === '1M' ? 1_000_000 : 1_000
  const unit = priceUnit.value === '1M' ? t('availableChannels.pricing.unitPerMillion') : '/ 1K token'
  return [
    pricing.input_price == null ? '' : `${t('availableChannels.pricing.inputPrice')} ${formatScaled(pricing.input_price, scale)}${unit}`,
    pricing.output_price == null ? '' : `${t('availableChannels.pricing.outputPrice')} ${formatScaled(pricing.output_price, scale)}${unit}`,
  ].filter(Boolean)
}

async function reload(silent = false) {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  if (!silent) loading.value = true
  try {
    const res = await listModelMarketplaceViews({ signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    items.value = res.items || []
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('modelMarketplaceStatus.loadError')))
  } finally {
    if (abortController === ctrl) {
      if (!silent) loading.value = false
      countdown.value = DEFAULT_INTERVAL_SECONDS
      abortController = null
    }
  }
}

async function manualReload() {
  await reload(false)
}

function openDetail(row: UserModelMarketplaceView | unknown) {
  const target = row as UserModelMarketplaceView
  detailTarget.value = target
  showDetail.value = true
}

function closeDetail() {
  showDetail.value = false
  detailTarget.value = null
}

async function copyModel(model: string) {
  try {
    await navigator.clipboard.writeText(model)
    appStore.showSuccess(t('common.copied', 'Copied'))
  } catch {
    appStore.showError(model)
  }
}

onMounted(() => {
  void reload(false)
  autoRefresh.setEnabled(true)
})

onBeforeUnmount(() => {
  if (abortController) abortController.abort()
})

const FilterSection = defineComponent({
  name: 'FilterSection',
  props: {
    title: { type: String, required: true },
    options: { type: Array as PropType<FilterOption[]>, required: true },
    active: { type: String, required: true },
  },
  emits: ['select'],
  setup(props, { emit }) {
    return () => h('section', { class: 'border-t border-gray-100 pt-4 first:border-t-0 first:pt-0 dark:border-dark-700' }, [
      h('div', { class: 'mb-3 flex items-center justify-between' }, [
        h('h4', { class: 'text-sm font-bold text-gray-950 dark:text-white' }, props.title),
        h(Icon, { name: 'chevronUp', size: 'xs', class: 'text-gray-400' }),
      ]),
      h('div', { class: 'flex flex-wrap gap-2' }, props.options.map((option) => h('button', {
        key: option.value,
        type: 'button',
        class: [
          'inline-flex max-w-full items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-xs font-semibold transition-colors',
          props.active === option.value
            ? 'border-gray-950 bg-gray-950 text-white dark:border-white dark:bg-white dark:text-gray-950'
            : 'border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700',
        ],
        onClick: () => emit('select', option.value),
      }, [
        h('span', { class: 'truncate' }, option.label),
        option.count !== undefined
          ? h('span', {
            class: [
              'rounded-md px-1.5 py-0.5 text-[10px]',
              props.active === option.value
                ? 'bg-white/15 text-current'
                : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-gray-400',
            ],
          }, String(option.count))
          : null,
      ]))),
    ])
  },
})

const MetricCell = defineComponent({
  name: 'MetricCell',
  props: {
    label: { type: String, required: true },
    value: { type: String, required: true },
    suffix: { type: String, default: '' },
  },
  setup(props) {
    return () => h('div', { class: 'rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-900/40' }, [
      h('div', { class: 'truncate text-[11px] text-gray-500 dark:text-gray-400' }, props.label),
      h('div', { class: 'mt-1 truncate font-mono text-sm font-semibold text-gray-950 dark:text-white' }, [
        props.value,
        props.suffix ? h('span', { class: 'text-xs font-normal text-gray-400' }, props.suffix) : null,
      ]),
    ])
  },
})
</script>
