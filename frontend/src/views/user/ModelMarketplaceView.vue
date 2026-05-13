<template>
  <AppLayout hide-sidebar content-class="p-0">
    <div class="min-h-[calc(100vh-4rem)] bg-gradient-to-b from-primary-50/45 via-white to-white px-4 py-8 dark:from-dark-900 dark:via-dark-950 dark:to-dark-950 md:px-8 lg:px-10">
      <div class="mx-auto max-w-[1500px] space-y-8">
        <section class="mx-auto max-w-3xl pt-2 text-center md:pt-6">
          <h1 class="text-4xl font-black text-gray-950 dark:text-white md:text-5xl">
            {{ t('modelMarketplaceStatus.squareHeading', 'Model Square') }}
          </h1>
          <p class="mx-auto mt-4 max-w-2xl text-sm leading-6 text-gray-500 dark:text-gray-400">
            {{ t('modelMarketplaceStatus.description') }}
            {{ t('modelMarketplaceStatus.totalModels', { count: modelCards.length }, `${modelCards.length} models`) }}
          </p>

          <div class="relative mx-auto mt-7 w-full max-w-2xl">
            <Icon
              name="search"
              size="md"
              class="pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
            />
            <input
              ref="searchInputRef"
              v-model="searchQuery"
              type="text"
              :placeholder="t('modelMarketplaceStatus.searchPlaceholderFull', 'Search model name, provider, group, endpoint or tag...')"
              class="input h-12 rounded-xl pl-11 pr-16"
            />
            <kbd class="pointer-events-none absolute right-4 top-1/2 hidden -translate-y-1/2 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 text-[11px] font-semibold text-gray-400 dark:border-dark-700 dark:bg-dark-900/70 sm:inline-flex">
              ⌘K
            </kbd>
          </div>
        </section>

        <div class="grid gap-5 lg:grid-cols-[280px_minmax(0,1fr)]">
          <aside class="space-y-4 rounded-lg border border-gray-200/80 bg-white/90 p-4 shadow-sm backdrop-blur dark:border-dark-700/70 dark:bg-dark-900/80 lg:sticky lg:top-24 lg:max-h-[calc(100vh-7rem)] lg:overflow-y-auto">
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
          <div class="flex flex-col gap-3 rounded-lg border border-gray-200/80 bg-white/90 p-3 shadow-sm dark:border-dark-700/70 dark:bg-dark-900/80 2xl:flex-row 2xl:items-center 2xl:justify-between">
            <div class="flex min-w-0 items-center gap-3">
              <div class="grid h-9 w-9 place-items-center rounded-lg bg-gray-950 text-sm font-semibold text-white dark:bg-white dark:text-gray-950">
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
            :title="emptyStateTitle"
            :description="emptyStateDescription"
          />

          <div v-else class="grid grid-cols-1 gap-4 xl:grid-cols-2 2xl:grid-cols-3">
            <article
              v-for="card in pagedModelCards"
              :key="card.key"
              class="group flex h-[310px] min-w-0 flex-col overflow-hidden rounded-lg border border-gray-200/80 bg-white p-4 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-md dark:border-dark-700/70 dark:bg-dark-900/80 dark:hover:border-primary-500/30"
            >
              <div class="flex min-h-[74px] items-start justify-between gap-3">
                <div class="flex min-w-0 items-start gap-3">
                  <span
                    class="grid h-10 w-10 flex-shrink-0 place-items-center rounded-lg ring-1 ring-black/5 dark:ring-white/10"
                    :class="[providerGradient(card.provider), providerTintClass(card.provider)]"
                  >
                    <ProviderIcon :provider="card.provider" :size="20" />
                  </span>

                  <div class="min-w-0">
                    <h3 v-if="card.displayName" class="truncate text-base font-bold text-gray-950 dark:text-white">
                      {{ card.displayName }}
                    </h3>
                    <div class="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
                      <span class="font-mono">{{ card.model }}</span>
                      <span class="text-gray-300 dark:text-dark-600">|</span>
                      <span>{{ channelSummary(card) }}</span>
                    </div>
                    <div v-if="modelPriceItems(card).length" class="mt-2 grid grid-cols-2 gap-1.5">
                      <div
                        v-for="item in modelPriceItems(card)"
                        :key="`${card.key}:price:${item.label}`"
                        class="min-w-0 rounded-lg bg-gray-50 px-2 py-1.5 ring-1 ring-gray-200/80 dark:bg-dark-950/45 dark:ring-dark-700"
                        :class="{ 'col-span-2': modelPriceItems(card).length === 1 }"
                      >
                        <div class="truncate text-[10px] font-medium text-gray-400 dark:text-gray-500">
                          {{ item.label }}
                        </div>
                        <div class="truncate font-mono text-xs font-bold text-gray-900 dark:text-gray-100">
                          {{ item.value }}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>

                <div class="flex flex-shrink-0 items-center gap-2">
                  <button
                    type="button"
                    class="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-semibold text-gray-600 hover:bg-gray-50 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-dark-700"
                    @click="openDetail(card)"
                  >
                    {{ t('modelMarketplaceStatus.detailButton', 'Details') }}
                    <Icon name="chevronRight" size="xs" />
                  </button>
                  <button
                    type="button"
                    class="grid h-8 w-8 place-items-center rounded-lg border border-gray-200 text-gray-500 hover:bg-gray-50 hover:text-gray-900 dark:border-dark-700 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-white"
                    :title="t('modelMarketplaceStatus.copyCallModel', 'Copy call model')"
                    @click="copyModel(card.callModel)"
                  >
                    <Icon name="copy" size="sm" />
                  </button>
                </div>
              </div>

              <div
                class="-mx-4 -mb-4 mt-auto flex h-[168px] flex-col border-t border-gray-100 bg-gray-50/80 px-4 py-3 dark:border-dark-800 dark:bg-dark-900/70"
              >
                <div class="mb-2 flex h-5 shrink-0 items-center justify-between gap-3">
                  <span class="text-xs font-bold text-gray-700 dark:text-gray-200">
                    {{ t('modelMarketplaceStatus.channels.title', 'Channels') }}
                  </span>
                </div>
                <div class="space-y-1.5">
                  <div
                    v-for="channel in visibleCardChannels(card)"
                    :key="`${card.key}:${channel.key}:channel`"
                    class="rounded-lg border border-gray-200/70 bg-white/80 p-2 dark:border-dark-700/70 dark:bg-dark-950/40"
                    :title="healthTimelineTitle(channel, 32)"
                  >
                    <div class="mb-1.5 flex min-w-0 items-center justify-between gap-2">
                      <div class="flex min-w-0 items-center gap-2">
                        <span class="grid h-6 w-6 shrink-0 place-items-center rounded-md bg-gray-100 dark:bg-dark-800" :class="providerTintClass(channel.provider)">
                          <ProviderIcon :provider="channel.provider" :size="14" />
                        </span>
                        <div class="min-w-0 truncate text-xs font-semibold text-gray-800 dark:text-gray-100">
                          {{ channel.channelName }}
                        </div>
                      </div>
                      <div class="flex shrink-0 items-center gap-1.5">
                        <span class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] text-gray-500 dark:bg-dark-800 dark:text-gray-300">
                          ×{{ formatRate(channel.effectiveRate ?? 1) }}
                        </span>
                        <span class="rounded-md px-1.5 py-0.5 text-[10px] font-semibold" :class="statusBadgeClass(channel.status)">
                          {{ statusLabel(channel.status) }}
                        </span>
                      </div>
                    </div>
                    <div
                      class="grid h-2 w-full min-w-0 items-end gap-[1px] overflow-hidden"
                      :style="{ gridTemplateColumns: `repeat(${healthTimelineBars(channel, 32).length}, minmax(0, 1fr))` }"
                    >
                      <span
                        v-for="(bar, idx) in healthTimelineBars(channel, 32)"
                        :key="`${channel.key}:health:${idx}`"
                        class="block w-full min-w-0 rounded-sm"
                        :class="bar.colorClass"
                        :style="{ height: bar.heightPct + '%' }"
                        :title="bar.title"
                      ></span>
                    </div>
                  </div>
                  <button
                    v-if="hiddenChannelCount(card) > 0"
                    type="button"
                    class="flex h-9 w-full items-center justify-center gap-1 rounded-lg border border-dashed border-gray-300 bg-white/60 px-3 py-2 text-xs font-semibold text-gray-500 hover:bg-white hover:text-gray-900 dark:border-dark-700 dark:bg-dark-950/30 dark:text-gray-400 dark:hover:bg-dark-900 dark:hover:text-white"
                    @click="openDetail(card)"
                  >
                    {{ t('modelMarketplaceStatus.channels.more', { count: hiddenChannelCount(card) }, `+${hiddenChannelCount(card)} more channels`) }}
                    <Icon name="chevronRight" size="xs" />
                  </button>
                </div>
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
                <option :value="12">{{ t('pagination.perPage') }} 12</option>
                <option :value="24">{{ t('pagination.perPage') }} 24</option>
                <option :value="48">{{ t('pagination.perPage') }} 48</option>
                <option :value="96">{{ t('pagination.perPage') }} 96</option>
              </select>
              <button
                type="button"
                class="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-3 py-2 text-sm font-semibold text-gray-600 hover:bg-gray-50 disabled:opacity-50 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-dark-700"
                :disabled="safeCurrentPage <= 1"
                @click="currentPage = safeCurrentPage - 1"
              >
                <Icon name="chevronLeft" size="xs" />
                {{ t('pagination.previous', 'Previous') }}
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
                {{ t('pagination.next', 'Next') }}
                <Icon name="chevronRight" size="xs" />
              </button>
            </div>
          </div>
        </section>
      </div>
    </div>
    </div>

    <ModelMarketplaceDetailDialog
      :show="showDetail"
      :monitor-id="detailTarget?.monitorId ?? null"
      :title="detailTitle"
      :channels="detailChannels"
      :price-unit="priceUnit"
      @close="closeDetail"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  list as listModelMarketplaceViews,
  type ModelMarketplaceTimelinePoint,
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

interface MarketplaceModelChannel {
  key: string
  monitor: UserModelMarketplaceView
  monitorId: number
  channelName: string
  model: string
  callModel: string
  configuredRequestUrl: string
  requestUrl: string
  displayName: string
  displayNameZh: string
  displayNameEn: string
  provider: Provider
  vendor: string
  groupName: string
  status: MonitorStatus | ''
  health: 'available' | 'degraded' | 'unavailable' | 'unknown'
  latencyMs: number | null
  pingLatencyMs: number | null
  availability7d: number | null
  pricing: UserSupportedModelPricing | null
  effectiveRate: number | null
  billingType: BillingMode | 'unknown'
  endpointTypes: string[]
  tags: string[]
  timeline: ModelMarketplaceTimelinePoint[]
  description: string
  isPrimary: boolean
}

interface MarketplaceModelCard extends MarketplaceModelChannel {
  channels: MarketplaceModelChannel[]
  channelCount: number
}

interface HealthTimelineBar {
  colorClass: string
  heightPct: number
  title: string
}

const { t, locale } = useI18n()
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
const detailTarget = ref<MarketplaceModelCard | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
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
  minimax: 'text-pink-600 dark:text-pink-300',
  zhipu: 'text-cyan-600 dark:text-cyan-300',
  zhipu_v4: 'text-cyan-600 dark:text-cyan-300',
  deepseek: 'text-indigo-600 dark:text-indigo-300',
}

const modelChannels = computed<MarketplaceModelChannel[]>(() => {
  return items.value.flatMap((monitor) => {
    const channels: MarketplaceModelChannel[] = [
      buildModelChannel({
        monitor,
        model: monitor.primary_model,
        callModel: monitor.primary_call_model || monitor.primary_model,
        requestUrl: monitor.primary_request_url || '',
        displayNameZh: monitor.primary_display_name_zh,
        displayNameEn: monitor.primary_display_name_en,
        status: monitor.primary_status,
        latencyMs: monitor.primary_latency_ms,
        pingLatencyMs: monitor.primary_ping_latency_ms,
        availability7d: monitor.availability_7d,
        pricing: monitor.primary_pricing,
        isPrimary: true,
        timeline: monitor.timeline || [],
      }),
    ]
    for (const extra of monitor.extra_models || []) {
      channels.push(extraToChannel(monitor, extra))
    }
    return channels
  })
})

const modelCards = computed<MarketplaceModelCard[]>(() => {
  const groups = new Map<string, MarketplaceModelChannel[]>()
  for (const channel of modelChannels.value) {
    const key = modelAggregateKey(channel)
    const group = groups.get(key)
    if (group) group.push(channel)
    else groups.set(key, [channel])
  }
  return Array.from(groups.entries()).map(([key, channels]) => aggregateModelCard(key, channels))
})

const filteredModelCards = computed(() => filterCards([]))

const sortedModelCards = computed(() => {
  const order = [...filteredModelCards.value]
  if (sortMode.value === 'health') {
    return order.sort((a, b) => healthRank(a) - healthRank(b) || modelSortName(a).localeCompare(modelSortName(b)) || a.model.localeCompare(b.model))
  }
  return order.sort((a, b) => modelSortName(a).localeCompare(modelSortName(b)) || a.model.localeCompare(b.model))
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

const emptyStateTitle = computed(() => {
  return modelCards.value.length === 0
    ? t('modelMarketplaceStatus.empty.title')
    : t('modelMarketplaceStatus.empty.filteredTitle')
})

const emptyStateDescription = computed(() => {
  return modelCards.value.length === 0
    ? t('modelMarketplaceStatus.empty.description')
    : t('modelMarketplaceStatus.empty.filteredDescription')
})

watch(
  [searchQuery, groupFilter, vendorFilter, tagFilter, billingFilter, endpointFilter, healthFilter, pageSize],
  () => {
    currentPage.value = 1
  },
)

const activeFilterSummary = computed(() => {
  const parts = [searchFilterSummary.value,
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

const searchFilterSummary = computed(() => {
  const query = searchQuery.value.trim()
  if (!query) return ''
  return t('modelMarketplaceStatus.searchSummary', { query }, `Search: ${query}`)
})

const groupOptions = computed<FilterOption[]>(() => {
  const values = uniqueSorted(modelCards.value.flatMap((m) => m.channels.map((channel) => channel.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped'))))
  return [
    { value: 'all', label: t('modelMarketplaceStatus.filters.allGroups', 'All Groups'), count: filterCards(['group']).length },
    ...values.map((value) => ({
      value,
      label: value,
      count: filterCards(['group']).filter((m) => cardHasGroup(m, value)).length,
    })),
  ]
})

const vendorOptions = computed<FilterOption[]>(() => {
  const values = uniqueSorted(modelCards.value.flatMap((m) => m.channels.map((channel) => channel.provider)))
  return [
    { value: 'all', label: t('modelMarketplaceStatus.filters.allProviders', 'All Providers'), count: filterCards(['vendor']).length },
    ...values.map((value) => ({
      value,
      label: providerLabel(value as Provider),
      count: filterCards(['vendor']).filter((m) => m.channels.some((channel) => channel.provider === value)).length,
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
      : filterCards(['billing']).filter((m) => m.channels.some((channel) => channel.billingType === option.value)).length,
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
      : filterCards(['health']).filter((m) => m.channels.some((channel) => channel.health === option.value)).length,
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
  return detailTarget.value?.displayName || detailTarget.value?.model || t('modelMarketplaceStatus.detailTitle')
})

const detailChannels = computed(() => detailTarget.value?.channels || [])

const CARD_CHANNEL_LIMIT = 2

function visibleCardChannels(card: MarketplaceModelCard): MarketplaceModelChannel[] {
  return card.channels.slice(0, CARD_CHANNEL_LIMIT)
}

function hiddenChannelCount(card: MarketplaceModelCard): number {
  return Math.max(0, card.channels.length - CARD_CHANNEL_LIMIT)
}

function channelSummary(card: MarketplaceModelCard): string {
  const available = card.channels.filter((channel) => channel.health === 'available').length
  const degraded = card.channels.filter((channel) => channel.health === 'degraded').length
  const unavailable = card.channels.filter((channel) => channel.health === 'unavailable').length
  const parts = [
    t('modelMarketplaceStatus.channels.count', { count: card.channelCount }, `${card.channelCount} channels`),
    available > 0 ? t('modelMarketplaceStatus.channels.availableCount', { count: available }, `${available} available`) : '',
    degraded > 0 ? t('modelMarketplaceStatus.channels.degradedCount', { count: degraded }, `${degraded} degraded`) : '',
    unavailable > 0 ? t('modelMarketplaceStatus.channels.unavailableCount', { count: unavailable }, `${unavailable} unavailable`) : '',
  ].filter(Boolean)
  return parts.join(' · ')
}

function buildModelChannel(input: {
  monitor: UserModelMarketplaceView
  model: string
  callModel?: string
  requestUrl?: string
  displayNameZh?: string
  displayNameEn?: string
  status: MonitorStatus | ''
  latencyMs: number | null
  pingLatencyMs: number | null
  availability7d: number | null
  pricing: UserSupportedModelPricing | null
  isPrimary: boolean
  timeline?: ModelMarketplaceTimelinePoint[]
}): MarketplaceModelChannel {
  const billingType = input.pricing?.billing_mode ?? 'unknown'
  const endpointTypes = inferEndpointTypes(input.monitor.provider, input.model, billingType)
  const tags = inferTags(input.model, input.status, billingType, endpointTypes, input.isPrimary)
  const displayName = localizedConfiguredModelName(input.displayNameZh, input.displayNameEn)
  const callModel = String(input.callModel || input.model).trim()
  const configuredRequestUrl = String(input.requestUrl || '').trim()
  return {
    key: `${input.monitor.id}:${input.model}:${input.isPrimary ? 'primary' : 'extra'}`,
    monitor: input.monitor,
    monitorId: input.monitor.id,
    channelName: input.monitor.name,
    model: input.model,
    callModel,
    configuredRequestUrl,
    requestUrl: resolveRequestUrl(input.monitor.provider, callModel, configuredRequestUrl),
    displayName,
    displayNameZh: input.displayNameZh || '',
    displayNameEn: input.displayNameEn || '',
    provider: input.monitor.provider,
    vendor: providerLabel(input.monitor.provider),
    groupName: input.monitor.group_name,
    status: input.status,
    health: resolveHealth(input.status),
    latencyMs: input.latencyMs,
    pingLatencyMs: input.pingLatencyMs,
    availability7d: input.availability7d,
    pricing: input.pricing,
    effectiveRate: normalizedMonitorEffectiveRate(input.monitor.effective_rate),
    billingType,
    endpointTypes,
    tags,
    timeline: input.timeline || [],
    description: buildDescription(input.monitor, input.model, displayName, input.status, input.isPrimary),
    isPrimary: input.isPrimary,
  }
}

function extraToChannel(monitor: UserModelMarketplaceView, extra: UserModelMarketplaceExtraModel): MarketplaceModelChannel {
  return buildModelChannel({
    monitor,
    model: extra.model,
    callModel: extra.call_model || extra.model,
    requestUrl: extra.request_url || '',
    displayNameZh: extra.display_name_zh,
    displayNameEn: extra.display_name_en,
    status: extra.status,
    latencyMs: extra.latency_ms,
    pingLatencyMs: extra.ping_latency_ms,
    availability7d: extra.availability_7d,
    pricing: extra.pricing,
    isPrimary: false,
    timeline: extra.timeline || [],
  })
}

function modelAggregateKey(channel: MarketplaceModelChannel): string {
  return String(channel.callModel || channel.model || '').trim().toLowerCase() || `${channel.monitorId}:${channel.model}`
}

function aggregateModelCard(key: string, channels: MarketplaceModelChannel[]): MarketplaceModelCard {
  const sortedChannels = [...channels].sort(compareChannels)
  const best = sortedChannels[0]
  const endpointTypes = uniqueSorted(sortedChannels.flatMap((channel) => channel.endpointTypes))
  const tags = uniqueSorted(sortedChannels.flatMap((channel) => channel.tags))
  const groups = uniqueSorted(sortedChannels.map((channel) => channel.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped')))
  return {
    ...best,
    key: `model:${key}`,
    channels: sortedChannels,
    channelCount: sortedChannels.length,
    groupName: best.groupName || groups[0] || '',
    endpointTypes,
    tags,
    description: buildAggregateDescription(best, sortedChannels.length),
  }
}

function compareChannels(a: MarketplaceModelChannel, b: MarketplaceModelChannel): number {
  return healthRank(b) - healthRank(a) ||
    (b.availability7d ?? -1) - (a.availability7d ?? -1) ||
    nullableLatency(a.latencyMs) - nullableLatency(b.latencyMs) ||
    a.channelName.localeCompare(b.channelName)
}

function nullableLatency(value: number | null): number {
  return value == null ? Number.MAX_SAFE_INTEGER : value
}

function filterCards(ignore: FilterKey[]): MarketplaceModelCard[] {
  const q = searchQuery.value.trim().toLowerCase()
  return modelCards.value.filter((card) => {
    if (!ignore.includes('group') && groupFilter.value !== 'all') {
      if (!cardHasGroup(card, groupFilter.value)) return false
    }
    if (!ignore.includes('vendor') && vendorFilter.value !== 'all' && !card.channels.some((channel) => channel.provider === vendorFilter.value)) return false
    if (!ignore.includes('tag') && tagFilter.value !== 'all' && !card.channels.some((channel) => channel.tags.includes(tagFilter.value))) return false
    if (!ignore.includes('billing') && billingFilter.value !== 'all' && !card.channels.some((channel) => channel.billingType === billingFilter.value)) return false
    if (!ignore.includes('endpoint') && endpointFilter.value !== 'all' && !card.channels.some((channel) => channel.endpointTypes.includes(endpointFilter.value))) return false
    if (!ignore.includes('health') && healthFilter.value !== 'all' && !card.channels.some((channel) => channel.health === healthFilter.value)) return false
    if (!q) return true
    return [
      card.model,
      card.callModel,
      card.requestUrl,
      card.displayName,
      card.displayNameZh,
      card.displayNameEn,
      ...card.channels.map((channel) => channel.channelName),
      card.groupName,
      card.vendor,
      card.provider,
      card.billingType,
      ...card.tags,
      ...card.endpointTypes,
      ...card.channels.flatMap((channel) => [channel.model, channel.callModel, channel.requestUrl, channel.groupName, channel.vendor, channel.provider]),
    ].some((value) => String(value || '').toLowerCase().includes(q))
  })
}

function cardHasGroup(card: MarketplaceModelCard, groupName: string): boolean {
  return card.channels.some((channel) => (channel.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped')) === groupName)
}

function localizedConfiguredModelName(zh: string | undefined, en: string | undefined): string {
  const zhName = String(zh || '').trim()
  const enName = String(en || '').trim()
  if (String(locale.value).toLowerCase().startsWith('zh')) {
    return zhName || enName
  }
  return enName
}

function modelSortName(card: MarketplaceModelCard): string {
  return card.displayName || card.model
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

function healthRank(card: MarketplaceModelChannel): number {
  if (card.health === 'unavailable') return 0
  if (card.health === 'degraded') return 1
  if (card.health === 'unknown') return 2
  return 3
}

const HEALTH_TIMELINE_LENGTH = 60

const HEALTH_TIMELINE_HEIGHT: Record<string, number> = {
  operational: 100,
  degraded: 68,
  failed: 36,
  error: 36,
  empty: 18,
}

const HEALTH_TIMELINE_COLOR: Record<string, string> = {
  operational: 'bg-emerald-500',
  degraded: 'bg-amber-500',
  failed: 'bg-red-500',
  error: 'bg-red-500',
  empty: 'bg-gray-300 dark:bg-dark-700',
}

function healthTimelineBars(source: MarketplaceModelChannel, length = HEALTH_TIMELINE_LENGTH): HealthTimelineBar[] {
  const realPoints = (source.timeline || []).length > 0
    ? source.timeline.slice(0, length)
    : fallbackHealthPoint(source)
  const points = [...realPoints].reverse()
  const padCount = Math.max(0, length - points.length)
  const bars: HealthTimelineBar[] = Array.from({ length: padCount }, () => ({
    colorClass: HEALTH_TIMELINE_COLOR.empty,
    heightPct: HEALTH_TIMELINE_HEIGHT.empty,
    title: t('monitorCommon.noData', 'No data'),
  }))

  for (const point of points) {
    const status = point.status || 'empty'
    bars.push({
      colorClass: HEALTH_TIMELINE_COLOR[status] ?? HEALTH_TIMELINE_COLOR.empty,
      heightPct: HEALTH_TIMELINE_HEIGHT[status] ?? HEALTH_TIMELINE_HEIGHT.empty,
      title: healthTimelinePointTitle(point),
    })
  }
  return bars
}

function fallbackHealthPoint(source: MarketplaceModelChannel): ModelMarketplaceTimelinePoint[] {
  if (!source.status) return []
  return [{
    status: source.status,
    latency_ms: source.latencyMs,
    ping_latency_ms: source.pingLatencyMs,
    checked_at: '',
  }]
}

function healthTimelinePointTitle(point: ModelMarketplaceTimelinePoint): string {
  const latency = formatLatency(point.latency_ms)
  const ping = formatLatency(point.ping_latency_ms)
  const checkedAt = point.checked_at ? new Date(point.checked_at).toLocaleString() : t('monitorCommon.now', 'Now')
  return `${checkedAt} · ${statusLabel(point.status)} · ${t('monitorCommon.dialogLatency')}: ${latency}ms · ${t('monitorCommon.endpointPing')}: ${ping}ms`
}

function healthTimelineTitle(card: MarketplaceModelChannel, length = HEALTH_TIMELINE_LENGTH): string {
  return `${t('modelMarketplaceStatus.health.title', 'Health')}: ${statusLabel(card.status)} · ${t('monitorCommon.history60pts', { n: length })} · ${t('modelMarketplaceStatus.availability7d', '7d Availability')} ${formatPercent(card.availability7d)}`
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

function buildDescription(monitor: UserModelMarketplaceView, model: string, displayName: string, status: MonitorStatus | '', isPrimary: boolean): string {
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
      displayName,
      status: statusText,
    },
    `${providerLabel(monitor.provider)} ${role}: ${displayName || model}. Group ${monitor.group_name || '-'}, current health ${statusText}.`,
  )
}

function buildAggregateDescription(best: MarketplaceModelChannel, channelCount: number): string {
  return t(
    'modelMarketplaceStatus.channels.description',
    {
      provider: providerLabel(best.provider),
      group: best.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped'),
      model: best.displayName || best.model,
      status: statusLabel(best.status),
      count: channelCount,
    },
    `${best.displayName || best.model} has ${channelCount} configured channels. Best current channel: ${providerLabel(best.provider)} / ${best.groupName || '-'}, health ${statusLabel(best.status)}.`,
  )
}

function formatRate(rate: number): string {
  return Number(rate.toFixed(4)).toString()
}

function modelPriceItems(card: MarketplaceModelChannel): Array<{ label: string, value: string }> {
  const pricing = card.pricing
  if (!pricing) return []
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST && pricing.per_request_price != null) {
    return [{
      label: t('modelMarketplaceStatus.officialRequestPrice', '官方请求价'),
      value: `${formatScaled(pricing.per_request_price, 1)}${t('availableChannels.pricing.unitPerRequest')}`,
    }]
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE && pricing.image_output_price != null) {
    return [{
      label: t('modelMarketplaceStatus.officialImagePrice', '官方图片价'),
      value: `${formatScaled(pricing.image_output_price, 1)}${t('availableChannels.pricing.unitPerRequest')}`,
    }]
  }
  if (pricing.billing_mode !== BILLING_MODE_TOKEN) return []
  const scale = priceUnit.value === '1M' ? 1_000_000 : 1_000
  const unit = priceUnit.value === '1M'
    ? t('availableChannels.pricing.unitPerMillion')
    : t('modelMarketplaceStatus.unitPerThousandTokens')
  return [
    pricing.input_price == null ? null : {
      label: t('modelMarketplaceStatus.officialInputPrice', '官方输入价'),
      value: `${formatScaled(pricing.input_price, scale)}${unit}`,
    },
    pricing.output_price == null ? null : {
      label: t('modelMarketplaceStatus.officialOutputPrice', '官方输出价'),
      value: `${formatScaled(pricing.output_price, scale)}${unit}`,
    },
  ].filter((item): item is { label: string, value: string } => item != null)
}

function normalizedMonitorEffectiveRate(rate: number | null | undefined): number {
  const n = Number(rate)
  return Number.isFinite(n) && n > 0 ? n : 1
}

function resolveRequestUrl(provider: Provider, callModel: string, configuredUrl: string): string {
  if (configuredUrl) return configuredUrl
  const origin = window.location.origin.replace(/\/+$/, '')
  const model = encodeURIComponent(String(callModel || '').trim())
  if (provider === 'anthropic') return `${origin}/v1/messages`
  if (provider === 'gemini') return model ? `${origin}/v1beta/models/${model}:generateContent` : `${origin}/v1beta/models/{model}:generateContent`
  return `${origin}/v1/chat/completions`
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

function openDetail(card: MarketplaceModelCard) {
  detailTarget.value = card
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

function handleSearchShortcut(event: KeyboardEvent) {
  if (!(event.metaKey || event.ctrlKey) || event.key.toLowerCase() !== 'k') return
  event.preventDefault()
  searchInputRef.value?.focus()
  searchInputRef.value?.select()
}

onMounted(() => {
  void reload(false)
  autoRefresh.setEnabled(true)
  window.addEventListener('keydown', handleSearchShortcut)
})

onBeforeUnmount(() => {
  if (abortController) abortController.abort()
  window.removeEventListener('keydown', handleSearchShortcut)
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

</script>
