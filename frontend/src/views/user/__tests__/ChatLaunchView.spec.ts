import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChatLaunchView from '../ChatLaunchView.vue'

const {
  appState,
  createKey,
  getLobeConfig,
  launch,
  launchWithModel,
  listKeys,
  navigateInCurrentTab,
} = vi.hoisted(() => ({
  appState: {
    siteName: 'Sub2API',
    siteLogo: '',
    publicSettingsLoaded: true,
    cachedPublicSettings: {
      chat_page_enabled: true,
      agent_page_enabled: false,
    } as Record<string, unknown>,
  },
  createKey: vi.fn(),
  getLobeConfig: vi.fn(),
  launch: vi.fn(),
  launchWithModel: vi.fn(),
  listKeys: vi.fn(),
  navigateInCurrentTab: vi.fn(),
}))

vi.mock('@/api', () => ({
  chatAPI: {
    getLobeConfig,
    launch,
    launchWithModel,
  },
  keysAPI: {
    create: createKey,
    list: listKeys,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appState,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ canAccessAdmin: false }),
}))

vi.mock('@/utils/navigation', () => ({
  navigateInCurrentTab,
}))

vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: { modelMarketplace: 'modelMarketplace' },
  isFeatureFlagEnabled: () => false,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

function mountView() {
  return mount(ChatLaunchView, {
    global: {
      stubs: {
        Icon: true,
        RouterLink: {
          props: ['to'],
          template: '<a><slot /></a>',
        },
      },
    },
  })
}

describe('ChatLaunchView', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    appState.cachedPublicSettings = {
      chat_page_enabled: true,
      agent_page_enabled: false,
    }
    getLobeConfig.mockResolvedValue({ providers: [] })
    listKeys.mockResolvedValue({ items: [] })
    launch.mockResolvedValue({ url: 'https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox' })
  })

  it('launches LobeHub as a top-level navigation instead of an iframe', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(launch).toHaveBeenCalledOnce()
    expect(navigateInCurrentTab).toHaveBeenCalledOnce()
    expect(navigateInCurrentTab).toHaveBeenCalledWith(
      'https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox'
    )
    expect(getLobeConfig).not.toHaveBeenCalled()
    expect(listKeys).not.toHaveBeenCalled()
    expect(wrapper.find('iframe').exists()).toBe(false)

    wrapper.unmount()
  })

  it('does not wait for model or API key loading before launching chat', async () => {
    getLobeConfig.mockReturnValue(new Promise(() => {}))
    listKeys.mockReturnValue(new Promise(() => {}))

    const wrapper = mountView()
    await flushPromises()

    expect(launch).toHaveBeenCalledOnce()
    expect(navigateInCurrentTab).toHaveBeenCalledWith(
      'https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox'
    )
    expect(getLobeConfig).not.toHaveBeenCalled()
    expect(listKeys).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('shows the existing failure state when the chat launch request fails', async () => {
    launch.mockRejectedValue(new Error('launch failed'))

    const wrapper = mountView()
    await flushPromises()

    expect(navigateInCurrentTab).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('chatLaunch.failed')

    wrapper.unmount()
  })

  it('keeps Agent in its iframe and navigates only after switching to chat', async () => {
    appState.cachedPublicSettings = {
      chat_page_enabled: true,
      agent_page_enabled: true,
      agent_page_url: 'https://agent.example.com',
    }
    localStorage.setItem('sub2api:use-token:last-mode', 'agent')
    getLobeConfig.mockResolvedValue({
      providers: [
        {
          id: 'sub2api-group-7',
          display_name: 'OpenAI',
          sdk_type: 'openai',
          base_url: 'https://api.example.com/v1',
          models: [{ id: 'gpt-5.4-mini', display_name: 'GPT-5.4 mini' }],
        },
      ],
    })
    listKeys.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'Agent key',
          key: 'sk-agent-test',
          status: 'active',
          group_id: 7,
        },
      ],
    })
    launchWithModel.mockResolvedValue({
      url: 'https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox',
    })

    const wrapper = mountView()
    await flushPromises()

    const frame = wrapper.get('iframe')
    const bridgeURL = new URL(frame.attributes('src'))
    expect(bridgeURL.pathname).toBe('/manus/launch')
    expect(navigateInCurrentTab).not.toHaveBeenCalled()

    const chatButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('useToken.modes.chat'))
    expect(chatButton).toBeDefined()
    await chatButton!.trigger('click')
    await flushPromises()

    expect(launchWithModel).toHaveBeenCalledWith('sub2api-group-7', 'gpt-5.4-mini')
    expect(navigateInCurrentTab).toHaveBeenCalledWith(
      'https://chat.example.com/signin?callbackUrl=%2Fagent%2Finbox'
    )

    wrapper.unmount()
  })
})
