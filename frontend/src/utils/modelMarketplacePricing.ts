import type { UserSupportedModelPricing } from '@/api/channels'
import { BILLING_MODE_IMAGE, BILLING_MODE_PER_REQUEST, BILLING_MODE_TOKEN } from '@/constants/channel'
import { formatScaled } from '@/utils/pricing'

export type ModelMarketplacePriceUnit = '1M' | '1K'

export interface ModelMarketplacePriceLabels {
  perRequestUnit: string
  unitPerThousandTokens: string
  unitPerMillionTokens: string
  inputPrice: string
  outputPrice: string
  cacheReadPrice: string
}

export function modelMarketplaceChannelFinalRate(
  effectiveRate: number | null | undefined,
  exchangeDiscountRate: number,
): number {
  const safeEffectiveRate = Number.isFinite(effectiveRate) && Number(effectiveRate) > 0
    ? Number(effectiveRate)
    : 1
  const safeExchangeDiscountRate = Number.isFinite(exchangeDiscountRate) && exchangeDiscountRate > 0
    ? exchangeDiscountRate
    : 1
  return safeEffectiveRate * safeExchangeDiscountRate
}

export function buildModelMarketplacePriceSummary(
  pricing: UserSupportedModelPricing | null,
  rate: number,
  priceUnit: ModelMarketplacePriceUnit,
  labels: ModelMarketplacePriceLabels,
): string[] {
  if (!pricing) return []
  if (pricing.billing_mode === BILLING_MODE_PER_REQUEST && pricing.per_request_price != null) {
    return [`${formatScaled(pricing.per_request_price * rate, 1)} ${labels.perRequestUnit}`]
  }
  if (pricing.billing_mode === BILLING_MODE_IMAGE && pricing.image_output_price != null) {
    return [`${formatScaled(pricing.image_output_price * rate, 1)} ${labels.perRequestUnit}`]
  }
  if (pricing.billing_mode !== BILLING_MODE_TOKEN) return []

  const scale = priceUnit === '1K' ? 1_000 : 1_000_000
  const unit = priceUnit === '1K' ? labels.unitPerThousandTokens : labels.unitPerMillionTokens
  const lines = [
    pricing.input_price == null
      ? ''
      : `${labels.inputPrice} ${formatScaled(pricing.input_price * rate, scale)} ${unit}`,
    pricing.output_price == null
      ? ''
      : `${labels.outputPrice} ${formatScaled(pricing.output_price * rate, scale)} ${unit}`,
    pricing.cache_read_price == null
      ? ''
      : `${labels.cacheReadPrice} ${formatScaled(pricing.cache_read_price * rate, scale)} ${unit}`,
  ]
  return lines.filter(Boolean)
}
