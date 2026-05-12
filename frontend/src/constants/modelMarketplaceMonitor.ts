/**
 * Channel monitor shared constants.
 *
 * Single source of truth for provider/status string values used by both the
 * admin (`views/admin/ModelMarketplaceMonitorView.vue`) and user-facing
 * (`views/user/ChannelStatusView.vue`) screens, plus the shared composable
 * `useModelMarketplaceMonitorFormat`.
 */

import type { Provider, MonitorStatus } from '@/api/admin/modelMarketplaceMonitor'

export interface ModelMarketplaceProviderOption {
  value: Provider
  label: string
  newApiType: number
}

export const PROVIDER_OPENAI: Provider = 'openai'
export const PROVIDER_OPENAI_MAX: Provider = 'openai_max'
export const PROVIDER_OHMYGPT: Provider = 'ohmygpt'
export const PROVIDER_CUSTOM: Provider = 'custom'
export const PROVIDER_AILS: Provider = 'ails'
export const PROVIDER_AI_PROXY: Provider = 'ai_proxy'
export const PROVIDER_API2GPT: Provider = 'api2gpt'
export const PROVIDER_AIGC2D: Provider = 'aigc2d'
export const PROVIDER_ANTHROPIC: Provider = 'anthropic'
export const PROVIDER_AWS: Provider = 'aws'
export const PROVIDER_GEMINI: Provider = 'gemini'
export const PROVIDER_DEEPSEEK: Provider = 'deepseek'
export const PROVIDER_AZURE: Provider = 'azure'
export const PROVIDER_VERTEX_AI: Provider = 'vertex_ai'
export const PROVIDER_XAI: Provider = 'xai'
export const PROVIDER_MISTRAL: Provider = 'mistral'
export const PROVIDER_COHERE: Provider = 'cohere'
export const PROVIDER_OPENROUTER: Provider = 'openrouter'
export const PROVIDER_OLLAMA: Provider = 'ollama'
export const PROVIDER_SILICONFLOW: Provider = 'siliconflow'
export const PROVIDER_PERPLEXITY: Provider = 'perplexity'
export const PROVIDER_MOONSHOT: Provider = 'moonshot'
export const PROVIDER_ALI: Provider = 'ali'
export const PROVIDER_ZHIPU_V4: Provider = 'zhipu_v4'
export const PROVIDER_BAIDU: Provider = 'baidu'
export const PROVIDER_ZHIPU: Provider = 'zhipu'
export const PROVIDER_BAIDU_V2: Provider = 'baidu_v2'
export const PROVIDER_TENCENT: Provider = 'tencent'
export const PROVIDER_XUNFEI: Provider = 'xunfei'
export const PROVIDER_VOLCENGINE: Provider = 'volcengine'
export const PROVIDER_LINGYIWANWU: Provider = 'lingyiwanwu'
export const PROVIDER_MINIMAX: Provider = 'minimax'
export const PROVIDER_COZE: Provider = 'coze'
export const PROVIDER_AI360: Provider = 'ai360'
export const PROVIDER_XINFERENCE: Provider = 'xinference'
export const PROVIDER_DIFY: Provider = 'dify'
export const PROVIDER_JINA: Provider = 'jina'
export const PROVIDER_CLOUDFLARE: Provider = 'cloudflare'
export const PROVIDER_PALM: Provider = 'palm'
export const PROVIDER_CODEX: Provider = 'codex'
export const PROVIDER_FASTGPT: Provider = 'fastgpt'
export const PROVIDER_AI_PROXY_LIBRARY: Provider = 'ai_proxy_library'
export const PROVIDER_MOKAAI: Provider = 'mokaai'
export const PROVIDER_MIDJOURNEY: Provider = 'midjourney'
export const PROVIDER_MIDJOURNEY_PLUS: Provider = 'midjourney_plus'
export const PROVIDER_SUNOAPI: Provider = 'sunoapi'
export const PROVIDER_KLING: Provider = 'kling'
export const PROVIDER_JIMENG: Provider = 'jimeng'
export const PROVIDER_VIDU: Provider = 'vidu'
export const PROVIDER_SUBMODEL: Provider = 'submodel'
export const PROVIDER_DOUBAO_VIDEO: Provider = 'doubao_video'
export const PROVIDER_SORA: Provider = 'sora'
export const PROVIDER_REPLICATE: Provider = 'replicate'

// Mirrors New API's channel type table/order, with stable string ids for Sub2API.
export const MODEL_MARKETPLACE_PROVIDER_OPTIONS: readonly ModelMarketplaceProviderOption[] = [
  { value: PROVIDER_OPENAI, label: 'OpenAI', newApiType: 1 },
  { value: PROVIDER_OPENAI_MAX, label: 'OpenAIMax', newApiType: 6 },
  { value: PROVIDER_OHMYGPT, label: 'OhMyGPT', newApiType: 7 },
  { value: PROVIDER_CUSTOM, label: 'Custom', newApiType: 8 },
  { value: PROVIDER_AILS, label: 'AILS', newApiType: 9 },
  { value: PROVIDER_AI_PROXY, label: 'AI Proxy', newApiType: 10 },
  { value: PROVIDER_API2GPT, label: 'API2GPT', newApiType: 12 },
  { value: PROVIDER_AIGC2D, label: 'AIGC2D', newApiType: 13 },
  { value: PROVIDER_ANTHROPIC, label: 'Anthropic', newApiType: 14 },
  { value: PROVIDER_AWS, label: 'AWS', newApiType: 33 },
  { value: PROVIDER_GEMINI, label: 'Gemini', newApiType: 24 },
  { value: PROVIDER_DEEPSEEK, label: 'DeepSeek', newApiType: 43 },
  { value: PROVIDER_AZURE, label: 'Azure', newApiType: 3 },
  { value: PROVIDER_VERTEX_AI, label: 'Vertex AI', newApiType: 41 },
  { value: PROVIDER_XAI, label: 'xAI', newApiType: 48 },
  { value: PROVIDER_MISTRAL, label: 'Mistral', newApiType: 42 },
  { value: PROVIDER_COHERE, label: 'Cohere', newApiType: 34 },
  { value: PROVIDER_OPENROUTER, label: 'OpenRouter', newApiType: 20 },
  { value: PROVIDER_OLLAMA, label: 'Ollama', newApiType: 4 },
  { value: PROVIDER_SILICONFLOW, label: 'SiliconFlow', newApiType: 40 },
  { value: PROVIDER_PERPLEXITY, label: 'Perplexity', newApiType: 27 },
  { value: PROVIDER_MOONSHOT, label: 'Moonshot', newApiType: 25 },
  { value: PROVIDER_ALI, label: 'Ali', newApiType: 17 },
  { value: PROVIDER_ZHIPU_V4, label: 'Zhipu V4', newApiType: 26 },
  { value: PROVIDER_BAIDU, label: 'Baidu', newApiType: 15 },
  { value: PROVIDER_BAIDU_V2, label: 'Baidu V2', newApiType: 46 },
  { value: PROVIDER_TENCENT, label: 'Tencent', newApiType: 23 },
  { value: PROVIDER_XUNFEI, label: 'Xunfei', newApiType: 18 },
  { value: PROVIDER_VOLCENGINE, label: 'VolcEngine', newApiType: 45 },
  { value: PROVIDER_LINGYIWANWU, label: 'LingYiWanWu', newApiType: 31 },
  { value: PROVIDER_MINIMAX, label: 'MiniMax', newApiType: 35 },
  { value: PROVIDER_COZE, label: 'Coze', newApiType: 49 },
  { value: PROVIDER_AI360, label: '360', newApiType: 19 },
  { value: PROVIDER_XINFERENCE, label: 'Xinference', newApiType: 47 },
  { value: PROVIDER_DIFY, label: 'Dify', newApiType: 37 },
  { value: PROVIDER_JINA, label: 'Jina', newApiType: 38 },
  { value: PROVIDER_CLOUDFLARE, label: 'Cloudflare', newApiType: 39 },
  { value: PROVIDER_PALM, label: 'PaLM', newApiType: 11 },
  { value: PROVIDER_CODEX, label: 'Codex', newApiType: 57 },
  { value: PROVIDER_FASTGPT, label: 'FastGPT', newApiType: 22 },
  { value: PROVIDER_AI_PROXY_LIBRARY, label: 'AI Proxy Library', newApiType: 21 },
  { value: PROVIDER_MOKAAI, label: 'MokaAI', newApiType: 44 },
  { value: PROVIDER_MIDJOURNEY, label: 'Midjourney', newApiType: 2 },
  { value: PROVIDER_MIDJOURNEY_PLUS, label: 'MidjourneyPlus', newApiType: 5 },
  { value: PROVIDER_SUNOAPI, label: 'SunoAPI', newApiType: 36 },
  { value: PROVIDER_KLING, label: 'Kling', newApiType: 50 },
  { value: PROVIDER_JIMENG, label: 'Jimeng', newApiType: 51 },
  { value: PROVIDER_VIDU, label: 'Vidu', newApiType: 52 },
  { value: PROVIDER_SUBMODEL, label: 'Submodel', newApiType: 53 },
  { value: PROVIDER_DOUBAO_VIDEO, label: 'DoubaoVideo', newApiType: 54 },
  { value: PROVIDER_SORA, label: 'Sora', newApiType: 55 },
  { value: PROVIDER_REPLICATE, label: 'Replicate', newApiType: 56 },
  { value: PROVIDER_ZHIPU, label: 'Zhipu', newApiType: 16 },
]

export const PROVIDERS: readonly Provider[] = MODEL_MARKETPLACE_PROVIDER_OPTIONS.map((p) => p.value)

const PROVIDER_LABELS = new Map(MODEL_MARKETPLACE_PROVIDER_OPTIONS.map((p) => [p.value, p.label]))

export function getModelMarketplaceProviderLabel(provider: Provider | string): string {
  return PROVIDER_LABELS.get(provider) || provider || '-'
}

export const STATUS_OPERATIONAL: MonitorStatus = 'operational'
export const STATUS_DEGRADED: MonitorStatus = 'degraded'
export const STATUS_FAILED: MonitorStatus = 'failed'
export const STATUS_ERROR: MonitorStatus = 'error'

export const MONITOR_STATUSES: readonly MonitorStatus[] = [
  STATUS_OPERATIONAL,
  STATUS_DEGRADED,
  STATUS_FAILED,
  STATUS_ERROR,
]

/** Default polling interval (seconds) for new monitors. */
export const DEFAULT_INTERVAL_SECONDS = 60
