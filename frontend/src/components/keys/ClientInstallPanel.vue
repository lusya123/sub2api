<template>
  <div class="space-y-4">
    <div
      v-if="!isSupportedPlatform"
      class="flex items-start gap-3 rounded-lg border border-yellow-200 bg-yellow-50 p-4 dark:border-yellow-800 dark:bg-yellow-900/20"
    >
      <Icon name="exclamationCircle" size="md" class="mt-0.5 flex-shrink-0 text-yellow-500" />
      <div>
        <p class="text-sm font-medium text-yellow-800 dark:text-yellow-200">
          {{ unsupportedTitle }}
        </p>
        <p class="mt-1 text-sm text-yellow-700 dark:text-yellow-300">
          {{ unsupportedDescription }}
        </p>
      </div>
    </div>

    <template v-else>
      <p class="text-sm text-gray-600 dark:text-gray-400">
        {{ description }}
      </p>

      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <button
          v-for="option in clientOptions"
          :key="option.id"
          type="button"
          :aria-pressed="selectedClient === option.id"
          :class="[
            'rounded-xl border p-3 text-left transition-colors sm:p-4',
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
        <nav class="-mb-px flex flex-wrap gap-x-5 gap-y-2 overflow-x-auto" aria-label="OS">
          <button
            v-for="option in osOptions"
            :key="option.id"
            type="button"
            :aria-pressed="selectedOs === option.id"
            :class="[
              'whitespace-nowrap border-b-2 px-1 py-2.5 text-sm font-medium transition-colors',
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
          {{ modelTitle }}
        </p>
        <div class="mt-3 flex flex-wrap gap-2">
          <button
            v-for="option in openclawModels"
            :key="option.value"
            type="button"
            :aria-pressed="selectedOpenClawModel === option.value"
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

      <div :key="commandPanelKey" class="rounded-xl border border-gray-200 dark:border-dark-700">
        <div class="flex flex-col gap-3 border-b border-gray-200 px-4 py-3 sm:flex-row sm:items-center sm:justify-between dark:border-dark-700">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">
              {{ commandTitle }}
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ currentSummary }}
            </p>
          </div>
          <button
            type="button"
            class="w-full rounded-lg bg-gray-100 px-3 py-2 text-xs font-medium text-gray-700 transition-colors hover:bg-gray-200 sm:w-auto dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
            @click="copyCommand"
          >
            {{ copyLabel }}
          </button>
        </div>
        <pre class="overflow-x-auto bg-gray-950 p-4 text-xs text-gray-100 sm:text-sm"><code class="whitespace-pre-wrap break-all sm:whitespace-pre sm:break-normal">{{ currentCommand }}</code></pre>
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

const { t, locale } = useI18n()
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
const isZhLocale = computed(() => locale.value.toLowerCase().startsWith('zh'))

function interpolate(template: string, params?: Record<string, string>): string {
  if (!params) return template
  return Object.entries(params).reduce(
    (result, [name, value]) => result.split(`{${name}}`).join(value),
    template
  )
}

function safeT(
  key: string,
  fallbacks: { zh: string; en: string },
  params?: Record<string, string>
): string {
  try {
    return params ? t(key, params) : t(key)
  } catch (error) {
    console.warn(`[ClientInstallPanel] Failed to translate ${key}, using fallback.`, error)
    return interpolate(isZhLocale.value ? fallbacks.zh : fallbacks.en, params)
  }
}

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
    label: safeT('keys.clientInstallModal.clients.claude.label', {
      zh: 'Claude Code',
      en: 'Claude Code',
    }),
    description: safeT('keys.clientInstallModal.clients.claude.description', {
      zh: '安装 Claude Code 与 CC Switch，并自动导入当前供应商。',
      en: 'Install Claude Code and CC Switch, then import this provider automatically.',
    })
  },
  {
    id: 'openclaw' as const,
    label: safeT('keys.clientInstallModal.clients.openclaw.label', {
      zh: 'OpenClaw',
      en: 'OpenClaw',
    }),
    description: safeT('keys.clientInstallModal.clients.openclaw.description', {
      zh: '安装官方 OpenClaw，并把当前密钥设为默认认证。',
      en: 'Install the official OpenClaw package and set this API key as default auth.',
    })
  }
])

const osOptions = computed(() => [
  {
    id: 'unix' as const,
    label: safeT('keys.clientInstallModal.os.unix', {
      zh: 'macOS',
      en: 'macOS',
    }),
  },
  {
    id: 'windows' as const,
    label: safeT('keys.clientInstallModal.os.windows', {
      zh: 'Windows',
      en: 'Windows',
    }),
  }
])

const openclawModels = computed(() => [
  {
    value: 'anthropic/claude-sonnet-4-6',
    label: safeT('keys.clientInstallModal.models.sonnet46', { zh: 'Sonnet 4.6', en: 'Sonnet 4.6' }),
  },
  {
    value: 'anthropic/claude-opus-4-6',
    label: safeT('keys.clientInstallModal.models.opus46', { zh: 'Opus 4.6', en: 'Opus 4.6' }),
  },
  {
    value: 'anthropic/claude-haiku-4-5',
    label: safeT('keys.clientInstallModal.models.haiku45', { zh: 'Haiku 4.5', en: 'Haiku 4.5' }),
  }
])

const unsupportedTitle = computed(() => safeT('keys.clientInstallModal.unsupportedTitle', {
  zh: '当前分组暂不支持此安装方式',
  en: 'This group is not supported',
}))

const unsupportedDescription = computed(() => safeT('keys.clientInstallModal.unsupportedDescription', {
  zh: '只有 Anthropic 和 Antigravity 分组会生成 Claude Code / OpenClaw 一键安装命令。',
  en: 'Only Anthropic and Antigravity groups generate Claude Code / OpenClaw one-click install commands.',
}))

const modelTitle = computed(() => safeT('keys.clientInstallModal.modelTitle', {
  zh: 'OpenClaw 默认模型',
  en: 'Default OpenClaw Model',
}))

const commandTitle = computed(() => safeT('keys.clientInstallModal.commandTitle', {
  zh: '一键命令',
  en: 'Install Command',
}))

const copyLabel = computed(() => (
  copied.value
    ? safeT('keys.clientInstallModal.copied', { zh: '已复制', en: 'Copied' })
    : safeT('keys.clientInstallModal.copy', { zh: '复制', en: 'Copy' })
))

const description = computed(() => (
  selectedClient.value === 'claude'
    ? safeT('keys.clientInstallModal.claudeDescription', {
      zh: '为当前 API Key 生成 Claude Code 一键安装命令。脚本会安装官方 npm 包并把代理地址持久化到本机环境变量中。',
      en: 'Generate a one-click Claude Code deployment command for this API key. The script installs the official npm package and persists the proxy endpoint in local environment variables.',
    })
    : safeT('keys.clientInstallModal.openclawDescription', {
      zh: '为当前 API Key 生成 OpenClaw 一键部署命令。脚本会安装官方 openclaw npm 包，并写入 ~/.openclaw 配置。',
      en: 'Generate a one-click OpenClaw deployment command for this API key. The script installs the official openclaw npm package and writes config into ~/.openclaw.',
    })
))

const note = computed(() => {
  if (selectedClient.value === 'claude') {
    return safeT('keys.clientInstallModal.claudeNote', {
      zh: 'Claude Code 脚本安装的是官方 @anthropic-ai/claude-code 包，包下载优先使用 npmmirror。安装后建议重新打开终端再执行 claude。',
      en: 'The Claude Code script installs the official @anthropic-ai/claude-code package and prefers npmmirror for package downloads. Reopen your terminal after installation, then run claude.',
    })
  }
  return selectedOs.value === 'windows'
    ? safeT('keys.clientInstallModal.openclawWindowsNote', {
      zh: 'OpenClaw 官方仍更推荐在 Windows 上通过 WSL2 使用。这里提供的是原生 PowerShell 部署命令，并要求 Node.js 22.16+；如果系统策略或 Node 环境受限，优先考虑 WSL。',
      en: 'OpenClaw officially still recommends WSL2 on Windows. A native PowerShell deployment command is provided here and requires Node.js 22.16+; prefer WSL if local policy or Node setup causes issues.',
    })
    : safeT('keys.clientInstallModal.openclawNote', {
      zh: 'OpenClaw 脚本安装的是官方 openclaw npm 包，并把默认模型和认证信息写入 ~/.openclaw。脚本会优先准备 Node.js 22.16+，包下载优先使用 npmmirror。',
      en: 'The OpenClaw script installs the official openclaw npm package and writes the default model plus auth config into ~/.openclaw. It also prepares Node.js 22.16+ and prefers npmmirror for downloads.',
    })
})

const commandPanelKey = computed(() => (
  `${selectedClient.value}:${selectedOs.value}:${selectedOpenClawModel.value}:${props.apiKey}:${effectiveApiUrl.value}`
))

const currentSummary = computed(() => {
  if (selectedClient.value === 'claude') {
    return selectedOs.value === 'unix'
      ? safeT('keys.clientInstallModal.summary.claudeUnix', {
        zh: 'Claude Code · macOS · CC Switch 自动导入',
        en: 'Claude Code · macOS · CC Switch auto import',
      })
      : safeT('keys.clientInstallModal.summary.claudeWindows', {
        zh: 'Claude Code · Windows PowerShell',
        en: 'Claude Code · Windows PowerShell',
      })
  }

  const modelLabel = openclawModels.value.find((option) => option.value === selectedOpenClawModel.value)?.label ?? selectedOpenClawModel.value
  return selectedOs.value === 'unix'
    ? safeT('keys.clientInstallModal.summary.openclawUnix', {
      zh: 'OpenClaw · macOS / Linux / WSL · 默认模型 {model}',
      en: 'OpenClaw · macOS / Linux / WSL · Default model {model}',
    }, { model: modelLabel })
    : safeT('keys.clientInstallModal.summary.openclawWindows', {
      zh: 'OpenClaw · Windows PowerShell · 默认模型 {model}',
      en: 'OpenClaw · Windows PowerShell · Default model {model}',
    }, { model: modelLabel })
})

const currentCommand = computed(() => {
  if (selectedClient.value === 'claude') {
    if (selectedOs.value === 'unix') {
      return `XDT_TOKEN="${props.apiKey}" XDT_API_URL="${effectiveApiUrl.value}" bash -c "$(curl -fsSL ${scriptBaseUrl.value}/install-claude-ccswitch.sh)"`
    }
    return `$env:XDT_TOKEN='${escapePowerShell(props.apiKey)}'; $env:XDT_API_URL='${escapePowerShell(effectiveApiUrl.value)}'; irm ${scriptBaseUrl.value}/install-claude-ccswitch-win.ps1 | iex`
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

watch([selectedClient, selectedOs, selectedOpenClawModel, () => props.apiKey, effectiveApiUrl], () => {
  copied.value = false
})
</script>
