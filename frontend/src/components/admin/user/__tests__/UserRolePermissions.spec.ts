import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UserApiKeysModal from '../UserApiKeysModal.vue'
import UserCreateModal from '../UserCreateModal.vue'
import UserEditModal from '../UserEditModal.vue'

const {
  createUser,
  updateUser,
  updateUserAttributeValues,
  getUserApiKeys,
  getAllGroups,
  updateApiKeyGroup,
  showError,
  showSuccess
} = vi.hoisted(() => ({
  createUser: vi.fn(),
  updateUser: vi.fn(),
  updateUserAttributeValues: vi.fn(),
  getUserApiKeys: vi.fn(),
  getAllGroups: vi.fn(),
  updateApiKeyGroup: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      create: createUser,
      update: updateUser,
      getUserApiKeys
    },
    groups: {
      getAll: getAllGroups
    },
    apiKeys: {
      updateApiKeyGroup
    },
    userAttributes: {
      updateUserAttributeValues
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const dialogStub = {
  props: ['show', 'title'],
  emits: ['close'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
}

const global = {
  stubs: {
    BaseDialog: dialogStub,
    Icon: true,
    TotpStepUpDialog: true,
    UserAttributeForm: true,
    GroupBadge: { template: '<span data-test="group-badge" />' },
    GroupOptionItem: true,
    Teleport: true
  }
}

const createUserRecord = (role: AdminUser['role'] = 'user'): AdminUser => ({
  id: 42,
  username: 'managed-user',
  email: 'managed@example.com',
  role,
  balance: 0,
  concurrency: 1,
  rpm_limit: 0,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-07-20T00:00:00Z',
  updated_at: '2026-07-20T00:00:00Z',
  notes: ''
})

describe('admin user role permissions', () => {
  beforeEach(() => {
    createUser.mockReset()
    updateUser.mockReset()
    updateUserAttributeValues.mockReset()
    getUserApiKeys.mockReset()
    getAllGroups.mockReset()
    updateApiKeyGroup.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    createUser.mockResolvedValue(createUserRecord())
    updateUser.mockResolvedValue(createUserRecord())
    getUserApiKeys.mockResolvedValue({
      items: [{
        id: 7,
        name: 'operator-visible-key',
        key: 'sk-test-abcdefghijklmnopqrstuvwxyz',
        status: 'active',
        group_id: 9,
        group: { id: 9, name: 'Allowed Group' },
        created_at: '2026-07-20T00:00:00Z'
      }]
    })
    getAllGroups.mockResolvedValue([])
  })

  it('keeps administrator creation hidden and forces a regular-user payload for operators', async () => {
    const wrapper = mount(UserCreateModal, {
      props: { show: true, canManageAdminRole: false },
      global
    })

    expect(wrapper.text()).not.toContain('admin.users.form.roleLabel')
    expect(wrapper.find('select').exists()).toBe(false)

    await wrapper.get('input[type="email"]').setValue('new-user@example.com')
    await wrapper.get('input[placeholder="admin.users.enterPassword"]').setValue('safe-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(createUser).toHaveBeenCalledWith(expect.objectContaining({ role: 'user' }))
  })

  it('keeps administrator creation available to super administrators', async () => {
    const wrapper = mount(UserCreateModal, {
      props: { show: true, canManageAdminRole: true },
      global
    })

    expect(wrapper.get('select').findAll('option').map((option) => option.attributes('value')))
      .toEqual(['user', 'admin'])

    await wrapper.get('select').setValue('admin')
    await wrapper.get('input[type="email"]').setValue('new-admin@example.com')
    await wrapper.get('input[placeholder="admin.users.enterPassword"]').setValue('safe-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(createUser).toHaveBeenCalledWith(expect.objectContaining({ role: 'admin' }))
  })

  it('omits role changes when an operator edits a regular user', async () => {
    const wrapper = mount(UserEditModal, {
      props: {
        show: true,
        user: createUserRecord(),
        canManageAdminRole: false
      },
      global
    })

    expect(wrapper.text()).not.toContain('admin.users.form.roleLabel')
    expect(wrapper.find('select').exists()).toBe(false)

    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledOnce()
    expect(updateUser.mock.calls[0][0]).toBe(42)
    expect(updateUser.mock.calls[0][1]).not.toHaveProperty('role')
  })

  it('allows a super administrator to promote a regular user', async () => {
    const wrapper = mount(UserEditModal, {
      props: {
        show: true,
        user: createUserRecord(),
        canManageAdminRole: true
      },
      global
    })

    await wrapper.get('select').setValue('admin')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(updateUser).toHaveBeenCalledWith(42, expect.objectContaining({ role: 'admin' }))
  })

  it('does not send the dedicated operator role through the generic edit endpoint', async () => {
    const wrapper = mount(UserEditModal, {
      props: {
        show: true,
        user: createUserRecord('operator'),
        canManageAdminRole: true
      },
      global
    })

    expect(wrapper.find('select').exists()).toBe(false)
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(updateUser.mock.calls[0][1]).not.toHaveProperty('role')
  })

  it('keeps API keys readable but group reassignment unavailable to operators', async () => {
    const wrapper = mount(UserApiKeysModal, {
      props: {
        show: false,
        user: createUserRecord(),
        canEditGroup: false
      },
      global
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(getUserApiKeys).toHaveBeenCalledWith(42)
    expect(getAllGroups).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="api-key-group-editor"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="api-key-group-readonly"]').exists()).toBe(true)
    expect(updateApiKeyGroup).not.toHaveBeenCalled()
  })

  it('keeps API key group reassignment available to super administrators', async () => {
    const wrapper = mount(UserApiKeysModal, {
      props: {
        show: false,
        user: createUserRecord(),
        canEditGroup: true
      },
      global
    })

    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(getAllGroups).toHaveBeenCalledOnce()
    expect(wrapper.get('[data-test="api-key-group-editor"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="api-key-group-readonly"]').exists()).toBe(false)
  })
})
