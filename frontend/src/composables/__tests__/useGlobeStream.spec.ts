import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useGlobeStream, type GlobeDataMode } from '@/composables/useGlobeStream'

describe('useGlobeStream', () => {
  const eventSources: MockEventSource[] = []

  class MockEventSource {
    url: string
    close = vi.fn()
    private listeners = new Map<string, Array<(event: Event) => void>>()
    addEventListener = vi.fn((type: string, listener: EventListenerOrEventListenerObject) => {
      const callback = typeof listener === 'function'
        ? listener
        : (event: Event) => listener.handleEvent(event)
      const callbacks = this.listeners.get(type) || []
      callbacks.push(callback)
      this.listeners.set(type, callbacks)
    })

    constructor(url: string) {
      this.url = url
      eventSources.push(this)
    }

    emit(type: string, event: Event) {
      for (const listener of this.listeners.get(type) || []) {
        listener(event)
      }
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

  it('falls back to REST polling when a live stream never delivers its first frame', async () => {
    const wrapper = mount(defineComponent({
      setup: () => useGlobeStream({ mode: 'live' }),
      template: '<div />',
    }))

    await flushPromises()
    expect(eventSources).toHaveLength(1)
    expect(fetch).not.toHaveBeenCalledWith('/api/public/globe/snapshot', expect.any(Object))

    // The SSE interval is five minutes and the watchdog allows another 15s.
    // Its 30s tick observes the stale first-frame deadline at 5m30s.
    await vi.advanceTimersByTimeAsync(330_000)
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('/api/public/globe/snapshot', expect.any(Object))

    wrapper.unmount()
  })

  it('uses the new connection start time after a failed all-time round trip', async () => {
    const mode = ref<GlobeDataMode>('live')
    const wrapper = mount(defineComponent({
      setup: () => useGlobeStream({ mode }),
      template: '<div />',
    }))
    await flushPromises()

    eventSources[0].emit('snapshot', new MessageEvent('snapshot', {
      data: JSON.stringify({ generated_at: '2026-05-08T00:00:00Z', arcs: [], countries: [] }),
    }))
    await vi.advanceTimersByTimeAsync(300_000)

    vi.mocked(fetch).mockRejectedValueOnce(new Error('all-time unavailable'))
    mode.value = 'all_time'
    await nextTick()
    await flushPromises()

    mode.value = 'live'
    await nextTick()
    await flushPromises()
    expect(eventSources).toHaveLength(2)

    vi.mocked(fetch).mockClear()
    await vi.advanceTimersByTimeAsync(30_000)
    await flushPromises()

    expect(fetch).not.toHaveBeenCalledWith('/api/public/globe/snapshot', expect.any(Object))

    wrapper.unmount()
  })

  it('stops degraded REST polling when the SSE connection recovers', async () => {
    const wrapper = mount(defineComponent({
      setup: () => useGlobeStream({ mode: 'live' }),
      template: '<div />',
    }))
    await flushPromises()

    await vi.advanceTimersByTimeAsync(330_000)
    await flushPromises()
    expect(fetch).toHaveBeenCalledWith('/api/public/globe/snapshot', expect.any(Object))

    eventSources[0].emit('snapshot', new MessageEvent('snapshot', {
      data: JSON.stringify({ generated_at: '2026-05-08T00:05:30Z', arcs: [], countries: [] }),
    }))
    vi.mocked(fetch).mockClear()

    // The degraded polling interval would fire after five minutes if the
    // recovered SSE frame had not cancelled it.
    await vi.advanceTimersByTimeAsync(300_000)
    await flushPromises()

    expect(fetch).not.toHaveBeenCalledWith('/api/public/globe/snapshot', expect.any(Object))

    wrapper.unmount()
  })
})
