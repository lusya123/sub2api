<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="grid gap-3 md:grid-cols-3 xl:grid-cols-6">
          <input v-model="filters.q" class="input" type="search" :placeholder="t('admin.auditLogs.searchPlaceholder')" @keyup.enter="applyFilters" />
          <input v-model="filters.actor_user_id" class="input" type="number" :placeholder="t('admin.auditLogs.actorId')" @keyup.enter="applyFilters" />
          <Select v-model="filters.actor_role" :options="roleOptions" />
          <Select v-model="filters.module" :options="moduleOptions" />
          <Select v-model="filters.action_type" :options="actionTypeOptions" />
          <Select v-model="filters.success" :options="successOptions" />
          <input v-model="filters.target_type" class="input" type="text" :placeholder="t('admin.auditLogs.targetType')" @keyup.enter="applyFilters" />
          <input v-model="filters.target_id" class="input" type="text" :placeholder="t('admin.auditLogs.targetId')" @keyup.enter="applyFilters" />
          <input v-model="filters.status_code" class="input" type="number" :placeholder="t('admin.auditLogs.statusCode')" @keyup.enter="applyFilters" />
          <input v-model="filters.start_time" class="input" type="datetime-local" />
          <input v-model="filters.end_time" class="input" type="datetime-local" />
          <label class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 px-3 py-2 text-sm text-gray-700 dark:border-dark-600 dark:text-dark-200">
            <span>{{ t('admin.auditLogs.showReadLogs') }}</span>
            <Toggle v-model="showReadLogs" />
          </label>
          <div class="flex gap-2">
            <button class="btn btn-primary flex-1" @click="applyFilters">
              {{ t('common.search') }}
            </button>
            <button class="btn btn-secondary flex-1" @click="resetFilters">
              {{ t('common.reset') }}
            </button>
          </div>
        </div>
      </div>

      <div class="overflow-hidden rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-900/60">
              <tr>
                <th v-for="col in columns" :key="col.key" class="px-4 py-3 text-left text-xs font-medium uppercase text-gray-500 dark:text-dark-400">
                  {{ col.label }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="loading">
                <td :colspan="columns.length" class="px-4 py-10 text-center text-sm text-gray-500">
                  {{ t('common.loading') }}
                </td>
              </tr>
              <tr v-else-if="logs.length === 0">
                <td :colspan="columns.length" class="px-4 py-10 text-center text-sm text-gray-500">
                  {{ t('admin.auditLogs.empty') }}
                </td>
              </tr>
              <tr v-for="log in logs" v-else :key="log.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-600 dark:text-dark-300">{{ formatDateTime(log.created_at) }}</td>
                <td class="px-4 py-3 text-sm">
                  <div class="font-medium text-gray-900 dark:text-white">{{ log.actor_email || '-' }}</div>
                  <div class="text-xs text-gray-500">#{{ log.actor_user_id || '-' }}</div>
                </td>
                <td class="px-4 py-3">
                  <span :class="['badge', log.actor_role === 'admin' ? 'badge-purple' : 'badge-gray']">{{ roleLabel(log.actor_role) }}</span>
                </td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-dark-200">{{ moduleLabel(log.module) }}</td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-dark-300">
                  {{ formatTarget(log) }}
                </td>
                <td class="min-w-80 px-4 py-3 text-sm">
                  <div class="font-medium text-gray-900 dark:text-white">{{ operationView(log).title }}</div>
                  <div v-if="operationView(log).details.length" class="mt-1 space-y-0.5 text-xs text-gray-500 dark:text-dark-400">
                    <div v-for="detail in operationView(log).details.slice(0, 3)" :key="detail.label">
                      <span class="font-medium">{{ detail.label }}:</span> {{ detail.value }}
                    </div>
                  </div>
                </td>
                <td class="px-4 py-3">
                  <span :class="['badge', log.success ? 'badge-green' : 'badge-red']">
                    {{ log.success ? t('admin.auditLogs.success') : t('admin.auditLogs.failed') }}
                  </span>
                  <span class="ml-2 text-xs text-gray-500">{{ log.status_code }}</span>
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-600 dark:text-dark-300">{{ log.duration_ms }}ms</td>
                <td class="whitespace-nowrap px-4 py-3 text-sm text-gray-600 dark:text-dark-300">{{ log.ip_address || '-' }}</td>
                <td class="px-4 py-3 text-right">
                  <button class="btn btn-ghost btn-sm" @click="openDetail(log)">
                    {{ t('admin.auditLogs.viewDetail') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </div>
    </div>

    <BaseDialog :show="showDetail" :title="t('admin.auditLogs.detailTitle')" width="extra-wide" @close="showDetail = false">
      <div v-if="selectedLog" class="space-y-4">
        <div class="rounded-lg border border-blue-100 bg-blue-50 p-4 dark:border-blue-500/30 dark:bg-blue-500/10">
          <div class="text-xs font-medium uppercase text-blue-600 dark:text-blue-300">{{ t('admin.auditLogs.humanSummary') }}</div>
          <div class="mt-1 text-base font-semibold text-gray-900 dark:text-white">{{ selectedOperation.title }}</div>
          <div v-if="selectedOperation.details.length" class="mt-3 grid gap-2 text-sm md:grid-cols-2">
            <div v-for="detail in selectedOperation.details" :key="detail.label" class="rounded-md bg-white/70 px-3 py-2 dark:bg-dark-800/60">
              <span class="font-medium text-gray-700 dark:text-dark-200">{{ detail.label }}:</span>
              <span class="ml-1 text-gray-900 dark:text-white">{{ detail.value }}</span>
            </div>
          </div>
        </div>
        <div class="grid gap-3 text-sm md:grid-cols-2">
          <div v-for="item in detailItems" :key="item.label" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-700/60">
            <div class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ item.label }}</div>
            <div class="mt-1 break-words text-gray-900 dark:text-white">{{ item.value || '-' }}</div>
          </div>
        </div>
        <div>
          <div class="mb-2 text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('admin.auditLogs.context') }}</div>
          <pre class="max-h-96 overflow-auto rounded-lg bg-gray-950 p-4 text-xs text-gray-100">{{ selectedLogContext }}</pre>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminAuditLog, AdminAuditLogFilters } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'

const { t } = useI18n()
const appStore = useAppStore()

const logs = ref<AdminAuditLog[]>([])
const loading = ref(false)
const showDetail = ref(false)
const selectedLog = ref<AdminAuditLog | null>(null)
const showReadLogs = ref(false)

const filters = reactive<AdminAuditLogFilters>({
  q: '',
  actor_user_id: '',
  actor_role: '',
  module: '',
  action_type: '',
  target_type: '',
  target_id: '',
  status_code: '',
  success: '',
  start_time: '',
  end_time: ''
})

const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0
})

const columns = computed(() => [
  { key: 'created_at', label: t('admin.auditLogs.columns.time') },
  { key: 'actor', label: t('admin.auditLogs.columns.actor') },
  { key: 'role', label: t('admin.auditLogs.columns.role') },
  { key: 'module', label: t('admin.auditLogs.columns.module') },
  { key: 'target', label: t('admin.auditLogs.columns.target') },
  { key: 'operation', label: t('admin.auditLogs.columns.operation') },
  { key: 'result', label: t('admin.auditLogs.columns.result') },
  { key: 'duration', label: t('admin.auditLogs.columns.duration') },
  { key: 'ip', label: t('admin.auditLogs.columns.ip') },
  { key: 'actions', label: t('admin.auditLogs.columns.actions') }
])

const roleOptions = computed(() => [
  { value: '', label: t('admin.auditLogs.allRoles') },
  { value: 'admin', label: t('admin.users.roles.admin') },
  { value: 'operator', label: t('admin.users.roles.operator') }
])

const moduleOptions = computed(() => [
  { value: '', label: t('admin.auditLogs.allModules') },
  ...['dashboard', 'ops', 'users', 'subscriptions', 'usage', 'settings', 'audit-logs'].map(value => ({
    value,
    label: moduleLabel(value)
  }))
])

const actionTypeOptions = computed(() => [
  { value: '', label: t('admin.auditLogs.allActionTypes') },
  { value: 'read', label: t('admin.auditLogs.actionTypes.read') },
  { value: 'write', label: t('admin.auditLogs.actionTypes.write') },
  { value: 'delete', label: t('admin.auditLogs.actionTypes.delete') }
])

const successOptions = computed(() => [
  { value: '', label: t('admin.auditLogs.allResults') },
  { value: 'true', label: t('admin.auditLogs.success') },
  { value: 'false', label: t('admin.auditLogs.failed') }
])

const loadLogs = async () => {
  loading.value = true
  const explicitReadFilter = filters.action_type === 'read'
  try {
    const response = await adminAPI.auditLogs.list({
      ...filters,
      page: pagination.page,
      page_size: pagination.page_size,
      exclude_successful_read: showReadLogs.value || explicitReadFilter ? undefined : true,
      start_time: normalizeDateTime(filters.start_time as string),
      end_time: normalizeDateTime(filters.end_time as string)
    })
    logs.value = response.items
    pagination.total = response.total
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.auditLogs.failedToLoad'))
  } finally {
    loading.value = false
  }
}

const applyFilters = () => {
  pagination.page = 1
  loadLogs()
}

const resetFilters = () => {
  Object.assign(filters, {
    q: '',
    actor_user_id: '',
    actor_role: '',
    module: '',
    action_type: '',
    target_type: '',
    target_id: '',
    status_code: '',
    success: '',
    start_time: '',
    end_time: ''
  })
  applyFilters()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadLogs()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadLogs()
}

watch(showReadLogs, () => {
  pagination.page = 1
  loadLogs()
})

const openDetail = async (log: AdminAuditLog) => {
  try {
    selectedLog.value = await adminAPI.auditLogs.getById(log.id)
  } catch {
    selectedLog.value = log
  }
  showDetail.value = true
}

const roleLabel = (role: string) => {
  const key = `admin.users.roles.${role}`
  const value = t(key)
  return value === key ? role : value
}

const moduleLabel = (module: string) => {
  const key = `admin.auditLogs.modules.${module}`
  const value = t(key)
  return value === key ? module : value
}

const actionTypeLabel = (actionType: string) => {
  const key = `admin.auditLogs.actionTypes.${actionType}`
  const value = t(key)
  return value === key ? actionType : value
}

const formatTarget = (log: AdminAuditLog) => {
  if (!log.target_type && !log.target_id) return '-'
  const type = log.target_type ? targetTypeLabel(log.target_type) : ''
  return [type, log.target_id].filter(Boolean).join(' #')
}

const targetTypeLabel = (targetType: string) => {
  const key = `admin.auditLogs.targetTypes.${targetType}`
  const value = t(key)
  return value === key ? targetType : value
}

const normalizeDateTime = (value?: string) => {
  if (!value) return undefined
  return new Date(value).toISOString()
}

const selectedLogContext = computed(() => {
  if (!selectedLog.value) return ''
  return JSON.stringify({
    route: selectedLog.value.route_template,
    path: selectedLog.value.path,
    method: selectedLog.value.method,
    summary: selectedLog.value.summary,
    query_params: selectedLog.value.query_params,
    request_body: selectedLog.value.request_body,
    error_code: selectedLog.value.error_code,
    error_message: selectedLog.value.error_message,
    user_agent: selectedLog.value.user_agent
  }, null, 2)
})

interface OperationDetail {
  label: string
  value: string
}

interface OperationView {
  title: string
  details: OperationDetail[]
}

const selectedOperation = computed(() => {
  if (!selectedLog.value) return { title: '-', details: [] }
  return operationView(selectedLog.value)
})

const operationView = (log: AdminAuditLog): OperationView => {
  const body = plainObject(log.request_body)
  const route = log.route_template || log.path || ''
  const method = (log.method || '').toUpperCase()
  const target = formatTarget(log)
  const user = userLabel(log, body)
  const subscription = subscriptionLabel(log)

  if (route === '/api/v1/admin/users' && method === 'POST') {
    return op('admin.auditLogs.operations.users.create', { user: valueOrDash(body.email) }, [
      detail('email', body.email),
      detail('username', body.username),
      detail('initialBalance', money(body.balance)),
      detail('concurrency', body.concurrency),
      detail('allowedGroups', idList(body.allowed_groups)),
      detail('notes', body.notes)
    ])
  }
  if (route === '/api/v1/admin/users/:id' && method === 'PUT') {
    return op('admin.auditLogs.operations.users.update', { user }, changedUserDetails(body))
  }
  if (route === '/api/v1/admin/users/:id' && method === 'DELETE') {
    return op('admin.auditLogs.operations.users.delete', { user }, [detail('user', user)])
  }
  if (route === '/api/v1/admin/users/:id/role') {
    const role = String(body.role || '')
    const key = role === 'operator' ? 'admin.auditLogs.operations.users.delegateOperator' : 'admin.auditLogs.operations.users.revokeOperator'
    return op(key, { user, role: roleDisplay(role) }, [detail('user', user), detail('role', roleDisplay(role))])
  }
  if (route === '/api/v1/admin/users/:id/balance') {
    const operation = String(body.operation || '')
    const key = operation === 'add'
      ? 'admin.auditLogs.operations.users.balanceAdd'
      : operation === 'subtract'
        ? 'admin.auditLogs.operations.users.balanceSubtract'
        : operation === 'set'
          ? 'admin.auditLogs.operations.users.balanceSet'
          : 'admin.auditLogs.operations.users.balanceChange'
    return op(key, { user, amount: money(body.balance) }, [
      detail('user', user),
      detail('balanceOperation', operationLabel(operation)),
      detail('amount', money(body.balance)),
      detail('notes', body.notes)
    ])
  }
  if (route === '/api/v1/admin/users/:id/replace-group') {
    return op('admin.auditLogs.operations.users.replaceGroup', {
      user,
      oldGroup: idRef(body.old_group_id),
      newGroup: idRef(body.new_group_id)
    }, [
      detail('user', user),
      detail('oldGroup', idRef(body.old_group_id)),
      detail('newGroup', idRef(body.new_group_id))
    ])
  }
  if (route === '/api/v1/admin/users/:id/attributes' && method === 'PUT') {
    return op('admin.auditLogs.operations.users.updateAttributes', { user }, [
      detail('user', user),
      detail('attributes', compactJson(body.values ?? body))
    ])
  }
  if (route === '/api/v1/admin/subscriptions/assign') {
    return op('admin.auditLogs.operations.subscriptions.assign', {
      user: userRef(body.user_id),
      group: idRef(body.group_id),
      days: days(body.validity_days)
    }, [
      detail('user', userRef(body.user_id)),
      detail('group', idRef(body.group_id)),
      detail('validityDays', days(body.validity_days)),
      detail('notes', body.notes)
    ])
  }
  if (route === '/api/v1/admin/subscriptions/bulk-assign') {
    return op('admin.auditLogs.operations.subscriptions.bulkAssign', {
      count: arrayValue(body.user_ids).length || valueOrDash(body.user_ids),
      group: idRef(body.group_id),
      days: days(body.validity_days)
    }, [
      detail('users', idList(body.user_ids)),
      detail('group', idRef(body.group_id)),
      detail('validityDays', days(body.validity_days)),
      detail('notes', body.notes)
    ])
  }
  if (route === '/api/v1/admin/subscriptions/:id/extend') {
    const n = numberValue(body.days)
    const key = n < 0 ? 'admin.auditLogs.operations.subscriptions.shorten' : 'admin.auditLogs.operations.subscriptions.extend'
    return op(key, { subscription, days: days(Math.abs(n || 0)) }, [
      detail('subscription', subscription),
      detail('daysChanged', signedDays(body.days))
    ])
  }
  if (route === '/api/v1/admin/subscriptions/:id/reset-quota') {
    return op('admin.auditLogs.operations.subscriptions.resetQuota', { subscription }, [
      detail('subscription', subscription),
      detail('resetWindows', resetWindows(body))
    ])
  }
  if (route === '/api/v1/admin/subscriptions/:id' && method === 'DELETE') {
    return op('admin.auditLogs.operations.subscriptions.revoke', { subscription }, [detail('subscription', subscription)])
  }
  if (route === '/api/v1/admin/usage/cleanup-tasks' && method === 'POST') {
    return op('admin.auditLogs.operations.usage.createCleanup', {}, [
      detail('dateRange', [body.start_date, body.end_date].filter(Boolean).join(' ~ ')),
      detail('user', body.user_id ? userRef(body.user_id) : ''),
      detail('apiKey', idRef(body.api_key_id)),
      detail('account', idRef(body.account_id)),
      detail('group', idRef(body.group_id)),
      detail('model', body.model),
      detail('requestType', body.request_type),
      detail('stream', booleanText(body.stream)),
      detail('billingType', body.billing_type),
      detail('timezone', body.timezone)
    ])
  }
  if (route === '/api/v1/admin/usage/cleanup-tasks/:id/cancel' && method === 'POST') {
    return op('admin.auditLogs.operations.usage.cancelCleanup', { task: idRef(log.target_id) }, [detail('task', idRef(log.target_id))])
  }
  if (method === 'GET') {
    return op('admin.auditLogs.operations.read', { module: moduleLabel(log.module), target }, queryDetails(log))
  }
  return op('admin.auditLogs.operations.fallback', { method, route }, [
    detail('route', route),
    detail('method', method),
    detail('rawSummary', log.summary)
  ])
}

const op = (key: string, params: Record<string, unknown>, details: Array<OperationDetail | null | undefined>): OperationView => ({
  title: t(key, params),
  details: details.filter((item): item is OperationDetail => Boolean(item && item.value && item.value !== '-'))
})

const detail = (key: string, raw: unknown): OperationDetail | null => {
  const value = valueOrDash(raw)
  if (!value || value === '-') return null
  return {
    label: fieldLabel(key),
    value
  }
}

const fieldLabel = (key: string) => {
  const i18nKey = `admin.auditLogs.fields.${key}`
  const value = t(i18nKey)
  return value === i18nKey ? key : value
}

const plainObject = (value: unknown): Record<string, unknown> => {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value as Record<string, unknown>
  return {}
}

const changedUserDetails = (body: Record<string, unknown>) => [
  detail('email', body.email),
  detail('username', body.username),
  detail('status', statusLabel(body.status)),
  detail('password', Object.prototype.hasOwnProperty.call(body, 'password') ? t('admin.auditLogs.values.changed') : ''),
  detail('balance', money(body.balance)),
  detail('concurrency', body.concurrency),
  detail('allowedGroups', idList(body.allowed_groups)),
  detail('groupRates', formatGroupRates(body.group_rates)),
  detail('soraStorageQuota', bytesText(body.sora_storage_quota_bytes)),
  detail('notes', body.notes)
]

const queryDetails = (log: AdminAuditLog) => {
  const query = plainObject(log.query_params)
  return [
    detail('route', log.route_template || log.path),
    detail('query', Object.keys(query).length ? compactJson(query) : '')
  ]
}

const userLabel = (log: AdminAuditLog, body: Record<string, unknown>) => {
  if (typeof body.email === 'string' && body.email) return `${body.email}${log.target_id ? ` (#${log.target_id})` : ''}`
  if (body.user_id) return userRef(body.user_id)
  if (log.target_type === 'user' && log.target_id) return userRef(log.target_id)
  return formatTarget(log)
}

const subscriptionLabel = (log: AdminAuditLog) => log.target_id ? `${t('admin.auditLogs.targetTypes.subscription')} #${log.target_id}` : t('admin.auditLogs.targetTypes.subscription')
const userRef = (id: unknown) => id ? `${t('admin.auditLogs.targetTypes.user')} #${id}` : '-'
const idRef = (id: unknown) => id ? `#${id}` : '-'

const valueOrDash = (value: unknown): string => {
  if (value === null || value === undefined || value === '') return '-'
  if (Array.isArray(value)) return value.length ? value.map(valueOrDash).join(', ') : '-'
  if (typeof value === 'object') return compactJson(value)
  return String(value)
}

const compactJson = (value: unknown) => {
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

const numberValue = (value: unknown) => {
  const n = Number(value)
  return Number.isFinite(n) ? n : 0
}

const money = (value: unknown) => {
  if (value === null || value === undefined || value === '') return '-'
  const n = Number(value)
  if (!Number.isFinite(n)) return valueOrDash(value)
  return `$${n.toFixed(2)}`
}

const days = (value: unknown) => {
  const n = numberValue(value)
  return n ? t('admin.auditLogs.values.days', { count: Math.abs(n) }) : '-'
}

const signedDays = (value: unknown) => {
  const n = numberValue(value)
  if (!n) return '-'
  const sign = n > 0 ? '+' : '-'
  return `${sign}${t('admin.auditLogs.values.days', { count: Math.abs(n) })}`
}

const arrayValue = (value: unknown) => Array.isArray(value) ? value : []

const idList = (value: unknown) => {
  const items = arrayValue(value)
  if (!items.length) return valueOrDash(value)
  return items.map(idRef).join(', ')
}

const formatGroupRates = (value: unknown) => {
  const obj = plainObject(value)
  const entries = Object.entries(obj)
  if (!entries.length) return ''
  return entries.map(([groupId, rate]) => {
    if (rate === null || rate === undefined) return `${idRef(groupId)} = ${t('admin.auditLogs.values.removed')}`
    return `${idRef(groupId)} = ${rate}x`
  }).join(', ')
}

const bytesText = (value: unknown) => {
  if (value === null || value === undefined || value === '') return ''
  const n = Number(value)
  if (!Number.isFinite(n)) return valueOrDash(value)
  return `${(n / 1024 / 1024 / 1024).toFixed(2)} GB`
}

const resetWindows = (body: Record<string, unknown>) => {
  const windows = [
    body.daily ? t('admin.auditLogs.fields.daily') : '',
    body.weekly ? t('admin.auditLogs.fields.weekly') : '',
    body.monthly ? t('admin.auditLogs.fields.monthly') : ''
  ].filter(Boolean)
  return windows.length ? windows.join(', ') : t('admin.auditLogs.values.none')
}

const booleanText = (value: unknown) => {
  if (value === null || value === undefined || value === '') return ''
  return value ? t('admin.auditLogs.values.yes') : t('admin.auditLogs.values.no')
}

const statusLabel = (value: unknown) => {
  if (!value) return ''
  const key = `admin.auditLogs.values.status.${value}`
  const label = t(key)
  return label === key ? String(value) : label
}

const roleDisplay = (role: string) => roleLabel(role)

const operationLabel = (operation: string) => {
  const key = `admin.auditLogs.values.balanceOperations.${operation}`
  const label = t(key)
  return label === key ? operation : label
}

const detailItems = computed(() => {
  if (!selectedLog.value) return []
  return [
    { label: t('admin.auditLogs.columns.time'), value: formatDateTime(selectedLog.value.created_at) },
    { label: t('admin.auditLogs.columns.actor'), value: `${selectedLog.value.actor_email || '-'} (#${selectedLog.value.actor_user_id || '-'})` },
    { label: t('admin.auditLogs.columns.role'), value: roleLabel(selectedLog.value.actor_role) },
    { label: t('admin.auditLogs.columns.module'), value: moduleLabel(selectedLog.value.module) },
    { label: t('admin.auditLogs.columns.action'), value: actionTypeLabel(selectedLog.value.action_type) },
    { label: t('admin.auditLogs.columns.target'), value: formatTarget(selectedLog.value) },
    { label: t('admin.auditLogs.route'), value: selectedLog.value.route_template || selectedLog.value.path },
    { label: t('admin.auditLogs.result'), value: `${selectedLog.value.success ? t('admin.auditLogs.success') : t('admin.auditLogs.failed')} (${selectedLog.value.status_code})` },
    { label: t('admin.auditLogs.duration'), value: `${selectedLog.value.duration_ms}ms` },
    { label: t('admin.auditLogs.ip'), value: selectedLog.value.ip_address || '-' }
  ]
})

onMounted(loadLogs)
</script>
