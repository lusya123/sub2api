/**
 * useGlobeStream — connects to /api/public/globe/stream (SSE) and exposes the
 * latest snapshot as a reactive ref. Falls back to polling /snapshot if SSE
 * isn't available (e.g. behind a proxy that buffers responses).
 *
 * The composable owns ONE EventSource for the lifetime of its mount; the
 * frontend component re-renders off the reactive snapshot. We do not maintain
 * a queue or a history — the LiveGlobe component itself synthesises animation
 * density from each fresh snapshot's `arcs` array.
 */

import { ref, shallowRef, onMounted, onBeforeUnmount, unref, watch, type Ref } from 'vue'

const GLOBE_REFRESH_MS = 5 * 60 * 1000
const WATCHDOG_STALE_MS = GLOBE_REFRESH_MS + 15_000

export interface GlobeArc {
  ip?: string
  cc: string
  country: string
  region?: string
  city?: string
  lat: number
  lng: number
  calls: number
  ip_mask?: string
}

export interface GlobeCountry {
  cc: string
  country: string
  lat: number
  lng: number
  calls: number
}

export interface ServerPoint {
  lat: number
  lng: number
  label: string
}

export interface GlobeSnapshot {
  generated_at: string
  mode?: GlobeDataMode
  window_ms: number
  interval_ms: number
  arcs: GlobeArc[]
  countries: GlobeCountry[]
  total_calls: number
  unique_ips: number
  unresolved_ips: number
  geo_cache_size: number
  server_location?: ServerPoint
}

export interface GlobeSummary {
  generated_at: string
  window_24h: { calls: number; unique_ips: number; unique_countries: number }
  window_all_time: { calls: number; unique_ips: number; unique_countries: number }
  top_countries: GlobeCountry[]
  hourly_history_24h: Array<{ hour_utc: string; calls: number }>
  server_location?: ServerPoint
  geo_coverage: { total_distinct_ips?: number; resolved_ips?: number; coverage_pct?: number }
}

export type GlobeDataMode = 'live' | 'all_time'

interface UseGlobeStreamOptions {
  mode?: Ref<GlobeDataMode> | GlobeDataMode
}

export function useGlobeStream(options: UseGlobeStreamOptions = {}) {
  const snapshot = shallowRef<GlobeSnapshot | null>(null)
  const summary = shallowRef<GlobeSummary | null>(null)
  const connected = ref(false)
  const lastEventAt = ref<number>(0)

  let es: EventSource | null = null
  let pollTimer: number | null = null
  let summaryTimer: number | null = null
  let watchdog: number | null = null

  // Resolve API base — same origin in production, proxy in dev.
  const base = '/api/public/globe'
  const getMode = (): GlobeDataMode => unref(options.mode) || 'live'
  const snapshotURL = () => {
    const mode = getMode()
    return mode === 'all_time' ? `${base}/snapshot?mode=all_time` : `${base}/snapshot`
  }

  const isSuccess = (json: any) => json && (json.code === 0 || json.code === 200) && json.data
  const normalizeSnapshot = (payload: any): GlobeSnapshot | null => {
    const data = payload?.data && !Array.isArray(payload.data) ? payload.data : payload
    if (!data || typeof data !== 'object') return null
    return {
      ...data,
      arcs: Array.isArray(data.arcs) ? data.arcs : [],
      countries: Array.isArray(data.countries) ? data.countries : [],
      total_calls: Number(data.total_calls || 0),
      unique_ips: Number(data.unique_ips || 0),
      unresolved_ips: Number(data.unresolved_ips || 0),
      geo_cache_size: Number(data.geo_cache_size || 0),
      interval_ms: Number(data.interval_ms ?? GLOBE_REFRESH_MS),
      window_ms: Number(data.window_ms ?? GLOBE_REFRESH_MS),
      generated_at: data.generated_at || new Date().toISOString(),
    } as GlobeSnapshot
  }

  const fetchSummary = async () => {
    try {
      const res = await fetch(`${base}/summary`, { headers: { 'Accept': 'application/json' } })
      const json = await res.json()
      if (isSuccess(json)) {
        summary.value = json.data as GlobeSummary
      }
    } catch {
      /* swallow */
    }
  }

  const fetchSnapshotOnce = async () => {
    const requestedMode = getMode()
    try {
      const res = await fetch(snapshotURL(), { headers: { 'Accept': 'application/json' } })
      const json = await res.json()
      if (isSuccess(json)) {
        if (getMode() !== requestedMode) return
        snapshot.value = normalizeSnapshot(json.data)
        if (requestedMode === 'live') {
          lastEventAt.value = Date.now()
        } else {
          connected.value = false
          lastEventAt.value = 0
        }
      }
    } catch {
      /* swallow */
    }
  }

  const stopSSE = () => {
    es?.close()
    es = null
    connected.value = false
  }

  const stopPolling = () => {
    if (pollTimer !== null) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  const startSSE = () => {
    stopSSE()
    if (typeof window === 'undefined' || typeof EventSource === 'undefined') {
      return startPolling()
    }
    try {
      es = new EventSource(`${base}/stream`)
      es.addEventListener('open', () => {
        connected.value = true
      })
      es.addEventListener('snapshot', (ev: MessageEvent) => {
        if (getMode() !== 'live') return
        try {
          const data = normalizeSnapshot(JSON.parse(ev.data))
          if (data) {
            snapshot.value = data
            lastEventAt.value = Date.now()
            connected.value = true
          }
        } catch {
          /* corrupt payload — ignore this frame */
        }
      })
      es.addEventListener('error', () => {
        connected.value = false
        // EventSource auto-reconnects; we just fall back to polling if it
        // stays dead for too long (handled by watchdog).
      })
    } catch {
      startPolling()
    }
  }

  const startPolling = () => {
    if (pollTimer) return
    fetchSnapshotOnce()
    pollTimer = window.setInterval(fetchSnapshotOnce, GLOBE_REFRESH_MS)
  }

  const startSnapshotFeed = () => {
    stopPolling()
    stopSSE()
    if (getMode() === 'live') {
      startSSE()
    } else {
      startPolling()
    }
  }

  onMounted(() => {
    startSnapshotFeed()
    fetchSummary()
    summaryTimer = window.setInterval(fetchSummary, GLOBE_REFRESH_MS)
    // Watchdog: if no 5-minute frame arrives, kick polling as a safety net.
    watchdog = window.setInterval(() => {
      const now = Date.now()
      if (getMode() === 'live' && lastEventAt.value && now - lastEventAt.value > WATCHDOG_STALE_MS) {
        startPolling()
      }
    }, 30_000)
  })

  watch(
    () => getMode(),
    () => {
      startSnapshotFeed()
    },
  )

  onBeforeUnmount(() => {
    stopSSE()
    stopPolling()
    if (summaryTimer !== null) {
      clearInterval(summaryTimer)
      summaryTimer = null
    }
    if (watchdog !== null) {
      clearInterval(watchdog)
      watchdog = null
    }
  })

  return { snapshot, summary, connected, lastEventAt }
}
