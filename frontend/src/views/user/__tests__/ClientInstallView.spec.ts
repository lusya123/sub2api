import { describe, expect, it, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ClientInstallView from '../ClientInstallView.vue'

const { list, getPublicSettings } = vi.hoisted(() => ({
  list: vi.fn(),
  getPublicSettings: vi.fn(),
}))

vi.mock('@/api', () => ({
  keysAPI: {
    list,
  },
  authAPI: {
    getPublicSettings,
  },
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {},
  }),
}))

const messages: Record<string, string> = {
  'clientInstallPage.noKeysTitle': 'No keys',
  'clientInstallPage.noKeysDescription': 'Create a key first',
  'clientInstallPage.selectKeyTitle': 'Select API Key',
  'clientInstallPage.selectKeyDescription': 'Pick a key to generate commands',
  'clientInstallPage.searchPlaceholder': 'Search key',
  'clientInstallPage.panelDescription': 'Current group: {group}',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        const template = messages[key] ?? key
        if (!params) return template
        return Object.entries(params).reduce(
          (result, [name, value]) => result.replace(`{${name}}`, value),
          template
        )
      },
    }),
  }
})

describe('ClientInstallView', () => {
  beforeEach(() => {
    list.mockReset()
    getPublicSettings.mockReset()
  })

  it('auto-selects the only supported API key and keeps the panel visible', async () => {
    list.mockResolvedValue({
      items: [
        {
          id: 1,
          user_id: 1,
          key: 'sk-test-only',
          name: 'Only Key',
          group_id: 10,
          status: 'active',
          ip_whitelist: [],
          ip_blacklist: [],
          last_used_at: null,
          quota: 0,
          quota_used: 0,
          expires_at: null,
          created_at: '2026-03-27T00:00:00Z',
          updated_at: '2026-03-27T00:00:00Z',
          rate_limit_5h: 0,
          rate_limit_1d: 0,
          rate_limit_7d: 0,
          usage_5h: 0,
          usage_1d: 0,
          usage_7d: 0,
          window_5h_start: null,
          window_1d_start: null,
          window_7d_start: null,
          reset_5h_at: null,
          reset_1d_at: null,
          reset_7d_at: null,
          group: {
            id: 10,
            name: 'Anthropic Group',
            description: null,
            platform: 'anthropic',
            rate_multiplier: 1,
            is_exclusive: false,
            status: 'active',
            subscription_type: 'standard',
            daily_limit_usd: null,
            weekly_limit_usd: null,
            monthly_limit_usd: null,
            image_price_1k: null,
            image_price_2k: null,
            image_price_4k: null,
            sora_image_price_360: null,
            sora_image_price_540: null,
            sora_video_price_per_request: null,
            sora_video_price_per_request_hd: null,
            sora_storage_quota_bytes: 0,
            claude_code_only: false,
            fallback_group_id: null,
            fallback_group_id_on_invalid_request: null,
            created_at: '2026-03-27T00:00:00Z',
            updated_at: '2026-03-27T00:00:00Z',
          },
        },
      ],
      total: 1,
      page: 1,
      page_size: 200,
      pages: 1,
    })
    getPublicSettings.mockResolvedValue({
      api_base_url: 'https://example.com/v1',
    })

    const wrapper = mount(ClientInstallView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          EmptyState: { template: '<div><slot /></div>', props: ['title', 'description'] },
          SearchInput: {
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
          },
          GroupBadge: { template: '<span>badge</span>' },
          ClientInstallPanel: {
            props: ['apiKey', 'baseUrl', 'platform'],
            template: '<div data-testid="install-panel">{{ apiKey }}|{{ baseUrl }}|{{ platform }}</div>',
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Only Key')
    expect(wrapper.text()).toContain('Current group: Anthropic Group')
    expect(wrapper.get('[data-testid="install-panel"]').text()).toContain('sk-test-only|https://example.com/v1|anthropic')

    const onlyKeyButton = wrapper.get('button')
    await onlyKeyButton.trigger('click')

    expect(wrapper.get('[data-testid="install-panel"]').exists()).toBe(true)
  })
})
