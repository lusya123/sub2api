import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import CustomOpenAIProviderBadge from '../CustomOpenAIProviderBadge.vue'

describe('CustomOpenAIProviderBadge', () => {
  it('renders provider metadata for OpenAI API key accounts', () => {
    const wrapper = mount(CustomOpenAIProviderBadge, {
      props: {
        platform: 'openai',
        type: 'apikey',
        extra: {
          custom_provider: 'deepseek',
          custom_provider_label: 'DeepSeek'
        }
      },
      global: {
        stubs: {
          ProviderIcon: true
        }
      }
    })

    expect(wrapper.text()).toContain('DeepSeek')
    expect(wrapper.findComponent({ name: 'ProviderIcon' }).exists()).toBe(true)
  })

  it('falls back to provider id when label is missing', () => {
    const wrapper = mount(CustomOpenAIProviderBadge, {
      props: {
        platform: 'openai',
        type: 'apikey',
        extra: {
          custom_provider: 'moonshot'
        }
      },
      global: {
        stubs: {
          ProviderIcon: true
        }
      }
    })

    expect(wrapper.text()).toContain('moonshot')
  })

  it('does not render for native OpenAI OAuth or non-OpenAI accounts', () => {
    const oauthWrapper = mount(CustomOpenAIProviderBadge, {
      props: {
        platform: 'openai',
        type: 'oauth',
        extra: {
          custom_provider: 'deepseek',
          custom_provider_label: 'DeepSeek'
        }
      },
      global: {
        stubs: {
          ProviderIcon: true
        }
      }
    })

    const anthropicWrapper = mount(CustomOpenAIProviderBadge, {
      props: {
        platform: 'anthropic',
        type: 'apikey',
        extra: {
          custom_provider: 'deepseek',
          custom_provider_label: 'DeepSeek'
        }
      },
      global: {
        stubs: {
          ProviderIcon: true
        }
      }
    })

    expect(oauthWrapper.text()).toBe('')
    expect(anthropicWrapper.text()).toBe('')
  })
})
