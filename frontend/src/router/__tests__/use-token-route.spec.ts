import { describe, expect, it } from 'vitest'
import router from '../index'

describe('Use Token route', () => {
  it('keeps the chat launch page available as the gated Use Token page', () => {
    const route = router.getRoutes().find((item) => item.path === '/chat')

    expect(route?.name).toBe('UseToken')
    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.requiresAdmin).toBe(false)
    expect(route?.meta.requiresChatPage).toBe(true)
    expect(route?.meta.titleKey).toBe('useToken.title')
  })

  it('keeps /use-token as an alias for the existing chat URL', () => {
    const alias = router.getRoutes().find((item) => item.path === '/use-token')

    expect(alias?.name).toBe('UseToken')
    expect(alias?.meta.requiresChatPage).toBe(true)
  })
})
