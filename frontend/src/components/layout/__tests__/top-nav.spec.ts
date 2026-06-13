import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(testDir, '../AppHeader.vue'), 'utf8')
const bottomNavSource = readFileSync(resolve(testDir, '../AppBottomNav.vue'), 'utf8')
const sidebarSource = readFileSync(resolve(testDir, '../AppSidebar.vue'), 'utf8')

describe('primary Use Token navigation', () => {
  it('wires Use Token into desktop and mobile primary navigation behind the chat-page flag', () => {
    for (const source of [headerSource, bottomNavSource]) {
      expect(source).toContain('FeatureFlags.chatPage')
      expect(source).toContain('FeatureFlags.modelMarketplace')
      expect(source).toContain("path: '/chat'")
      expect(source).toContain("label: t('nav.useToken')")
      expect(source).toContain("route.path === '/chat' || route.path === '/use-token'")
    }
  })

  it('keeps the old chat entry out of the console sidebar', () => {
    expect(sidebarSource).not.toContain("path: '/chat'")
    expect(sidebarSource).not.toContain("label: t('nav.chat')")
  })
})
