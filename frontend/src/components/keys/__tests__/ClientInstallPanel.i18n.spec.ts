import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string, params?: Record<string, string>) => {
      if (key.startsWith('keys.clientInstallModal')) {
        throw new SyntaxError(`Invalid linked format for ${key}`)
      }
      if (!params) return key
      return `${key}:${Object.values(params).join(',')}`
    },
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true),
  }),
}))

import ClientInstallPanel from '../ClientInstallPanel.vue'

describe('ClientInstallPanel i18n resilience', () => {
  it('renders fallback content when client install translations throw', async () => {
    const wrapper = mount(ClientInstallPanel, {
      props: {
        apiKey: 'sk-test',
        baseUrl: 'https://example.com',
        platform: 'anthropic',
      },
      global: {
        stubs: {
          Icon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('Claude Code')
    expect(wrapper.text()).toContain('@anthropic-ai/claude-code')
    expect(wrapper.text()).toContain('/install-claude.sh')

    const openclawButton = wrapper.findAll('button').find((button) =>
      button.text().includes('OpenClaw')
    )
    expect(openclawButton).toBeDefined()
    await openclawButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('/install-openclaw.sh')

    const claudeButton = wrapper.findAll('button').find((button) =>
      button.text().includes('Claude Code')
    )
    expect(claudeButton).toBeDefined()
    await claudeButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('/install-claude.sh')
    expect(wrapper.text()).not.toContain('/install-openclaw.sh')
  })
})
