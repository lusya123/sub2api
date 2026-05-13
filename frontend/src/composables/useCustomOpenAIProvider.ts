import { computed, type Ref } from 'vue'
import {
  MODEL_MARKETPLACE_PROVIDER_OPTIONS,
  getModelMarketplaceProviderLabel
} from '@/constants/modelMarketplaceMonitor'

export const DEFAULT_CUSTOM_OPENAI_PROVIDER = 'deepseek'
export const OPENAI_DEFAULT_BASE_URL = 'https://api.openai.com'

const EXCLUDED_CUSTOM_ACCOUNT_PROVIDER_VALUES = new Set([
  'openai',
  'anthropic',
  'gemini',
  'aws',
  'vertex_ai',
  'midjourney',
  'midjourney_plus',
  'sunoapi',
  'kling',
  'jimeng',
  'vidu',
  'doubao_video',
  'sora',
  'replicate'
])

const CUSTOM_PROVIDER_BASE_URLS: Record<string, string> = {
  deepseek: 'https://api.deepseek.com',
  moonshot: 'https://api.moonshot.cn',
  zhipu: 'https://open.bigmodel.cn/api/paas',
  zhipu_v4: 'https://open.bigmodel.cn/api/paas',
  ali: 'https://dashscope.aliyuncs.com/compatible-mode',
  volcengine: 'https://ark.cn-beijing.volces.com/api',
  xai: 'https://api.x.ai',
  mistral: 'https://api.mistral.ai',
  openrouter: 'https://openrouter.ai/api',
  siliconflow: 'https://api.siliconflow.cn',
  perplexity: 'https://api.perplexity.ai',
  minimax: 'https://api.minimax.chat',
  cloudflare: 'https://api.cloudflare.com/client/v4/accounts',
  custom: OPENAI_DEFAULT_BASE_URL
}

const CUSTOM_PROVIDER_MODEL_KEY_BY_PROVIDER: Record<string, string> = {
  ali: 'qwen',
  baidu_v2: 'baidu',
  lingyiwanwu: 'yi',
  tencent: 'hunyuan',
  xunfei: 'spark',
  volcengine: 'doubao',
  zhipu_v4: 'zhipu'
}

const CUSTOM_PROVIDER_MODEL_KEYS = new Set([
  'openai',
  'zhipu',
  'qwen',
  'deepseek',
  'mistral',
  'meta',
  'xai',
  'cohere',
  'yi',
  'moonshot',
  'doubao',
  'minimax',
  'baidu',
  'spark',
  'hunyuan',
  'perplexity'
])

const KNOWN_CUSTOM_PROVIDER_BASE_URLS = new Set([
  OPENAI_DEFAULT_BASE_URL,
  ...Object.values(CUSTOM_PROVIDER_BASE_URLS).filter(Boolean)
])

export const customOpenAIProviderOptions = MODEL_MARKETPLACE_PROVIDER_OPTIONS
  .filter((provider) => !EXCLUDED_CUSTOM_ACCOUNT_PROVIDER_VALUES.has(String(provider.value)))
  .map((provider) => ({ value: String(provider.value), label: provider.label }))

export function getCustomOpenAIProviderLabel(provider: string): string {
  return getModelMarketplaceProviderLabel(provider)
}

export function getCustomOpenAIProviderBaseUrl(provider: string): string {
  return CUSTOM_PROVIDER_BASE_URLS[provider] || ''
}

export function getCustomOpenAIProviderModelKey(provider: string): string {
  const providerKey = CUSTOM_PROVIDER_MODEL_KEY_BY_PROVIDER[provider] || provider
  return CUSTOM_PROVIDER_MODEL_KEYS.has(providerKey) ? providerKey : 'openai'
}

export function isKnownCustomOpenAIProviderBaseUrl(value: string): boolean {
  const trimmed = value.trim()
  if (!trimmed) return true
  return KNOWN_CUSTOM_PROVIDER_BASE_URLS.has(trimmed)
}

export function buildCustomOpenAIProviderExtra(provider: string, modelKey: string): Record<string, unknown> {
  return {
    custom_provider: provider,
    custom_provider_label: getCustomOpenAIProviderLabel(provider),
    custom_provider_model_key: modelKey,
    openai_compatible_provider: true
  }
}

export function useCustomOpenAIProvider(provider: Ref<string>) {
  return {
    customProviderOptions: customOpenAIProviderOptions,
    selectedCustomProviderLabel: computed(() => getCustomOpenAIProviderLabel(provider.value)),
    suggestedCustomProviderBaseUrl: computed(() => getCustomOpenAIProviderBaseUrl(provider.value))
  }
}
