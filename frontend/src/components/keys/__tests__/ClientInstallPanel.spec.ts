import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
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

    expect(wrapper.text()).toContain('CLAUDE_API_URL="https://example.com/antigravity"')
    expect(wrapper.text()).toContain('/install-claude.sh')
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

    const openclawButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.clientInstallModal.clients.openclaw.label')
    )
    expect(openclawButton).toBeDefined()
    await openclawButton!.trigger('click')
    await nextTick()

    const opusButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.clientInstallModal.models.opus46')
    )
    expect(opusButton).toBeDefined()
    await opusButton!.trigger('click')
    await nextTick()

    const windowsButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.clientInstallModal.os.windows')
    )
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

    const openclawButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.clientInstallModal.clients.openclaw.label')
    )
    expect(openclawButton).toBeDefined()
    await openclawButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('/install-openclaw.sh')

    const claudeButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.clientInstallModal.clients.claude.label')
    )
    expect(claudeButton).toBeDefined()
    await claudeButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('CLAUDE_API_URL="https://example.com"')
    expect(wrapper.text()).toContain('/install-claude.sh')
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

    const windowsButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.clientInstallModal.os.windows')
    )
    expect(windowsButton).toBeDefined()
    await windowsButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain("$env:CLAUDE_API_URL='https://example.com'")
    expect(wrapper.text()).toContain('/install-claude-win.ps1')

    const openclawButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.clientInstallModal.clients.openclaw.label')
    )
    expect(openclawButton).toBeDefined()
    await openclawButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain("$env:OPENCLAW_BASE_URL='https://example.com'")
    expect(wrapper.text()).toContain('/install-openclaw-win.ps1')

    const claudeButton = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.clientInstallModal.clients.claude.label')
    )
    expect(claudeButton).toBeDefined()
    await claudeButton!.trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain("$env:CLAUDE_API_URL='https://example.com'")
    expect(wrapper.text()).toContain('/install-claude-win.ps1')
    expect(wrapper.text()).not.toContain('/install-openclaw-win.ps1')
  })
})
