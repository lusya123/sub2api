import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import VersionBadge from '../VersionBadge.vue'

const {
  performUpdate,
  restartService,
  getRollbackVersions,
  rollback,
  stepUpTotp,
  authStore,
  appStore,
  showError,
  clearVersionCache
} = vi.hoisted(() => {
  const showError = vi.fn()
  const clearVersionCache = vi.fn()
  return {
    performUpdate: vi.fn(),
    restartService: vi.fn(),
    getRollbackVersions: vi.fn(),
    rollback: vi.fn(),
    stepUpTotp: vi.fn(),
    authStore: { isAdmin: true },
    appStore: {
      versionLoading: false,
      currentVersion: '1.0.0',
      latestVersion: '1.1.0',
      hasUpdate: true,
      releaseInfo: {
        name: 'v1.1.0',
        body: '',
        published_at: '2026-07-20T00:00:00Z',
        html_url: 'https://example.com/release'
      },
      buildType: 'release',
      fetchVersion: vi.fn(),
      clearVersionCache,
      showError
    },
    showError,
    clearVersionCache
  }
})

vi.mock('@/api/admin/system', () => ({
  performUpdate,
  restartService,
  getRollbackVersions,
  rollback
}))

vi.mock('@/api', () => ({
  totpAPI: {
    stepUp: stepUpTotp
  }
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => authStore,
  useAppStore: () => appStore
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: false,
    copyToClipboard: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) =>
        params?.version ? `${key}:${params.version}` : key
    })
  }
})

function mountBadge(): VueWrapper {
  return mount(VersionBadge, {
    props: { version: '1.0.0' },
    global: {
      stubs: {
        Icon: true
      }
    }
  })
}

async function openDropdown(wrapper: VueWrapper) {
  await wrapper.find('button').trigger('click')
  await flushPromises()
}

async function completeStepUp(wrapper: VueWrapper, code = '123456') {
  const cells = wrapper.findAll('input[pattern="[0-9]"]')
  expect(cells).toHaveLength(6)
  for (const [index, digit] of [...code].entries()) {
    await cells[index]!.setValue(digit)
  }
  await flushPromises()
}

describe('VersionBadge sensitive system actions', () => {
  beforeEach(() => {
    vi.useRealTimers()
    performUpdate.mockReset()
    restartService.mockReset()
    getRollbackVersions.mockReset()
    rollback.mockReset()
    stepUpTotp.mockReset()
    showError.mockReset()
    clearVersionCache.mockReset()
    appStore.fetchVersion.mockReset()

    authStore.isAdmin = true
    appStore.versionLoading = false
    appStore.currentVersion = '1.0.0'
    appStore.latestVersion = '1.1.0'
    appStore.hasUpdate = true
    appStore.buildType = 'release'
    appStore.releaseInfo = {
      name: 'v1.1.0',
      body: '',
      published_at: '2026-07-20T00:00:00Z',
      html_url: 'https://example.com/release'
    }

    stepUpTotp.mockResolvedValue(undefined)
    getRollbackVersions.mockResolvedValue({ versions: [] })
  })

  it('prompts and retries an update after successful TOTP verification', async () => {
    performUpdate
      .mockRejectedValueOnce({ status: 403, code: 'STEP_UP_REQUIRED' })
      .mockResolvedValueOnce({ message: 'updated', need_restart: true })

    const wrapper = mountBadge()
    await openDropdown(wrapper)
    await wrapper.get('[data-testid="version-update"]').trigger('click')
    await flushPromises()

    expect(performUpdate).toHaveBeenCalledTimes(1)
    await completeStepUp(wrapper)

    expect(stepUpTotp).toHaveBeenCalledWith('123456')
    expect(performUpdate).toHaveBeenCalledTimes(2)
    expect(clearVersionCache).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-testid="version-restart"]').exists()).toBe(true)
  })

  it('keeps update state retryable and shows the dedicated message when step-up is blocked', async () => {
    performUpdate.mockRejectedValue({
      status: 403,
      reason: 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
    })

    const wrapper = mountBadge()
    await openDropdown(wrapper)
    await wrapper.get('[data-testid="version-update"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('stepUp.adminApiKeyForbidden')
    expect(wrapper.find('[data-testid="version-update"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('[data-testid="version-update-retry"]').exists()).toBe(false)
    expect(clearVersionCache).not.toHaveBeenCalled()
  })

  it('prompts and retries a selected rollback without losing its target version', async () => {
    appStore.hasUpdate = false
    getRollbackVersions.mockResolvedValue({
      versions: [
        {
          version: '0.9.0',
          published_at: '2026-07-01T00:00:00Z',
          html_url: 'https://example.com/release/0.9.0'
        }
      ]
    })
    rollback
      .mockRejectedValueOnce({ status: 403, code: 'STEP_UP_REQUIRED' })
      .mockResolvedValueOnce({ message: 'rolled back', need_restart: true })

    const wrapper = mountBadge()
    await openDropdown(wrapper)
    await wrapper.get('[data-testid="version-rollback-toggle"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="version-rollback-option-0.9.0"]').trigger('click')
    await wrapper.get('[data-testid="version-rollback-confirm"]').trigger('click')
    await flushPromises()

    expect(rollback).toHaveBeenCalledTimes(1)
    await completeStepUp(wrapper, '654321')

    expect(stepUpTotp).toHaveBeenCalledWith('654321')
    expect(rollback).toHaveBeenNthCalledWith(1, '0.9.0')
    expect(rollback).toHaveBeenNthCalledWith(2, '0.9.0')
    expect(clearVersionCache).toHaveBeenCalledOnce()
    expect(wrapper.find('[data-testid="version-restart"]').exists()).toBe(true)
  })

  it('cancels a restart challenge without starting the restart countdown', async () => {
    performUpdate.mockResolvedValue({ message: 'updated', need_restart: true })
    restartService.mockRejectedValue({ status: 403, code: 'STEP_UP_REQUIRED' })

    const wrapper = mountBadge()
    await openDropdown(wrapper)
    await wrapper.get('[data-testid="version-update"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="version-restart"]').trigger('click')
    await flushPromises()

    const cancel = wrapper
      .findAll('button')
      .find((button) => button.text() === 'common.cancel')
    expect(cancel).toBeDefined()
    await cancel!.trigger('click')
    await flushPromises()

    expect(restartService).toHaveBeenCalledOnce()
    expect(showError).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="version-restart"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="version-restart"]').text()).toContain('version.restartNow')
    expect(wrapper.get('[data-testid="version-restart"]').text()).not.toContain('8s')
  })
})
