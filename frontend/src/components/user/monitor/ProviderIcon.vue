<template>
  <ModelIcon :model="iconKey" :size="`${size}px`" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Provider } from '@/api/admin/channelMonitor'
import ModelIcon from '@/components/common/ModelIcon.vue'

// Mirrors New API's getChannelTypeIcon mapping, translated from channel type
// names to Sub2API's stable provider keys.
const PROVIDER_ICON_KEYS: Record<string, string> = {
  openai: 'openai',
  openai_max: 'openai',
  ohmygpt: 'openai',
  custom: 'openai',
  ails: 'openai',
  ai_proxy: 'openai',
  api2gpt: 'openai',
  aigc2d: 'openai',
  anthropic: 'claude',
  aws: 'aws',
  gemini: 'gemini',
  deepseek: 'deepseek',
  azure: 'azure',
  vertex_ai: 'gemini',
  xai: 'xai',
  mistral: 'mistral',
  cohere: 'cohere',
  openrouter: 'openrouter',
  ollama: 'ollama',
  siliconflow: 'siliconcloud',
  perplexity: 'perplexity',
  moonshot: 'moonshot',
  ali: 'qwen',
  zhipu: 'zhipu',
  zhipu_v4: 'zhipu',
  baidu: 'baidu',
  baidu_v2: 'baidu',
  tencent: 'hunyuan',
  xunfei: 'spark',
  volcengine: 'volcengine',
  lingyiwanwu: 'yi',
  minimax: 'minimax',
  coze: 'coze',
  ai360: 'ai360',
  xinference: 'xinference',
  dify: 'dify',
  jina: 'jina',
  cloudflare: 'cloudflare',
  palm: 'google',
  codex: 'openai',
  fastgpt: 'fastgpt',
  ai_proxy_library: 'openai',
  mokaai: 'openai',
  midjourney: 'midjourney',
  midjourney_plus: 'midjourney',
  sunoapi: 'suno',
  kling: 'kling',
  jimeng: 'jimeng',
  vidu: 'vidu',
  submodel: 'openai',
  doubao_video: 'doubao',
  sora: 'openai',
  replicate: 'replicate',
}

const props = withDefaults(defineProps<{
  provider: Provider | string
  size?: number
}>(), {
  size: 20,
})

const iconKey = computed(() => PROVIDER_ICON_KEYS[String(props.provider)] || String(props.provider || ''))
</script>
