<template>
  <!-- Full-bleed art piece. Hero overlay sits on top of the globe canvas. -->
  <div class="showcase">
    <GlobeStage :snapshot="snapshot" detail="public" :interactive="true" toggle-position="top-center" :sync-to-url="true" class="showcase-globe" />

    <!-- Top-left: project mark -->
    <header class="showcase-top">
      <div class="brand-mark">
        <span class="brand-glyph">●</span>
        <span class="brand-name">sub2api</span>
        <span class="brand-rule"></span>
        <span class="brand-sub">live globe</span>
      </div>
      <div class="status-pill" :class="{ 'is-live': connected }">
        <span class="status-dot"></span>
        <span>{{ connected ? 'LIVE' : 'CONNECTING' }}</span>
      </div>
    </header>

    <!-- Centre-left: editorial headline -->
    <section class="showcase-headline">
      <div class="chapter-mark">
        <span class="chapter-n">№ {{ generationLabel }}</span>
        <span class="chapter-rule-h"></span>
      </div>
      <h1 class="showcase-h1">
        <span class="word">此</span><span class="word">时</span><span class="word">此</span><span class="word">刻</span><span class="comma">，</span>
        <br />
        <span class="ital">全 世 界</span> 在用我们的 token<span class="punc">.</span>
      </h1>
      <p class="showcase-sub">
        每一道光弧 ＝ 一次真实的 API 调用<span class="dot-sep">·</span>
        实时数据<span class="dot-sep">·</span>
        无需登录
      </p>
    </section>

    <!-- Bottom-left: rolling counters -->
    <section class="showcase-counters">
      <div class="counter">
        <div class="counter-num">{{ formatN(summary?.window_24h?.calls || 0) }}</div>
        <div class="counter-lbl">CALLS · 24H</div>
      </div>
      <div class="counter-sep"></div>
      <div class="counter">
        <div class="counter-num">{{ formatN(summary?.window_24h?.unique_ips || 0) }}</div>
        <div class="counter-lbl">UNIQUE IPS · 24H</div>
      </div>
      <div class="counter-sep"></div>
      <div class="counter">
        <div class="counter-num">{{ summary?.window_24h?.unique_countries || 0 }}</div>
        <div class="counter-lbl">COUNTRIES · 24H</div>
      </div>
      <div class="counter-sep"></div>
      <div class="counter is-live-counter">
        <div class="counter-num accent">+{{ liveCalls }}</div>
        <div class="counter-lbl">CALLS · LAST TICK</div>
      </div>
    </section>

    <!-- Bottom-right: top countries leaderboard -->
    <aside class="showcase-leader">
      <div class="leader-h">TOP REGIONS · 24H</div>
      <ul class="leader-list">
        <li v-for="(c, i) in (summary?.top_countries || []).slice(0, 6)" :key="c.cc" class="leader-row">
          <span class="leader-rank">{{ String(i + 1).padStart(2, '0') }}</span>
          <span class="leader-cc">{{ flag(c.cc) }} {{ c.cc }}</span>
          <span class="leader-name">{{ c.country }}</span>
          <span class="leader-bar"><span :style="{ width: pct(c.calls, summary?.top_countries?.[0]?.calls || 1) + '%' }"></span></span>
          <span class="leader-n">{{ formatN(c.calls) }}</span>
        </li>
      </ul>
    </aside>

    <!-- Bottom credit -->
    <footer class="showcase-foot">
      <span>{{ now }} · UTC</span>
      <span class="dot-sep">·</span>
      <span>data updates every 5 min</span>
      <span class="dot-sep">·</span>
      <span>geo: ip-api</span>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, ref } from 'vue'
import GlobeStage from '@/components/globe/GlobeStage.vue'
import { useGlobeStream } from '@/composables/useGlobeStream'

const { snapshot, summary, connected } = useGlobeStream()

const liveCalls = computed(() => snapshot.value?.total_calls || 0)
const generationLabel = computed(() => {
  const t = snapshot.value?.generated_at
  if (!t) return '00'
  return new Date(t).getUTCSeconds().toString().padStart(2, '0')
})

const now = ref('')
let nowTimer: number | null = null
function updateNow() {
  const d = new Date()
  const pad = (n: number) => n.toString().padStart(2, '0')
  now.value = `${d.getUTCFullYear()}.${pad(d.getUTCMonth() + 1)}.${pad(d.getUTCDate())} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`
}
onMounted(() => {
  updateNow()
  nowTimer = window.setInterval(updateNow, 1000)
})
onBeforeUnmount(() => {
  if (nowTimer !== null) clearInterval(nowTimer)
})

function formatN(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return n.toLocaleString()
}
function pct(v: number, max: number): number {
  if (!max) return 0
  return Math.min(100, Math.round((v / max) * 100))
}
function flag(cc: string): string {
  if (!cc || cc.length !== 2) return ''
  const codePoints = cc.toUpperCase().split('').map((c) => 0x1f1e6 - 65 + c.charCodeAt(0))
  return String.fromCodePoint(...codePoints)
}
</script>

<style scoped>
/* ──────────────────────────────────────────────────────────────────────────
   Showcase layout — full-bleed art piece, dark, editorial.
   The aesthetic philosophy:
   • Two-color discipline: cyan for "now / signal", amber-gold for "destination".
   • Deep negative space — 60%+ of the screen is empty so the globe is the hero.
   • Editorial typography: serif for the headline numerals, mono for chrome.
   • All chrome floats — nothing has a background tile, everything reads on
     the starfield. Border-bottoms and rules carry hierarchy.
   ────────────────────────────────────────────────────────────────────────── */
.showcase {
  position: fixed;
  inset: 0;
  background: #02060c;
  color: #e6eef8;
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Helvetica Neue', sans-serif;
  overflow: hidden;
}
.showcase-globe {
  position: absolute !important;
  inset: 0;
}

/* ── top header ──────────────────────────────────────────────────────── */
.showcase-top {
  position: absolute;
  top: 32px;
  left: 0;
  right: 0;
  padding: 0 48px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  z-index: 5;
  font-family: 'JetBrains Mono', 'IBM Plex Mono', ui-monospace, monospace;
}
.brand-mark {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: #b8c8dc;
}
.brand-glyph {
  color: #5fc7ff;
  font-size: 8px;
  filter: drop-shadow(0 0 6px #5fc7ff);
  animation: brand-pulse 1.6s ease-in-out infinite;
}
.brand-name { color: #fff; font-weight: 600; letter-spacing: 0.22em; }
.brand-rule { width: 28px; height: 1px; background: rgba(255,255,255,0.25); }
.brand-sub { color: #5fc7ff; }

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border: 1px solid rgba(255,255,255,0.15);
  border-radius: 999px;
  font-size: 11px;
  letter-spacing: 0.22em;
  color: #b8c8dc;
}
.status-pill.is-live { border-color: rgba(95,199,255,0.55); color: #9be7ff; }
.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #ff8b6f;
  box-shadow: 0 0 8px currentColor;
}
.status-pill.is-live .status-dot {
  background: #5fc7ff;
  animation: dot-pulse 1.4s ease-in-out infinite;
}
@keyframes dot-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
@keyframes brand-pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }

/* ── headline ────────────────────────────────────────────────────────── */
.showcase-headline {
  position: absolute;
  left: 56px;
  top: 18%;
  max-width: 580px;
  z-index: 5;
}
.chapter-mark {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 32px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  letter-spacing: 0.32em;
  color: rgba(155, 231, 255, 0.7);
  text-transform: uppercase;
}
.chapter-rule-h { flex: 1; height: 1px; background: linear-gradient(to right, rgba(155,231,255,0.6), transparent); max-width: 240px; }
.showcase-h1 {
  font-family: 'Cormorant Garamond', 'Source Han Serif SC', 'Songti SC', Georgia, serif;
  font-size: clamp(40px, 5.4vw, 88px);
  line-height: 1.04;
  letter-spacing: -0.02em;
  font-weight: 400;
  margin: 0 0 28px;
  color: #fff;
}
.showcase-h1 .word {
  display: inline-block;
  animation: word-in 0.9s cubic-bezier(0.2, 0.7, 0.2, 1) backwards;
}
.showcase-h1 .word:nth-child(2) { animation-delay: 0.05s; }
.showcase-h1 .word:nth-child(3) { animation-delay: 0.10s; }
.showcase-h1 .word:nth-child(4) { animation-delay: 0.15s; }
.showcase-h1 .comma { color: rgba(255,255,255,0.4); }
.showcase-h1 .ital {
  font-style: italic;
  color: #ffd87a;
  letter-spacing: 0.04em;
  font-weight: 300;
}
.showcase-h1 .punc { color: #ffd87a; }
@keyframes word-in {
  from { opacity: 0; transform: translateY(14px); }
  to { opacity: 1; transform: translateY(0); }
}
.showcase-sub {
  font-size: 14px;
  color: #88a0bd;
  letter-spacing: 0.04em;
  line-height: 1.7;
}
.showcase-sub .dot-sep { margin: 0 10px; color: rgba(255,255,255,0.3); }

/* ── counters ────────────────────────────────────────────────────────── */
.showcase-counters {
  position: absolute;
  left: 56px;
  bottom: 56px;
  display: flex;
  align-items: stretch;
  gap: 32px;
  z-index: 5;
}
.counter { display: flex; flex-direction: column; gap: 4px; }
.counter-num {
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: 44px;
  font-weight: 500;
  font-feature-settings: 'tnum' 1;
  color: #fff;
  letter-spacing: -0.02em;
  line-height: 1;
}
.counter-num.accent { color: #ffd87a; }
.counter-lbl {
  font-family: 'JetBrains Mono', monospace;
  font-size: 9.5px;
  letter-spacing: 0.32em;
  color: rgba(184, 200, 220, 0.6);
}
.counter-sep {
  width: 1px;
  background: linear-gradient(to bottom, transparent, rgba(255,255,255,0.18), transparent);
}
.is-live-counter .counter-num {
  animation: counter-flash 1.5s ease-out infinite;
}
@keyframes counter-flash {
  0%, 60%, 100% { text-shadow: 0 0 0 transparent; }
  10% { text-shadow: 0 0 22px rgba(255, 216, 122, 0.55); }
}

/* ── leaderboard ─────────────────────────────────────────────────────── */
.showcase-leader {
  position: absolute;
  right: 48px;
  bottom: 56px;
  width: 360px;
  z-index: 5;
}
.leader-h {
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  letter-spacing: 0.34em;
  color: rgba(184, 200, 220, 0.55);
  margin-bottom: 16px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
  padding-bottom: 10px;
}
.leader-list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 10px; }
.leader-row {
  display: grid;
  grid-template-columns: 22px 56px 1fr 80px 56px;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  font-family: 'JetBrains Mono', monospace;
}
.leader-rank { color: rgba(184, 200, 220, 0.4); }
.leader-cc { color: #fff; }
.leader-name {
  color: rgba(184, 200, 220, 0.7);
  font-family: 'Inter', sans-serif;
  font-size: 11px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.leader-bar {
  position: relative;
  height: 1px;
  background: rgba(255,255,255,0.08);
  overflow: hidden;
}
.leader-bar > span {
  position: absolute;
  inset: 0;
  background: linear-gradient(to right, transparent, #5fc7ff);
  transition: width 0.6s ease;
}
.leader-n {
  text-align: right;
  color: #ffd87a;
  font-feature-settings: 'tnum' 1;
}

/* ── footer ──────────────────────────────────────────────────────────── */
.showcase-foot {
  position: absolute;
  left: 0; right: 0; bottom: 18px;
  text-align: center;
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  letter-spacing: 0.22em;
  color: rgba(184, 200, 220, 0.35);
  z-index: 5;
}
.showcase-foot .dot-sep { margin: 0 12px; color: rgba(255,255,255,0.2); }

/* ── responsive ──────────────────────────────────────────────────────── */
@media (max-width: 900px) {
  .showcase-top { padding: 0 24px; }
  .showcase-headline { left: 28px; right: 28px; top: 14%; }
  .showcase-counters {
    left: 28px;
    right: 28px;
    bottom: 200px;
    flex-wrap: wrap;
    gap: 18px 24px;
  }
  .counter-num { font-size: 32px; }
  .counter-sep { display: none; }
  .showcase-leader { right: 28px; left: 28px; bottom: 64px; width: auto; }
  .leader-row { grid-template-columns: 22px 50px 1fr 60px 56px; }
  .leader-name { display: none; }
}
</style>
