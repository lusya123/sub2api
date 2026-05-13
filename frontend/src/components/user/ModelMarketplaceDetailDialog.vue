<template>
  <BaseDialog
    :show="show"
    :title="title"
    width="wide"
    @close="$emit('close')"
  >
    <div v-if="loading" class="py-8 text-center text-sm text-gray-500">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="!detail" class="py-8 text-center text-sm text-gray-500">
      {{ t('modelMarketplaceStatus.detailLoadError') }}
    </div>
    <div v-else class="overflow-x-auto">
      <table class="w-full text-left text-sm">
        <thead class="border-b border-gray-200 dark:border-dark-700">
          <tr class="text-xs uppercase tracking-wider text-gray-500 dark:text-gray-400">
            <th class="py-2 pr-3">{{ t('modelMarketplaceStatus.detailColumns.model') }}</th>
            <th class="py-2 pr-3">{{ t('modelMarketplaceStatus.price') }}</th>
            <th class="py-2 pr-3">{{ t('modelMarketplaceStatus.detailColumns.latestStatus') }}</th>
            <th class="py-2 pr-3">{{ t('modelMarketplaceStatus.detailColumns.latestLatency') }}</th>
            <th class="py-2 pr-3">{{ t('modelMarketplaceStatus.detailColumns.availability7d') }}</th>
            <th class="py-2 pr-3">{{ t('modelMarketplaceStatus.detailColumns.availability15d') }}</th>
            <th class="py-2 pr-3">{{ t('modelMarketplaceStatus.detailColumns.availability30d') }}</th>
            <th class="py-2 pr-3">{{ t('modelMarketplaceStatus.detailColumns.avgLatency7d') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="m in detail.models"
            :key="m.model"
            class="border-b border-gray-100 dark:border-dark-800"
          >
            <td class="py-2 pr-3">
              <div v-if="localizedConfiguredModelName(m.display_name_zh, m.display_name_en)" class="font-medium text-gray-900 dark:text-gray-100">
                {{ localizedConfiguredModelName(m.display_name_zh, m.display_name_en) }}
              </div>
              <div
                class="font-mono text-xs text-gray-400"
                :class="{ 'mt-0.5': localizedConfiguredModelName(m.display_name_zh, m.display_name_en) }"
              >
                {{ m.model }}
              </div>
              <div class="mt-1 max-w-[320px] truncate font-mono text-[11px] text-gray-500 dark:text-gray-400" :title="m.call_model || m.model">
                {{ t('modelMarketplaceStatus.callModel', 'Call model') }}: {{ m.call_model || m.model }}
              </div>
              <div class="mt-0.5 max-w-[320px] truncate font-mono text-[11px] text-gray-500 dark:text-gray-400" :title="requestUrlFor(m)">
                {{ t('modelMarketplaceStatus.requestUrl', 'Request URL') }}: {{ requestUrlFor(m) }}
              </div>
            </td>
            <td class="py-2 pr-3 text-gray-700 dark:text-gray-300">
              <div v-if="priceSummary(m.pricing).length > 0" class="flex flex-col gap-0.5 font-mono text-xs">
                <span v-for="line in priceSummary(m.pricing)" :key="line">{{ line }}</span>
              </div>
              <span v-else class="text-gray-400">{{ t('modelMarketplaceStatus.priceUnavailable') }}</span>
            </td>
            <td class="py-2 pr-3">
              <span
                class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px]"
                :class="statusBadgeClass(m.latest_status)"
              >
                {{ statusLabel(m.latest_status) }}
              </span>
            </td>
            <td class="py-2 pr-3 text-gray-700 dark:text-gray-300">{{ formatLatency(m.latest_latency_ms) }}</td>
            <td class="py-2 pr-3 text-gray-700 dark:text-gray-300">{{ formatPercent(m.availability_7d) }}</td>
            <td class="py-2 pr-3 text-gray-700 dark:text-gray-300">{{ formatPercent(m.availability_15d) }}</td>
            <td class="py-2 pr-3 text-gray-700 dark:text-gray-300">{{ formatPercent(m.availability_30d) }}</td>
            <td class="py-2 pr-3 text-gray-700 dark:text-gray-300">{{ formatLatency(m.avg_latency_7d_ms) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button @click="$emit('close')" class="btn btn-secondary">
          {{ t('modelMarketplaceStatus.closeDetail') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  status as fetchModelMarketplaceDetail,
  type UserModelMarketplaceDetail,
  type UserModelMarketplaceModelDetail,
} from '@/api/modelMarketplace'
import type { UserSupportedModelPricing } from '@/api/channels'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { useModelMarketplaceMonitorFormat } from '@/composables/useModelMarketplaceMonitorFormat'
import { BILLING_MODE_IMAGE, BILLING_MODE_PER_REQUEST, BILLING_MODE_TOKEN } from '@/constants/channel'
import { formatScaled } from '@/utils/pricing'

const props = defineProps<{
  show: boolean
  monitorId: number | null
  title: string
}>()

defineEmits<{
  (e: 'close'): void
}>()

const { t, locale } = useI18n()
const appStore = useAppStore()
const { statusLabel, statusBadgeClass, formatLatency, formatPercent } = useModelMarketplaceMonitorFormat()

const detail = ref<UserModelMarketplaceDetail | null>(null)
const loading = ref(false)

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

function priceSummary(pricing: UserSupportedModelPricing | null): string[] {
  if (!pricing) return []
  const perRequestUnit = t('availableChannels.pricing.unitPerRequest')
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST && pricing.per_request_price != null) {
    return [`${formatScaled(pricing.per_request_price, 1)} ${perRequestUnit}`]
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE && pricing.image_output_price != null) {
    return [`${formatScaled(pricing.image_output_price, 1)} ${perRequestUnit}`]
  }
  if (pricing.billing_mode !== BILLING_MODE_TOKEN) return []
  const unit = t('availableChannels.pricing.unitPerMillion')
  const lines = [
    pricing.input_price == null
      ? ''
      : `${t('availableChannels.pricing.inputPrice')} ${formatScaled(pricing.input_price, 1_000_000)} ${unit}`,
    pricing.output_price == null
      ? ''
      : `${t('availableChannels.pricing.outputPrice')} ${formatScaled(pricing.output_price, 1_000_000)} ${unit}`,
    pricing.cache_read_price == null
      ? ''
      : `${t('availableChannels.pricing.cacheReadPrice')} ${formatScaled(pricing.cache_read_price, 1_000_000)} ${unit}`,
  ]
  return lines.filter(Boolean)
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
  ([show, id]) => {
    if (!show) {
      detail.value = null
      return
    }
    if (id != null) void load(id)
  },
  { immediate: true },
)
</script>
