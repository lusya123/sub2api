<template>
  <AppLayout>
    <div class="mx-auto flex min-h-[55vh] w-full max-w-5xl items-center justify-center px-4 py-10">
      <section class="w-full rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-900 sm:p-6">
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-lg bg-sky-100 text-sky-600 dark:bg-sky-900/30 dark:text-sky-300">
          <Icon name="chat" size="lg" />
        </div>
        <h1 class="mt-4 text-center text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('chatLaunch.title') }}
        </h1>
        <p class="mx-auto mt-2 max-w-2xl text-center text-sm text-gray-500 dark:text-dark-400">
          {{ isOpening ? t('chatLaunch.opening') : loading ? t('chatLaunch.loadingModels') : t('chatLaunch.description') }}
        </p>

        <div v-if="loading" class="mt-8 flex justify-center">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>

        <div v-else-if="modelOptions.length > 0" class="mt-6 grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div class="min-w-0 rounded-lg border border-gray-200 dark:border-dark-700">
            <div class="border-b border-gray-200 px-3 py-2 dark:border-dark-700 sm:px-4">
              <input
                v-model="search"
                class="w-full bg-transparent text-sm text-gray-900 outline-none placeholder:text-gray-400 dark:text-white"
                :placeholder="t('chatLaunch.searchPlaceholder')"
              />
            </div>
            <div class="grid max-h-[420px] gap-2 overflow-y-auto p-3 sm:grid-cols-2 sm:p-4">
              <button
                v-for="option in filteredModelOptions"
                :key="option.model"
                type="button"
                :aria-pressed="selectedModel === option.model"
                :class="[
                  'rounded-lg border p-3 text-left transition-colors',
                  selectedModel === option.model
                    ? 'border-primary-500 bg-primary-50 dark:border-primary-500 dark:bg-primary-900/20'
                    : 'border-gray-200 hover:border-primary-300 dark:border-dark-600 dark:hover:border-primary-600'
                ]"
                @click="selectModel(option.model)"
              >
                <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                  {{ option.displayName }}
                </p>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
                  {{ t('chatLaunch.groupCount', { count: option.providers.length }) }}
                </p>
              </button>
            </div>
          </div>

          <aside class="rounded-lg border border-gray-200 p-3 dark:border-dark-700 sm:p-4">
            <p class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ selectedModelOption?.displayName || t('chatLaunch.selectModel') }}
            </p>
            <div class="mt-3 space-y-2">
              <button
                v-for="provider in selectedProviders"
                :key="provider.id"
                type="button"
                :aria-pressed="selectedProvider === provider.id"
                :class="[
                  'w-full rounded-lg border p-3 text-left transition-colors',
                  selectedProvider === provider.id
                    ? 'border-primary-500 bg-primary-50 dark:border-primary-500 dark:bg-primary-900/20'
                    : 'border-gray-200 hover:border-primary-300 dark:border-dark-600 dark:hover:border-primary-600'
                ]"
                @click="selectedProvider = provider.id"
              >
                <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                  {{ provider.display_name }}
                </p>
                <p class="mt-1 text-xs uppercase text-gray-500 dark:text-dark-400">
                  {{ provider.sdk_type }}
                </p>
              </button>
            </div>
          </aside>
        </div>

        <div v-else class="mt-6 rounded-lg border border-yellow-200 bg-yellow-50 p-4 text-sm text-yellow-800 dark:border-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-200">
          {{ t('chatLaunch.noModels') }}
        </div>

        <button
          type="button"
          class="btn-primary mx-auto mt-5 w-full justify-center sm:max-w-sm"
          :disabled="isOpening || loading || !canOpen"
          @click="openChat"
        >
          {{ isOpening ? t('chatLaunch.openingButton') : t('chatLaunch.openButton') }}
        </button>
        <p v-if="hasError" class="mt-3 text-sm text-red-600 dark:text-red-400">
          {{ t('chatLaunch.failed') }}
        </p>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { chatAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import type { LobeProviderConfig } from '@/api/chat'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const isOpening = ref(false)
const hasError = ref(false)
const search = ref('')
const providers = ref<LobeProviderConfig[]>([])
const selectedModel = ref('')
const selectedProvider = ref('')

interface ModelOption {
  model: string
  displayName: string
  providers: LobeProviderConfig[]
}

const modelOptions = computed<ModelOption[]>(() => {
  const byModel = new Map<string, ModelOption>()
  for (const provider of providers.value) {
    for (const model of provider.models || []) {
      if (!model.id) continue
      const existing = byModel.get(model.id)
      if (existing) {
        existing.providers.push(provider)
        continue
      }
      byModel.set(model.id, {
        model: model.id,
        displayName: model.display_name || model.id,
        providers: [provider]
      })
    }
  }
  return Array.from(byModel.values()).sort((a, b) => a.displayName.localeCompare(b.displayName))
})

const filteredModelOptions = computed(() => {
  const query = search.value.trim().toLowerCase()
  if (!query) return modelOptions.value
  return modelOptions.value.filter((option) =>
    option.model.toLowerCase().includes(query) ||
    option.displayName.toLowerCase().includes(query) ||
    option.providers.some((provider) => provider.display_name.toLowerCase().includes(query))
  )
})

const selectedModelOption = computed(() =>
  modelOptions.value.find((option) => option.model === selectedModel.value) || null
)
const selectedProviders = computed(() => selectedModelOption.value?.providers || [])
const canOpen = computed(() => {
  return Boolean(selectedModel.value && selectedProvider.value)
})

function selectModel(model: string) {
  selectedModel.value = model
  const currentProviders = modelOptions.value.find((option) => option.model === model)?.providers || []
  if (!currentProviders.some((provider) => provider.id === selectedProvider.value)) {
    selectedProvider.value = currentProviders[0]?.id || ''
  }
}

async function loadModels() {
  loading.value = true
  try {
    const config = await chatAPI.getLobeConfig()
    providers.value = config.providers || []
    const first = modelOptions.value[0]
    if (first) {
      selectModel(first.model)
    }
  } catch {
    providers.value = []
  } finally {
    loading.value = false
  }
}

async function openChat() {
  if (isOpening.value) return
  isOpening.value = true
  hasError.value = false

  try {
    const { url } = selectedProvider.value && selectedModel.value
      ? await chatAPI.launchWithModel(selectedProvider.value, selectedModel.value)
      : await chatAPI.launch()
    if (!url) throw new Error('Missing chat launch URL')
    window.location.assign(url)
  } catch {
    isOpening.value = false
    hasError.value = true
    appStore.showError(t('dashboard.chatOpenFailed'))
  }
}

onMounted(loadModels)
</script>
