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
const zhLocalePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../i18n/locales/zh.ts')
const zhLocaleSource = readFileSync(zhLocalePath, 'utf8')
const enLocalePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../i18n/locales/en.ts')
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
    expect(zhLocaleSource).toContain("clientInstall: '一键部署'")
    expect(zhLocaleSource).toContain("title: '一键部署'")
    expect(enLocaleSource).toContain("clientInstall: 'One-Click Deploy'")
    expect(enLocaleSource).toContain("title: 'One-Click Deploy'")
  })
})
