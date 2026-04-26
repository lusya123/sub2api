import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import PurchaseSubscriptionView from '../PurchaseSubscriptionView.vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import type { PublicSettings, User } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: ref('zh-CN'),
    }),
  }
})

function createPublicSettings(overrides: Partial<PublicSettings> = {}): PublicSettings {
  return {
    registration_enabled: true,
    email_verify_enabled: false,
    registration_email_suffix_whitelist: [],
    promo_code_enabled: true,
    password_reset_enabled: false,
    invitation_code_enabled: false,
    turnstile_enabled: false,
    turnstile_site_key: '',
    site_name: 'Sub2API',
    site_logo: '',
    site_subtitle: '',
    api_base_url: '',
    contact_info: '',
    doc_url: '',
    home_content: '',
    hide_ccs_import_button: false,
    purchase_subscription_enabled: true,
    purchase_subscription_mode: 'embedded',
    purchase_subscription_embedded_url: '',
    purchase_subscription_redirect_url: '',
    purchase_subscription_url: '',
    custom_menu_items: [],
    custom_endpoints: [],
    linuxdo_oauth_enabled: false,
    sora_client_enabled: false,
    model_health_page_enabled: true,
    backend_mode_enabled: false,
    version: '',
    ...overrides,
  }
}

function createUser(overrides: Partial<User> = {}): User {
  return {
    id: 7,
    username: 'tester',
    email: 'tester@example.com',
    role: 'user',
    balance: 0,
    concurrency: 1,
    status: 'active',
    allowed_groups: null,
    created_at: '2026-03-27T00:00:00Z',
    updated_at: '2026-03-27T00:00:00Z',
    ...overrides,
  }
}

describe('PurchaseSubscriptionView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  it('renders embedded purchase iframe in embedded mode', async () => {
    const appStore = useAppStore()
    const authStore = useAuthStore()
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = createPublicSettings({
      purchase_subscription_mode: 'embedded',
      purchase_subscription_embedded_url: 'https://pay.example.com/embed?plan=pro',
    })
    authStore.user = createUser()
    authStore.token = 'token-123'

    const wrapper = mount(PurchaseSubscriptionView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const iframe = wrapper.find('iframe')
    expect(iframe.exists()).toBe(true)
    const url = new URL(iframe.attributes('src'))
    expect(url.searchParams.get('plan')).toBe('pro')
    expect(url.searchParams.get('user_id')).toBe('7')
    expect(url.searchParams.get('token')).toBe('token-123')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
  })

  it('renders a manual new-tab link in redirect mode', async () => {
    const appStore = useAppStore()
    const authStore = useAuthStore()
    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = createPublicSettings({
      purchase_subscription_mode: 'redirect',
      purchase_subscription_redirect_url: 'https://pay.example.com/topup',
    })
    authStore.user = createUser()
    authStore.token = 'token-123'

    const wrapper = mount(PurchaseSubscriptionView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.text()).toContain('purchase.redirectTitle')
    const link = wrapper.get('a.btn.btn-primary')
    expect(link.attributes('href')).toBe('https://pay.example.com/topup')
    expect(link.attributes('target')).toBe('_blank')
    expect(link.attributes('rel')).toBe('noopener noreferrer')
  })
})
