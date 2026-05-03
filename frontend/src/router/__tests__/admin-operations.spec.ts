import { describe, expect, it } from 'vitest'
import router from '../index'

describe('admin operations route', () => {
  it('allows any admin account', () => {
    const route = router.getRoutes().find((item) => item.path === '/admin/operations')

    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(true)
    expect(route?.meta.requiresSuperAdmin).toBeUndefined()
  })
})
