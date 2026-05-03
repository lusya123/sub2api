<template>
  <div class="globe-admin">
    <!-- Top bar — operator chrome -->
    <header class="ga-head">
      <div class="ga-title">
        <router-link to="/admin/dashboard" class="ga-back" title="Back to admin">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M15 18l-6-6 6-6" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <span>ADMIN</span>
        </router-link>
        <span class="ga-title-rule"></span>
        <span class="ga-title-line">GLOBE · OPERATIONS</span>
        <span class="ga-title-sub">{{ now }} UTC</span>
      </div>

      <div class="ga-pills">
        <div class="ga-pill" :class="{ ok: connected }">
          <span class="dot"></span>
          <span>{{ connected ? 'STREAM LIVE' : 'STREAM OFFLINE' }}</span>
        </div>
        <div class="ga-pill">
          <span>Δ {{ snapshot?.interval_ms || 1500 }}ms</span>
        </div>
        <div class="ga-pill">
          <span>RESOLVED {{ summary?.geo_coverage?.resolved_ips ?? '—' }}/{{ summary?.geo_coverage?.total_distinct_ips ?? '—' }}</span>
        </div>
        <router-link to="/globe" class="ga-pill ga-pill-link" title="Open the public full-bleed view (good for wall displays)">
          <span>FULLSCREEN ↗</span>
        </router-link>
      </div>
    </header>

    <!-- 3-col layout: left stats, center globe, right leaderboards -->
    <div class="ga-grid">
      <!-- Left rail -->
      <aside class="ga-side ga-side-l">
        <div class="ga-block">
          <div class="ga-block-h">24-HOUR ROLLUP</div>
          <div class="ga-stat">
            <div class="ga-stat-n">{{ formatN(summary?.window_24h?.calls || 0) }}</div>
            <div class="ga-stat-l">CALLS</div>
          </div>
          <div class="ga-stat">
            <div class="ga-stat-n">{{ formatN(summary?.window_24h?.unique_ips || 0) }}</div>
            <div class="ga-stat-l">UNIQUE IPS</div>
          </div>
          <div class="ga-stat">
            <div class="ga-stat-n">{{ summary?.window_24h?.unique_countries || 0 }}</div>
            <div class="ga-stat-l">COUNTRIES</div>
          </div>
        </div>

        <div class="ga-block">
          <div class="ga-block-h">LIFETIME</div>
          <div class="ga-stat">
            <div class="ga-stat-n">{{ formatN(summary?.window_all_time?.calls || 0) }}</div>
            <div class="ga-stat-l">TOTAL CALLS</div>
          </div>
          <div class="ga-stat">
            <div class="ga-stat-n">{{ formatN(summary?.window_all_time?.unique_ips || 0) }}</div>
            <div class="ga-stat-l">UNIQUE IPS</div>
          </div>
          <div class="ga-stat">
            <div class="ga-stat-n">{{ summary?.window_all_time?.unique_countries || 0 }}</div>
            <div class="ga-stat-l">COUNTRIES</div>
          </div>
        </div>

        <div class="ga-block">
          <div class="ga-block-h">LIVE TICK</div>
          <div class="ga-stat accent">
            <div class="ga-stat-n">+{{ snapshot?.total_calls || 0 }}</div>
            <div class="ga-stat-l">CALLS · LAST {{ snapshot?.window_ms || 1500 }}MS</div>
          </div>
          <div class="ga-stat">
            <div class="ga-stat-n">{{ snapshot?.unique_ips || 0 }}</div>
            <div class="ga-stat-l">DISTINCT IPS</div>
          </div>
          <div class="ga-stat warn" v-if="(snapshot?.unresolved_ips || 0) > 0">
            <div class="ga-stat-n">{{ snapshot?.unresolved_ips || 0 }}</div>
            <div class="ga-stat-l">UNRESOLVED</div>
          </div>
        </div>

        <div class="ga-block ga-spark">
          <div class="ga-block-h">CALLS · LAST 24H (HOURLY)</div>
          <svg class="ga-sparkline" :viewBox="`0 0 ${sparkW} ${sparkH}`" preserveAspectRatio="none">
            <path :d="sparkPath" fill="rgba(95,199,255,0.15)" stroke="#5fc7ff" stroke-width="1" />
          </svg>
        </div>
      </aside>

      <!-- Center globe -->
      <main class="ga-globe-wrap">
        <GlobeStage :snapshot="snapshot" detail="admin" :interactive="true" toggle-position="top-right" :sync-to-url="true" />

        <!-- Hover crosshair / corner ticks -->
        <div class="ga-crosshair">
          <span class="tl"></span><span class="tr"></span>
          <span class="bl"></span><span class="br"></span>
        </div>
      </main>

      <!-- Right rail -->
      <aside class="ga-side ga-side-r">
        <div class="ga-block">
          <div class="ga-block-h">TOP COUNTRIES · 24H</div>
          <ul class="ga-leader">
            <li v-for="(c, i) in summary?.top_countries || []" :key="c.cc">
              <span class="rk">{{ String(i + 1).padStart(2, '0') }}</span>
              <span class="cc">{{ flag(c.cc) }} {{ c.cc }}</span>
              <span class="nm">{{ c.country }}</span>
              <span class="bar"><span :style="{ width: pct(c.calls, summary?.top_countries?.[0]?.calls || 1) + '%' }"></span></span>
              <span class="n">{{ formatN(c.calls) }}</span>
            </li>
          </ul>
        </div>

        <div class="ga-block">
          <div class="ga-block-h">LIVE ORIGINS · LAST TICK</div>
          <ul class="ga-leader sm">
            <li v-for="a in (snapshot?.arcs || []).slice(0, 12)" :key="`${a.cc}:${a.region || ''}:${a.city || a.country}:${a.lat}:${a.lng}`">
              <span class="cc">{{ flag(a.cc) }} {{ a.cc || '—' }}</span>
              <span class="nm">{{ a.city || a.country || a.ip_mask }}</span>
              <span class="ip">{{ a.ip_mask }}</span>
              <span class="n">{{ a.calls }}</span>
            </li>
            <li v-if="!snapshot?.arcs?.length" class="empty">
              waiting for traffic…
            </li>
          </ul>
        </div>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import GlobeStage from '@/components/globe/GlobeStage.vue'
import { useGlobeStream } from '@/composables/useGlobeStream'

const { snapshot, summary, connected } = useGlobeStream()

const now = ref('')
let nowTimer: number | null = null
function updateNow() {
  const d = new Date()
  const pad = (n: number) => n.toString().padStart(2, '0')
  now.value = `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`
}
onMounted(() => { updateNow(); nowTimer = window.setInterval(updateNow, 1000) })
onBeforeUnmount(() => { if (nowTimer !== null) clearInterval(nowTimer) })

// Sparkline path
const sparkW = 240
const sparkH = 48
const sparkPath = computed(() => {
  const data = summary.value?.hourly_history_24h || []
  if (data.length === 0) return ''
  const max = Math.max(1, ...data.map((d) => d.calls))
  const stepX = sparkW / Math.max(1, data.length - 1)
  const points = data.map((d, i) => {
    const x = i * stepX
    const y = sparkH - (d.calls / max) * (sparkH - 4) - 2
    return `${x},${y}`
  })
  return `M0,${sparkH} L${points.join(' L')} L${sparkW},${sparkH} Z`
})

function formatN(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return n.toLocaleString()
}
function pct(v: number, max: number) { return Math.min(100, Math.round((v / Math.max(1, max)) * 100)) }
function flag(cc: string) {
  if (!cc || cc.length !== 2) return ''
  const codePoints = cc.toUpperCase().split('').map((c) => 0x1f1e6 - 65 + c.charCodeAt(0))
  return String.fromCodePoint(...codePoints)
}
</script>

<style scoped>
.globe-admin {
  position: fixed;
  inset: 0;
  background: #02060c;
  color: #cfe1f5;
  font-family: 'JetBrains Mono', 'IBM Plex Mono', ui-monospace, monospace;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.ga-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 24px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
  flex-shrink: 0;
}
.ga-title { display: flex; align-items: center; gap: 14px; }
.ga-title-rule { width: 1px; height: 20px; background: rgba(255,255,255,0.12); }
.ga-back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border: 1px solid rgba(255,255,255,0.1);
  color: rgba(184, 200, 220, 0.7);
  font-size: 10px;
  letter-spacing: 0.22em;
  text-decoration: none;
  transition: all 0.15s ease;
}
.ga-back:hover {
  color: #fff;
  border-color: rgba(95,199,255,0.4);
}
.ga-back svg { width: 12px; height: 12px; }
.ga-title-line {
  font-size: 11px;
  letter-spacing: 0.32em;
  color: #fff;
  font-weight: 600;
}
.ga-title-sub {
  font-size: 10px;
  color: rgba(184, 200, 220, 0.5);
  letter-spacing: 0.18em;
}
.ga-pill-link {
  text-decoration: none;
  cursor: pointer;
  color: rgba(255,216,122,0.85);
  border-color: rgba(255,216,122,0.3);
}
.ga-pill-link:hover { color: #ffd87a; border-color: rgba(255,216,122,0.6); }
.ga-pills { display: flex; gap: 10px; }
.ga-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border: 1px solid rgba(255,255,255,0.1);
  font-size: 10px;
  letter-spacing: 0.22em;
  color: rgba(184, 200, 220, 0.7);
}
.ga-pill .dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: #ff8b6f;
}
.ga-pill.ok { border-color: rgba(95,199,255,0.4); color: #9be7ff; }
.ga-pill.ok .dot {
  background: #5fc7ff;
  box-shadow: 0 0 6px #5fc7ff;
  animation: ga-pulse 1.4s ease-in-out infinite;
}
@keyframes ga-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }

.ga-grid {
  flex: 1;
  display: grid;
  grid-template-columns: 280px 1fr 360px;
  min-height: 0;
}
.ga-side {
  border-right: 1px solid rgba(255,255,255,0.06);
  padding: 18px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}
.ga-side-r { border-right: none; border-left: 1px solid rgba(255,255,255,0.06); }
.ga-block-h {
  font-size: 9.5px;
  letter-spacing: 0.32em;
  color: rgba(184, 200, 220, 0.45);
  border-bottom: 1px solid rgba(255,255,255,0.06);
  padding-bottom: 8px;
  margin-bottom: 14px;
}
.ga-stat { margin-bottom: 14px; }
.ga-stat-n {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: 32px;
  font-weight: 500;
  color: #fff;
  letter-spacing: -0.01em;
  line-height: 1;
  font-feature-settings: 'tnum' 1;
}
.ga-stat.accent .ga-stat-n { color: #ffd87a; }
.ga-stat.warn .ga-stat-n { color: #ff8b6f; }
.ga-stat-l {
  font-size: 9px;
  letter-spacing: 0.26em;
  color: rgba(184, 200, 220, 0.55);
  margin-top: 4px;
}

.ga-spark { padding-top: 4px; }
.ga-sparkline { width: 100%; height: 48px; }

.ga-globe-wrap {
  position: relative;
  min-height: 0;
}
.ga-crosshair {
  position: absolute;
  inset: 24px;
  pointer-events: none;
}
.ga-crosshair span {
  position: absolute;
  width: 14px;
  height: 14px;
  border-color: rgba(95, 199, 255, 0.4);
  border-style: solid;
}
.ga-crosshair .tl { top: 0; left: 0; border-width: 1px 0 0 1px; }
.ga-crosshair .tr { top: 0; right: 0; border-width: 1px 1px 0 0; }
.ga-crosshair .bl { bottom: 0; left: 0; border-width: 0 0 1px 1px; }
.ga-crosshair .br { bottom: 0; right: 0; border-width: 0 1px 1px 0; }

.ga-leader { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 8px; font-size: 11px; }
.ga-leader li {
  display: grid;
  grid-template-columns: 22px 50px 1fr 64px 50px;
  gap: 8px;
  align-items: center;
}
.ga-leader.sm li {
  grid-template-columns: 50px 1fr 110px 32px;
}
.ga-leader .rk { color: rgba(184,200,220,0.35); }
.ga-leader .cc { color: #fff; }
.ga-leader .nm { color: rgba(184,200,220,0.7); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-family: 'Inter', sans-serif; font-size: 10.5px; }
.ga-leader .ip { color: rgba(184,200,220,0.4); font-size: 10px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ga-leader .bar { position: relative; height: 1px; background: rgba(255,255,255,0.08); overflow: hidden; }
.ga-leader .bar > span { position: absolute; inset: 0; background: linear-gradient(to right, transparent, #5fc7ff); transition: width 0.6s ease; }
.ga-leader .n { text-align: right; color: #ffd87a; font-feature-settings: 'tnum' 1; }
.ga-leader li.empty { color: rgba(184,200,220,0.4); font-style: italic; padding: 12px 0; }

@media (max-width: 1280px) {
  .ga-grid { grid-template-columns: 240px 1fr 320px; }
}
@media (max-width: 980px) {
  .ga-grid { grid-template-columns: 1fr; grid-template-rows: auto 60vh auto; }
  .ga-side { border-left: none !important; border-right: none; border-bottom: 1px solid rgba(255,255,255,0.06); flex-direction: row; overflow-x: auto; }
}
</style>
