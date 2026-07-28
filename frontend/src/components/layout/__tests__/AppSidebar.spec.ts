import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')
const routerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../router/index.ts')
const routerSource = readFileSync(routerPath, 'utf8')
const zhLocalePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../i18n/locales/zh/custom.ts')
const zhLocaleSource = readFileSync(zhLocalePath, 'utf8')
const enLocalePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../i18n/locales/en/custom.ts')
const enLocaleSource = readFileSync(enLocalePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar client install entry', () => {
  it('keeps one-click deploy wired through sidebar, route, and locales', () => {
    expect(componentSource).toContain("path: '/client-install'")
    expect(componentSource).toContain("label: t('nav.clientInstall')")
    expect(routerSource).toContain("path: '/client-install'")
    expect(routerSource).toContain("name: 'ClientInstall'")
    expect(routerSource).toContain("titleKey: 'clientInstallPage.title'")
    expect(zhLocaleSource).toContain('"clientInstall": "一键部署"')
    expect(zhLocaleSource).toContain('"title": "一键部署"')
    expect(enLocaleSource).toContain('"clientInstall": "One-Click Deploy"')
    expect(enLocaleSource).toContain('"title": "One-Click Deploy"')
  })
})

describe('AppSidebar audit entries', () => {
  it('uses distinct operation and security audit labels', () => {
    expect(componentSource).toContain("path: '/admin/audit-logs', label: t('nav.operationAuditLogs')")
    expect(componentSource).toContain("path: '/admin/security-audit-logs', label: t('nav.securityAuditLogs')")
    expect(componentSource).not.toContain("path: '/admin/security-audit-logs', label: t('admin.audit.title')")
    expect(zhLocaleSource).toContain('"operationAuditLogs": "操作审计日志"')
    expect(zhLocaleSource).toContain('"securityAuditLogs": "安全审计日志"')
    expect(enLocaleSource).toContain('"operationAuditLogs": "Operation Audit Logs"')
    expect(enLocaleSource).toContain('"securityAuditLogs": "Security Audit Logs"')
  })
})
