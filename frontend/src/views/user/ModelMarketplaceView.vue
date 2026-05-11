<template>
  <AppLayout>
    <MonitorHero
      :overall-status="overallStatus"
      :interval-seconds="DEFAULT_INTERVAL_SECONDS"
      :window="currentWindow"
      :loading="loading"
      i18n-prefix="modelMarketplaceStatus"
      :auto-refresh="autoRefresh"
      @update:window="handleWindowChange"
      @refresh="manualReload"
    />

    <div class="mb-5 flex flex-col justify-between gap-4 lg:flex-row lg:items-center">
      <div class="flex flex-1 flex-wrap items-center gap-3">
        <div class="relative w-full sm:w-80">
          <Icon
            name="search"
            size="md"
            class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
          />
          <input
            v-model="searchQuery"
            type="text"
            :placeholder="t('modelMarketplaceStatus.searchPlaceholder')"
            class="input pl-10"
          />
        </div>

        <div class="flex flex-wrap gap-2">
          <button
            v-for="option in providerOptions"
            :key="option.value || 'all'"
            type="button"
            class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors"
            :class="providerFilter === option.value ? option.activeClass : option.inactiveClass"
            @click="providerFilter = option.value"
          >
            {{ option.label }}
          </button>
        </div>

        <select v-model="statusFilter" class="input w-full sm:w-44">
          <option value="">{{ t('modelMarketplaceStatus.allStatuses') }}</option>
          <option v-for="s in MONITOR_STATUSES" :key="s" :value="s">
            {{ statusLabel(s) }}
          </option>
        </select>
      </div>

      <button
        type="button"
        class="btn btn-secondary"
        :disabled="loading"
        :title="t('common.refresh', 'Refresh')"
        @click="manualReload"
      >
        <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
      </button>
    </div>

    <div
      v-if="loading && modelCards.length === 0"
      class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
    >
      <div
        v-for="i in 8"
        :key="i"
        class="min-h-[250px] rounded-xl border border-gray-200/80 bg-white/70 p-4 animate-pulse dark:border-dark-700/70 dark:bg-dark-800/60"
      >
        <div class="flex items-start gap-3">
          <div class="h-10 w-10 rounded-lg bg-gray-200 dark:bg-dark-700"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-3/4 rounded bg-gray-200 dark:bg-dark-700"></div>
            <div class="h-3 w-1/2 rounded bg-gray-100 dark:bg-dark-700"></div>
          </div>
        </div>
        <div class="mt-5 h-12 rounded-lg bg-gray-100 dark:bg-dark-900/40"></div>
        <div class="mt-4 grid grid-cols-3 gap-2">
          <div class="h-14 rounded-lg bg-gray-100 dark:bg-dark-900/40"></div>
          <div class="h-14 rounded-lg bg-gray-100 dark:bg-dark-900/40"></div>
          <div class="h-14 rounded-lg bg-gray-100 dark:bg-dark-900/40"></div>
        </div>
      </div>
    </div>

    <EmptyState
      v-else-if="filteredModelCards.length === 0"
      :title="t('modelMarketplaceStatus.empty.title')"
      :description="t('modelMarketplaceStatus.empty.description')"
    />

    <div v-else class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      <button
        v-for="card in filteredModelCards"
        :key="card.key"
        type="button"
        class="group flex min-h-[250px] w-full min-w-0 flex-col rounded-xl border border-gray-200/80 bg-white/75 p-4 text-left shadow-card transition-all duration-200 hover:-translate-y-0.5 hover:border-gray-300 hover:shadow-card-hover dark:border-dark-700/70 dark:bg-dark-800/65 dark:hover:border-primary-500/30"
        @click="openDetail(card.monitor)"
      >
        <div class="flex items-start gap-3">
          <span
            class="grid h-10 w-10 flex-shrink-0 place-items-center rounded-lg ring-1 ring-black/5 dark:ring-white/10"
            :class="[providerGradient(card.provider), providerTintClass(card.provider)]"
          >
            <ProviderIcon :provider="card.provider" :size="20" />
          </span>

          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <h3 class="truncate font-mono text-[15px] font-bold text-gray-950 dark:text-gray-50">
                {{ card.model }}
              </h3>
              <span
                v-if="card.isPrimary"
                class="rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
              >
                {{ t('modelMarketplaceStatus.primaryModel') }}
              </span>
            </div>
            <div class="mt-1 flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              <span
                class="inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium"
                :class="providerBadgeClass(card.provider)"
              >
                {{ providerLabel(card.provider) }}
              </span>
              <span class="truncate">{{ card.monitor.name }}</span>
              <span v-if="card.groupName" class="text-gray-400">/</span>
              <span v-if="card.groupName" class="truncate">{{ card.groupName }}</span>
            </div>
          </div>

          <span
            class="flex-shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold"
            :class="statusBadgeClass(card.status)"
          >
            {{ statusLabel(card.status) }}
          </span>
        </div>

        <div class="mt-4 rounded-lg border border-gray-100 bg-gray-50/70 p-3 dark:border-dark-700 dark:bg-dark-900/30">
          <div class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('modelMarketplaceStatus.price') }}
          </div>
          <div v-if="pricePieces(card).length > 0" class="flex flex-wrap gap-x-3 gap-y-1">
            <span
              v-for="piece in pricePieces(card)"
              :key="piece.label"
              class="text-xs text-gray-500 dark:text-gray-400"
            >
              {{ piece.label }}
              <span class="font-mono font-semibold text-gray-900 dark:text-gray-100">
                {{ piece.value }}
              </span>
              <span class="text-gray-400">{{ piece.unit }}</span>
            </span>
          </div>
          <div v-else class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('modelMarketplaceStatus.priceUnavailable') }}
          </div>
        </div>

        <div class="mt-4 grid grid-cols-3 gap-2">
          <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-900/40">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('monitorCommon.dialogLatency') }}</div>
            <div class="mt-1 font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">
              {{ formatLatency(card.latencyMs) }}<span v-if="card.latencyMs != null" class="text-xs font-normal text-gray-400">ms</span>
            </div>
          </div>
          <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-900/40">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ t('monitorCommon.endpointPing') }}</div>
            <div class="mt-1 font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">
              {{ formatLatency(card.pingLatencyMs) }}<span v-if="card.pingLatencyMs != null" class="text-xs font-normal text-gray-400">ms</span>
            </div>
          </div>
          <div class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-900/40">
            <div class="text-[11px] text-gray-500 dark:text-gray-400">{{ availabilityShortLabel }}</div>
            <div class="mt-1 font-mono text-sm font-semibold text-gray-900 dark:text-gray-100">
              {{ formatPercent(resolveCardAvailability(card)) }}
            </div>
          </div>
        </div>
      </button>
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
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  list as listModelMarketplaceViews,
  status as fetchModelMarketplaceDetail,
  type UserModelMarketplaceView,
  type UserModelMarketplaceDetail,
  type UserModelMarketplaceExtraModel,
} from '@/api/modelMarketplace'
import type { Provider, MonitorStatus } from '@/api/admin/modelMarketplaceMonitor'
import type { UserSupportedModelPricing } from '@/api/channels'
import AppLayout from '@/components/layout/AppLayout.vue'
import MonitorHero, {
  type MonitorWindow,
  type OverallStatus,
} from '@/components/user/monitor/MonitorHero.vue'
import ModelMarketplaceDetailDialog from '@/components/user/ModelMarketplaceDetailDialog.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  DEFAULT_INTERVAL_SECONDS,
  MONITOR_STATUSES,
  PROVIDERS,
  STATUS_OPERATIONAL,
} from '@/constants/modelMarketplaceMonitor'
import { BILLING_MODE_IMAGE, BILLING_MODE_PER_REQUEST, BILLING_MODE_TOKEN } from '@/constants/channel'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { useModelMarketplaceMonitorFormat, providerGradient } from '@/composables/useModelMarketplaceMonitorFormat'
import { formatScaled } from '@/utils/pricing'

const { t } = useI18n()
const appStore = useAppStore()
const {
  statusLabel,
  statusBadgeClass,
  providerLabel,
  providerBadgeClass,
  providerPickerClass,
  formatLatency,
  formatPercent,
} = useModelMarketplaceMonitorFormat()

const items = ref<UserModelMarketplaceView[]>([])
const loading = ref(false)
const currentWindow = ref<MonitorWindow>('7d')
const detailCache = reactive<Record<number, UserModelMarketplaceDetail>>({})
const showDetail = ref(false)
const detailTarget = ref<UserModelMarketplaceView | null>(null)
const searchQuery = ref('')
const providerFilter = ref<Provider | ''>('')
const statusFilter = ref<MonitorStatus | ''>('')

let abortController: AbortController | null = null

interface MarketplaceModelCard {
  key: string
  monitor: UserModelMarketplaceView
  monitorId: number
  model: string
  provider: Provider
  groupName: string
  status: MonitorStatus | ''
  latencyMs: number | null
  pingLatencyMs: number | null
  availability7d: number | null
  pricing: UserSupportedModelPricing | null
  isPrimary: boolean
}

const PROVIDER_TINT: Record<string, string> = {
  openai: 'text-emerald-600 dark:text-emerald-300',
  anthropic: 'text-orange-600 dark:text-orange-300',
  gemini: 'text-sky-600 dark:text-sky-300',
}

const autoRefresh = useAutoRefresh({
  storageKey: 'model-marketplace-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: DEFAULT_INTERVAL_SECONDS,
  onRefresh: () => reload(true),
  shouldPause: () => document.hidden || loading.value,
})
const countdown = autoRefresh.countdown

const overallStatus = computed<OverallStatus>(() => {
  if (modelCards.value.length === 0) return 'operational'
  for (const it of modelCards.value) {
    if (it.status === 'failed' || it.status === 'error') return 'degraded'
    if (it.status !== STATUS_OPERATIONAL) return 'degraded'
  }
  return 'operational'
})

const providerOptions = computed(() => [
  {
    value: '' as const,
    label: t('modelMarketplaceStatus.allProviders'),
    activeClass: 'border-gray-400 bg-gray-50 text-gray-800 dark:border-dark-500 dark:bg-dark-700 dark:text-gray-100',
    inactiveClass: 'border-gray-200 bg-white text-gray-600 hover:border-gray-300 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-400',
  },
  ...PROVIDERS.map((p) => ({
    value: p,
    label: providerLabel(p),
    activeClass: providerPickerClass(p, true),
    inactiveClass: providerPickerClass(p, false),
  })),
])

const modelCards = computed<MarketplaceModelCard[]>(() => {
  return items.value.flatMap((monitor) => {
    const cards: MarketplaceModelCard[] = [
      {
        key: `${monitor.id}:${monitor.primary_model}:primary`,
        monitor,
        monitorId: monitor.id,
        model: monitor.primary_model,
        provider: monitor.provider,
        groupName: monitor.group_name,
        status: monitor.primary_status,
        latencyMs: monitor.primary_latency_ms,
        pingLatencyMs: monitor.primary_ping_latency_ms,
        availability7d: monitor.availability_7d,
        pricing: monitor.primary_pricing,
        isPrimary: true,
      },
    ]
    for (const extra of monitor.extra_models || []) {
      cards.push(extraToCard(monitor, extra))
    }
    return cards
  })
})

const filteredModelCards = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  return modelCards.value.filter((card) => {
    if (providerFilter.value && card.provider !== providerFilter.value) return false
    if (statusFilter.value && card.status !== statusFilter.value) return false
    if (!q) return true
    return (
      card.model.toLowerCase().includes(q) ||
      card.monitor.name.toLowerCase().includes(q) ||
      card.groupName.toLowerCase().includes(q) ||
      providerLabel(card.provider).toLowerCase().includes(q)
    )
  })
})

const availabilityShortLabel = computed(() =>
  t(`modelMarketplaceStatus.windowTab.${currentWindow.value}`)
)

const detailTitle = computed(() => {
  return detailTarget.value?.name || t('modelMarketplaceStatus.detailTitle')
})

function extraToCard(
  monitor: UserModelMarketplaceView,
  extra: UserModelMarketplaceExtraModel,
): MarketplaceModelCard {
  return {
    key: `${monitor.id}:${extra.model}:extra`,
    monitor,
    monitorId: monitor.id,
    model: extra.model,
    provider: monitor.provider,
    groupName: monitor.group_name,
    status: extra.status,
    latencyMs: extra.latency_ms,
    pingLatencyMs: extra.ping_latency_ms,
    availability7d: extra.availability_7d,
    pricing: extra.pricing,
    isPrimary: false,
  }
}

function providerTintClass(provider: string): string {
  return PROVIDER_TINT[provider] ?? 'text-gray-500 dark:text-gray-300'
}

function pricePieces(card: MarketplaceModelCard): Array<{ label: string; value: string; unit: string }> {
  const pricing = card.pricing
  if (!pricing) return []
  const perRequestUnit = t('availableChannels.pricing.unitPerRequest')
  const perMillionUnit = t('availableChannels.pricing.unitPerMillion')
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST) {
    return pricing.per_request_price == null
      ? []
      : [{
          label: t('availableChannels.pricing.perRequestPrice'),
          value: formatScaled(pricing.per_request_price, 1),
          unit: perRequestUnit,
        }]
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE) {
    return pricing.image_output_price == null
      ? []
      : [{
          label: t('availableChannels.pricing.imageOutputPrice'),
          value: formatScaled(pricing.image_output_price, 1),
          unit: perRequestUnit,
        }]
  }
  if (pricing.billing_mode !== BILLING_MODE_TOKEN) return []
  return [
    {
      label: t('availableChannels.pricing.inputPrice'),
      value: formatScaled(pricing.input_price, 1_000_000),
      unit: perMillionUnit,
    },
    {
      label: t('availableChannels.pricing.outputPrice'),
      value: formatScaled(pricing.output_price, 1_000_000),
      unit: perMillionUnit,
    },
    ...(pricing.cache_read_price != null
      ? [{
          label: t('availableChannels.pricing.cacheReadPrice'),
          value: formatScaled(pricing.cache_read_price, 1_000_000),
          unit: perMillionUnit,
        }]
      : []),
  ].filter((piece) => piece.value !== '-')
}

function resolveCardAvailability(card: MarketplaceModelCard): number | null {
  if (currentWindow.value === '7d') return card.availability7d ?? null
  const detail = detailCache[card.monitorId]
  if (!detail) return null
  const model = detail.models.find((m) => m.model === card.model)
  if (!model) return null
  return currentWindow.value === '15d' ? model.availability_15d : model.availability_30d
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
    await ensureDetailsForWindow(true)
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

async function loadDetail(id: number, force = false) {
  if (!force && detailCache[id]) return
  try {
    detailCache[id] = await fetchModelMarketplaceDetail(id)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('modelMarketplaceStatus.detailLoadError')))
  }
}

async function ensureDetailsForWindow(force = false) {
  if (currentWindow.value === '7d') return
  await Promise.all(items.value.map(it => loadDetail(it.id, force)))
}

async function handleWindowChange(value: MonitorWindow) {
  currentWindow.value = value
  await ensureDetailsForWindow()
}

function openDetail(row: UserModelMarketplaceView | unknown) {
  detailTarget.value = row as UserModelMarketplaceView
  showDetail.value = true
}

function closeDetail() {
  showDetail.value = false
  detailTarget.value = null
}

onMounted(() => {
  void reload(false)
  autoRefresh.setEnabled(true)
})

onBeforeUnmount(() => {
  if (abortController) abortController.abort()
})
</script>
