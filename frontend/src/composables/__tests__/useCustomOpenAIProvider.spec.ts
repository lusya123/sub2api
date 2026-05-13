import { describe, expect, it } from 'vitest'
import { ref } from 'vue'
import {
  DEFAULT_CUSTOM_OPENAI_PROVIDER,
  OPENAI_DEFAULT_BASE_URL,
  buildCustomOpenAIProviderExtra,
  customOpenAIProviderOptions,
  getCustomOpenAIProviderBaseUrl,
  getCustomOpenAIProviderLabel,
  getCustomOpenAIProviderModelKey,
  isKnownCustomOpenAIProviderBaseUrl,
  useCustomOpenAIProvider
} from '../useCustomOpenAIProvider'

describe('useCustomOpenAIProvider', () => {
  it('keeps custom account provider options independent from native platforms', () => {
    const values = customOpenAIProviderOptions.map((option) => option.value)

    expect(values).toContain(DEFAULT_CUSTOM_OPENAI_PROVIDER)
    expect(values).toContain('moonshot')
    expect(values).toContain('zhipu_v4')
    expect(values).toContain('ali')
    expect(values).toContain('dify')
    expect(values).toContain('jina')
    expect(values).not.toContain('openai')
    expect(values).not.toContain('anthropic')
    expect(values).not.toContain('gemini')
    expect(values).not.toContain('midjourney')
    expect(values).not.toContain('sora')
  })

  it('resolves provider labels and suggested OpenAI-compatible base URLs', () => {
    expect(getCustomOpenAIProviderLabel('moonshot')).toBe('Moonshot')
    expect(getCustomOpenAIProviderBaseUrl('deepseek')).toBe('https://api.deepseek.com')
    expect(getCustomOpenAIProviderBaseUrl('moonshot')).toBe('https://api.moonshot.cn')
    expect(getCustomOpenAIProviderBaseUrl('zhipu_v4')).toBe('https://open.bigmodel.cn/api/paas')
    expect(getCustomOpenAIProviderBaseUrl('not-configured')).toBe('')
  })

  it('maps marketplace provider ids to existing model whitelist keys', () => {
    expect(getCustomOpenAIProviderModelKey('deepseek')).toBe('deepseek')
    expect(getCustomOpenAIProviderModelKey('moonshot')).toBe('moonshot')
    expect(getCustomOpenAIProviderModelKey('zhipu_v4')).toBe('zhipu')
    expect(getCustomOpenAIProviderModelKey('ali')).toBe('qwen')
    expect(getCustomOpenAIProviderModelKey('volcengine')).toBe('doubao')
    expect(getCustomOpenAIProviderModelKey('unknown-provider')).toBe('openai')
  })

  it('only auto-replaces empty or known suggested base URLs', () => {
    expect(isKnownCustomOpenAIProviderBaseUrl('')).toBe(true)
    expect(isKnownCustomOpenAIProviderBaseUrl(OPENAI_DEFAULT_BASE_URL)).toBe(true)
    expect(isKnownCustomOpenAIProviderBaseUrl('https://api.deepseek.com')).toBe(true)
    expect(isKnownCustomOpenAIProviderBaseUrl('https://my-private-proxy.example.com')).toBe(false)
  })

  it('builds stable metadata for OpenAI-compatible custom provider accounts', () => {
    expect(buildCustomOpenAIProviderExtra('ali', 'qwen')).toEqual({
      custom_provider: 'ali',
      custom_provider_label: 'Ali',
      custom_provider_model_key: 'qwen',
      openai_compatible_provider: true
    })
  })

  it('exposes reactive label and base URL helpers for selector components', () => {
    const provider = ref('deepseek')
    const { selectedCustomProviderLabel, suggestedCustomProviderBaseUrl } = useCustomOpenAIProvider(provider)

    expect(selectedCustomProviderLabel.value).toBe('DeepSeek')
    expect(suggestedCustomProviderBaseUrl.value).toBe('https://api.deepseek.com')

    provider.value = 'moonshot'

    expect(selectedCustomProviderLabel.value).toBe('Moonshot')
    expect(suggestedCustomProviderBaseUrl.value).toBe('https://api.moonshot.cn')
  })
})
