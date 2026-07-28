import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('model-facing surface labels', () => {
  it('keeps Model Status distinct from Model Plaza in both locales', () => {
    expect(zh.nav.channelStatus).toBe('渠道状态')
    expect(zh.nav.modelMarketplace).toBe('模型状态')
    expect(zh.nav.modelPlaza).toBe('模型广场')
    expect(zh.nav.modelMarketplace).not.toBe(zh.nav.modelPlaza)

    expect(en.nav.channelStatus).toBe('Channel Status')
    expect(en.nav.modelMarketplace).toBe('Model Status')
    expect(en.nav.modelPlaza).toBe('Model Plaza')
    expect(en.nav.modelMarketplace).not.toBe(en.nav.modelPlaza)

    expect(zh.status.title).toBe('渠道状态')
    expect(en.status.title).toBe('Channel Status')
    expect(zh.admin.modelMarketplaceMonitor.title).toBe('模型监控配置')
    expect(en.admin.modelMarketplaceMonitor.title).toBe('Model Monitoring')
  })

  it('labels monitoring prices as references rather than billing prices', () => {
    expect(zh.modelMarketplaceStatus.channelFinalPrice).toBe('监控展示价')
    expect(zh.modelMarketplaceStatus.priceDisclaimer).toContain('不参与实际结算')
    expect(zh.admin.modelMarketplaceMonitor.form.effectiveRate).toBe('监控展示倍率')
    expect(zh.admin.modelMarketplaceMonitor.form.effectiveRateHint).toContain('使用记录或账单')

    expect(en.modelMarketplaceStatus.channelFinalPrice).toBe('Monitoring display price')
    expect(en.modelMarketplaceStatus.priceDisclaimer).toContain('do not determine billing')
    expect(en.admin.modelMarketplaceMonitor.form.effectiveRate).toBe('Monitoring display rate')
    expect(en.admin.modelMarketplaceMonitor.form.effectiveRateHint).toContain('Usage records or invoices')
  })

  it('labels Model Plaza prices as estimates rather than exact charges', () => {
    expect(zh.modelPlaza.table.paidPrice).toBe('基础展示价（未含动态计费）')
    expect(zh.modelPlaza.table.officialPrice).toBe('LiteLLM 参考价')
    expect(zh.modelPlaza.filters.rateLabel).toBe('展示倍率')
    expect(zh.modelPlaza.priceDisclaimer).toContain('最终以使用记录或账单为准')

    expect(en.modelPlaza.table.paidPrice).toBe('Base Display Price (Excludes Dynamic Billing)')
    expect(en.modelPlaza.table.officialPrice).toBe('LiteLLM Reference')
    expect(en.modelPlaza.filters.rateLabel).toBe('Display rate')
    expect(en.modelPlaza.priceDisclaimer).toContain('usage records or invoices are authoritative')
  })

  it('keeps monitor validation copy in the namespace used by the form', () => {
    expect(zh.admin.modelMarketplaceMonitor.effectiveRateRequired).toContain('监控展示倍率')
    expect(en.admin.modelMarketplaceMonitor.effectiveRateRequired).toContain('monitoring display rate')
  })
})
