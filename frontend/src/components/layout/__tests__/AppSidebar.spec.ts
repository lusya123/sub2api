import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { ref } from 'vue'
import AppSidebar from '../AppSidebar.vue'
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

function createTestRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/keys', component: { template: '<div />' } },
      { path: '/client-install', component: { template: '<div />' } },
      { path: '/status', component: { template: '<div />' } },
      { path: '/usage', component: { template: '<div />' } },
      { path: '/subscriptions', component: { template: '<div />' } },
      { path: '/purchase', component: { template: '<div />' } },
      { path: '/redeem', component: { template: '<div />' } },
      { path: '/profile', component: { template: '<div />' } },
    ],
  })
}

describe('AppSidebar', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockReturnValue({
        matches: false,
        media: '',
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }),
    )
  })

  it('renders purchase entry as an external link in redirect mode', async () => {
    const appStore = useAppStore()
    const authStore = useAuthStore()
    const router = createTestRouter()

    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = createPublicSettings({
      purchase_subscription_mode: 'redirect',
      purchase_subscription_redirect_url: 'https://pay.example.com/topup',
    })
    authStore.user = createUser()

    await router.push('/dashboard')
    await router.isReady()

    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router],
        stubs: {
          VersionBadge: true,
        },
      },
    })

    await flushPromises()

    const purchaseLink = wrapper
      .findAll('a')
      .find((link) => link.attributes('href') === 'https://pay.example.com/topup')

    expect(purchaseLink).toBeDefined()
    expect(purchaseLink?.attributes('target')).toBe('_blank')
    expect(purchaseLink?.attributes('rel')).toBe('noopener noreferrer')
    expect(wrapper.findAll('a').some((link) => link.attributes('href') === '/purchase')).toBe(false)
  })

  it('shows model health but not model marketplace for regular users', async () => {
    const appStore = useAppStore()
    const authStore = useAuthStore()
    const router = createTestRouter()

    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = createPublicSettings()
    authStore.user = createUser()

    await router.push('/dashboard')
    await router.isReady()

    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router],
        stubs: {
          VersionBadge: true,
        },
      },
    })

    await flushPromises()

    const hrefs = wrapper.findAll('a').map((link) => link.attributes('href'))

    expect(hrefs).toContain('/status')
    expect(hrefs).not.toContain('/models')
  })

  it('hides model health for regular users when disabled in public settings', async () => {
    const appStore = useAppStore()
    const authStore = useAuthStore()
    const router = createTestRouter()

    appStore.publicSettingsLoaded = true
    appStore.cachedPublicSettings = createPublicSettings({
      model_health_page_enabled: false,
    })
    authStore.user = createUser()

    await router.push('/dashboard')
    await router.isReady()

    const wrapper = mount(AppSidebar, {
      global: {
        plugins: [router],
        stubs: {
          VersionBadge: true,
        },
      },
    })

    await flushPromises()

    const hrefs = wrapper.findAll('a').map((link) => link.attributes('href'))

    expect(hrefs).not.toContain('/status')
    expect(hrefs).not.toContain('/models')
  })
})
