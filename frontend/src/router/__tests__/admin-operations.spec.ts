import { describe, expect, it } from 'vitest'
import router from '../index'

describe('admin operations route', () => {
  it('allows any admin account', () => {
    const route = router.getRoutes().find((item) => item.path === '/admin/operations')

    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(true)
    expect(route?.meta.requiresSuperAdmin).toBeUndefined()
  })

  it('registers the public model status page', () => {
    const route = router.getRoutes().find((item) => item.path === '/status')

    expect(route?.name).toBe('Status')
    expect(route?.meta.requiresAuth).toBe(false)
  })

  it('registers the admin model marketplace and legacy model-health redirect', () => {
    const marketplace = router.getRoutes().find((item) => item.path === '/admin/model-marketplace')
    const modelHealth = router.getRoutes().find((item) => item.path === '/admin/model-health')

    expect(marketplace?.meta.requiresAuth).toBe(true)
    expect(marketplace?.meta.requiresAdmin).toBe(true)
    expect(modelHealth?.redirect).toBe('/admin/model-marketplace')
  })
})
