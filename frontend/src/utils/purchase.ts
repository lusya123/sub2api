import type { PublicSettings, PurchaseSubscriptionMode } from '@/types'
import { buildEmbeddedUrl } from './embedded-url'

type PurchaseSettingsLike = Pick<
  PublicSettings,
  'purchase_subscription_mode' |
  'purchase_subscription_embedded_url' |
  'purchase_subscription_redirect_url' |
  'purchase_subscription_url'
>

export function normalizePurchaseSubscriptionMode(
  mode?: string | null,
): PurchaseSubscriptionMode {
  return mode === 'redirect' ? 'redirect' : 'embedded'
}

export function getPurchaseEmbeddedBaseUrl(settings?: PurchaseSettingsLike | null): string {
  return (
    settings?.purchase_subscription_embedded_url ||
    settings?.purchase_subscription_url ||
    ''
  ).trim()
}

export function getPurchaseRedirectUrl(settings?: PurchaseSettingsLike | null): string {
  return (settings?.purchase_subscription_redirect_url || '').trim()
}

export function buildPurchaseEmbeddedUrl(
  settings: PurchaseSettingsLike | null | undefined,
  userId?: number,
  authToken?: string,
  theme?: 'light' | 'dark',
  lang?: string,
): string {
  return buildEmbeddedUrl(
    getPurchaseEmbeddedBaseUrl(settings),
    userId,
    authToken,
    theme,
    lang,
  )
}

export function isAbsoluteHttpUrl(rawUrl: string): boolean {
  try {
    const url = new URL(rawUrl)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}
