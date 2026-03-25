<template>
  <div class="space-y-4">
    <div
      v-if="!isSupportedPlatform"
      class="flex items-start gap-3 rounded-lg border border-yellow-200 bg-yellow-50 p-4 dark:border-yellow-800 dark:bg-yellow-900/20"
    >
      <Icon name="exclamationCircle" size="md" class="mt-0.5 flex-shrink-0 text-yellow-500" />
      <div>
        <p class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
          {{ t('keys.clientInstallModal.unsupportedTitle') }}
        </p>
        <p class="mt-1 text-sm text-yellow-700 dark:text-yellow-300">
          {{ t('keys.clientInstallModal.unsupportedDescription') }}
        </p>
      </div>
    </div>

    <template v-else>
      <p class="text-sm text-gray-600 dark:text-gray-400">
        {{ description }}
      </p>

      <div class="grid grid-cols-2 gap-3">
        <button
          v-for="option in clientOptions"
          :key="option.id"
          type="button"
          :class="[
            'rounded-xl border p-4 text-left transition-colors',
            selectedClient === option.id
              ? 'border-primary-500 bg-primary-50 dark:border-primary-500 dark:bg-primary-900/20'
              : 'border-gray-200 hover:border-primary-300 dark:border-dark-600 dark:hover:border-primary-600'
          ]"
          @click="selectedClient = option.id"
        >
          <p class="text-sm font-medium text-gray-900 dark:text-white">
            {{ option.label }}
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ option.description }}
          </p>
        </button>
      </div>

      <div class="border-b border-gray-200 dark:border-dark-700">
        <nav class="-mb-px flex space-x-5" aria-label="OS">
          <button
            v-for="option in osOptions"
            :key="option.id"
            type="button"
            :class="[
              'border-b-2 px-1 py-2.5 text-sm font-medium transition-colors',
              selectedOs === option.id
                ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
            ]"
            @click="selectedOs = option.id"
          >
            {{ option.label }}
          </button>
        </nav>
      </div>

      <div
        v-if="selectedClient === 'openclaw'"
        class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <p class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('keys.clientInstallModal.modelTitle') }}
        </p>
        <div class="mt-3 flex flex-wrap gap-2">
          <button
            v-for="option in openclawModels"
            :key="option.value"
            type="button"
            :class="[
              'rounded-lg border px-3 py-2 text-sm transition-colors',
              selectedOpenClawModel === option.value
                ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-500 dark:bg-primary-900/20 dark:text-primary-300'
                : 'border-gray-200 text-gray-600 hover:border-primary-300 dark:border-dark-600 dark:text-gray-300 dark:hover:border-primary-600'
            ]"
            @click="selectedOpenClawModel = option.value"
          >
            {{ option.label }}
          </button>
        </div>
      </div>

      <div class="rounded-xl border border-gray-200 dark:border-dark-700">
        <div class="flex items-center justify-between border-b border-gray-200 px-4 py-2 dark:border-dark-700">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">
              {{ t('keys.clientInstallModal.commandTitle') }}
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ currentSummary }}
            </p>
          </div>
          <button
            type="button"
            class="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
            @click="copyCommand"
          >
            {{ copied ? t('keys.clientInstallModal.copied') : t('keys.clientInstallModal.copy') }}
          </button>
        </div>
        <pre class="overflow-x-auto bg-gray-950 p-4 text-sm text-gray-100"><code>{{ currentCommand }}</code></pre>
      </div>

      <div class="rounded-lg border border-blue-100 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-900/20">
        <p class="text-sm text-blue-700 dark:text-blue-300">
          {{ note }}
        </p>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import type { GroupPlatform } from '@/types'

interface Props {
  apiKey: string
  baseUrl: string
  platform: GroupPlatform | null
}

type ClientType = 'claude' | 'openclaw'
type OsType = 'unix' | 'windows'

const props = defineProps<Props>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const selectedClient = ref<ClientType>('claude')
const selectedOs = ref<OsType>('unix')
const selectedOpenClawModel = ref('anthropic/claude-sonnet-4-6')
const copied = ref(false)

watch(() => props.platform, () => {
  selectedClient.value = 'claude'
  selectedOs.value = 'unix'
  selectedOpenClawModel.value = 'anthropic/claude-sonnet-4-6'
  copied.value = false
}, { immediate: true })

const isSupportedPlatform = computed(() => props.platform === 'anthropic' || props.platform === 'antigravity')

const scriptBaseUrl = computed(() => {
  if (typeof window === 'undefined' || !window.location?.origin) {
    return 'https://xuedingtoken.com'
  }
  return window.location.origin.replace(/\/+$/, '')
})

const normalizedBaseRoot = computed(() => {
  const fallback = scriptBaseUrl.value
  const source = (props.baseUrl || fallback).trim() || fallback
  return source.replace(/\/v1\/?$/, '').replace(/\/+$/, '')
})

const effectiveApiUrl = computed(() => {
  if (props.platform === 'antigravity') {
    return `${normalizedBaseRoot.value}/antigravity`
  }
  return normalizedBaseRoot.value
})

const clientOptions = computed(() => [
  {
    id: 'claude' as const,
    label: t('keys.clientInstallModal.clients.claude.label'),
    description: t('keys.clientInstallModal.clients.claude.description')
  },
  {
    id: 'openclaw' as const,
    label: t('keys.clientInstallModal.clients.openclaw.label'),
    description: t('keys.clientInstallModal.clients.openclaw.description')
  }
])

const osOptions = computed(() => [
  { id: 'unix' as const, label: t('keys.clientInstallModal.os.unix') },
  { id: 'windows' as const, label: t('keys.clientInstallModal.os.windows') }
])

const openclawModels = computed(() => [
  { value: 'anthropic/claude-sonnet-4-6', label: t('keys.clientInstallModal.models.sonnet46') },
  { value: 'anthropic/claude-opus-4-6', label: t('keys.clientInstallModal.models.opus46') },
  { value: 'anthropic/claude-haiku-4-5', label: t('keys.clientInstallModal.models.haiku45') }
])

const description = computed(() => (
  selectedClient.value === 'claude'
    ? t('keys.clientInstallModal.claudeDescription')
    : t('keys.clientInstallModal.openclawDescription')
))

const note = computed(() => {
  if (selectedClient.value === 'claude') {
    return t('keys.clientInstallModal.claudeNote')
  }
  return selectedOs.value === 'windows'
    ? t('keys.clientInstallModal.openclawWindowsNote')
    : t('keys.clientInstallModal.openclawNote')
})

const currentSummary = computed(() => {
  if (selectedClient.value === 'claude') {
    return selectedOs.value === 'unix'
      ? t('keys.clientInstallModal.summary.claudeUnix')
      : t('keys.clientInstallModal.summary.claudeWindows')
  }

  const modelLabel = openclawModels.value.find((option) => option.value === selectedOpenClawModel.value)?.label ?? selectedOpenClawModel.value
  return selectedOs.value === 'unix'
    ? t('keys.clientInstallModal.summary.openclawUnix', { model: modelLabel })
    : t('keys.clientInstallModal.summary.openclawWindows', { model: modelLabel })
})

const currentCommand = computed(() => {
  if (selectedClient.value === 'claude') {
    if (selectedOs.value === 'unix') {
      return `CLAUDE_TOKEN="${props.apiKey}" CLAUDE_API_URL="${effectiveApiUrl.value}" bash -c "$(curl -fsSL ${scriptBaseUrl.value}/install-claude.sh)"`
    }
    return `$env:CLAUDE_CLIENT_TOKEN='${escapePowerShell(props.apiKey)}'; $env:CLAUDE_API_URL='${escapePowerShell(effectiveApiUrl.value)}'; irm ${scriptBaseUrl.value}/install-claude-win.ps1 | iex`
  }

  if (selectedOs.value === 'unix') {
    return `OPENCLAW_TOKEN="${props.apiKey}" OPENCLAW_BASE_URL="${effectiveApiUrl.value}" OPENCLAW_MODEL="${selectedOpenClawModel.value}" OPENCLAW_INSTALLER_BASE="${scriptBaseUrl.value}" bash -c "$(curl -fsSL ${scriptBaseUrl.value}/install-openclaw.sh)"`
  }
  return `$env:OPENCLAW_TOKEN='${escapePowerShell(props.apiKey)}'; $env:OPENCLAW_BASE_URL='${escapePowerShell(effectiveApiUrl.value)}'; $env:OPENCLAW_MODEL='${escapePowerShell(selectedOpenClawModel.value)}'; $env:OPENCLAW_INSTALLER_BASE='${escapePowerShell(scriptBaseUrl.value)}'; irm ${scriptBaseUrl.value}/install-openclaw-win.ps1 | iex`
})

function escapePowerShell(value: string): string {
  return value.replace(/'/g, "''")
}

async function copyCommand() {
  const ok = await copyToClipboard(currentCommand.value)
  if (!ok) return
  copied.value = true
  window.setTimeout(() => {
    copied.value = false
  }, 1500)
}
</script>
