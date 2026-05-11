import { describe, expect, it } from 'vitest'
import router from '../index'

describe('admin operations route', () => {
  it('allows any admin account', () => {
    const route = router.getRoutes().find((item) => item.path === '/admin/operations')

    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(true)
    expect(route?.meta.requiresSuperAdmin).toBeUndefined()
  })

  it('registers the user model status page on channel monitor logic', () => {
    const route = router.getRoutes().find((item) => item.path === '/status')

    expect(route?.name).toBe('Status')
    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(false)
  })

  it('registers the public and admin live globe pages', () => {
    const publicRoute = router.getRoutes().find((item) => item.path === '/globe')
    const adminRoute = router.getRoutes().find((item) => item.path === '/admin/globe')

    expect(publicRoute?.name).toBe('GlobeShowcase')
    expect(publicRoute?.meta.requiresAuth).toBe(false)
    expect(adminRoute?.name).toBe('AdminGlobe')
    expect(adminRoute?.meta.requiresAdmin).toBe(true)
  })

  it('registers the admin model marketplace and legacy model-health redirect', () => {
    const marketplace = router.getRoutes().find((item) => item.path === '/admin/model-marketplace')
    const modelHealth = router.getRoutes().find((item) => item.path === '/admin/model-health')

    expect(marketplace?.meta.requiresAuth).toBe(true)
    expect(marketplace?.meta.requiresAdmin).toBe(true)
    expect(marketplace?.meta.requiresSuperAdmin).toBe(true)
    expect(modelHealth?.redirect).toBe('/admin/model-marketplace')
  })
})
