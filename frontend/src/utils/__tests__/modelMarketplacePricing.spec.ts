import { describe, expect, it } from 'vitest'
import type { UserSupportedModelPricing } from '@/api/channels'
import { BILLING_MODE_PER_REQUEST, BILLING_MODE_TOKEN } from '@/constants/channel'
import {
  buildModelMarketplacePriceSummary,
  modelMarketplaceChannelFinalRate,
  type ModelMarketplacePriceLabels,
} from '@/utils/modelMarketplacePricing'

const labels: ModelMarketplacePriceLabels = {
  perRequestUnit: '/ request',
  unitPerThousandTokens: '/1K',
  unitPerMillionTokens: '/1M',
  inputPrice: 'Input',
  outputPrice: 'Output',
  cacheReadPrice: 'Cache read',
}

describe('model marketplace pricing', () => {
  it('combines channel multiplier and exchange discount into the final channel rate', () => {
    expect(modelMarketplaceChannelFinalRate(0.5, 1 / 8)).toBe(0.0625)
    expect(modelMarketplaceChannelFinalRate(null, 1 / 8)).toBe(0.125)
    expect(modelMarketplaceChannelFinalRate(0.5, 0)).toBe(0.5)
  })

  it('formats channel final prices from original price multiplied by the combined rate', () => {
    const pricing: UserSupportedModelPricing = {
      billing_mode: BILLING_MODE_TOKEN,
      input_price: 0.00001,
      output_price: 0.00002,
      cache_write_price: null,
      cache_read_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: [],
    }

    const finalRate = modelMarketplaceChannelFinalRate(0.5, 1 / 8)

    expect(buildModelMarketplacePriceSummary(pricing, finalRate, '1M', labels)).toEqual([
      'Input $0.625 /1M',
      'Output $1.25 /1M',
    ])
  })

  it('applies the same combined rate to per-request prices', () => {
    const pricing: UserSupportedModelPricing = {
      billing_mode: BILLING_MODE_PER_REQUEST,
      input_price: null,
      output_price: null,
      cache_write_price: null,
      cache_read_price: null,
      image_output_price: null,
      per_request_price: 2,
      intervals: [],
    }

    expect(buildModelMarketplacePriceSummary(pricing, 0.25, '1M', labels)).toEqual([
      '$0.5 / request',
    ])
  })
})
