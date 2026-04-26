<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.modelMarketplace.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            选择要监控的模型，打开要展示的分组，然后保存。
          </p>
        </div>
        <div class="flex items-center gap-2">
          <button class="btn btn-secondary" :disabled="statusConfigLoading" @click="loadStatusConfig">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': statusConfigLoading }" />
            <span>刷新</span>
          </button>
          <button class="btn btn-primary" :disabled="statusConfigSaving || !statusConfig" @click="saveStatusConfig">
            <Icon name="check" size="sm" :class="{ 'animate-spin': statusConfigSaving }" />
            <span>保存配置</span>
          </button>
        </div>
      </div>

      <div v-if="statusConfigError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-900/20 dark:text-red-300">
        {{ statusConfigError }}
      </div>

      <div v-if="statusConfig" class="grid gap-4 md:grid-cols-3">
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-dark-900">
          <div class="flex items-center gap-3">
            <span class="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100 text-sm font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">1</span>
            <div>
              <p class="text-sm font-semibold text-gray-900 dark:text-white">监控模型</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ enabledModelCount }} / {{ statusConfig.config.models.length }} 已启用</p>
            </div>
          </div>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-dark-900">
          <div class="flex items-center gap-3">
            <span class="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100 text-sm font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">2</span>
            <div>
              <p class="text-sm font-semibold text-gray-900 dark:text-white">展示分组</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ enabledGroupCount }} / {{ statusConfig.group_options.length }} 已展示</p>
            </div>
          </div>
        </div>
        <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-dark-900">
          <div class="flex items-center gap-3">
            <span class="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100 text-sm font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">3</span>
            <div>
              <p class="text-sm font-semibold text-gray-900 dark:text-white">保存生效</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">用户看板只显示已启用分组</p>
            </div>
          </div>
        </div>
      </div>

      <section v-if="statusConfig" class="grid gap-5 xl:grid-cols-[360px_minmax(0,1fr)]">
        <div class="card p-4">
          <div class="flex items-center justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">监控模型</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">打开的模型会出现在状态看板。</p>
            </div>
            <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
              {{ enabledModelCount }} 个
            </span>
          </div>
          <div class="mt-4 flex gap-2">
            <input
              v-model="newStatusModelName"
              class="input h-10 min-w-0 flex-1 font-mono text-sm"
              placeholder="glm-5 / minimax-m2.5"
              @keydown.enter.prevent="addStatusModel"
            />
            <button class="btn btn-secondary h-10 px-3" type="button" @click="addStatusModel">
              <Icon name="plus" size="sm" />
              <span>添加</span>
            </button>
          </div>
          <div class="mt-4 space-y-2">
            <label
              v-for="model in statusConfig.config.models"
              :key="model.name"
              class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2.5 dark:border-gray-800 dark:bg-gray-800/60"
            >
              <span class="min-w-0 truncate font-mono text-sm text-gray-800 dark:text-gray-100">{{ model.name }}</span>
              <div class="flex shrink-0 items-center gap-2">
                <span class="text-xs text-gray-500 dark:text-gray-400">{{ model.enabled ? '显示' : '隐藏' }}</span>
                <input v-model="model.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
                <button class="btn btn-ghost h-8 px-1.5 text-gray-400 hover:text-red-500" type="button" @click.stop.prevent="removeStatusModel(model.name)">
                  <Icon name="x" size="xs" />
                </button>
              </div>
            </label>
          </div>
        </div>

        <div class="card p-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">展示分组</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">打开哪个分组，用户就看到哪个分组。</p>
            </div>
            <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
              {{ enabledGroupCount }} 个已打开
            </span>
          </div>

          <div class="mt-4 space-y-3">
            <article
              v-for="group in statusConfig.group_options"
              :key="group.group_id"
              class="rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-dark-900"
            >
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <label class="flex min-w-0 items-start gap-3">
                  <input
                    :checked="editableGroup(group.group_id).enabled"
                    type="checkbox"
                    class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                    @change="onGroupEnabledChange(group, $event)"
                  />
                  <span class="min-w-0">
                    <span class="block truncate text-sm font-semibold text-gray-900 dark:text-white" :title="group.name">
                      {{ editableGroup(group.group_id).display_name || group.suggested_name || group.name }}
                    </span>
                    <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
                      后台分组：{{ group.name }} · {{ group.account_count }} 个账号
                    </span>
                  </span>
                </label>
                <span
                  class="w-fit rounded-full px-2.5 py-1 text-xs font-medium"
                  :class="editableGroup(group.group_id).enabled
                    ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
                    : 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-400'"
                >
                  {{ editableGroup(group.group_id).enabled ? '用户可见' : '不展示' }}
                </span>
              </div>

              <div v-if="editableGroup(group.group_id).enabled" class="mt-4 grid gap-3 md:grid-cols-3">
                <label class="block">
                  <span class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">用户看到的名字</span>
                  <input
                    :value="editableGroup(group.group_id).display_name || group.suggested_name"
                    class="input h-10 text-sm"
                    @input="onGroupDisplayNameInput(group.group_id, $event)"
                  />
                </label>
                <label class="block">
                  <span class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">合并名称</span>
                  <input
                    :value="editableGroup(group.group_id).aggregate_key || group.suggested_key"
                    class="input h-10 font-mono text-sm"
                    placeholder="例如 monthly-card"
                    @input="onGroupAggregateKeyInput(group.group_id, $event)"
                  />
                </label>
                <label class="block">
                  <span class="mb-1 block text-xs font-medium text-gray-500 dark:text-gray-400">排序</span>
                  <input
                    :value="editableGroup(group.group_id).sort_order || ''"
                    class="input h-10 text-sm"
                    placeholder="数字越小越靠前"
                    inputmode="numeric"
                    @input="onGroupSortOrderInput(group.group_id, $event)"
                  />
                </label>
              </div>

              <details v-if="editableGroup(group.group_id).enabled" class="mt-4 rounded-lg bg-gray-50 p-3 dark:bg-gray-800/60">
                <summary class="cursor-pointer select-none text-sm font-medium text-gray-700 dark:text-gray-200">
                  高级设置：{{ groupModelSummary(group.group_id) }} · {{ groupLineSummary(group.group_id, editableGroup(group.group_id).display_name || group.suggested_name || group.name) }}
                </summary>
                <div class="mt-3 grid gap-4 lg:grid-cols-2">
                  <div>
                    <div class="mb-2 text-xs font-semibold text-gray-500 dark:text-gray-400">这个分组检测哪些模型</div>
                    <div class="flex max-h-40 flex-wrap gap-2 overflow-y-auto">
                      <label
                        v-for="model in statusModelOptions"
                        :key="`${group.group_id}-${model.name}`"
                        class="inline-flex min-w-0 items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700 dark:border-gray-700 dark:bg-dark-900 dark:text-gray-200"
                      >
                        <input
                          :checked="groupUsesModel(group.group_id, model.name)"
                          type="checkbox"
                          class="h-3.5 w-3.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                          @change="onGroupModelChange(group.group_id, model.name, $event)"
                        />
                        <span class="max-w-36 truncate font-mono" :title="model.name">{{ shortModelName(model.name) }}</span>
                      </label>
                    </div>
                  </div>

                  <div>
                    <div class="mb-2 flex items-center justify-between gap-2">
                      <span class="text-xs font-semibold text-gray-500 dark:text-gray-400">探测线路</span>
                      <button class="btn btn-secondary h-7 px-2 text-xs" type="button" @click="addProbeLine(group.group_id)">
                        <Icon name="plus" size="xs" />
                        <span>加线路</span>
                      </button>
                    </div>
                    <div class="space-y-2">
                      <div
                        v-for="(line, lineIndex) in editableProbeLines(group.group_id)"
                        :key="line.id || `${group.group_id}-line-${lineIndex}`"
                        class="grid grid-cols-[18px_1fr_0.8fr_28px] items-center gap-2"
                      >
                        <input
                          v-model="line.enabled"
                          type="checkbox"
                          class="h-3.5 w-3.5 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                        />
                        <input v-model="line.name" class="input h-8 text-xs" placeholder="线路名" />
                        <input v-model="line.region" class="input h-8 text-xs" placeholder="US/EU/Asia" />
                        <button class="btn btn-ghost h-8 px-1" type="button" @click="removeProbeLine(group.group_id, lineIndex)">
                          <Icon name="x" size="xs" />
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </details>
            </article>
          </div>
        </div>
      </section>

      <div v-else-if="statusConfigLoading" class="flex items-center justify-center py-16">
        <LoadingSpinner />
      </div>

      <details class="card p-4">
        <summary class="cursor-pointer select-none text-sm font-semibold text-gray-800 dark:text-gray-100">
          可用模型来源
        </summary>
        <div class="mt-4 space-y-4">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
            <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-800/70">
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelMarketplace.totalModels') }}</p>
                <p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ data?.total_models ?? 0 }}</p>
              </div>
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-800/70">
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelMarketplace.totalAccounts') }}</p>
                <p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ data?.total_accounts ?? 0 }}</p>
              </div>
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-800/70">
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelMarketplace.visibleModels') }}</p>
                <p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ filteredModels.length }}</p>
              </div>
              <div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-800/70">
                <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelMarketplace.pricedModels') }}</p>
                <p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{{ pricedCount }}</p>
              </div>
            </div>
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
              <div class="relative">
                <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
                <input
                  v-model="search"
                  type="text"
                  class="input w-full pl-9 sm:w-72"
                  :placeholder="t('admin.modelMarketplace.searchPlaceholder')"
                />
              </div>
              <select v-model="platform" class="input sm:w-44">
                <option value="">{{ t('admin.modelMarketplace.allPlatforms') }}</option>
                <option v-for="item in platformOptions" :key="item" :value="item">
                  {{ platformLabel(item) }}
                </option>
              </select>
              <button class="btn btn-secondary" :disabled="loading" @click="load">
                <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
                <span>{{ t('common.refresh') }}</span>
              </button>
            </div>
          </div>

          <div v-if="loading" class="flex items-center justify-center py-12">
            <LoadingSpinner />
          </div>

          <div v-else-if="error" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800/60 dark:bg-red-900/20 dark:text-red-300">
            {{ error }}
          </div>

          <div v-else-if="filteredModels.length === 0" class="rounded-lg bg-gray-50 py-12 text-center dark:bg-gray-800/70">
            <Icon name="cube" size="xl" class="mx-auto text-gray-300 dark:text-gray-600" />
            <p class="mt-3 text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.modelMarketplace.empty') }}</p>
          </div>

          <div v-else class="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
            <article v-for="model in filteredModels" :key="model.model_id" class="rounded-lg border border-gray-200 p-4 dark:border-gray-800">
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white" :title="model.display_name">
                      {{ model.display_name }}
                    </h2>
                    <span
                      v-for="item in model.platforms"
                      :key="item"
                      class="rounded-md px-2 py-0.5 text-xs font-medium"
                      :class="platformClass(item)"
                    >
                      {{ platformLabel(item) }}
                    </span>
                  </div>
                  <p class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-gray-400">{{ model.model_id }}</p>
                </div>
                <div class="flex shrink-0 items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-gray-800 dark:text-gray-300">
                  <Icon name="server" size="xs" />
                  {{ model.account_count }}
                </div>
              </div>

              <div class="mt-4 grid grid-cols-3 gap-2">
                <div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-800/70">
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelMarketplace.inputPrice') }}</p>
                  <p class="mt-1 text-sm font-bold text-gray-900 dark:text-white">{{ money(model.price.input_price_per_mtok) }}</p>
                </div>
                <div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-800/70">
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelMarketplace.outputPrice') }}</p>
                  <p class="mt-1 text-sm font-bold text-gray-900 dark:text-white">{{ money(model.price.output_price_per_mtok) }}</p>
                </div>
                <div class="rounded-lg bg-gray-50 p-3 dark:bg-gray-800/70">
                  <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.modelMarketplace.totalPrice') }}</p>
                  <p class="mt-1 text-sm font-bold text-gray-900 dark:text-white">{{ money(model.price.total_price_per_mtok) }}</p>
                </div>
              </div>

              <div class="mt-3 flex min-h-6 flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                <span>{{ t('admin.modelMarketplace.perMillionTokens') }}</span>
                <span v-if="model.price.source_model_id && model.price.source_model_id !== model.model_id" class="break-all">
                  {{ t('admin.modelMarketplace.pricedBy', { model: model.price.source_model_id }) }}
                </span>
                <span v-else-if="!model.price.available" class="text-amber-600 dark:text-amber-400">
                  {{ t('admin.modelMarketplace.priceMissing') }}
                </span>
              </div>
            </article>
          </div>
        </div>
      </details>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import type {
  ModelMarketplaceResponse,
  PublicStatusConfigAdminView,
  PublicStatusGroupConfig,
  PublicStatusGroupOption,
  PublicStatusModelConfig,
  PublicStatusProbeLineConfig
} from '@/types'

const { t } = useI18n()

const data = ref<ModelMarketplaceResponse | null>(null)
const loading = ref(false)
const error = ref('')
const search = ref('')
const platform = ref('')
const statusConfig = ref<PublicStatusConfigAdminView | null>(null)
const statusConfigLoading = ref(false)
const statusConfigSaving = ref(false)
const statusConfigError = ref('')
const newStatusModelName = ref('')

const models = computed(() => data.value?.models ?? [])
const platformOptions = computed(() => {
  const set = new Set<string>()
  for (const model of models.value) {
    for (const item of model.platforms) set.add(item)
  }
  return Array.from(set).sort()
})

const filteredModels = computed(() => {
  const q = search.value.trim().toLowerCase()
  return models.value.filter((model) => {
    const matchesPlatform = !platform.value || model.platforms.includes(platform.value)
    if (!matchesPlatform) return false
    if (!q) return true
    return [
      model.model_id,
      model.display_name,
      ...(model.upstream_names ?? []),
      ...model.platforms
    ].some((value) => value.toLowerCase().includes(q))
  })
})

const pricedCount = computed(() => models.value.filter((model) => model.price.available).length)
const statusModelOptions = computed(() => statusConfig.value?.config.models ?? [])
const enabledModelCount = computed(() => statusModelOptions.value.filter((model) => model.enabled).length)
const enabledGroupCount = computed(() => {
  if (!statusConfig.value) return 0
  return statusConfig.value.group_options.filter((group) => editableGroup(group.group_id).enabled).length
})

function createStatusModelConfig(name: string): PublicStatusModelConfig {
  return {
    name,
    provider: '',
    prompt_caching: false,
    enabled: true
  }
}

function addStatusModel() {
  if (!statusConfig.value) return
  const name = newStatusModelName.value.trim()
  if (!name) return
  const exists = statusConfig.value.config.models.some(
    (model) => model.name.toLowerCase() === name.toLowerCase()
  )
  if (!exists) {
    statusConfig.value.config.models.push(createStatusModelConfig(name))
  }
  newStatusModelName.value = ''
}

function removeStatusModel(name: string) {
  if (!statusConfig.value) return
  statusConfig.value.config.models = statusConfig.value.config.models.filter(
    (model) => model.name !== name
  )
  for (const group of statusConfig.value.config.groups) {
    if (group.models?.length) {
      group.models = group.models.filter((model) => model !== name)
    }
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    data.value = await adminAPI.modelMarketplace.list()
  } catch (err: any) {
    error.value = err?.message || t('admin.modelMarketplace.loadFailed')
  } finally {
    loading.value = false
  }
}

async function loadStatusConfig() {
  statusConfigLoading.value = true
  statusConfigError.value = ''
  try {
    statusConfig.value = await adminAPI.modelMarketplace.getStatusConfig()
  } catch (err: any) {
    statusConfigError.value = err?.message || '加载公开状态页配置失败'
  } finally {
    statusConfigLoading.value = false
  }
}

async function saveStatusConfig() {
  if (!statusConfig.value) return
  statusConfigSaving.value = true
  statusConfigError.value = ''
  try {
    statusConfig.value = await adminAPI.modelMarketplace.updateStatusConfig(statusConfig.value.config)
  } catch (err: any) {
    statusConfigError.value = err?.message || '保存公开状态页配置失败'
  } finally {
    statusConfigSaving.value = false
  }
}

function editableGroup(groupID: number): PublicStatusGroupConfig {
  if (!statusConfig.value) {
    return { group_id: groupID, enabled: false }
  }
  let item = statusConfig.value.config.groups.find((group) => group.group_id === groupID)
  if (!item) {
    item = { group_id: groupID, enabled: false, display_name: '', aggregate_key: '', probe_lines: [] }
    statusConfig.value.config.groups.push(item)
  }
  return item
}

function setGroupEnabled(groupID: number, enabled: boolean, option: PublicStatusGroupOption) {
  const item = editableGroup(groupID)
  item.enabled = enabled
  if (enabled) {
    if (!item.display_name) item.display_name = option.suggested_name || option.name
    if (!item.aggregate_key) item.aggregate_key = option.suggested_key || ''
  }
}

function onGroupEnabledChange(option: PublicStatusGroupOption, event: Event) {
  const checked = (event.target as HTMLInputElement | null)?.checked ?? false
  setGroupEnabled(option.group_id, checked, option)
}

function onGroupDisplayNameInput(groupID: number, event: Event) {
  editableGroup(groupID).display_name = (event.target as HTMLInputElement | null)?.value ?? ''
}

function onGroupAggregateKeyInput(groupID: number, event: Event) {
  editableGroup(groupID).aggregate_key = (event.target as HTMLInputElement | null)?.value ?? ''
}

function onGroupSortOrderInput(groupID: number, event: Event) {
  const raw = (event.target as HTMLInputElement | null)?.value?.trim() ?? ''
  const n = Number.parseInt(raw, 10)
  editableGroup(groupID).sort_order = raw && Number.isFinite(n) ? n : undefined
}

function editableProbeLines(groupID: number): PublicStatusProbeLineConfig[] {
  const item = editableGroup(groupID)
  if (!item.probe_lines) {
    item.probe_lines = []
  }
  return item.probe_lines
}

function addProbeLine(groupID: number) {
  const lines = editableProbeLines(groupID)
  lines.push({
    id: `line-${Date.now()}-${lines.length + 1}`,
    name: lines.length === 0 ? '默认线路' : `线路 ${lines.length + 1}`,
    region: '',
    enabled: true,
    sort_order: lines.length + 1
  })
}

function removeProbeLine(groupID: number, index: number) {
  editableProbeLines(groupID).splice(index, 1)
}

function groupUsesModel(groupID: number, modelName: string): boolean {
  const selected = editableGroup(groupID).models
  return !selected || selected.length === 0 || selected.includes(modelName)
}

function groupModelSummary(groupID: number): string {
  const selected = editableGroup(groupID).models
  if (!selected || selected.length === 0 || selected.length === statusModelOptions.value.length) {
    return '全部模型'
  }
  return `${selected.length} 个模型`
}

function groupLineSummary(groupID: number, fallbackName: string): string {
  const lines = editableProbeLines(groupID)
  const enabled = lines.filter((line) => line.enabled !== false)
  if (enabled.length === 0) return fallbackName || '默认线路'
  if (enabled.length === 1) return enabled[0].name || enabled[0].region || fallbackName || '默认线路'
  return `${enabled.length} 条线路`
}

function onGroupModelChange(groupID: number, modelName: string, event: Event) {
  const checked = (event.target as HTMLInputElement | null)?.checked ?? false
  const item = editableGroup(groupID)
  const allModels = statusModelOptions.value.map((model) => model.name)
  const selected = new Set(item.models && item.models.length > 0 ? item.models : allModels)
  if (checked) {
    selected.add(modelName)
  } else {
    selected.delete(modelName)
  }
  const next = allModels.filter((name) => selected.has(name))
  item.models = next.length === allModels.length ? [] : next
}

function shortModelName(name: string): string {
  return name
    .replace(/^claude-/, '')
    .replace(/-20\d{6}$/, '')
}

function money(value: number | null | undefined): string {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return '-'
  }
  return `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })}`
}

function platformLabel(value: string): string {
  return t(`admin.modelMarketplace.platforms.${value}`, value)
}

function platformClass(value: string): string {
  const classes: Record<string, string> = {
    openai: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
    anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300',
    gemini: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
    antigravity: 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-300',
    sora: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300'
  }
  return classes[value] ?? 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300'
}

onMounted(() => {
  load()
  loadStatusConfig()
})
</script>
