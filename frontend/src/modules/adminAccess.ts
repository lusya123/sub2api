import type { RouteRecordRaw } from 'vue-router'

// Custom admin access policy. Keep role checks, operator-visible admin pages,
// and super-admin-only routes here so upstream router/sidebar changes only need
// a small policy review during future merges.

export const ADMIN_ROLE = 'admin'
export const OPERATOR_ROLE = 'operator'

export const ADMIN_DASHBOARD_PATH = '/admin/dashboard'
export const ADMIN_SETTINGS_PATH = '/admin/settings'
export const USER_DASHBOARD_PATH = '/dashboard'

export const OPERATOR_ADMIN_PATHS = new Set([
  '/admin/dashboard',
  '/admin/operations',
  '/admin/globe',
  '/admin/ops',
  '/admin/users',
  '/admin/subscriptions',
  '/admin/usage'
])

export const SUPER_ADMIN_ROUTE_PATHS = new Set([
  '/admin/audit-logs',
  '/admin/model-marketplace',
  '/admin/groups',
  '/admin/channels/pricing',
  '/admin/channels/monitor',
  '/admin/accounts',
  '/admin/announcements',
  '/admin/proxies',
  '/admin/redeem',
  '/admin/promo-codes',
  '/admin/settings',
  '/admin/risk-control',
  '/admin/affiliates/invites',
  '/admin/affiliates/rebates',
  '/admin/affiliates/transfers',
  '/admin/orders/dashboard',
  '/admin/orders',
  '/admin/orders/plans'
])

export interface AdminAccessState {
  isAdmin: boolean
  isOperator?: boolean
}

export function isAdminRole(role: unknown): boolean {
  return role === ADMIN_ROLE
}

export function isOperatorRole(role: unknown): boolean {
  return role === OPERATOR_ROLE
}

export function canAccessAdminRole(role: unknown): boolean {
  return isAdminRole(role) || isOperatorRole(role)
}

export function canAccessAdminState(state: AdminAccessState): boolean {
  return state.isAdmin || state.isOperator === true
}

export function adminHomePath(canAccessAdmin: boolean): string {
  return canAccessAdmin ? ADMIN_DASHBOARD_PATH : USER_DASHBOARD_PATH
}

export function adminSettingsOrHomePath(state: AdminAccessState): string {
  return state.isAdmin ? ADMIN_SETTINGS_PATH : adminHomePath(canAccessAdminState(state))
}

export function isOperatorAdminPath(path: string): boolean {
  return OPERATOR_ADMIN_PATHS.has(normalizePath(path))
}

export function isSuperAdminRoutePath(path: string): boolean {
  return SUPER_ADMIN_ROUTE_PATHS.has(normalizePath(path))
}

export function applyAdminAccessPolicy(routes: RouteRecordRaw[]): RouteRecordRaw[] {
  for (const route of routes) {
    if (isSuperAdminRoutePath(route.path)) {
      route.meta = {
        ...route.meta,
        requiresSuperAdmin: true
      }
    }
    if (route.children) {
      applyAdminAccessPolicy(route.children)
    }
  }
  return routes
}

function normalizePath(path: string): string {
  const trimmed = path.trim()
  if (trimmed === '/') return trimmed
  return trimmed.replace(/\/+$/, '')
}
