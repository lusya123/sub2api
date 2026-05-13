<template>
  <BaseDialog
    :show="show"
    :title="editing ? t('admin.modelMarketplaceMonitor.editTitle') : t('admin.modelMarketplaceMonitor.createTitle')"
    width="wide"
    @close="$emit('close')"
  >
    <form id="model-marketplace-monitor-form" @submit.prevent="handleSubmit" class="space-y-5">
      <div>
        <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.name') }} <span class="text-red-500">*</span></label>
        <input v-model="form.name" type="text" required class="input" :placeholder="t('admin.modelMarketplaceMonitor.form.namePlaceholder')" />
      </div>

      <div>
        <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.provider') }} <span class="text-red-500">*</span></label>
        <div class="grid max-h-72 grid-cols-2 gap-3 overflow-y-auto pr-1 sm:grid-cols-3 lg:grid-cols-4">
          <button
            v-for="opt in providerOptions"
            :key="opt.value"
            type="button"
            :aria-pressed="form.provider === opt.value"
            class="flex items-center justify-center gap-2 rounded-lg border-2 px-3 py-2.5 text-sm font-medium transition-colors"
            :class="providerPickerClass(opt.value, form.provider === opt.value)"
            @click="form.provider = opt.value"
          >
            <ProviderIcon :provider="opt.value" :size="18" />
            <span>{{ opt.label }}</span>
          </button>
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.endpoint') }} <span class="text-red-500">*</span></label>
        <div class="flex gap-2">
          <input v-model="form.endpoint" type="text" required class="input flex-1" :placeholder="t('admin.modelMarketplaceMonitor.form.endpointPlaceholder')" />
          <button type="button" @click="useCurrentDomain" class="btn btn-secondary whitespace-nowrap">
            {{ t('admin.modelMarketplaceMonitor.form.useCurrentDomain') }}
          </button>
        </div>
      </div>

      <div>
        <label class="input-label">
          {{ t('admin.modelMarketplaceMonitor.form.apiKey') }}<span v-if="!editing" class="text-red-500"> *</span>
        </label>
        <div class="flex gap-2">
          <input
            v-model="form.api_key"
            type="password"
            :required="!editing"
            class="input flex-1"
            :placeholder="editing ? t('admin.modelMarketplaceMonitor.form.apiKeyEditPlaceholder') : t('admin.modelMarketplaceMonitor.form.apiKeyPlaceholder')"
          />
          <button type="button" @click="openMyKeyPicker" class="btn btn-secondary whitespace-nowrap">
            {{ t('admin.modelMarketplaceMonitor.form.useMyKey') }}
          </button>
        </div>
        <p v-if="editing && editing.api_key_masked" class="mt-1 text-xs text-gray-400">{{ editing.api_key_masked }}</p>
      </div>

      <div>
        <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.primaryModel') }} <span class="text-red-500">*</span></label>
        <input
          v-model="form.primary_model"
          type="text"
          required
          class="input font-medium"
          :class="getPlatformTextClass(form.provider)"
          :placeholder="t('admin.modelMarketplaceMonitor.form.primaryModelPlaceholder')"
        />
      </div>

      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.displayNameZh') }}</label>
          <input
            :value="displayNameFor(form.primary_model).zh || ''"
            type="text"
            class="input"
            :placeholder="t('admin.modelMarketplaceMonitor.form.displayNameZhPlaceholder')"
            @input="updateModelDisplayName(form.primary_model, 'zh', ($event.target as HTMLInputElement).value)"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.displayNameEn') }}</label>
          <input
            :value="displayNameFor(form.primary_model).en || ''"
            type="text"
            class="input"
            :placeholder="t('admin.modelMarketplaceMonitor.form.displayNameEnPlaceholder')"
            @input="updateModelDisplayName(form.primary_model, 'en', ($event.target as HTMLInputElement).value)"
          />
        </div>
      </div>
      <p class="-mt-3 text-xs text-gray-400">
        {{ t('admin.modelMarketplaceMonitor.form.displayNameHint') }}
      </p>

      <div class="grid gap-3 sm:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.callModel') }}</label>
          <input
            :value="callConfigFor(form.primary_model).model || ''"
            type="text"
            class="input font-mono"
            :placeholder="form.primary_model || t('admin.modelMarketplaceMonitor.form.primaryModelPlaceholder')"
            @input="updateModelCallConfig(form.primary_model, 'model', ($event.target as HTMLInputElement).value)"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.requestUrl') }}</label>
          <input
            :value="callConfigFor(form.primary_model).request_url || ''"
            type="text"
            class="input font-mono"
            :placeholder="defaultRequestUrlFor(form.provider, callConfigFor(form.primary_model).model || form.primary_model)"
            @input="updateModelCallConfig(form.primary_model, 'request_url', ($event.target as HTMLInputElement).value)"
          />
        </div>
      </div>
      <p class="-mt-3 text-xs text-gray-400">
        {{ t('admin.modelMarketplaceMonitor.form.callConfigHint') }}
      </p>

      <details class="-mt-2 rounded-lg border border-gray-200 bg-gray-50/60 p-3 dark:border-dark-700 dark:bg-dark-900/30">
        <summary class="cursor-pointer text-sm font-semibold text-gray-700 dark:text-gray-200">
          {{ t('admin.modelMarketplaceMonitor.form.customPricing') }}
        </summary>
        <p class="mt-1 text-xs text-gray-400">
          {{ t('admin.modelMarketplaceMonitor.form.customPricingHint') }}
        </p>
        <div class="mt-3 grid gap-3 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.inputPricePerMillion') }}</label>
            <input
              :value="pricingOverrideFor(form.primary_model).input_price_per_million ?? ''"
              type="number"
              min="0"
              step="0.000001"
              class="input font-mono"
              :placeholder="t('admin.modelMarketplaceMonitor.form.defaultPricingPlaceholder')"
              @input="updateModelPricingOverride(form.primary_model, 'input_price_per_million', ($event.target as HTMLInputElement).value)"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.outputPricePerMillion') }}</label>
            <input
              :value="pricingOverrideFor(form.primary_model).output_price_per_million ?? ''"
              type="number"
              min="0"
              step="0.000001"
              class="input font-mono"
              :placeholder="t('admin.modelMarketplaceMonitor.form.defaultPricingPlaceholder')"
              @input="updateModelPricingOverride(form.primary_model, 'output_price_per_million', ($event.target as HTMLInputElement).value)"
            />
          </div>
        </div>
      </details>

      <div>
        <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.extraModels') }}</label>
        <ModelTagInput
          :models="form.extra_models"
          :platform="form.provider"
          :placeholder="t('admin.modelMarketplaceMonitor.form.extraModelsPlaceholder')"
          @update:models="form.extra_models = $event"
        />
      </div>

      <div v-if="form.extra_models.length > 0" class="space-y-3 rounded-lg border border-gray-200 bg-gray-50/60 p-3 dark:border-dark-700 dark:bg-dark-900/30">
        <div class="text-sm font-semibold text-gray-700 dark:text-gray-200">
          {{ t('admin.modelMarketplaceMonitor.form.extraDisplayNames') }}
        </div>
        <div
          v-for="model in form.extra_models"
          :key="model"
          class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-950"
        >
          <div class="truncate font-mono text-xs font-semibold text-gray-600 dark:text-gray-300" :title="model">
            {{ model }}
          </div>
          <div class="mt-2 grid gap-2 sm:grid-cols-2">
            <input
              :value="displayNameFor(model).zh || ''"
              type="text"
              class="input"
              :placeholder="t('admin.modelMarketplaceMonitor.form.displayNameZh')"
              @input="updateModelDisplayName(model, 'zh', ($event.target as HTMLInputElement).value)"
            />
            <input
              :value="displayNameFor(model).en || ''"
              type="text"
              class="input"
              :placeholder="t('admin.modelMarketplaceMonitor.form.displayNameEn')"
              @input="updateModelDisplayName(model, 'en', ($event.target as HTMLInputElement).value)"
            />
            <input
              :value="callConfigFor(model).model || ''"
              type="text"
              class="input font-mono"
              :placeholder="t('admin.modelMarketplaceMonitor.form.callModel')"
              @input="updateModelCallConfig(model, 'model', ($event.target as HTMLInputElement).value)"
            />
            <input
              :value="callConfigFor(model).request_url || ''"
              type="text"
              class="input font-mono"
              :placeholder="defaultRequestUrlFor(form.provider, callConfigFor(model).model || model)"
              @input="updateModelCallConfig(model, 'request_url', ($event.target as HTMLInputElement).value)"
            />
          </div>
          <details class="mt-2 rounded-lg border border-gray-200 bg-gray-50/80 p-3 dark:border-dark-700 dark:bg-dark-900/50">
            <summary class="cursor-pointer text-xs font-semibold text-gray-600 dark:text-gray-300">
              {{ t('admin.modelMarketplaceMonitor.form.customPricing') }}
            </summary>
            <div class="mt-3 grid gap-2 sm:grid-cols-2">
              <input
                :value="pricingOverrideFor(model).input_price_per_million ?? ''"
                type="number"
                min="0"
                step="0.000001"
                class="input font-mono"
                :placeholder="t('admin.modelMarketplaceMonitor.form.inputPricePerMillion')"
                @input="updateModelPricingOverride(model, 'input_price_per_million', ($event.target as HTMLInputElement).value)"
              />
              <input
                :value="pricingOverrideFor(model).output_price_per_million ?? ''"
                type="number"
                min="0"
                step="0.000001"
                class="input font-mono"
                :placeholder="t('admin.modelMarketplaceMonitor.form.outputPricePerMillion')"
                @input="updateModelPricingOverride(model, 'output_price_per_million', ($event.target as HTMLInputElement).value)"
              />
            </div>
          </details>
        </div>
      </div>

      <div>
        <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.groupName') }}</label>
        <input v-model="form.group_name" type="text" class="input" :placeholder="t('admin.modelMarketplaceMonitor.form.groupNamePlaceholder')" />
      </div>

      <div>
        <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.effectiveRate') }} <span class="text-red-500">*</span></label>
        <input
          v-model.number="form.effective_rate"
          type="number"
          min="0.0001"
          step="0.0001"
          required
          class="input"
          :placeholder="t('admin.modelMarketplaceMonitor.form.effectiveRatePlaceholder')"
        />
        <p class="mt-1 text-xs text-gray-400">{{ t('admin.modelMarketplaceMonitor.form.effectiveRateHint') }}</p>
      </div>

      <div>
        <label class="input-label">{{ t('admin.modelMarketplaceMonitor.form.intervalSeconds') }} <span class="text-red-500">*</span></label>
        <input v-model.number="form.interval_seconds" type="number" min="15" max="3600" required class="input" />
        <p class="mt-1 text-xs text-gray-400">{{ t('admin.modelMarketplaceMonitor.form.intervalSecondsHint') }}</p>
      </div>

      <div class="flex items-center justify-between">
        <label class="input-label mb-0">{{ t('admin.modelMarketplaceMonitor.form.enabled') }}</label>
        <Toggle v-model="form.enabled" />
      </div>

      <!-- 高级设置区：请求模板 + 自定义 headers/body -->
      <details class="rounded-lg border border-gray-200 bg-gray-50/50 p-3 dark:border-dark-700 dark:bg-dark-900/30">
        <summary class="cursor-pointer text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.modelMarketplaceMonitor.advanced.section') }}
        </summary>
        <p class="mt-1 text-xs text-gray-400">{{ t('admin.modelMarketplaceMonitor.advanced.sectionHint') }}</p>

        <div class="mt-4 space-y-4">
          <div>
            <label class="input-label">{{ t('admin.modelMarketplaceMonitor.templateField.label') }}</label>
            <Select
              v-model="templateSelectValue"
              :options="templateOptions"
              :placeholder="t('admin.modelMarketplaceMonitor.templateField.placeholder')"
            />
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.modelMarketplaceMonitor.templateField.applyHint') }}</p>
          </div>

          <MonitorAdvancedRequestConfig
            :extra-headers="form.extra_headers"
            :body-override-mode="form.body_override_mode"
            :body-override="form.body_override"
            @update:extra-headers="form.extra_headers = $event"
            @update:body-override-mode="form.body_override_mode = $event"
            @update:body-override="form.body_override = $event"
          />
        </div>
      </details>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button @click="$emit('close')" type="button" class="btn btn-secondary">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="model-marketplace-monitor-form"
          :disabled="submitting"
          class="btn btn-primary"
        >
          {{ submitting
            ? t('common.submitting')
            : editing ? t('common.update') : t('common.create') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <MonitorKeyPickerDialog
    :show="showKeyPicker"
    :loading="myKeysLoading"
    :keys="myActiveKeys"
    :provider="form.provider"
    :user-group-rates="userGroupRates"
    @close="showKeyPicker = false"
    @pick="pickMyKey"
  />
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import { keysAPI } from '@/api/keys'
import { userGroupsAPI } from '@/api/groups'
import type {
  BodyOverrideMode,
  ModelMarketplaceMonitor,
  CreateParams,
  ModelCallConfig,
  ModelDisplayName,
  ModelPricingOverride,
  Provider,
  UpdateParams,
} from '@/api/admin/modelMarketplaceMonitor'
import type { ModelMarketplaceTemplate } from '@/api/admin/modelMarketplaceTemplate'
import type { ApiKey } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import Select from '@/components/common/Select.vue'
import ModelTagInput from '@/components/admin/marketplace/ModelTagInput.vue'
import { getPlatformTextClass } from '@/components/admin/marketplace/types'
import MonitorKeyPickerDialog from '@/components/admin/marketplace/MonitorKeyPickerDialog.vue'
import MonitorAdvancedRequestConfig from '@/components/admin/marketplace/MonitorAdvancedRequestConfig.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { useModelMarketplaceMonitorFormat } from '@/composables/useModelMarketplaceMonitorFormat'
import {
  PROVIDER_ANTHROPIC,
  MODEL_MARKETPLACE_PROVIDER_OPTIONS,
  DEFAULT_INTERVAL_SECONDS,
} from '@/constants/modelMarketplaceMonitor'

const props = defineProps<{
  show: boolean
  monitor: ModelMarketplaceMonitor | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { providerPickerClass } = useModelMarketplaceMonitorFormat()

const systemDefaultInterval = computed<number>(() => DEFAULT_INTERVAL_SECONDS)

// editing is true when we have an existing monitor
const editing = computed<ModelMarketplaceMonitor | null>(() => props.monitor)

const submitting = ref(false)

// API key picker
const showKeyPicker = ref(false)
const myKeysLoading = ref(false)
const myActiveKeys = ref<ApiKey[]>([])
const userGroupRates = ref<Record<number, number>>({})

interface MonitorForm {
  name: string
  provider: Provider
  endpoint: string
  api_key: string
  primary_model: string
  extra_models: string[]
  model_display_names: Record<string, ModelDisplayName>
  model_call_configs: Record<string, ModelCallConfig>
  group_name: string
  effective_rate: number
  interval_seconds: number
  enabled: boolean
  // 高级设置快照
  template_id: number | null
  extra_headers: Record<string, string>
  body_override_mode: BodyOverrideMode
  body_override: Record<string, unknown> | null
}

const form = reactive<MonitorForm>({
  name: '',
  provider: PROVIDER_ANTHROPIC,
  endpoint: '',
  api_key: '',
  primary_model: '',
  extra_models: [],
  model_display_names: {},
  model_call_configs: {},
  group_name: '',
  effective_rate: 1,
  interval_seconds: systemDefaultInterval.value,
  enabled: true,
  template_id: null,
  extra_headers: {},
  body_override_mode: 'off',
  body_override: null,
})

// 可用模板列表（进入 dialog 时一次性拉取 cache；按 provider 过滤）。
const templatesCache = ref<ModelMarketplaceTemplate[]>([])
const templatesLoading = ref(false)

const templateOptions = computed(() => {
  const items = templatesCache.value.filter((t) => t.provider === form.provider)
  return [
    { value: '', label: t('admin.modelMarketplaceMonitor.templateField.none') },
    ...items.map((t) => ({ value: String(t.id), label: t.name })),
  ]
})

async function loadTemplates() {
  if (templatesCache.value.length > 0) return
  templatesLoading.value = true
  try {
    const { items } = await adminAPI.modelMarketplaceTemplate.list()
    templatesCache.value = items
  } catch (err: unknown) {
    // 模板拉取失败不阻塞监控表单，用户可以不选模板
    console.warn('load monitor templates failed', err)
  } finally {
    templatesLoading.value = false
  }
}

// 模板下拉绑定：value 是 string（Select 组件约束），需要与 number | null 互转。
const templateSelectValue = computed<string>({
  get: () => (form.template_id == null ? '' : String(form.template_id)),
  set: (raw: string) => {
    if (raw === '') {
      form.template_id = null
      return
    }
    const id = Number(raw)
    if (!Number.isFinite(id)) return
    form.template_id = id
    // 应用模板 = 拷贝快照
    const tpl = templatesCache.value.find((t) => t.id === id)
    if (tpl) {
      form.extra_headers = { ...(tpl.extra_headers || {}) }
      form.body_override_mode = tpl.body_override_mode
      form.body_override = tpl.body_override ? { ...tpl.body_override } : null
    }
  },
})

interface ProviderOption {
  value: Provider
  label: string
}

const providerOptions = computed<ProviderOption[]>(() => [
  ...MODEL_MARKETPLACE_PROVIDER_OPTIONS.map((opt) => ({ value: opt.value, label: opt.label })),
])

// Clear api_key whenever provider changes to avoid cross-provider key mismatch.
// Editing mode loads api_key='' via loadFromMonitor and only sets it on user
// typing, so clearing on provider change is always a safe no-op until the user
// picks a new key.
// 同时清空 template_id（模板有 provider 归属，跨平台不通用）。
watch(() => form.provider, () => {
  form.api_key = ''
  form.template_id = null
})

function resetForm() {
  form.name = ''
  form.provider = PROVIDER_ANTHROPIC
  form.endpoint = ''
  form.api_key = ''
  form.primary_model = ''
  form.extra_models = []
  form.model_display_names = {}
  form.model_call_configs = {}
  form.group_name = ''
  form.effective_rate = 1
  form.interval_seconds = systemDefaultInterval.value
  form.enabled = true
  form.template_id = null
  form.extra_headers = {}
  form.body_override_mode = 'off'
  form.body_override = null
}

function loadFromMonitor(m: ModelMarketplaceMonitor) {
  form.name = m.name
  form.provider = m.provider
  form.endpoint = m.endpoint
  form.api_key = ''
  form.primary_model = m.primary_model
  form.extra_models = [...(m.extra_models || [])]
  form.model_display_names = { ...(m.model_display_names || {}) }
  form.model_call_configs = { ...(m.model_call_configs || {}) }
  form.group_name = m.group_name || ''
  form.effective_rate = normalizeEffectiveRateForForm(m.effective_rate)
  form.interval_seconds = m.interval_seconds || systemDefaultInterval.value
  form.enabled = m.enabled
  form.template_id = m.template_id ?? null
  form.extra_headers = { ...(m.extra_headers || {}) }
  form.body_override_mode = m.body_override_mode || 'off'
  form.body_override = m.body_override ? { ...m.body_override } : null
}

// Re-sync form whenever the dialog is opened or the target monitor changes.
// 同时拉取模板列表（cache 过的话一次性返回）。
watch(
  () => [props.show, props.monitor] as const,
  ([show, m]) => {
    if (!show) return
    void loadTemplates()
    if (m) loadFromMonitor(m)
    else resetForm()
  },
  { immediate: true },
)

function useCurrentDomain() {
  form.endpoint = window.location.origin
}

async function openMyKeyPicker() {
  showKeyPicker.value = true
  if (myActiveKeys.value.length > 0) return
  myKeysLoading.value = true
  try {
    const [res, rates] = await Promise.all([
      keysAPI.list(1, 100, { status: 'active' }),
      userGroupsAPI.getUserGroupRates(),
    ])
    const items = res.items || []
    const now = Date.now()
    myActiveKeys.value = items.filter(k => {
      if (k.status !== 'active') return false
      if (!k.expires_at) return true
      return new Date(k.expires_at).getTime() > now
    })
    userGroupRates.value = rates
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.modelMarketplaceMonitor.form.noActiveKey')))
  } finally {
    myKeysLoading.value = false
  }
}

function pickMyKey(k: ApiKey) {
  form.api_key = k.key
  showKeyPicker.value = false
}

function buildPayload(): CreateParams {
  return {
    name: form.name.trim(),
    provider: form.provider,
    endpoint: form.endpoint.trim(),
    api_key: form.api_key.trim(),
    primary_model: form.primary_model.trim(),
    extra_models: form.extra_models,
    model_display_names: normalizedModelDisplayNames(),
    model_call_configs: normalizedModelCallConfigs(),
    group_name: form.group_name.trim(),
    effective_rate: normalizeEffectiveRateForForm(form.effective_rate),
    enabled: form.enabled,
    interval_seconds: form.interval_seconds,
    template_id: form.template_id,
    extra_headers: form.extra_headers,
    body_override_mode: form.body_override_mode,
    body_override: form.body_override,
  }
}

function displayNameFor(model: string): ModelDisplayName {
  const key = model.trim()
  if (!key) return {}
  return form.model_display_names[key] || {}
}

function updateModelDisplayName(model: string, field: keyof ModelDisplayName, value: string) {
  const key = model.trim()
  if (!key) return
  const next = { ...(form.model_display_names[key] || {}), [field]: value }
  if (!String(next.zh || '').trim() && !String(next.en || '').trim()) {
    delete form.model_display_names[key]
    return
  }
  form.model_display_names[key] = next
}

function normalizedModelDisplayNames(): Record<string, ModelDisplayName> {
  const allowed = new Set([form.primary_model.trim(), ...form.extra_models.map((m) => m.trim())].filter(Boolean))
  const out: Record<string, ModelDisplayName> = {}
  for (const model of allowed) {
    const names = form.model_display_names[model]
    if (!names) continue
    const zh = String(names.zh || '').trim()
    const en = String(names.en || '').trim()
    if (zh || en) out[model] = { ...(zh ? { zh } : {}), ...(en ? { en } : {}) }
  }
  return out
}

function callConfigFor(model: string): ModelCallConfig {
  const key = model.trim()
  if (!key) return {}
  return form.model_call_configs[key] || {}
}

function updateModelCallConfig(model: string, field: keyof ModelCallConfig, value: string) {
  const key = model.trim()
  if (!key) return
  const next = { ...(form.model_call_configs[key] || {}), [field]: value }
  if (!String(next.model || '').trim() && !String(next.request_url || '').trim() && !hasPricingOverride(next.pricing)) {
    delete form.model_call_configs[key]
    return
  }
  form.model_call_configs[key] = next
}

function pricingOverrideFor(model: string): ModelPricingOverride {
  const key = model.trim()
  if (!key) return {}
  return form.model_call_configs[key]?.pricing || {}
}

function updateModelPricingOverride(model: string, field: keyof ModelPricingOverride, value: string) {
  const key = model.trim()
  if (!key) return
  const existing = form.model_call_configs[key] || {}
  const pricing = { ...(existing.pricing || {}) }
  const parsed = parseOptionalModelPrice(value)
  if (parsed == null) delete pricing[field]
  else pricing[field] = parsed
  const next: ModelCallConfig = { ...existing, pricing: hasPricingOverride(pricing) ? pricing : undefined }
  if (!String(next.model || '').trim() && !String(next.request_url || '').trim() && !hasPricingOverride(next.pricing)) {
    delete form.model_call_configs[key]
    return
  }
  form.model_call_configs[key] = next
}

function parseOptionalModelPrice(value: string): number | undefined {
  const raw = String(value ?? '').trim()
  if (raw === '') return undefined
  const n = Number(raw)
  return Number.isFinite(n) && n >= 0 ? n : undefined
}

function hasPricingOverride(pricing: ModelPricingOverride | undefined): boolean {
  if (!pricing) return false
  return pricing.input_price_per_million != null || pricing.output_price_per_million != null
}

function normalizedModelCallConfigs(): Record<string, ModelCallConfig> {
  const allowed = new Set([form.primary_model.trim(), ...form.extra_models.map((m) => m.trim())].filter(Boolean))
  const out: Record<string, ModelCallConfig> = {}
  for (const model of allowed) {
    const cfg = form.model_call_configs[model]
    if (!cfg) continue
    const callModel = String(cfg.model || '').trim()
    const requestUrl = String(cfg.request_url || '').trim()
    const pricing = normalizePricingOverride(cfg.pricing)
    if (callModel || requestUrl || pricing) {
      out[model] = {
        ...(callModel ? { model: callModel } : {}),
        ...(requestUrl ? { request_url: requestUrl } : {}),
        ...(pricing ? { pricing } : {}),
      }
    }
  }
  return out
}

function normalizePricingOverride(pricing: ModelPricingOverride | undefined): ModelPricingOverride | undefined {
  if (!pricing) return undefined
  const input = pricing.input_price_per_million
  const output = pricing.output_price_per_million
  const out: ModelPricingOverride = {}
  if (Number.isFinite(input) && Number(input) >= 0) out.input_price_per_million = Number(input)
  if (Number.isFinite(output) && Number(output) >= 0) out.output_price_per_million = Number(output)
  return hasPricingOverride(out) ? out : undefined
}

function hasInvalidPricingOverride(): boolean {
  return Object.values(form.model_call_configs).some((cfg) => {
    const p = cfg.pricing
    if (!p) return false
    return [p.input_price_per_million, p.output_price_per_million].some((value) => value != null && (!Number.isFinite(value) || Number(value) < 0))
  })
}

function defaultRequestUrlFor(provider: Provider, model: string): string {
  const origin = window.location.origin.replace(/\/+$/, '')
  const callModel = encodeURIComponent(String(model || '').trim())
  if (provider === 'anthropic') return `${origin}/v1/messages`
  if (provider === 'gemini') return callModel ? `${origin}/v1beta/models/${callModel}:generateContent` : `${origin}/v1beta/models/{model}:generateContent`
  return `${origin}/v1/chat/completions`
}

function normalizeEffectiveRateForForm(rate: number | null | undefined): number {
  const n = Number(rate)
  return Number.isFinite(n) && n > 0 ? n : 1
}

async function handleSubmit() {
  if (submitting.value) return
  if (!form.name.trim()) {
    appStore.showError(t('admin.modelMarketplaceMonitor.nameRequired'))
    return
  }
  if (!form.primary_model.trim()) {
    appStore.showError(t('admin.modelMarketplaceMonitor.primaryModelRequired'))
    return
  }
  if (!Number.isFinite(Number(form.effective_rate)) || Number(form.effective_rate) <= 0) {
    appStore.showError(t('admin.modelMarketplaceMonitor.effectiveRateRequired'))
    return
  }
  if (hasInvalidPricingOverride()) {
    appStore.showError(t('admin.modelMarketplaceMonitor.invalidPricing'))
    return
  }

  submitting.value = true
  try {
    const target = editing.value
    if (target) {
      const { api_key, ...rest } = buildPayload()
      const req: UpdateParams = { ...rest }
      // Only send api_key if user typed a new value
      if (api_key) req.api_key = api_key
      // template_id=null 用 clear_template=true 明确告诉后端清空（pointer 语义）
      if (form.template_id == null) {
        req.clear_template = true
        delete req.template_id
      }
      await adminAPI.modelMarketplaceMonitor.update(target.id, req)
      appStore.showSuccess(t('admin.modelMarketplaceMonitor.updateSuccess'))
    } else {
      await adminAPI.modelMarketplaceMonitor.create(buildPayload())
      appStore.showSuccess(t('admin.modelMarketplaceMonitor.createSuccess'))
    }
    emit('saved')
    emit('close')
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    submitting.value = false
  }
}
</script>
