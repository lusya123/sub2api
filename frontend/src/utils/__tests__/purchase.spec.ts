import { describe, expect, it } from 'vitest'
import {
  buildPurchaseEmbeddedUrl,
  getPurchaseEmbeddedBaseUrl,
  getPurchaseRedirectUrl,
  isAbsoluteHttpUrl,
  normalizePurchaseSubscriptionMode,
} from '../purchase'

describe('purchase utils', () => {
  it('normalizes purchase mode to embedded by default', () => {
    expect(normalizePurchaseSubscriptionMode(undefined)).toBe('embedded')
    expect(normalizePurchaseSubscriptionMode('invalid')).toBe('embedded')
    expect(normalizePurchaseSubscriptionMode('redirect')).toBe('redirect')
  })

  it('prefers the dedicated embedded url and falls back to the legacy url', () => {
    expect(
      getPurchaseEmbeddedBaseUrl({
        purchase_subscription_mode: 'embedded',
        purchase_subscription_embedded_url: 'https://pay.example.com/embed',
        purchase_subscription_redirect_url: '',
        purchase_subscription_url: 'https://pay.example.com/legacy',
      }),
    ).toBe('https://pay.example.com/embed')

    expect(
      getPurchaseEmbeddedBaseUrl({
        purchase_subscription_mode: 'embedded',
        purchase_subscription_embedded_url: '',
        purchase_subscription_redirect_url: '',
        purchase_subscription_url: 'https://pay.example.com/legacy',
      }),
    ).toBe('https://pay.example.com/legacy')
  })

  it('returns redirect url as-is', () => {
    expect(
      getPurchaseRedirectUrl({
        purchase_subscription_mode: 'redirect',
        purchase_subscription_embedded_url: '',
        purchase_subscription_redirect_url: 'https://pay.example.com/topup',
        purchase_subscription_url: '',
      }),
    ).toBe('https://pay.example.com/topup')
  })

  it('builds embedded purchase url with existing iframe params', () => {
    const result = buildPurchaseEmbeddedUrl(
      {
        purchase_subscription_mode: 'embedded',
        purchase_subscription_embedded_url: 'https://pay.example.com/embed?plan=pro',
        purchase_subscription_redirect_url: '',
        purchase_subscription_url: '',
      },
      7,
      'token-abc',
      'dark',
      'zh-CN',
    )

    const url = new URL(result)
    expect(url.searchParams.get('plan')).toBe('pro')
    expect(url.searchParams.get('user_id')).toBe('7')
    expect(url.searchParams.get('token')).toBe('token-abc')
    expect(url.searchParams.get('ui_mode')).toBe('embedded')
  })

  it('validates absolute http(s) urls only', () => {
    expect(isAbsoluteHttpUrl('https://example.com')).toBe(true)
    expect(isAbsoluteHttpUrl('http://example.com')).toBe(true)
    expect(isAbsoluteHttpUrl('javascript:alert(1)')).toBe(false)
    expect(isAbsoluteHttpUrl('/purchase')).toBe(false)
  })
})
