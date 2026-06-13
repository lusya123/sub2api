import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import type { PublicSettings } from '@/types'
import { FeatureFlags, isFeatureFlagEnabled } from '../featureFlags'

describe('feature flags', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('keeps model marketplace visible by default and hides it when disabled', () => {
    const appStore = useAppStore()

    expect(isFeatureFlagEnabled(FeatureFlags.modelMarketplace)).toBe(true)

    appStore.cachedPublicSettings = {
      model_health_page_enabled: false,
    } as PublicSettings

    expect(isFeatureFlagEnabled(FeatureFlags.modelMarketplace)).toBe(false)
  })
})
