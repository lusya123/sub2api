import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import { StepUpCancelledError } from '@/composables/useStepUp'
import UsersView from '../UsersView.vue'

const {
  listUsers,
  getAllGroups,
  getBatchUsersUsage,
  listEnabledDefinitions,
  getBatchUserAttributes,
  getPlatformQuotas,
  updateRole,
  stepUpRun,
  showError,
  showSuccess,
  authState
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn(),
  getPlatformQuotas: vi.fn(),
  updateRole: vi.fn(),
  stepUpRun: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  authState: {
    isAdmin: true,
    isOperator: false
  }
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      toggleStatus: vi.fn(),
      delete: vi.fn(),
      updateRole,
      getPlatformQuotas
    },
    groups: {
      getAll: getAllGroups
    },
    dashboard: {
      getBatchUsersUsage
    },
    userAttributes: {
      listEnabledDefinitions,
      getBatchUserAttributes
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/composables/useStepUp', async () => {
  const actual = await vi.importActual<typeof import('@/composables/useStepUp')>(
    '@/composables/useStepUp'
  )
  return {
    ...actual,
    useStepUp: () => ({
      visible: { value: false },
      blockedReason: { value: '' },
      prompt: vi.fn(),
      onVerified: vi.fn(),
      onCancel: vi.fn(),
      run: stepUpRun
    })
  }
})

vi.mock('@/components/auth/TotpStepUpDialog.vue', () => ({
  default: {
    props: ['controller'],
    template: '<div data-test="users-step-up-dialog" />'
  }
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

const createAdminUser = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: 42,
  username: 'scoped-user',
  email: 'scoped@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-17T00:00:00Z',
  updated_at: '2026-04-17T00:00:00Z',
  notes: '',
  last_active_at: '2026-04-16T02:00:00Z',
  last_used_at: '2026-04-17T02:00:00Z',
  current_concurrency: 0,
  ...overrides
})

const DataTableStub = {
  props: ['columns', 'data', 'selectedKeys', 'selectable'],
  emits: ['sort', 'update:selectedKeys'],
  template: `
    <div data-test="users-table" :data-selectable="String(selectable)">
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <div data-test="row-order">{{ data.map(row => row.email).join(',') }}</div>
      <div data-test="selected-keys">{{ (selectedKeys || []).join(',') }}</div>
      <button data-test="sort-last-used" @click="$emit('sort', 'last_used_at', 'desc')">sort</button>
      <button
        v-if="selectable"
        v-for="row in data"
        :key="'select-' + row.id"
        :data-test="'select-' + row.id"
        @click="$emit('update:selectedKeys', Array.from(new Set([...(selectedKeys || []), row.id])))"
      >
        select
      </button>
      <template v-for="col in columns" :key="col.key">
        <slot :name="'header-' + col.key" :column="col" />
      </template>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-last_used_at" :value="row.last_used_at" :row="row" />
        <slot name="cell-balance" :value="row.balance" :row="row" />
        <slot name="cell-balance_platform_quota" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
}

const PaginationStub = {
  emits: ['update:page'],
  template: '<button data-test="next-page" @click="$emit(\'update:page\', 2)">next</button>'
}

const BulkEditUserModalStub = {
  props: ['show', 'selectedIds'],
  emits: ['close', 'success'],
  template: `
    <div v-if="show" data-test="bulk-modal">
      <span data-test="bulk-modal-ids">{{ selectedIds.join(',') }}</span>
      <button data-test="bulk-success" @click="$emit('success', selectedIds.length)">success</button>
    </div>
  `
}

const UserCreateRoleModalStub = {
  props: ['show', 'canManageAdminRole'],
  template: `
    <div
      data-test="create-user-modal"
      :data-show="String(show)"
      :data-can-manage-admin-role="String(canManageAdminRole)"
    />
  `
}

const UserEditRoleModalStub = {
  props: ['show', 'user', 'canManageAdminRole'],
  template: `
    <div
      data-test="edit-user-modal"
      :data-show="String(show)"
      :data-can-manage-admin-role="String(canManageAdminRole)"
    />
  `
}

const UserApiKeysPermissionStub = {
  props: ['show', 'user', 'canEditGroup'],
  template: `
    <div
      data-test="api-keys-modal"
      :data-can-edit-group="String(canEditGroup)"
    />
  `
}

function mountUsersView() {
  return mount(UsersView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        Pagination: true,
        ConfirmDialog: true,
        EmptyState: true,
        GroupBadge: true,
        Select: true,
        UserAttributesConfigModal: true,
        UserConcurrencyCell: true,
        UserCreateModal: true,
        UserEditModal: true,
        BulkEditUserModal: BulkEditUserModalStub,
        UserPlatformQuotaModal: true,
        UserApiKeysModal: UserApiKeysPermissionStub,
        UserAllowedGroupsModal: true,
        UserBalanceModal: true,
        UserBalanceHistoryModal: true,
        GroupReplaceModal: true,
        Icon: true,
        Teleport: true
      }
    }
  })
}

async function openFirstUserActionMenu(wrapper: ReturnType<typeof mountUsersView>) {
  await wrapper.get('.action-menu-trigger').trigger('click', {
    clientX: 400,
    clientY: 200
  })
  await flushPromises()
}

describe('admin UsersView', () => {
  beforeEach(() => {
    vi.useRealTimers()
    localStorage.clear()
    authState.isAdmin = true
    authState.isOperator = false

    listUsers.mockReset()
    getAllGroups.mockReset()
    getBatchUsersUsage.mockReset()
    listEnabledDefinitions.mockReset()
    getBatchUserAttributes.mockReset()
    getPlatformQuotas.mockReset()
    updateRole.mockReset()
    stepUpRun.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    updateRole.mockResolvedValue(undefined)
    stepUpRun.mockImplementation(async (action: () => Promise<unknown>) => action())

    listUsers.mockResolvedValue({
      items: [createAdminUser()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getAllGroups.mockResolvedValue([])
    getBatchUsersUsage.mockResolvedValue({ stats: {} })
    listEnabledDefinitions.mockResolvedValue([])
    getBatchUserAttributes.mockResolvedValue({ attributes: {} })
    getPlatformQuotas.mockResolvedValue({ platform_quotas: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('runs operator delegation through step-up and refreshes after success', async () => {
    const wrapper = mountUsersView()
    await flushPromises()
    await openFirstUserActionMenu(wrapper)

    expect(wrapper.find('[data-test="users-step-up-dialog"]').exists()).toBe(true)
    await wrapper.get('[data-testid="delegate-operator"]').trigger('click')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledOnce()
    expect(updateRole).toHaveBeenCalledWith(42, 'operator')
    expect(showSuccess).toHaveBeenCalledWith('admin.users.operatorDelegated')
    expect(listUsers).toHaveBeenCalledTimes(2)
  })

  it('silently stops operator delegation when the step-up dialog is cancelled', async () => {
    stepUpRun.mockRejectedValueOnce(new StepUpCancelledError())

    const wrapper = mountUsersView()
    await flushPromises()
    await openFirstUserActionMenu(wrapper)
    await wrapper.get('[data-testid="delegate-operator"]').trigger('click')
    await flushPromises()

    expect(updateRole).not.toHaveBeenCalled()
    expect(showSuccess).not.toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
  })

  it('shows the dedicated reason when operator delegation step-up is blocked', async () => {
    stepUpRun.mockRejectedValueOnce({
      status: 403,
      reason: 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
    })

    const wrapper = mountUsersView()
    await flushPromises()
    await openFirstUserActionMenu(wrapper)
    await wrapper.get('[data-testid="delegate-operator"]').trigger('click')
    await flushPromises()

    expect(updateRole).not.toHaveBeenCalled()
    expect(showSuccess).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('stepUp.adminApiKeyForbidden')
  })

  it('also routes operator revocation through the same step-up controller', async () => {
    listUsers.mockResolvedValue({
      items: [createAdminUser({ role: 'operator' })],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountUsersView()
    await flushPromises()
    await openFirstUserActionMenu(wrapper)
    await wrapper.get('[data-testid="revoke-operator"]').trigger('click')
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledOnce()
    expect(updateRole).toHaveBeenCalledWith(42, 'user')
    expect(showSuccess).toHaveBeenCalledWith('admin.users.operatorRevoked')
  })

  it('shows active, used, and created activity columns in order and requests last_used_at sort', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: UserApiKeysPermissionStub,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const columns = wrapper.get('[data-test="columns"]').text()
    const visibleColumns = columns.split(',')
    expect(visibleColumns.slice(-4, -1)).toEqual(['last_active_at', 'last_used_at', 'created_at'])
    expect(visibleColumns).not.toContain('last_login_at')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('clears usage current-page sort when switching to last_used_at server sort', async () => {
    vi.useFakeTimers()
    localStorage.setItem('user-column-settings-version', '3')
    localStorage.setItem(
      'user-hidden-columns',
      JSON.stringify([
        'notes',
        'groups',
        'subscriptions',
        'concurrency',
        'usage_anthropic',
        'usage_openai',
        'usage_gemini',
        'usage_antigravity',
        'balance_platform_quota'
      ])
    )

    listUsers.mockResolvedValue({
      items: [
        createAdminUser({ id: 1, email: 'last-used-first@example.com' }),
        createAdminUser({ id: 2, email: 'usage-first@example.com' })
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getBatchUsersUsage.mockResolvedValue({
      stats: {
        1: { user_id: 1, today_actual_cost: 1, total_actual_cost: 1, by_platform: [] },
        2: { user_id: 2, today_actual_cost: 9, total_actual_cost: 9, by_platform: [] }
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(50)
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('last-used-first@example.com,usage-first@example.com')

    await wrapper.get('[data-test="usage-sort-trigger-usage"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="usage-sort-usage-today"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('usage-first@example.com,last-used-first@example.com')
    expect(localStorage.getItem('admin-users-usage-sort')).toContain('"key":"usage"')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(localStorage.getItem('admin-users-usage-sort')).toBeNull()
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('last-used-first@example.com,usage-first@example.com')
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('keeps selected user IDs across pages and clears them after a successful bulk update', async () => {
    let refreshed = false
    listUsers.mockImplementation(async (page: number) => {
      const user = page === 2
        ? createAdminUser({
            id: 43,
            email: refreshed ? 'refreshed-page-two@example.com' : 'page-two@example.com'
          })
        : createAdminUser({ id: 42, email: 'page-one@example.com' })
      return {
        items: [user],
        total: 2,
        page,
        page_size: 20,
        pages: 2
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(false)
    await wrapper.get('[data-test="select-42"]').trigger('click')
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')
    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(true)

    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')

    await wrapper.get('[data-test="select-43"]').trigger('click')
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42,43')

    await wrapper.get('[data-test="bulk-edit-limits"]').trigger('click')
    expect(wrapper.get('[data-test="bulk-modal-ids"]').text()).toBe('42,43')

    const callsBeforeSuccess = listUsers.mock.calls.length
    refreshed = true
    await wrapper.get('[data-test="bulk-success"]').trigger('click')
    await flushPromises()

    expect(listUsers.mock.calls.length).toBeGreaterThan(callsBeforeSuccess)
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('refreshed-page-two@example.com')
    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('')
  })

  it('hides super-admin-only user controls from an operator', async () => {
    authState.isAdmin = false
    authState.isOperator = true
    localStorage.setItem('user-column-settings-version', '3')
    localStorage.setItem('user-hidden-columns', JSON.stringify([]))
    listEnabledDefinitions.mockResolvedValue([{
      id: 1,
      key: 'operator_visible_attribute',
      name: 'Operator Visible Attribute',
      description: '',
      type: 'text',
      options: [],
      required: false,
      validation: {},
      placeholder: '',
      display_order: 1,
      enabled: true,
      created_at: '2026-07-20T00:00:00Z',
      updated_at: '2026-07-20T00:00:00Z'
    }])
    listUsers.mockResolvedValue({
      items: [
        createAdminUser({ id: 41, email: 'admin@example.com', role: 'admin', balance: 10 }),
        createAdminUser({ id: 42, email: 'operator@example.com', role: 'operator', balance: 20 }),
        createAdminUser({ id: 43, email: 'user@example.com', role: 'user', balance: 30 })
      ],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: UserCreateRoleModalStub,
          UserEditModal: UserEditRoleModalStub,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: UserApiKeysPermissionStub,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          UserPlatformQuotaCell: { template: '<span data-test="quota-summary">quota</span>' },
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await new Promise((resolve) => window.setTimeout(resolve, 60))
    await flushPromises()

    expect(wrapper.get('[data-test="users-table"]').attributes('data-selectable')).toBe('false')
    expect(wrapper.get('[data-test="columns"]').text().split(',')).not.toContain('balance_platform_quota')
    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="create-user-modal"]').attributes('data-can-manage-admin-role')).toBe('false')
    expect(wrapper.get('[data-test="edit-user-modal"]').attributes('data-can-manage-admin-role')).toBe('false')
    expect(wrapper.get('[data-test="api-keys-modal"]').attributes('data-can-edit-group')).toBe('false')
    expect(wrapper.findAll('.action-menu-trigger')).toHaveLength(1)
    expect(wrapper.findAll('button[title="admin.users.platformQuota.cellColumnTooltip"]')).toHaveLength(0)
    expect(wrapper.text()).not.toContain('admin.users.attributes.configButton')
    expect(getPlatformQuotas).not.toHaveBeenCalled()
    expect(getBatchUsersUsage).toHaveBeenCalledWith([41, 42, 43])
    expect(getBatchUserAttributes).toHaveBeenCalledWith([41, 42, 43])
  })
})
