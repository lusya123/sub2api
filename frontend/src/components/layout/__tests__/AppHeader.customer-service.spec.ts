import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

import { useAppStore, useAuthStore } from '@/stores'
import AppHeader from '../AppHeader.vue'

const qrCode = 'data:image/png;base64,customer-service-qr'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => ({
        'common.balance': 'Balance',
        'common.close': 'Close',
        'common.contactSupport': 'Contact Support',
        'nav.apiKeys': 'API Keys',
        'nav.console': 'Console',
        'nav.github': 'GitHub',
        'nav.logout': 'Logout',
        'nav.modelMarketplace': 'Marketplace',
        'nav.profile': 'Profile',
        'nav.useToken': 'Use Token',
      })[key] || key,
    }),
  }
})

function removeTeleports() {
  document.body.querySelectorAll('[data-test-app-header], [role="dialog"]').forEach((el) => el.remove())
}

async function mountHeader() {
  const pinia = createPinia()
  setActivePinia(pinia)

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/model-marketplace', component: { template: '<div />' } },
      { path: '/chat', component: { template: '<div />' } },
      { path: '/profile', component: { template: '<div />' } },
      { path: '/keys', component: { template: '<div />' } },
    ],
  })
  router.push('/dashboard')
  await router.isReady()

  const appStore = useAppStore()
  appStore.contactInfo = 'wechat-support'
  appStore.customerServiceQrcode = qrCode
  appStore.publicSettingsLoaded = true

  const authStore = useAuthStore()
  authStore.token = 'token'
  authStore.user = {
    id: 1,
    username: 'demo',
    email: 'demo@example.com',
    role: 'user',
    balance: 10,
    concurrency: 1,
  }

  const host = document.createElement('div')
  host.dataset.testAppHeader = 'true'
  document.body.appendChild(host)

  return mount(AppHeader, {
    attachTo: host,
    global: {
      plugins: [pinia, router],
      stubs: {
        AnnouncementBell: { template: '<span />' },
        Icon: { template: '<span />' },
        LocaleSwitcher: { template: '<span />' },
        SubscriptionProgressMini: { template: '<span />' },
      },
    },
  })
}

describe('AppHeader customer service QR code', () => {
  afterEach(() => {
    removeTeleports()
  })

  it('opens an enlarged QR code dialog from the right user menu', async () => {
    const wrapper = await mountHeader()

    await wrapper.find('button[aria-label="User Menu"]').trigger('click')

    const qrButton = wrapper
      .findAll('button')
      .find((button) => button.find(`img[src="${qrCode}"]`).exists())

    expect(qrButton).toBeTruthy()
    await qrButton!.trigger('click')
    await nextTick()

    const dialog = document.body.querySelector('[role="dialog"]')
    expect(dialog).toBeTruthy()
    expect(dialog?.querySelector(`img[src="${qrCode}"]`)).toBeTruthy()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()

    expect(document.body.querySelector('[role="dialog"]')).toBeNull()
    wrapper.unmount()
  })
})
