import { describe, expect, it } from 'vitest'
import type { RouteRecordRaw } from 'vue-router'
import {
  adminHomePath,
  adminSettingsOrHomePath,
  applyAdminAccessPolicy,
  canAccessAdminRole,
  isOperatorAdminPath,
  isSuperAdminRoutePath
} from '../adminAccess'

describe('admin access policy', () => {
  it('centralizes role access checks', () => {
    expect(canAccessAdminRole('admin')).toBe(true)
    expect(canAccessAdminRole('operator')).toBe(true)
    expect(canAccessAdminRole('user')).toBe(false)
  })

  it('keeps the operator admin menu allowlist focused', () => {
    expect(isOperatorAdminPath('/admin/users')).toBe(true)
    expect(isOperatorAdminPath('/admin/subscriptions')).toBe(true)
    expect(isOperatorAdminPath('/admin/settings')).toBe(false)
    expect(isOperatorAdminPath('/admin/orders')).toBe(false)
  })

  it('marks super admin routes from the shared policy', () => {
    const routes: RouteRecordRaw[] = [
      { path: '/admin/users', component: {} },
      { path: '/admin/settings', component: {} }
    ]

    applyAdminAccessPolicy(routes)

    expect(routes[0].meta?.requiresSuperAdmin).toBeUndefined()
    expect(routes[1].meta?.requiresSuperAdmin).toBe(true)
    expect(isSuperAdminRoutePath('/admin/settings')).toBe(true)
  })

  it('resolves admin fallback destinations consistently', () => {
    expect(adminHomePath(true)).toBe('/admin/dashboard')
    expect(adminHomePath(false)).toBe('/dashboard')
    expect(adminSettingsOrHomePath({ isAdmin: true })).toBe('/admin/settings')
    expect(adminSettingsOrHomePath({ isAdmin: false, isOperator: true })).toBe('/admin/dashboard')
  })
})
