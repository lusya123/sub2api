<template>
  <Teleport to="body">
    <Transition name="marketplace-drawer">
      <div
        v-if="show"
        class="fixed inset-0 z-[60] bg-black/40 backdrop-blur-sm"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
        @click.self="emit('close')"
      >
        <aside
          ref="drawerRef"
          class="ml-auto flex h-full w-full max-w-[860px] flex-col border-l border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-900"
          tabindex="-1"
          @click.stop
        >
          <header class="flex shrink-0 items-start justify-between gap-4 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
            <div class="min-w-0">
              <p class="text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">
                {{ t('modelMarketplaceStatus.detailTitle', 'Model detail') }}
              </p>
              <h2 class="mt-1 truncate text-xl font-bold text-gray-950 dark:text-white">
                {{ title }}
              </h2>
              <p v-if="channels.length > 0" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('modelMarketplaceStatus.channels.count', { count: channels.length }, `${channels.length} channels`) }}
              </p>
            </div>
            <button
              type="button"
              class="grid h-10 w-10 shrink-0 place-items-center rounded-xl border border-gray-200 text-gray-500 transition hover:bg-gray-50 hover:text-gray-900 dark:border-dark-700 dark:text-gray-400 dark:hover:bg-dark-800 dark:hover:text-white"
              :aria-label="t('modelMarketplaceStatus.closeDetail')"
              @click="emit('close')"
            >
              <Icon name="x" size="md" />
            </button>
          </header>

          <div class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
            <div v-if="loading" class="py-10 text-center text-sm text-gray-500">
              {{ t('common.loading') }}
            </div>

            <div v-else-if="channels.length > 0" class="space-y-4">
              <section class="rounded-xl border border-gray-200 bg-gray-50/70 p-4 dark:border-dark-700 dark:bg-dark-950/35">
                <div class="grid gap-3 md:grid-cols-2">
                  <div class="min-w-0 rounded-lg bg-white p-3 ring-1 ring-gray-200/80 dark:bg-dark-900/70 dark:ring-dark-700">
                    <div class="mb-1 text-xs font-semibold text-gray-400">
                      {{ t('modelMarketplaceStatus.callModel', 'Call model') }}
                    </div>
                    <code class="block truncate font-mono text-sm font-bold text-gray-900 dark:text-gray-100" :title="modelCallModel">
                      {{ modelCallModel }}
                    </code>
                  </div>

                  <div class="min-w-0 rounded-lg bg-white p-3 ring-1 ring-gray-200/80 dark:bg-dark-900/70 dark:ring-dark-700">
                    <div class="mb-1 text-xs font-semibold text-gray-400">
                      {{ t('modelMarketplaceStatus.requestUrl', 'Request URL') }}
                    </div>
                    <code class="block truncate font-mono text-sm font-bold text-gray-900 dark:text-gray-100" :title="modelRequestUrl">
                      {{ modelRequestUrl }}
                    </code>
                  </div>

                  <div class="min-w-0 rounded-lg bg-white p-3 ring-1 ring-gray-200/80 dark:bg-dark-900/70 dark:ring-dark-700">
                    <div class="mb-1 text-xs font-semibold text-gray-400">
                      {{ t('modelMarketplaceStatus.originalPrice', 'Original') }}
                    </div>
                    <div v-if="modelOriginalPriceLines.length > 0" class="flex flex-wrap gap-x-4 gap-y-1">
                      <span
                        v-for="line in modelOriginalPriceLines"
                        :key="`model-original:${line}`"
                        class="font-mono text-sm font-bold text-gray-900 dark:text-gray-100"
                      >
                        {{ line }}
                      </span>
                    </div>
                    <span v-else class="text-sm text-gray-400">{{ t('modelMarketplaceStatus.priceUnavailable') }}</span>
                  </div>

                  <div class="min-w-0 rounded-lg bg-white p-3 ring-1 ring-gray-200/80 dark:bg-dark-900/70 dark:ring-dark-700">
                    <div class="mb-1 text-xs font-semibold text-gray-400">
                      {{ t('modelMarketplaceStatus.exchangeDiscount', 'Exchange discount') }}
                    </div>
                    <div class="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                      <span class="font-mono text-sm font-bold text-gray-900 dark:text-gray-100">
                        {{ exchangeDiscountLabel }}
                      </span>
                      <span class="text-xs text-gray-400">
                        {{ t('modelMarketplaceStatus.exchangeRateHint', { rate: formatRate(usdCnyRate) }, `$1≈¥${formatRate(usdCnyRate)}`) }}
                      </span>
                    </div>
                  </div>
                </div>
              </section>

              <section class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-950/30">
                <div class="hidden grid-cols-[minmax(150px,1.2fr)_minmax(160px,1fr)_88px_minmax(180px,1.2fr)_104px] gap-3 border-b border-gray-200 bg-gray-50 px-4 py-2 text-xs font-semibold text-gray-500 dark:border-dark-700 dark:bg-dark-900/70 dark:text-gray-400 md:grid">
                  <div>{{ t('modelMarketplaceStatus.channel', 'Channel') }}</div>
                  <div>{{ t('modelMarketplaceStatus.channels.title', 'Channel health') }}</div>
                  <div>{{ t('modelMarketplaceStatus.discount', 'Discount') }}</div>
                  <div>{{ t('modelMarketplaceStatus.channelFinalPrice', 'Channel price') }}</div>
                  <div>{{ t('modelMarketplaceStatus.availability7d', '7d') }}</div>
                </div>

                <div
                  v-for="channel in channels"
                  :key="channel.key"
                  class="grid gap-3 border-b border-gray-100 px-4 py-3 last:border-b-0 dark:border-dark-800 md:grid-cols-[minmax(150px,1.2fr)_minmax(160px,1fr)_88px_minmax(180px,1.2fr)_104px] md:items-center"
                >
                  <div class="min-w-0">
                    <div class="flex min-w-0 items-center gap-2">
                      <h3 class="truncate text-sm font-bold text-gray-950 dark:text-white">
                        {{ channel.channelName }}
                      </h3>
                      <span class="shrink-0 rounded-md px-1.5 py-0.5 text-[10px] font-semibold" :class="statusBadgeClass(channel.status)">
                        {{ statusLabel(channel.status) }}
                      </span>
                    </div>
                    <div class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                      {{ channel.groupName || t('modelMarketplaceStatus.ungrouped', 'Ungrouped') }} · {{ providerLabel(channel.provider) }}
                    </div>
                  </div>

                  <div class="min-w-0">
                    <div class="mb-1 flex items-center justify-between gap-2 text-xs text-gray-400 md:hidden">
                      <span>{{ t('modelMarketplaceStatus.channels.title', 'Channel health') }}</span>
                      <span>{{ formatPercent(channel.availability7d) }}</span>
                    </div>
                    <div
                      class="grid h-3 w-full min-w-0 items-end gap-[2px] overflow-hidden"
                      :style="{ gridTemplateColumns: `repeat(${healthTimelineBars(channel).length}, minmax(0, 1fr))` }"
                    >
                      <span
                        v-for="(bar, idx) in healthTimelineBars(channel)"
                        :key="`${channel.key}:detail-health:${idx}`"
                        class="block w-full min-w-0 rounded-sm"
                        :class="bar.colorClass"
                        :style="{ height: bar.heightPct + '%' }"
                        :title="bar.title"
                      ></span>
                    </div>
                    <div class="mt-1 truncate text-[11px] text-gray-400">
                      {{ t('monitorCommon.endpointPing') }} {{ formatLatency(channel.pingLatencyMs) }}
                    </div>
                  </div>

                  <div>
                    <div class="mb-1 text-xs font-semibold text-gray-400 md:hidden">
                      {{ t('modelMarketplaceStatus.discount', 'Discount') }}
                    </div>
                    <div class="font-mono text-sm font-bold text-gray-900 dark:text-gray-100">
                      ×{{ formatRate(channel.effectiveRate ?? 1) }}
                    </div>
                    <div class="mt-0.5 text-[11px] font-semibold text-gray-500 dark:text-gray-400">
                      {{ t('modelMarketplaceStatus.rateDiscountValue', { discount: discountFold(channel.effectiveRate ?? 1) }, `Rate ${discountFold(channel.effectiveRate ?? 1)}/10`) }}
                    </div>
                    <div class="mt-0.5 text-[11px] font-semibold text-emerald-600 dark:text-emerald-300">
                      {{ t('modelMarketplaceStatus.finalDiscountValue', { discount: finalDiscountFold(channel.effectiveRate ?? 1) }, `Final ${finalDiscountFold(channel.effectiveRate ?? 1)}/10`) }}
                    </div>
                  </div>

                  <div class="min-w-0">
                    <div class="mb-1 flex items-center justify-between gap-2 text-xs md:hidden">
                      <span class="font-semibold text-gray-400">{{ t('modelMarketplaceStatus.channelFinalPrice', 'Channel price') }}</span>
                      <span v-if="finalSavingLabel(channel.effectiveRate ?? 1)" class="text-emerald-600 dark:text-emerald-300">
                        {{ finalSavingLabel(channel.effectiveRate ?? 1) }}
                      </span>
                    </div>
                    <div v-if="priceSummary(channel.pricing, channelFinalRate(channel.effectiveRate)).length > 0" class="space-y-0.5">
                      <div
                        v-for="line in priceSummary(channel.pricing, channelFinalRate(channel.effectiveRate))"
                        :key="`rated:${channel.key}:${line}`"
                        class="truncate font-mono text-xs font-bold text-emerald-700 dark:text-emerald-300"
                        :title="line"
                      >
                        {{ line }}
                      </div>
                    </div>
                    <div v-else class="text-xs text-gray-400">
                      {{ t('modelMarketplaceStatus.priceUnavailable') }}
                    </div>
                    <div v-if="finalSavingLabel(channel.effectiveRate ?? 1)" class="mt-0.5 hidden text-[11px] text-emerald-600 dark:text-emerald-300 md:block">
                      {{ finalSavingLabel(channel.effectiveRate ?? 1) }}
                    </div>
                  </div>

                  <div>
                    <div class="mb-1 text-xs font-semibold text-gray-400 md:hidden">
                      {{ t('modelMarketplaceStatus.availability7d', '7d') }}
                    </div>
                    <div class="font-mono text-sm font-bold text-gray-900 dark:text-gray-100">
                      {{ formatPercent(channel.availability7d) }}
                    </div>
                    <div class="mt-0.5 text-[11px] text-gray-400">
                      {{ t('monitorCommon.dialogLatency') }} {{ formatLatency(channel.latencyMs) }}
                    </div>
                  </div>
                </div>
              </section>
            </div>

            <div v-else-if="detail" class="space-y-3">
              <section
                v-for="m in detail.models"
                :key="m.model"
                class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-950/30"
              >
                <div v-if="localizedConfiguredModelName(m.display_name_zh, m.display_name_en)" class="font-semibold text-gray-900 dark:text-gray-100">
                  {{ localizedConfiguredModelName(m.display_name_zh, m.display_name_en) }}
                </div>
                <code class="mt-1 block truncate font-mono text-xs text-gray-500 dark:text-gray-400">
                  {{ m.model }}
                </code>
                <div class="mt-3 grid grid-cols-3 gap-2 text-xs">
                  <div class="rounded-lg bg-gray-50 p-2 dark:bg-dark-900/80">
                    <div class="text-gray-400">{{ t('modelMarketplaceStatus.detailColumns.latestStatus') }}</div>
                    <span class="mt-1 inline-flex rounded-md px-2 py-0.5 font-semibold" :class="statusBadgeClass(m.latest_status)">
                      {{ statusLabel(m.latest_status) }}
                    </span>
                  </div>
                  <div class="rounded-lg bg-gray-50 p-2 dark:bg-dark-900/80">
                    <div class="text-gray-400">{{ t('modelMarketplaceStatus.detailColumns.latestLatency') }}</div>
                    <div class="font-mono font-bold text-gray-900 dark:text-gray-100">{{ formatLatency(m.latest_latency_ms) }}</div>
                  </div>
                  <div class="rounded-lg bg-gray-50 p-2 dark:bg-dark-900/80">
                    <div class="text-gray-400">{{ t('modelMarketplaceStatus.detailColumns.availability7d') }}</div>
                    <div class="font-mono font-bold text-gray-900 dark:text-gray-100">{{ formatPercent(m.availability_7d) }}</div>
                  </div>
                </div>
                <div class="mt-3 rounded-lg bg-gray-50 p-3 text-xs dark:bg-dark-900/80">
                  <div v-if="priceSummary(m.pricing, 1).length > 0" class="space-y-0.5 font-mono text-gray-700 dark:text-gray-200">
                    <div v-for="line in priceSummary(m.pricing, 1)" :key="line">{{ line }}</div>
                  </div>
                  <span v-else class="text-gray-400">{{ t('modelMarketplaceStatus.priceUnavailable') }}</span>
                </div>
                <div class="mt-3 space-y-1 rounded-lg bg-gray-50 px-3 py-2 text-xs dark:bg-dark-900/80">
                  <div class="truncate" :title="m.call_model || m.model">
                    <span class="text-gray-400">{{ t('modelMarketplaceStatus.callModel', 'Call model') }}:</span>
                    <code class="ml-1 font-mono text-gray-700 dark:text-gray-200">{{ m.call_model || m.model }}</code>
                  </div>
                  <div class="truncate" :title="requestUrlFor(m)">
                    <span class="text-gray-400">{{ t('modelMarketplaceStatus.requestUrl', 'Request URL') }}:</span>
                    <code class="ml-1 font-mono text-gray-700 dark:text-gray-200">{{ requestUrlFor(m) }}</code>
                  </div>
                </div>
              </section>
            </div>

            <div v-else class="py-10 text-center text-sm text-gray-500">
              {{ t('modelMarketplaceStatus.detailLoadError') }}
            </div>
          </div>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  exchangeRate as fetchExchangeRate,
  status as fetchModelMarketplaceDetail,
  type ModelMarketplaceTimelinePoint,
  type UserModelMarketplaceDetail,
  type UserModelMarketplaceModelDetail,
} from '@/api/modelMarketplace'
import type { UserSupportedModelPricing } from '@/api/channels'
import type { MonitorStatus, Provider } from '@/api/admin/modelMarketplaceMonitor'
import Icon from '@/components/icons/Icon.vue'
import { useModelMarketplaceMonitorFormat } from '@/composables/useModelMarketplaceMonitorFormat'
import {
  buildModelMarketplacePriceSummary,
  modelMarketplaceChannelFinalRate,
} from '@/utils/modelMarketplacePricing'

type PriceUnit = '1M' | '1K'

interface DetailChannel {
  key: string
  channelName: string
  model: string
  callModel: string
  requestUrl: string
  provider: Provider
  groupName: string
  status: MonitorStatus | ''
  latencyMs: number | null
  pingLatencyMs: number | null
  availability7d: number | null
  pricing: UserSupportedModelPricing | null
  effectiveRate: number | null
  timeline: ModelMarketplaceTimelinePoint[]
}

const props = defineProps<{
  show: boolean
  monitorId: number | null
  title: string
  channels?: DetailChannel[]
  priceUnit?: PriceUnit
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t, locale } = useI18n()
const appStore = useAppStore()
const { statusLabel, statusBadgeClass, providerLabel, formatLatency, formatPercent } = useModelMarketplaceMonitorFormat()

const detail = ref<UserModelMarketplaceDetail | null>(null)
const loading = ref(false)
const channels = computed(() => props.channels || [])
const primaryChannel = computed(() => channels.value[0] || null)
const titleFallback = computed(() => props.title || t('modelMarketplaceStatus.detailTitle', 'Model detail'))
const usdCnyRate = ref(Math.max(1, Number(import.meta.env.VITE_MODEL_MARKETPLACE_USD_CNY_RATE || 7.2) || 7.2))
const exchangeDiscountRate = computed(() => 1 / usdCnyRate.value)
const exchangeDiscountLabel = computed(() => {
  return t(
    'modelMarketplaceStatus.exchangeDiscountValue',
    { discount: discountFold(exchangeDiscountRate.value) },
    `${discountFold(exchangeDiscountRate.value)}/10 exchange price`,
  )
})
const modelCallModel = computed(() => primaryChannel.value?.callModel || primaryChannel.value?.model || titleFallback.value)
const modelRequestUrl = computed(() => primaryChannel.value?.requestUrl || '-')
const modelOriginalPriceLines = computed(() => priceSummary(primaryChannel.value?.pricing || null, 1))
const drawerRef = ref<HTMLElement | null>(null)

function handleEscape(event: KeyboardEvent) {
  if (props.show && event.key === 'Escape') {
    emit('close')
  }
}

async function load(id: number) {
  detail.value = null
  loading.value = true
  try {
    detail.value = await fetchModelMarketplaceDetail(id)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('modelMarketplaceStatus.detailLoadError')))
  } finally {
    loading.value = false
  }
}

async function loadExchangeRate() {
  try {
    const res = await fetchExchangeRate()
    if (Number.isFinite(res.rate) && res.rate > 0) {
      usdCnyRate.value = res.rate
    }
  } catch {
    // Keep the local fallback rate when the public FX service or backend cache is unavailable.
  }
}

function priceSummary(pricing: UserSupportedModelPricing | null, rate: number): string[] {
  return buildModelMarketplacePriceSummary(pricing, rate, props.priceUnit || '1M', {
    perRequestUnit: t('availableChannels.pricing.unitPerRequest'),
    unitPerThousandTokens: t('modelMarketplaceStatus.unitPerThousandTokens'),
    unitPerMillionTokens: t('availableChannels.pricing.unitPerMillion'),
    inputPrice: t('availableChannels.pricing.inputPrice'),
    outputPrice: t('availableChannels.pricing.outputPrice'),
    cacheReadPrice: t('availableChannels.pricing.cacheReadPrice'),
  })
}

const HEALTH_TIMELINE_COLOR: Record<string, string> = {
  operational: 'bg-emerald-500',
  degraded: 'bg-amber-500',
  failed: 'bg-red-500',
  error: 'bg-red-500',
  empty: 'bg-gray-300 dark:bg-dark-700',
}

const HEALTH_TIMELINE_HEIGHT: Record<string, number> = {
  operational: 100,
  degraded: 68,
  failed: 36,
  error: 36,
  empty: 18,
}

function healthTimelineBars(channel: DetailChannel) {
  const points = channel.timeline?.slice(0, 48).reverse() || []
  return points.map((point) => {
    const status = point.status || 'empty'
    return {
      colorClass: HEALTH_TIMELINE_COLOR[status] || HEALTH_TIMELINE_COLOR.empty,
      heightPct: HEALTH_TIMELINE_HEIGHT[status] || HEALTH_TIMELINE_HEIGHT.empty,
      title: `${point.checked_at || '-'} · ${statusLabel(point.status)}`,
    }
  })
}

function formatRate(rate: number): string {
  if (!Number.isFinite(rate) || rate <= 0) return '1'
  if (rate < 0.0001) return '<0.0001'
  return Number(rate.toFixed(4)).toString()
}

function discountFold(rate: number): string {
  if (!Number.isFinite(rate) || rate <= 0) return '-'
  return Number((rate * 10).toFixed(2)).toString()
}

function finalDiscountFold(rate: number): string {
  return discountFold(channelFinalRate(rate))
}

function finalSavingLabel(rate: number): string {
  const finalRate = channelFinalRate(rate)
  if (!Number.isFinite(finalRate) || finalRate >= 1) return ''
  const saving = Number(((1 - finalRate) * 100).toFixed(1)).toString()
  return t('modelMarketplaceStatus.officialSavingValue', { saving }, `Save ${saving}%`)
}

function channelFinalRate(rate: number | null | undefined): number {
  return modelMarketplaceChannelFinalRate(rate, exchangeDiscountRate.value)
}

function localizedConfiguredModelName(zh: string | undefined, en: string | undefined): string {
  const zhName = String(zh || '').trim()
  const enName = String(en || '').trim()
  if (String(locale.value).toLowerCase().startsWith('zh')) {
    return zhName || enName
  }
  return enName
}

function requestUrlFor(m: UserModelMarketplaceModelDetail): string {
  if (m.request_url) return m.request_url
  const origin = window.location.origin.replace(/\/+$/, '')
  const callModel = encodeURIComponent(String(m.call_model || m.model || '').trim())
  const provider = detail.value?.provider || ''
  if (provider === 'anthropic') return `${origin}/v1/messages`
  if (provider === 'gemini') return callModel ? `${origin}/v1beta/models/${callModel}:generateContent` : `${origin}/v1beta/models/{model}:generateContent`
  return `${origin}/v1/chat/completions`
}

watch(
  () => [props.show, props.monitorId] as const,
  async ([show, id]) => {
    if (!show) {
      detail.value = null
      document.body.classList.remove('modal-open')
      return
    }
    document.body.classList.add('modal-open')
    void loadExchangeRate()
    await nextTick()
    drawerRef.value?.focus()
    if (props.channels?.length) {
      detail.value = null
      loading.value = false
      return
    }
    if (id != null) void load(id)
  },
  { immediate: true },
)

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
  document.body.classList.remove('modal-open')
})
</script>

<style scoped>
.marketplace-drawer-enter-active,
.marketplace-drawer-leave-active {
  transition: opacity 180ms ease;
}

.marketplace-drawer-enter-active aside,
.marketplace-drawer-leave-active aside {
  transition: transform 220ms ease;
}

.marketplace-drawer-enter-from,
.marketplace-drawer-leave-to {
  opacity: 0;
}

.marketplace-drawer-enter-from aside,
.marketplace-drawer-leave-to aside {
  transform: translateX(100%);
}

@media (prefers-reduced-motion: reduce) {
  .marketplace-drawer-enter-active,
  .marketplace-drawer-leave-active,
  .marketplace-drawer-enter-active aside,
  .marketplace-drawer-leave-active aside {
    transition-duration: 1ms;
  }
}
</style>
