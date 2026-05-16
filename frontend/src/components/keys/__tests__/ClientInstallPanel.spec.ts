import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    locale: { value: 'zh' },
    t: (key: string, params?: Record<string, string>) => {
      if (!params) return key
      return `${key}:${Object.values(params).join(',')}`
    }
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

import ClientInstallPanel from '../ClientInstallPanel.vue'

describe('ClientInstallPanel', () => {
  it('builds antigravity Claude Code install command with antigravity base path', () => {
    const wrapper = mount(ClientInstallPanel, {
      props: {
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'antigravity'
      },
      global: {
        stubs: {
          Icon: { template: '<span />' }
        }
      }
    })

    expect(wrapper.text()).toContain('XDT_API_URL="https://example.com/antigravity"')
    expect(wrapper.text()).toContain('/install-claude-ccswitch.sh')
  })

  it('updates OpenClaw command when model and os change', async () => {
    const wrapper = mount(ClientInstallPanel, {
      props: {
        apiKey: 'sk-test',
        baseUrl: 'https://example.com',
        platform: 'anthropic'
      },
      global: {
        stubs: {
          Icon: { template: '<span />' }
        }
      }
    })

    const openclawButton = wrapper.findAll('button').find((button) => button.text().includes('OpenClaw'))
    expect(openclawButton).toBeDefined()
    await openclawButton!.trigger('click')
    await nextTick()

    const opusButton = wrapper.findAll('button').find((button) => button.text().includes('Opus 4.6'))
    expect(opusButton).toBeDefined()
    await opusButton!.trigger('click')
    await nextTick()

    const windowsButton = wrapper.findAll('button').find((button) => button.text().includes('Windows'))
    expect(windowsButton).toBeDefined()
    await windowsButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain("$env:OPENCLAW_MODEL='anthropic/claude-opus-4-6'")
    expect(wrapper.text()).toContain('/install-openclaw-win.ps1')
  })

  it('restores the Claude Code command after switching back from OpenClaw', async () => {
    const wrapper = mount(ClientInstallPanel, {
      props: {
        apiKey: 'sk-test',
        baseUrl: 'https://example.com',
        platform: 'anthropic'
      },
      global: {
        stubs: {
          Icon: { template: '<span />' }
        }
      }
    })

    const openclawButton = wrapper.findAll('button').find((button) => button.text().includes('OpenClaw'))
    expect(openclawButton).toBeDefined()
    await openclawButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('/install-openclaw.sh')

    const claudeButton = wrapper.findAll('button').find((button) => button.text().includes('Claude Code'))
    expect(claudeButton).toBeDefined()
    await claudeButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('XDT_API_URL="https://example.com"')
    expect(wrapper.text()).toContain('/install-claude-ccswitch.sh')
    expect(wrapper.text()).not.toContain('/install-openclaw.sh')
  })

  it('keeps the selected OS command when switching from OpenClaw back to Claude Code', async () => {
    const wrapper = mount(ClientInstallPanel, {
      props: {
        apiKey: 'sk-test',
        baseUrl: 'https://example.com',
        platform: 'anthropic'
      },
      global: {
        stubs: {
          Icon: { template: '<span />' }
        }
      }
    })

    const windowsButton = wrapper.findAll('button').find((button) => button.text().includes('Windows'))
    expect(windowsButton).toBeDefined()
    await windowsButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain("$env:XDT_API_URL='https://example.com'")
    expect(wrapper.text()).toContain('/install-claude-ccswitch-win.ps1')

    const openclawButton = wrapper.findAll('button').find((button) => button.text().includes('OpenClaw'))
    expect(openclawButton).toBeDefined()
    await openclawButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain("$env:OPENCLAW_BASE_URL='https://example.com'")
    expect(wrapper.text()).toContain('/install-openclaw-win.ps1')

    const claudeButton = wrapper.findAll('button').find((button) => button.text().includes('Claude Code'))
    expect(claudeButton).toBeDefined()
    await claudeButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain("$env:XDT_API_URL='https://example.com'")
    expect(wrapper.text()).toContain('/install-claude-ccswitch-win.ps1')
    expect(wrapper.text()).not.toContain('/install-openclaw-win.ps1')
  })

  it('builds Codex CLI install commands for OpenAI groups', async () => {
    const wrapper = mount(ClientInstallPanel, {
      props: {
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/api',
        platform: 'openai'
      },
      global: {
        stubs: {
          Icon: { template: '<span />' }
        }
      }
    })

    expect(wrapper.text()).toContain('Codex CLI')
    expect(wrapper.text()).toContain('CODEX_TOKEN="sk-test"')
    expect(wrapper.text()).toContain('CODEX_API_URL="https://example.com/api/v1"')
    expect(wrapper.text()).toContain('/install-codex.sh')
    expect(wrapper.text()).not.toContain('/install-claude-ccswitch.sh')

    const windowsButton = wrapper.findAll('button').find((button) => button.text().includes('Windows'))
    expect(windowsButton).toBeDefined()
    await windowsButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain("$env:CODEX_TOKEN='sk-test'")
    expect(wrapper.text()).toContain('https://xuedingtoken.com/install-codex-win-bootstrap.ps1')
    expect(wrapper.text()).not.toContain("$env:CODEX_API_URL='https://example.com/api/v1'")
  })
})
