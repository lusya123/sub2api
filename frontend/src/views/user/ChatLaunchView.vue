<template>
  <div class="use-token-fullscreen">
    <header class="use-token-topbar">
      <div class="use-token-topbar__left">
        <router-link :to="consolePath" class="use-token-brand" :title="siteName">
          <span class="use-token-brand__logo" v-if="settingsLoaded">
            <img :src="siteLogo || '/logo.png'" :alt="siteName" />
          </span>
          <span class="use-token-brand__name">{{ siteName }}</span>
        </router-link>
        <nav class="use-token-nav" :aria-label="t('useToken.title')">
          <router-link :to="consolePath" class="use-token-nav__item">
            {{ t('nav.console') }}
          </router-link>
          <router-link to="/model-marketplace" class="use-token-nav__item">
            {{ t('nav.modelMarketplace') }}
          </router-link>
        </nav>
      </div>

      <div class="use-token-topbar__center">
        <div
          v-if="visibleModes.length > 1"
          ref="pillRef"
          class="use-token-pill"
          role="tablist"
          :aria-label="t('useToken.title')"
        >
          <span class="use-token-pill__slider" :style="sliderStyle" aria-hidden="true" />
          <button
            v-for="m in visibleModes"
            :key="m.key"
            :ref="(el) => registerPillButton(m.key, el as HTMLButtonElement | null)"
            type="button"
            role="tab"
            :aria-selected="mode === m.key"
            :class="[
              'use-token-pill__option',
              mode === m.key && 'use-token-pill__option--active',
              firstHintMode === m.key && 'use-token-pill__option--pulse'
            ]"
            @click="switchTo(m.key)"
            @mouseenter="hoveredMode = m.key"
            @mouseleave="hoveredMode = ''"
            @focus="hoveredMode = m.key"
            @blur="hoveredMode = ''"
          >
            <Icon :name="m.icon" size="sm" />
            <span>{{ m.label }}</span>
          </button>
        </div>
      </div>

      <div class="use-token-topbar__right">
        <button
          type="button"
          class="use-token-action"
          :class="{ 'use-token-action--active': overflowOpen }"
          :title="t('common.more')"
          :aria-label="t('common.more')"
          :aria-expanded="overflowOpen"
          @click="overflowOpen = !overflowOpen"
        >
          <Icon name="more" size="sm" />
        </button>

        <Transition name="use-token-pop">
          <div v-if="overflowOpen" class="use-token-overflow" role="menu" @click="overflowOpen = false">
            <button class="use-token-overflow__item" type="button" @click="refreshCurrent">
              <Icon name="refresh" size="sm" />
              <span>{{ t('useToken.refresh') }}</span>
            </button>
            <button class="use-token-overflow__item" type="button" @click="openCurrentInNewTab">
              <Icon name="externalLink" size="sm" />
              <span>{{ t('useToken.openInNewTab') }}</span>
            </button>
            <button class="use-token-overflow__item" type="button" @click="showAdvanced = true">
              <Icon name="cog" size="sm" />
              <span>{{ t('useToken.advanced') }}</span>
            </button>
          </div>
        </Transition>
      </div>

      <Transition name="use-token-tooltip">
        <div v-if="tooltipMessage" class="use-token-tooltip" role="status" aria-live="polite">
          {{ tooltipMessage }}
        </div>
      </Transition>
    </header>

    <main class="use-token-stage">
      <iframe
        v-for="m in visibleModes"
        :key="m.key + '-' + frameKeys[m.key]"
        :src="frameUrls[m.key] || 'about:blank'"
        :class="[
          'use-token-frame',
          mode === m.key && 'use-token-frame--visible'
        ]"
        :title="m.label"
        :aria-hidden="mode !== m.key"
        referrerpolicy="strict-origin-when-cross-origin"
        sandbox="allow-same-origin allow-scripts allow-forms allow-popups allow-popups-to-escape-sandbox allow-downloads allow-modals allow-presentation allow-clipboard-read allow-clipboard-write"
      />

      <div
        v-if="!currentFrameUrl"
        class="use-token-placeholder"
        :class="{ 'use-token-placeholder--error': !!launchError }"
        role="status"
      >
        <div class="use-token-placeholder__dot" />
        <p>{{ launchError || t('useToken.preparing') }}</p>
      </div>
    </main>

    <Transition name="use-token-fade">
      <div
        v-if="showAdvanced"
        class="use-token-modal-backdrop"
        role="dialog"
        aria-modal="true"
        @click.self="showAdvanced = false"
      >
        <section class="use-token-modal" :aria-label="t('useToken.advanced')">
          <header class="use-token-modal__head">
            <h2>{{ t('useToken.advanced') }}</h2>
            <button class="use-token-modal__close" type="button" :aria-label="t('common.close')" @click="showAdvanced = false">
              <Icon name="x" size="sm" />
            </button>
          </header>
          <p class="use-token-modal__hint">{{ t('useToken.advancedHint') }}</p>
          <div class="use-token-modal__grid">
            <label class="use-token-field" v-if="mode === 'agent'">
              <span>{{ t('chatLaunch.agent.apiKey') }}</span>
              <select v-model="selectedAgentKey" :disabled="agentLoading">
                <option value="">{{ t('chatLaunch.agent.selectApiKey') }}</option>
                <option v-for="key in compatibleAgentKeys" :key="key.id" :value="key.key">
                  {{ key.name }} · {{ maskApiKey(key.key) }}
                </option>
              </select>
            </label>
            <label class="use-token-field">
              <span>{{ t('useToken.modelLabel') }}</span>
              <select :value="selectedModel" :disabled="modelOptions.length === 0" @change="handleModelSelect">
                <option value="">{{ t('chatLaunch.selectModel') }}</option>
                <option v-for="option in modelOptions" :key="option.model" :value="option.model">
                  {{ option.displayName }}
                </option>
              </select>
            </label>
            <label class="use-token-field">
              <span>{{ t('useToken.providerLabel') }}</span>
              <select v-model="selectedProvider" :disabled="selectedProviders.length === 0">
                <option value="">{{ t('chatLaunch.selectModel') }}</option>
                <option v-for="provider in selectedProviders" :key="provider.id" :value="provider.id">
                  {{ provider.display_name }} · {{ provider.sdk_type }}
                </option>
              </select>
            </label>
          </div>
        </section>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { chatAPI, keysAPI } from '@/api'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import type { LobeProviderConfig } from '@/api/chat'
import type { ApiKey } from '@/types'

type ModeKey = 'chat' | 'agent'
interface ModeDescriptor {
  key: ModeKey
  label: string
  icon: 'chat' | 'bolt'
}
interface ModelOption {
  model: string
  displayName: string
  providers: LobeProviderConfig[]
}

const STORAGE_LAST_MODE = 'sub2api:use-token:last-mode'
const STORAGE_HINT_SEEN = 'sub2api:use-token:hint-seen'
const FIRST_HINT_TARGET: ModeKey = 'agent'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.siteLogo)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const consolePath = computed(() => (authStore.canAccessAdmin ? '/admin/dashboard' : '/dashboard'))

const providers = ref<LobeProviderConfig[]>([])
const agentKeys = ref<ApiKey[]>([])
const selectedModel = ref('')
const selectedProvider = ref('')
const selectedAgentKey = ref('')

const loading = ref(false)
const keysLoading = ref(false)
const launchError = ref('')

const mode = ref<ModeKey>('chat')
const frameUrls = ref<Record<ModeKey, string>>({ chat: '', agent: '' })
const frameKeys = ref<Record<ModeKey, number>>({ chat: 0, agent: 0 })

const overflowOpen = ref(false)
const showAdvanced = ref(false)
const hoveredMode = ref<ModeKey | ''>('')
const firstHintMode = ref<ModeKey | ''>('')

const pillRef = ref<HTMLElement | null>(null)
const pillButtons = ref<Record<string, HTMLButtonElement | null>>({})
const sliderStyle = ref<Record<string, string>>({ opacity: '0' })

const chatAppEnabled = computed(() => appStore.cachedPublicSettings?.chat_page_enabled !== false)
const agentAppEnabled = computed(() => appStore.cachedPublicSettings?.agent_page_enabled !== false)

const allModes: ModeDescriptor[] = [
  { key: 'chat', label: '', icon: 'chat' },
  { key: 'agent', label: '', icon: 'bolt' }
]
const visibleModes = computed<ModeDescriptor[]>(() =>
  allModes
    .filter((m) => (m.key === 'chat' ? chatAppEnabled.value : agentAppEnabled.value))
    .map((m) => ({ ...m, label: t(`useToken.modes.${m.key}`) }))
)

const currentFrameUrl = computed(() => frameUrls.value[mode.value])
const tooltipMessage = computed(() => {
  if (hoveredMode.value && hoveredMode.value !== mode.value) {
    return t(`useToken.explanations.${hoveredMode.value}`)
  }
  if (firstHintMode.value) {
    return t(`useToken.explanations.${firstHintMode.value}`)
  }
  return ''
})

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

const selectedModelOption = computed(() =>
  modelOptions.value.find((option) => option.model === selectedModel.value) || null
)
const selectedProviders = computed(() => selectedModelOption.value?.providers || [])
const selectedProviderOption = computed(() =>
  selectedProviders.value.find((provider) => provider.id === selectedProvider.value) || null
)
const selectedProviderGroupId = computed(() => {
  const match = selectedProvider.value.match(/^sub2api-group-(\d+)$/)
  return match ? Number(match[1]) : null
})
const activeAgentKeys = computed(() => agentKeys.value.filter((key) => key.status === 'active' && key.key))
const compatibleAgentKeys = computed(() => {
  const groupId = selectedProviderGroupId.value
  return activeAgentKeys.value.filter((key) => {
    if (groupId === null) return true
    return key.group_id === null || key.group_id === groupId
  })
})
const agentLoading = computed(() => loading.value || keysLoading.value)

function registerPillButton(key: ModeKey, el: HTMLButtonElement | null) {
  pillButtons.value[key] = el
}

function updateSlider() {
  const button = pillButtons.value[mode.value]
  const pill = pillRef.value
  if (!button || !pill) {
    sliderStyle.value = { opacity: '0' }
    return
  }
  const buttonRect = button.getBoundingClientRect()
  const pillRect = pill.getBoundingClientRect()
  sliderStyle.value = {
    opacity: '1',
    transform: `translateX(${buttonRect.left - pillRect.left}px)`,
    width: `${buttonRect.width}px`
  }
}

function readStoredMode(): ModeKey | null {
  try {
    const stored = localStorage.getItem(STORAGE_LAST_MODE)
    if (stored === 'chat' || stored === 'agent') return stored
  } catch {
    /* ignore */
  }
  return null
}

function persistMode(next: ModeKey) {
  try {
    localStorage.setItem(STORAGE_LAST_MODE, next)
  } catch {
    /* ignore */
  }
}

function maybeShowFirstHint() {
  if (visibleModes.value.length < 2) return
  try {
    if (localStorage.getItem(STORAGE_HINT_SEEN)) return
  } catch {
    return
  }
  firstHintMode.value = FIRST_HINT_TARGET
  setTimeout(() => {
    firstHintMode.value = ''
    try {
      localStorage.setItem(STORAGE_HINT_SEEN, '1')
    } catch {
      /* ignore */
    }
  }, 5200)
}

function handleModelSelect(event: Event) {
  const target = event.target as HTMLSelectElement | null
  selectModel(target?.value || '')
}

function selectModel(modelId: string) {
  selectedModel.value = modelId
  const currentProviders = modelOptions.value.find((option) => option.model === modelId)?.providers || []
  if (!currentProviders.some((provider) => provider.id === selectedProvider.value)) {
    selectedProvider.value = currentProviders[0]?.id || ''
  }
}

function selectRecommendedAgentModel(): boolean {
  const preferredModels = ['gpt-5.4-mini', 'gpt-5.2', 'gpt-5.4']
  const openAIOptions = modelOptions.value
    .map((option) => ({
      option,
      provider: option.providers.find((provider) => provider.sdk_type === 'openai')
    }))
    .filter((item): item is { option: ModelOption; provider: LobeProviderConfig } => Boolean(item.provider))
  if (openAIOptions.length === 0) return false
  const preferred = preferredModels
    .map((modelId) => openAIOptions.find((item) => item.option.model === modelId))
    .find(Boolean) || openAIOptions[0]
  selectedModel.value = preferred.option.model
  selectedProvider.value = preferred.provider.id
  return true
}

function selectDefaultAgentKey() {
  const currentStillCompatible = compatibleAgentKeys.value.some((key) => key.key === selectedAgentKey.value)
  if (currentStillCompatible) return
  selectedAgentKey.value = compatibleAgentKeys.value[0]?.key || ''
}

function maskApiKey(key: string): string {
  if (!key) return ''
  if (key.length <= 10) return `${key.slice(0, 3)}...`
  return `${key.slice(0, 6)}...${key.slice(-4)}`
}

function resolveManusBaseUrl(): string {
  const fromSettings = String(appStore.cachedPublicSettings?.agent_page_url || '').trim()
  if (fromSettings) return fromSettings
  const configured = String(import.meta.env.VITE_MANUS_BASE_URL || '').trim()
  return configured || 'http://127.0.0.1:5174'
}

function createAgentKeyName(): string {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `Agent ${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}`
}

async function ensureAgentKey(): Promise<string> {
  if (selectedAgentKey.value) return selectedAgentKey.value
  const created = await keysAPI.create(createAgentKeyName(), selectedProviderGroupId.value)
  agentKeys.value = [created, ...agentKeys.value.filter((key) => key.id !== created.id)]
  selectedAgentKey.value = created.key
  return created.key
}

async function loadModels() {
  loading.value = true
  try {
    const config = await chatAPI.getLobeConfig()
    providers.value = config.providers || []
    if (!selectedModel.value) {
      if (!selectRecommendedAgentModel()) {
        const first = modelOptions.value[0]
        if (first) selectModel(first.model)
      }
    }
  } catch {
    providers.value = []
  } finally {
    loading.value = false
  }
}

async function loadAgentKeys() {
  keysLoading.value = true
  try {
    const result = await keysAPI.list(1, 100, { status: 'active' })
    agentKeys.value = result.items || []
    selectDefaultAgentKey()
  } catch {
    agentKeys.value = []
  } finally {
    keysLoading.value = false
  }
}

async function prepareChatUrl(): Promise<string> {
  const { url } = selectedProvider.value && selectedModel.value
    ? await chatAPI.launchWithModel(selectedProvider.value, selectedModel.value)
    : await chatAPI.launch()
  if (!url) throw new Error('Missing chat launch URL')
  return url
}

async function prepareAgentUrl(): Promise<string> {
  const provider = selectedProviderOption.value
  if (!provider) throw new Error('agent provider not ready')
  const apiKey = await ensureAgentKey()
  const target = new URL('/chat', resolveManusBaseUrl())
  const params = new URLSearchParams()
  params.set('manus_api_key', apiKey)
  params.set('manus_model', selectedModel.value)
  params.set('manus_model_provider', provider.sdk_type || provider.id)
  if (provider.base_url) {
    params.set('manus_api_base', provider.base_url)
  }
  target.hash = params.toString()
  const bridge = new URL('/manus/launch', window.location.origin)
  bridge.searchParams.set('redirect_uri', target.toString())
  return bridge.toString()
}

async function prewarmChat() {
  if (!chatAppEnabled.value || frameUrls.value.chat) return
  try {
    frameUrls.value.chat = await prepareChatUrl()
  } catch {
    if (mode.value === 'chat') launchError.value = t('chatLaunch.failed')
  }
}

async function prewarmAgent() {
  if (!agentAppEnabled.value || frameUrls.value.agent) return
  try {
    frameUrls.value.agent = await prepareAgentUrl()
  } catch {
    if (mode.value === 'agent') launchError.value = t('chatLaunch.agent.failed')
  }
}

async function switchTo(next: ModeKey) {
  if (!visibleModes.value.some((m) => m.key === next)) return
  mode.value = next
  persistMode(next)
  launchError.value = ''
  if (firstHintMode.value) firstHintMode.value = ''
  await nextTick()
  updateSlider()
  if (next === 'chat') {
    if (!frameUrls.value.chat) await prewarmChat()
  } else if (!frameUrls.value.agent) {
    if (selectedProviderOption.value) {
      await prewarmAgent()
    }
  }
}

async function refreshCurrent() {
  launchError.value = ''
  try {
    const next = mode.value === 'chat' ? await prepareChatUrl() : await prepareAgentUrl()
    frameUrls.value[mode.value] = next
    frameKeys.value[mode.value] += 1
  } catch {
    launchError.value = mode.value === 'chat' ? t('chatLaunch.failed') : t('chatLaunch.agent.failed')
  }
}

async function openCurrentInNewTab() {
  try {
    const url = frameUrls.value[mode.value]
      || (mode.value === 'chat' ? await prepareChatUrl() : await prepareAgentUrl())
    window.open(url, '_blank', 'noopener,noreferrer')
  } catch {
    launchError.value = mode.value === 'chat' ? t('chatLaunch.failed') : t('chatLaunch.agent.failed')
  }
}

function handleDocClick(event: MouseEvent) {
  if (!overflowOpen.value) return
  const target = event.target as HTMLElement | null
  if (!target) return
  if (target.closest('.use-token-topbar__right')) return
  overflowOpen.value = false
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    if (showAdvanced.value) {
      showAdvanced.value = false
      return
    }
    if (overflowOpen.value) {
      overflowOpen.value = false
    }
  }
}

watch([selectedProvider, agentKeys], selectDefaultAgentKey)

const advancedDirty = { chat: false, agent: false }
watch([selectedModel, selectedProvider], () => {
  if (!showAdvanced.value) return
  advancedDirty.chat = true
  advancedDirty.agent = true
})
watch(selectedAgentKey, () => {
  if (!showAdvanced.value) return
  advancedDirty.agent = true
})
watch(showAdvanced, async (open) => {
  if (open) return
  if (advancedDirty.chat) {
    frameUrls.value.chat = ''
    advancedDirty.chat = false
    if (chatAppEnabled.value) await prewarmChat()
  }
  if (advancedDirty.agent) {
    frameUrls.value.agent = ''
    advancedDirty.agent = false
    if (agentAppEnabled.value && selectedProviderOption.value) await prewarmAgent()
  }
})

watch(visibleModes, (modes) => {
  if (modes.length === 0) return
  if (!modes.some((m) => m.key === mode.value)) {
    mode.value = modes[0].key
  }
  nextTick(updateSlider)
})

watch(mode, () => {
  nextTick(updateSlider)
})

onMounted(async () => {
  const stored = readStoredMode()
  if (stored && visibleModes.value.some((m) => m.key === stored)) {
    mode.value = stored
  }

  await Promise.allSettled([loadModels(), loadAgentKeys()])
  await nextTick()
  updateSlider()
  maybeShowFirstHint()

  void prewarmChat()
  if (agentAppEnabled.value) {
    const ready = selectedProviderOption.value || selectRecommendedAgentModel()
    if (ready) void prewarmAgent()
  }

  window.addEventListener('resize', updateSlider, { passive: true })
  document.addEventListener('click', handleDocClick, true)
  document.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateSlider)
  document.removeEventListener('click', handleDocClick, true)
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<style scoped>
.use-token-fullscreen {
  position: fixed;
  inset: 0;
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
  background: #fafaf9;
  color: #1f2937;
  overflow: hidden;
  z-index: 40;
}
:global(.dark) .use-token-fullscreen {
  background: #0f1115;
  color: rgba(255, 255, 255, 0.92);
}

.use-token-topbar {
  position: relative;
  flex-shrink: 0;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  height: 48px;
  padding: 0 12px;
  background: #fafaf9;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}
:global(.dark) .use-token-topbar {
  background: #0f1115;
  border-bottom-color: rgba(255, 255, 255, 0.06);
}

.use-token-topbar__left {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}
.use-token-topbar__center {
  display: flex;
  align-items: center;
  justify-content: center;
}
.use-token-topbar__right {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
}

.use-token-brand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px 4px 4px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: #111827;
  text-decoration: none;
  transition: background 160ms ease-out;
  flex-shrink: 0;
}
.use-token-brand:hover {
  background: rgba(0, 0, 0, 0.04);
}
:global(.dark) .use-token-brand {
  color: rgba(255, 255, 255, 0.95);
}
:global(.dark) .use-token-brand:hover {
  background: rgba(255, 255, 255, 0.05);
}
.use-token-brand__logo {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  overflow: hidden;
  background: rgba(0, 0, 0, 0.06);
}
.use-token-brand__logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
:global(.dark) .use-token-brand__logo {
  background: rgba(255, 255, 255, 0.06);
}
@media (max-width: 640px) {
  .use-token-brand__name {
    display: none;
  }
}

.use-token-nav {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
.use-token-nav__item {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  letter-spacing: -0.005em;
  color: #6b7280;
  text-decoration: none;
  transition: color 160ms ease-out, background 160ms ease-out;
  white-space: nowrap;
}
.use-token-nav__item:hover {
  color: #111827;
  background: rgba(0, 0, 0, 0.04);
}
:global(.dark) .use-token-nav__item {
  color: rgba(255, 255, 255, 0.55);
}
:global(.dark) .use-token-nav__item:hover {
  color: rgba(255, 255, 255, 0.95);
  background: rgba(255, 255, 255, 0.05);
}
@media (max-width: 720px) {
  .use-token-nav {
    display: none;
  }
}

.use-token-pill {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 3px;
  border-radius: 9999px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(255, 255, 255, 0.6);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
}
:global(.dark) .use-token-pill {
  border-color: rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.04);
}

.use-token-pill__slider {
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 0;
  border-radius: 9999px;
  background: #ffffff;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08), 0 1px 1px rgba(15, 23, 42, 0.04);
  transition: transform 240ms cubic-bezier(0.32, 0.72, 0, 1),
              width 240ms cubic-bezier(0.32, 0.72, 0, 1),
              opacity 180ms ease-out;
  pointer-events: none;
  will-change: transform, width;
}
:global(.dark) .use-token-pill__slider {
  background: rgba(255, 255, 255, 0.12);
  box-shadow: none;
}

.use-token-pill__option {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 500;
  letter-spacing: -0.01em;
  color: #6b7280;
  background: transparent;
  border: 0;
  border-radius: 9999px;
  cursor: pointer;
  transition: color 180ms ease-out;
}
.use-token-pill__option:hover {
  color: #111827;
}
.use-token-pill__option--active,
.use-token-pill__option--active:hover {
  color: #111827;
  font-weight: 600;
}
:global(.dark) .use-token-pill__option {
  color: rgba(255, 255, 255, 0.55);
}
:global(.dark) .use-token-pill__option:hover,
:global(.dark) .use-token-pill__option--active {
  color: rgba(255, 255, 255, 0.95);
}

.use-token-pill__option--pulse::after {
  content: '';
  position: absolute;
  inset: -3px;
  border-radius: 9999px;
  border: 2px solid rgba(99, 102, 241, 0.55);
  opacity: 0;
  animation: useTokenPulseRing 1.6s ease-out 0.3s infinite;
  pointer-events: none;
}
@keyframes useTokenPulseRing {
  0% { opacity: 0; transform: scale(0.9); }
  35% { opacity: 0.85; transform: scale(1); }
  100% { opacity: 0; transform: scale(1.18); }
}

.use-token-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 9999px;
  border: 1px solid transparent;
  background: transparent;
  color: #6b7280;
  cursor: pointer;
  transition: color 160ms ease-out, background 160ms ease-out, border-color 160ms ease-out;
}
.use-token-action:hover,
.use-token-action--active {
  color: #111827;
  background: rgba(0, 0, 0, 0.05);
  border-color: rgba(0, 0, 0, 0.06);
}
:global(.dark) .use-token-action {
  color: rgba(255, 255, 255, 0.55);
}
:global(.dark) .use-token-action:hover,
:global(.dark) .use-token-action--active {
  color: rgba(255, 255, 255, 0.95);
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.1);
}

.use-token-overflow {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 180px;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  background: #ffffff;
  border: 1px solid rgba(0, 0, 0, 0.06);
  border-radius: 12px;
  box-shadow: 0 12px 32px -8px rgba(15, 23, 42, 0.12), 0 4px 12px -4px rgba(15, 23, 42, 0.05);
  z-index: 5;
}
:global(.dark) .use-token-overflow {
  background: #16181d;
  border-color: rgba(255, 255, 255, 0.06);
  box-shadow: 0 12px 32px -8px rgba(0, 0, 0, 0.5);
}
.use-token-overflow__item {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  font-size: 13px;
  color: #1f2937;
  text-align: left;
  cursor: pointer;
  transition: background 140ms ease-out;
}
.use-token-overflow__item:hover {
  background: rgba(0, 0, 0, 0.04);
}
:global(.dark) .use-token-overflow__item {
  color: rgba(255, 255, 255, 0.9);
}
:global(.dark) .use-token-overflow__item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.use-token-tooltip {
  position: absolute;
  top: calc(100% + 8px);
  left: 50%;
  transform: translateX(-50%);
  padding: 8px 12px;
  max-width: min(560px, calc(100vw - 32px));
  font-size: 12.5px;
  line-height: 1.5;
  color: #4b5563;
  background: #ffffff;
  border: 1px solid rgba(0, 0, 0, 0.06);
  border-radius: 10px;
  box-shadow: 0 8px 24px -8px rgba(15, 23, 42, 0.12);
  text-align: center;
  white-space: normal;
  pointer-events: none;
  z-index: 4;
}
:global(.dark) .use-token-tooltip {
  color: rgba(255, 255, 255, 0.75);
  background: #16181d;
  border-color: rgba(255, 255, 255, 0.06);
}

.use-token-tooltip-enter-active,
.use-token-tooltip-leave-active {
  transition: opacity 180ms ease-out, transform 180ms ease-out;
}
.use-token-tooltip-enter-from,
.use-token-tooltip-leave-to {
  opacity: 0;
  transform: translate(-50%, -4px);
}

.use-token-pop-enter-active,
.use-token-pop-leave-active {
  transition: opacity 160ms ease-out, transform 160ms ease-out;
}
.use-token-pop-enter-from,
.use-token-pop-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}

.use-token-stage {
  position: relative;
  flex: 1;
  overflow: hidden;
  background: #fafaf9;
}
:global(.dark) .use-token-stage {
  background: #0f1115;
}
.use-token-frame {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  border: 0;
  background: #ffffff;
  opacity: 0;
  pointer-events: none;
  transition: opacity 220ms ease-out;
}
:global(.dark) .use-token-frame {
  background: #0f1115;
}
.use-token-frame--visible {
  opacity: 1;
  pointer-events: auto;
}

.use-token-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 14px;
  color: #6b7280;
  font-size: 13px;
}
:global(.dark) .use-token-placeholder {
  color: rgba(255, 255, 255, 0.5);
}
.use-token-placeholder--error {
  color: #b91c1c;
}
:global(.dark) .use-token-placeholder--error {
  color: #fca5a5;
}
.use-token-placeholder__dot {
  width: 10px;
  height: 10px;
  border-radius: 9999px;
  background: currentColor;
  opacity: 0.4;
  animation: useTokenPulse 1.4s ease-in-out infinite;
}
@keyframes useTokenPulse {
  0%, 100% { opacity: 0.2; transform: scale(0.9); }
  50% { opacity: 0.9; transform: scale(1); }
}

.use-token-modal-backdrop {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(15, 17, 21, 0.32);
  backdrop-filter: blur(6px);
  -webkit-backdrop-filter: blur(6px);
  z-index: 60;
}
.use-token-modal {
  width: 100%;
  max-width: 480px;
  padding: 20px;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 24px 60px -12px rgba(15, 23, 42, 0.25);
}
:global(.dark) .use-token-modal {
  background: #16181d;
  box-shadow: 0 24px 60px -12px rgba(0, 0, 0, 0.6);
}
.use-token-modal__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}
.use-token-modal__head h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.use-token-modal__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 9999px;
  border: 0;
  background: transparent;
  color: #6b7280;
  cursor: pointer;
  transition: background 160ms ease-out, color 160ms ease-out;
}
.use-token-modal__close:hover {
  background: rgba(0, 0, 0, 0.05);
  color: #111827;
}
:global(.dark) .use-token-modal__close {
  color: rgba(255, 255, 255, 0.55);
}
:global(.dark) .use-token-modal__close:hover {
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.95);
}
.use-token-modal__hint {
  margin: 0 0 14px;
  font-size: 12.5px;
  color: #6b7280;
}
:global(.dark) .use-token-modal__hint {
  color: rgba(255, 255, 255, 0.5);
}
.use-token-modal__grid {
  display: grid;
  gap: 12px;
}
.use-token-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12.5px;
  color: #374151;
}
:global(.dark) .use-token-field {
  color: rgba(255, 255, 255, 0.7);
}
.use-token-field select {
  width: 100%;
  padding: 8px 10px;
  font-size: 13px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  background: #ffffff;
  color: inherit;
  outline: none;
  transition: border-color 160ms ease-out, box-shadow 160ms ease-out;
}
.use-token-field select:focus {
  border-color: rgba(0, 0, 0, 0.4);
  box-shadow: 0 0 0 3px rgba(0, 0, 0, 0.05);
}
:global(.dark) .use-token-field select {
  border-color: rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.04);
  color: rgba(255, 255, 255, 0.9);
}

.use-token-fade-enter-active,
.use-token-fade-leave-active {
  transition: opacity 180ms ease-out;
}
.use-token-fade-enter-from,
.use-token-fade-leave-to {
  opacity: 0;
}
</style>
