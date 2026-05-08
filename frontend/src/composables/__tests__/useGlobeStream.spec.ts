import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useGlobeStream, type GlobeDataMode } from '@/composables/useGlobeStream'

describe('useGlobeStream', () => {
  const eventSources: MockEventSource[] = []

  class MockEventSource {
    url: string
    close = vi.fn()
    addEventListener = vi.fn()

    constructor(url: string) {
      this.url = url
      eventSources.push(this)
    }
  }

  beforeEach(() => {
    vi.useFakeTimers()
    eventSources.length = 0
    vi.stubGlobal('EventSource', MockEventSource)
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      const data = url.includes('/summary')
        ? {
            generated_at: '2026-05-08T00:00:00Z',
            window_24h: { calls: 0, unique_ips: 0, unique_countries: 0 },
            window_all_time: { calls: 0, unique_ips: 0, unique_countries: 0 },
            top_countries: [],
            hourly_history_24h: [],
            geo_coverage: {},
          }
        : {
            generated_at: '2026-05-08T00:00:00Z',
            mode: url.includes('mode=all_time') ? 'all_time' : 'live',
            window_ms: url.includes('mode=all_time') ? 0 : 300000,
            interval_ms: 300000,
            arcs: [],
            countries: [],
            total_calls: 0,
            unique_ips: 0,
            unresolved_ips: 0,
            geo_cache_size: 0,
          }
      return { json: async () => ({ code: 0, data }) } as Response
    }))
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('uses the all-time REST snapshot instead of opening SSE in historical mode', async () => {
    const mode = ref<GlobeDataMode>('all_time')
    const wrapper = mount(defineComponent({
      setup: () => useGlobeStream({ mode }),
      template: '<div />',
    }))

    await flushPromises()

    expect(eventSources).toHaveLength(0)
    expect(fetch).toHaveBeenCalledWith('/api/public/globe/snapshot?mode=all_time', expect.any(Object))

    wrapper.unmount()
  })

  it('closes the live stream and fetches the all-time snapshot when mode changes', async () => {
    const mode = ref<GlobeDataMode>('live')
    const wrapper = mount(defineComponent({
      setup: () => useGlobeStream({ mode }),
      template: '<div />',
    }))

    await flushPromises()
    expect(eventSources).toHaveLength(1)
    expect(eventSources[0].url).toBe('/api/public/globe/stream')

    mode.value = 'all_time'
    await nextTick()
    await flushPromises()

    expect(eventSources[0].close).toHaveBeenCalled()
    expect(fetch).toHaveBeenCalledWith('/api/public/globe/snapshot?mode=all_time', expect.any(Object))

    wrapper.unmount()
  })
})
