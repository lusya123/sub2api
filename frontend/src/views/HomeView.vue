<template>
  <!-- Custom Home Content: Full Page Mode (admin override) -->
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home: "Token — 我们的硬通货" landing -->
  <div v-else class="token-landing" :class="{ 'is-dark': isDark }">
    <!-- Film grain overlay -->
    <div class="grain" aria-hidden="true"></div>

    <!-- Scroll progress rail (right edge) -->
    <div class="scroll-rail" aria-hidden="true">
      <div class="scroll-rail-fill" :style="{ height: scrollProgress + '%' }"></div>
      <div class="scroll-rail-ticks">
        <span v-for="n in 5" :key="n"></span>
      </div>
    </div>

    <!-- Top Nav -->
    <header class="nav">
      <div class="nav-inner">
        <div class="brand">
          <div class="brand-logo">
            <img :src="siteLogo || '/logo.png'" alt="" />
          </div>
          <span class="brand-name">{{ siteName }}</span>
        </div>

        <div class="nav-actions">
          <div class="nav-locale"><LocaleSwitcher /></div>
          <button class="icon-btn" :title="isDark ? '切换浅色' : '切换深色'" @click="toggleTheme">
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link v-if="isAuthenticated" :to="dashboardPath" class="nav-cta-hero">
            <span class="nav-cta-avatar">{{ userInitial }}</span>
            <span>进入控制台</span>
          </router-link>
          <template v-else>
            <router-link to="/login" class="nav-ghost">登录</router-link>
            <router-link to="/login" class="nav-cta-hero">立即注册</router-link>
          </template>
        </div>
      </div>
    </header>

    <!-- HERO — editorial, asymmetric -->
    <section class="hero">
      <div class="hero-inner">
        <div class="hero-left">
          <h1 class="hero-title">
            <span class="line" data-reveal>Token，</span>
            <span class="line" data-reveal data-delay="100">是 AI 时代</span>
            <span class="line gold-ital" data-reveal data-delay="200">的硬通货。</span>
          </h1>

          <p class="hero-sub" data-reveal data-delay="350">
            花的不是钱，是在买 AI 替你思考的<em>每一秒</em>。<br />
            早一天开始，早一天把自己从琐事里赎回来。
          </p>

          <div class="hero-ctas" data-reveal data-delay="450">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="cta-primary"
            >
              <span>{{ isAuthenticated ? '进入控制台' : '让 AI 替你上班' }}</span>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M5 12h14M13 5l7 7-7 7" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </router-link>
          </div>

          <div class="hero-foot" data-reveal data-delay="600">
            <div class="stat">
              <div class="stat-num">¥1</div>
              <div class="stat-lbl">少花一点 多跑一轮</div>
            </div>
            <div class="sep-v"></div>
            <div class="stat">
              <div class="stat-num">0 秒</div>
              <div class="stat-lbl">下单立刻开跑</div>
            </div>
            <div class="sep-v"></div>
            <div class="stat">
              <div class="stat-num">∞</div>
              <div class="stat-lbl">买了总会用上</div>
            </div>
          </div>
        </div>

        <div class="hero-right">
          <div class="token-stage">
            <div class="orbit orbit-1"></div>
            <div class="orbit orbit-2"></div>
            <div class="orbit orbit-3"></div>

            <div class="token-coin">
              <div class="token-face">
                <svg viewBox="0 0 220 220" class="hex-svg">
                  <defs>
                    <linearGradient id="goldGrad" x1="0" y1="0" x2="1" y2="1">
                      <stop offset="0%"  stop-color="#FFF1C2"/>
                      <stop offset="42%" stop-color="#F2B418"/>
                      <stop offset="78%" stop-color="#A66D06"/>
                      <stop offset="100%" stop-color="#4A2E04"/>
                    </linearGradient>
                    <linearGradient id="goldRim" x1="0" y1="0" x2="1" y2="1">
                      <stop offset="0%"  stop-color="#FFF4CE"/>
                      <stop offset="55%" stop-color="#E8A317"/>
                      <stop offset="100%" stop-color="#6B3F04"/>
                    </linearGradient>
                    <linearGradient id="tokenInk" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stop-color="#2a1802"/>
                      <stop offset="58%" stop-color="#3a2608"/>
                      <stop offset="100%" stop-color="#1b1002"/>
                    </linearGradient>
                  </defs>
                  <polygon
                    points="110,8 200,58 200,162 110,212 20,162 20,58"
                    fill="url(#goldGrad)"
                    stroke="url(#goldRim)"
                    stroke-width="3"
                  />
                  <polygon
                    points="110,24 185,66 185,154 110,196 35,154 35,66"
                    fill="none"
                    stroke="rgba(255,241,194,0.45)"
                    stroke-width="1"
                  />
                  <polygon
                    points="110,32 178,70 178,150 110,188 42,150 42,70"
                    fill="none"
                    stroke="rgba(74,46,4,0.45)"
                    stroke-width="0.5"
                    stroke-dasharray="2 2"
                  />
                  <text x="110" y="140" text-anchor="middle" dominant-baseline="middle" class="token-glyph">T</text>
                </svg>
              </div>
              <div class="token-shadow"></div>
            </div>

            <span class="spark s1">¥</span>
            <span class="spark s2">✦</span>
            <span class="spark s3">+1s</span>
            <span class="spark s4">✦</span>
            <span class="spark s5">¥</span>
          </div>
        </div>
      </div>

      <div class="scroll-hint">— 向下滚，看看你错过了什么 —</div>
    </section>

    <!-- Marquee -->
    <div class="marquee" aria-hidden="true">
      <div class="marquee-track">
        <span v-for="n in 2" :key="n" class="marquee-group">
          <span>TOKEN 不睡觉</span><span class="dot">✦</span>
          <span>别人的 AI 已经在上班了</span><span class="dot">✦</span>
          <span>AI 替你干活，你负责躺平</span><span class="dot">✦</span>
          <span>一分钱，一秒钟智商</span><span class="dot">✦</span>
          <span>早一天用上，早一天不加班</span><span class="dot">✦</span>
          <span>TOKEN IS THE NEW OIL</span><span class="dot">✦</span>
        </span>
      </div>
    </div>

    <!-- LIVE COUNTER -->
    <section class="live">
      <div class="chapter-mark center" data-reveal>
        <span class="chapter-n">№ 01</span>
        <span class="chapter-rule"></span>
        <span class="chapter-title">REAL-TIME · 此时此刻</span>
      </div>

      <div class="live-inner">
        <div class="live-intro" data-reveal>此时此刻</div>
        <div class="live-number" data-reveal>{{ formatNumber(liveTokens) }}</div>
        <div class="live-unit" data-reveal>枚 Token 正在为人类打工。</div>
        <div class="live-sub" data-reveal>
          平均每秒 <strong>+{{ formatNumber(tickRate) }}</strong>。<br />
          它们不休息，不请假，不涨工资。
        </div>
      </div>
    </section>

    <!-- GLOBE — live world map of API calls -->
    <section class="globe-section" ref="globeSectionEl">
      <div class="chapter-mark center" data-reveal>
        <span class="chapter-n">№ 02</span>
        <span class="chapter-rule"></span>
        <span class="chapter-title">GLOBAL · 全 球 在 用</span>
      </div>

      <div class="globe-section-headline" data-reveal>
        <h2 class="globe-h2">
          每一道光弧，<br />
          都是世界某处的 <span class="gold-ital">一次真实调用</span>。
        </h2>
        <p class="globe-sub">
          数据每 1.5 秒刷新一次<span class="dot-sep">·</span>
          来自 {{ liveCountries }} 个国家 / 地区<span class="dot-sep">·</span>
          匿名展示
        </p>
      </div>

      <div class="globe-stage" data-reveal>
        <!--
          Lazy-mounted: the ~620KB three.js + world-atlas chunk only loads
          when this section is about to scroll into view. Above-the-fold
          experience stays light.
        -->
        <GlobeStage v-if="globeInView" :snapshot="liveGlobeSnapshot" detail="public" toggle-position="top-right" class="globe-canvas-wrap" />
        <div v-else class="globe-placeholder">
          <div class="globe-placeholder-orb"></div>
        </div>

        <div class="globe-overlay-meta">
          <div class="meta-row">
            <span class="meta-k">CALLS · 24H</span>
            <span class="meta-v">{{ formatNumber(globe24hCalls) }}</span>
          </div>
          <div class="meta-row">
            <span class="meta-k">UNIQUE IPS</span>
            <span class="meta-v">{{ formatNumber(globe24hIPs) }}</span>
          </div>
          <div class="meta-row">
            <span class="meta-k">COUNTRIES</span>
            <span class="meta-v">{{ liveCountries }}</span>
          </div>
        </div>

        <router-link to="/globe" class="globe-cta-link">
          <span>看 全 屏 版 本</span>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M7 17L17 7M17 7H9M17 7V15" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </router-link>
      </div>
    </section>

    <!-- TRUST -->
    <section class="trust">
      <div class="trust-inner" data-reveal>
        <div class="trust-label">不是什么 AI 都能上架</div>
        <h3 class="trust-headline">
          目前，我们只卖 <span class="gold-ital">Claude</span>。
        </h3>
        <p class="trust-reason">
          因为它现在，就是最能替你干活的那一个。
        </p>
        <div class="trust-row">
          <span class="tag">Claude · 在架</span>
          <span class="tag tag-soon">下一位 · 值得了再上</span>
        </div>
      </div>
    </section>

    <!-- FINAL CTA -->
    <section class="final">
      <div class="chapter-mark center" data-reveal>
        <span class="chapter-n">№ 02</span>
        <span class="chapter-rule"></span>
        <span class="chapter-title">LAST CALL · 最后一问</span>
      </div>

      <div class="final-inner">
        <h2 class="final-title" data-reveal>
          此刻，<br />
          最聪明的人，<br />
          都让 <span class="gold-ital">AI</span> 替自己思考。<br />
          <span class="soft">—— 而你，还在亲自熬夜。</span>
        </h2>

        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="final-cta"
          data-reveal
          data-delay="200"
        >
          <span>{{ isAuthenticated ? '进入控制台 · 继续干' : '现在开始 · 永远不晚' }}</span>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M5 12h14M13 5l7 7-7 7" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </router-link>

        <div class="final-foot" data-reveal data-delay="300">
          30 秒开通，用多少扣多少。
        </div>
      </div>
    </section>

    <!-- Footer -->
    <footer class="foot">
      <div class="foot-inner">
        <span>&copy; {{ currentYear }} {{ siteName }} · Printed with care.</span>
        <div class="foot-links">
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">文档</a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
          <router-link to="/login">登录</router-link>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import GlobeStage from '@/components/globe/GlobeStage.vue'
import { useGlobeStream } from '@/composables/useGlobeStream'

// Live globe data feed for the homepage section. Same SSE stream the
// /globe page uses — data is shared, the visualisation is shared.
const { snapshot: liveGlobeSnapshot, summary: liveGlobeSummary } = useGlobeStream()
const liveCountries = computed(() => liveGlobeSummary.value?.window_24h?.unique_countries ?? 0)
const globe24hCalls = computed(() => liveGlobeSummary.value?.window_24h?.calls ?? 0)
const globe24hIPs = computed(() => liveGlobeSummary.value?.window_24h?.unique_ips ?? 0)

// Lazy-mount the globe when its section enters the viewport (with a 200px
// rootMargin so the canvas has time to render before user scrolls to it).
const globeSectionEl = ref<HTMLElement | null>(null)
const globeInView = ref(false)
let globeObserver: IntersectionObserver | null = null

const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(
  () => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API'
)
const siteLogo = computed(
  () => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || ''
)
const docUrl = computed(
  () => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''
)
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

const isHomeContentUrl = computed(() => {
  const c = homeContent.value.trim()
  return c.startsWith('http://') || c.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const userInitial = computed(() => {
  const u = authStore.user
  if (!u || !u.email) return '我'
  return u.email.charAt(0).toUpperCase()
})
const currentYear = computed(() => new Date().getFullYear())

// Live token counter — display max(real-from-backend, floor).
// Floor is a dignity guarantee; real data (when wired) takes over once it exceeds it.
const LIVE_TOKEN_FLOOR = 132_400_000_000
const tickRate = ref(41237)
const liveTokens = ref(LIVE_TOKEN_FLOOR + Math.floor(Math.random() * 2_000_000))
const scrollProgress = ref(0)

let tickTimer: number | null = null
let revealObserver: IntersectionObserver | null = null

function formatNumber(n: number) {
  return n.toLocaleString('en-US')
}

function startTicker() {
  tickTimer = window.setInterval(() => {
    const jitter = Math.floor(Math.random() * 8_000) - 2_000
    const add = Math.max(10_000, Math.floor(tickRate.value / 10) + jitter)
    liveTokens.value += add
  }, 800)
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const saved = localStorage.getItem('theme')
  if (saved === 'dark' || (!saved && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  } else {
    isDark.value = false
    document.documentElement.classList.remove('dark')
  }
}

function onScroll() {
  const doc = document.documentElement
  const max = doc.scrollHeight - doc.clientHeight
  scrollProgress.value = max > 0 ? Math.min(100, (doc.scrollTop / max) * 100) : 0
}

function setupReveal() {
  const els = document.querySelectorAll<HTMLElement>('[data-reveal]')
  revealObserver = new IntersectionObserver(
    (entries) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          const el = e.target as HTMLElement
          const delay = Number(el.dataset.delay || 0)
          window.setTimeout(() => el.classList.add('in-view'), delay)
          revealObserver?.unobserve(el)
        }
      })
    },
    { threshold: 0.15, rootMargin: '0px 0px -8% 0px' }
  )
  els.forEach((el) => revealObserver?.observe(el))
}

onMounted(async () => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  startTicker()
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
  await nextTick()
  setupReveal()

  // Lazy-mount the globe when its section comes within 200px of the viewport.
  if (globeSectionEl.value) {
    globeObserver = new IntersectionObserver(
      (entries) => {
        if (entries.some((e) => e.isIntersecting)) {
          globeInView.value = true
          globeObserver?.disconnect()
          globeObserver = null
        }
      },
      { rootMargin: '200px 0px 200px 0px' },
    )
    globeObserver.observe(globeSectionEl.value)
  }
})

onBeforeUnmount(() => {
  if (tickTimer) window.clearInterval(tickTimer)
  window.removeEventListener('scroll', onScroll)
  revealObserver?.disconnect()
  globeObserver?.disconnect()
})
</script>

<style scoped>
/* ========================================================================
   THEME TOKENS — default is LIGHT (paper).
   Dark mode is triggered by <html class="dark"> and inverts the palette.
   ======================================================================== */
.token-landing {
  /* ---- LIGHT (default) — "Maison d'Or" ---- */
  /* Handmade ivory paper + warm espresso ink + saffron-gold two-tier accent
     with a rare flame-orange urgency signal. Hermès cream meets Binance gold,
     grounded in museum-quality neutrals. */
  --bg: #fdfcf8;              /* clean warm-white, gallery canvas */
  --bg-2: #f5ecd4;            /* cream sash — only for feature sections */
  --bg-invert: #0a0704;
  --fg: #1a0f06;              /* warm espresso, reads brown-black */
  --fg-2: rgba(26, 15, 6, 0.74);
  --fg-3: rgba(26, 15, 6, 0.5);
  --fg-4: rgba(26, 15, 6, 0.26);
  --rule: rgba(26, 15, 6, 0.12);
  --rule-strong: rgba(26, 15, 6, 0.32);
  /* Tier 1 — SAFFRON GOLD: the page's currency, used generously but always
     against generous white space. The "hard money" tone. */
  --gold: #d99411;            /* saffron — the page's spine of color */
  --gold-bright: #f2b418;     /* morning sun — hover peaks, highlights */
  --gold-deep: #6b3f04;       /* aged bronze — gradient depth, shadow ink */
  --gold-soft: rgba(217, 148, 17, 0.1);
  --gold-glow: rgba(242, 180, 24, 0.38);
  /* Tier 2 — FLAME ORANGE: rare, only for "alive/hot" signals
     (pulse dots, scroll rail, sparks). Hermès heat against Binance gold. */
  --flame: #e85d1f;           /* Hermès-leaning ember */
  --flame-bright: #f97b38;    /* lit flame */
  --flame-deep: #a73a0e;      /* scorched copper */
  --flame-glow: rgba(232, 93, 31, 0.3);
  --grain-opacity: 0.055;
  --grain-mode: multiply;
  --cta-ink: #1a0f06;
  /* CTA = polished gold bar: sunlit peak → aged bronze bottom. */
  --cta-gold-a: #ffd96b;
  --cta-gold-b: #ce8906;

  position: relative;
  min-height: 100vh;
  background: var(--bg);
  color: var(--fg);
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'PingFang SC',
    'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  overflow-x: hidden;
}

/* Dark mode — applied when the landing itself has `.is-dark`,
   mirrored from the global <html class="dark"> toggle. Scoped class
   is more robust than :global() in scoped SFC CSS. */
.token-landing.is-dark {
  /* ---- DARK — "Noir d'Or" ---- */
  /* Warm india-ink base + parchment gold ink + molten-saffron gleam
     with a flame signal that burns brighter against black. */
  --bg: #080604;              /* warm noir, almost black */
  --bg-2: #12100a;             /* deep umber shadow */
  --bg-invert: #f4e9d1;
  --fg: #f3e4c1;               /* parchment, golden-cast cream */
  --fg-2: rgba(243, 228, 193, 0.74);
  --fg-3: rgba(243, 228, 193, 0.5);
  --fg-4: rgba(243, 228, 193, 0.28);
  --rule: rgba(243, 228, 193, 0.1);
  --rule-strong: rgba(243, 228, 193, 0.3);
  --gold: #f5c542;             /* sunlit saffron on ink */
  --gold-bright: #ffd96b;      /* molten bullion peak */
  --gold-deep: #8a5f14;        /* buried bullion */
  --gold-soft: rgba(245, 197, 66, 0.1);
  --gold-glow: rgba(255, 217, 107, 0.45);
  --flame: #ff7a2e;            /* flame burns brighter on black */
  --flame-bright: #ff9450;
  --flame-deep: #bf3f10;
  --flame-glow: rgba(255, 122, 46, 0.42);
  --grain-opacity: 0.095;
  --grain-mode: overlay;
  --cta-gold-a: #ffd96b;
  --cta-gold-b: #f2b418;
}

/* ---- Shared helpers ---- */
.gold-ital {
  font-style: italic;
  background: linear-gradient(90deg, var(--gold-bright), var(--gold), var(--gold-deep));
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}
.soft { opacity: 0.55; }

/* Reveal animation */
[data-reveal] {
  opacity: 0;
  transform: translateY(16px);
  transition: opacity 0.9s cubic-bezier(.2,.7,.2,1), transform 0.9s cubic-bezier(.2,.7,.2,1);
  will-change: opacity, transform;
}
[data-reveal].in-view {
  opacity: 1;
  transform: translateY(0);
}

/* Film grain overlay */
.grain {
  pointer-events: none;
  position: fixed;
  inset: 0;
  z-index: 1;
  opacity: var(--grain-opacity);
  mix-blend-mode: var(--grain-mode);
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='160' height='160'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2' seed='7'/><feColorMatrix values='0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 0.6 0'/></filter><rect width='100%' height='100%' filter='url(%23n)'/></svg>");
}

/* Scroll rail */
.scroll-rail {
  position: fixed;
  top: 50%;
  right: 22px;
  transform: translateY(-50%);
  width: 1px;
  height: 220px;
  background: var(--rule);
  z-index: 10;
  pointer-events: none;
}
.scroll-rail-fill {
  position: absolute;
  top: 0; left: -1px;
  width: 3px;
  background: linear-gradient(180deg, var(--flame), var(--gold));
  transition: height 0.2s ease;
  box-shadow: 0 0 8px var(--flame-glow);
}
.scroll-rail-ticks {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}
.scroll-rail-ticks span {
  width: 9px;
  height: 1px;
  background: var(--rule-strong);
  margin-left: -4px;
}
@media (max-width: 720px) { .scroll-rail { display: none; } }

/* ========================================================================
   NAV
   ======================================================================== */
.nav {
  position: relative;
  z-index: 10;
  padding: 22px 32px;
  border-bottom: 1px solid var(--rule);
}
.nav-inner {
  margin: 0 auto;
  max-width: 1320px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.brand { display: flex; align-items: center; gap: 12px; }
.brand-logo {
  width: 34px; height: 34px;
  border-radius: 10px;
  overflow: hidden;
  background: var(--gold-soft);
  display: flex; align-items: center; justify-content: center;
  border: 1px solid var(--rule);
}
.brand-logo img { width: 100%; height: 100%; object-fit: contain; }
.brand-name {
  font-family: 'Cormorant Garamond', 'Songti SC', 'Noto Serif CJK SC', Georgia, serif;
  font-size: 22px;
  font-weight: 500;
  letter-spacing: 0.04em;
  color: var(--fg);
}
.nav-actions { display: flex; align-items: center; gap: 12px; }
.icon-btn {
  width: 38px; height: 38px;
  display: inline-flex; align-items: center; justify-content: center;
  border-radius: 10px;
  color: var(--fg-2);
  transition: all 0.2s;
  background: transparent;
  border: 1px solid var(--rule);
}
.icon-btn:hover { color: var(--gold); background: var(--gold-soft); border-color: var(--gold); }

/* LocaleSwitcher inherits palette via :deep() — the shared component
   uses neutral Tailwind grays that clash with the warm ivory/ink palette. */
.nav-locale :deep(> div > button) {
  color: var(--fg-2);
  font-weight: 500;
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid var(--rule);
  background: transparent;
  transition: all 0.2s;
}
.nav-locale :deep(> div > button:hover) {
  color: var(--gold);
  background: var(--gold-soft);
  border-color: var(--gold);
}
.nav-locale :deep(> div > button svg) { color: var(--fg-3); }

/* Ghost link — secondary, "登录" */
.nav-ghost {
  display: inline-flex; align-items: center;
  padding: 10px 16px;
  font-size: 14px;
  font-weight: 500;
  color: var(--fg-2);
  border-radius: 999px;
  transition: color 0.2s;
}
.nav-ghost:hover { color: var(--gold); }

/* Hero CTA — solid gold bar, the "天生该被点的" button */
.nav-cta-hero {
  display: inline-flex; align-items: center; gap: 8px;
  padding: 11px 22px 11px 11px;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--cta-ink);
  background: linear-gradient(180deg, var(--cta-gold-a) 0%, var(--cta-gold-b) 100%);
  border-radius: 999px;
  transition: all 0.25s cubic-bezier(.2,.8,.2,1);
  box-shadow:
    0 8px 22px -8px var(--gold-glow),
    inset 0 1px 0 rgba(255,255,255,0.5);
}
.nav-cta-hero:hover {
  transform: translateY(-1px);
  box-shadow:
    0 14px 32px -8px var(--gold-glow),
    inset 0 1px 0 rgba(255,255,255,0.5);
}
/* When the Hero CTA wraps an avatar (authed state), give the avatar breathing room. */
.nav-cta-hero:not(:has(.nav-cta-avatar)) { padding: 11px 22px; }
.nav-cta-avatar {
  width: 26px; height: 26px;
  border-radius: 50%;
  display: inline-flex; align-items: center; justify-content: center;
  background: var(--cta-ink);
  color: var(--cta-gold-a);
  font-size: 12px;
  font-weight: 700;
}
@media (max-width: 640px) {
  .nav-ghost { padding: 8px 10px; font-size: 13px; }
  .nav-cta-hero { padding: 9px 16px; font-size: 14px; }
}

/* ========================================================================
   CHAPTER MARKS
   ======================================================================== */
.chapter-mark {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 11px;
  letter-spacing: 0.32em;
  color: var(--fg-3);
  text-transform: uppercase;
  font-feature-settings: 'tnum';
}
.chapter-mark.center { justify-content: center; text-align: center; }
.chapter-n {
  color: var(--gold);
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-style: italic;
  font-size: 18px;
  letter-spacing: 0;
  text-transform: none;
}
.chapter-rule {
  flex: 0 0 54px;
  height: 1px;
  background: var(--rule-strong);
}
.chapter-mark.center .chapter-rule { flex-basis: 36px; }

/* ========================================================================
   HERO
   ======================================================================== */
.hero {
  position: relative;
  z-index: 2;
  padding: 44px 32px 90px;
  min-height: calc(100vh - 78px);
  display: flex;
  flex-direction: column;
  justify-content: center;
  max-width: 1320px;
  margin: 0 auto;
}
.hero-inner {
  display: grid;
  grid-template-columns: 1.15fr 1fr;
  gap: 60px;
  align-items: center;
}
@media (max-width: 960px) {
  .hero-inner { grid-template-columns: 1fr; gap: 40px; }
}

.hero-title {
  margin: 28px 0 22px;
  font-family: 'Cormorant Garamond', 'Songti SC', 'Noto Serif CJK SC', Georgia, serif;
  font-weight: 500;
  font-size: clamp(44px, 7.2vw, 104px);
  line-height: 1.02;
  letter-spacing: -0.02em;
}
.hero-title .line { display: block; }

.hero-sub {
  max-width: 540px;
  font-size: 18px;
  line-height: 1.68;
  color: var(--fg-2);
}
.hero-sub em {
  font-style: italic;
  font-family: 'Cormorant Garamond', 'Songti SC', serif;
  font-size: 1.18em;
  color: var(--gold);
  letter-spacing: 0.01em;
}

.hero-ctas {
  margin-top: 38px;
  display: flex;
  align-items: center;
  gap: 22px;
  flex-wrap: wrap;
}
.cta-primary {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 16px 30px;
  font-size: 16px;
  font-weight: 500;
  color: var(--cta-ink);
  background: linear-gradient(180deg, var(--cta-gold-a) 0%, var(--cta-gold-b) 100%);
  border-radius: 999px;
  transition: all 0.25s cubic-bezier(.2,.8,.2,1);
  box-shadow:
    0 10px 30px -10px var(--gold-glow),
    inset 0 1px 0 rgba(255,255,255,0.5);
}
.cta-primary svg { width: 18px; height: 18px; transition: transform 0.25s; }
.cta-primary:hover {
  transform: translateY(-2px);
  box-shadow:
    0 16px 44px -10px var(--gold-glow),
    inset 0 1px 0 rgba(255,255,255,0.5);
}
.cta-primary:hover svg { transform: translateX(4px); }

.hero-foot {
  margin-top: 60px;
  padding-top: 28px;
  border-top: 1px solid var(--rule);
  display: flex;
  gap: 40px;
  align-items: center;
  flex-wrap: wrap;
}
.stat { display: flex; flex-direction: column; gap: 4px; }
.stat-num {
  font-family: 'Cormorant Garamond', 'Songti SC', Georgia, serif;
  font-size: 30px;
  font-weight: 500;
  color: var(--gold);
  letter-spacing: -0.01em;
  font-variant-numeric: tabular-nums;
}
.stat-lbl {
  font-size: 11px;
  color: var(--fg-3);
  letter-spacing: 0.22em;
  text-transform: uppercase;
}
.sep-v {
  width: 1px;
  height: 30px;
  background: var(--rule-strong);
}
@media (max-width: 640px) { .sep-v { display: none; } }

/* ====== TOKEN VISUAL ====== */
.hero-right { position: relative; display: flex; justify-content: center; }
.token-stage {
  position: relative;
  width: 480px;
  height: 480px;
  display: flex;
  align-items: center;
  justify-content: center;
}
@media (max-width: 520px) { .token-stage { width: 340px; height: 340px; } }

.orbit {
  position: absolute;
  border-radius: 50%;
  border: 1px solid var(--gold-glow);
}
.orbit-1 { inset: 0; animation: spin 30s linear infinite; opacity: 0.55; }
.orbit-2 { inset: 40px; border-style: dashed; animation: spin 55s linear infinite reverse; opacity: 0.35; }
.orbit-3 { inset: 90px; animation: spin 80s linear infinite; opacity: 0.7; }
@keyframes spin { to { transform: rotate(360deg); } }

.token-coin {
  position: relative;
  width: 300px;
  height: 300px;
  animation: float 6s ease-in-out infinite;
}
@media (max-width: 520px) { .token-coin { width: 220px; height: 220px; } }

@keyframes float {
  0%,100% { transform: translateY(0) rotate(-3deg); }
  50%     { transform: translateY(-14px) rotate(3deg); }
}

.token-face {
  position: relative;
  width: 100%;
  height: 100%;
  filter:
    drop-shadow(0 0 34px var(--gold-glow))
    drop-shadow(0 22px 40px rgba(0,0,0,0.35));
}
.hex-svg { width: 100%; height: 100%; }
.token-glyph {
  font-family: 'Cormorant Garamond', 'Songti SC', Georgia, serif;
  font-size: 154px;
  font-weight: 500;
  fill: url(#tokenInk);
  letter-spacing: -0.04em;
  line-height: 1;
  dominant-baseline: middle;
  paint-order: stroke fill;
  stroke: rgba(58,38,8,0.18);
  stroke-width: 0.6px;
}

.token-shadow {
  position: absolute;
  left: 10%;
  right: 10%;
  bottom: -30px;
  height: 30px;
  background: radial-gradient(ellipse at center, var(--gold-glow), transparent 70%);
  filter: blur(8px);
  animation: shadowPulse 6s ease-in-out infinite;
}
@keyframes shadowPulse {
  0%,100% { transform: scaleX(1); opacity: 0.75; }
  50%     { transform: scaleX(0.85); opacity: 0.5; }
}

.spark {
  position: absolute;
  color: var(--gold-bright);
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-size: 22px;
  opacity: 0;
  animation: sparkle 5s linear infinite;
  text-shadow: 0 0 12px var(--gold-glow);
}
.s1 { top: 8%;  left: 18%; animation-delay: 0s; }
.s2 { top: 22%; right: 10%; animation-delay: 1.2s; font-size: 16px; color: var(--flame); text-shadow: 0 0 12px var(--flame-glow); }
.s3 { bottom: 22%; left: 6%; animation-delay: 2.4s; font-size: 14px; font-family: 'Inter',sans-serif; font-weight: 600; letter-spacing: 0.05em; color: var(--flame-bright); text-shadow: 0 0 12px var(--flame-glow); }
.s4 { bottom: 14%; right: 18%; animation-delay: 3.3s; font-size: 14px; }
.s5 { top: 48%; left: 2%; animation-delay: 4.1s; }
@keyframes sparkle {
  0%   { opacity: 0; transform: translateY(10px) scale(0.5); }
  25%  { opacity: 1; transform: translateY(0) scale(1); }
  75%  { opacity: 1; }
  100% { opacity: 0; transform: translateY(-20px) scale(0.6); }
}

.scroll-hint {
  margin-top: 60px;
  text-align: center;
  font-family: 'Cormorant Garamond', 'Songti SC', serif;
  font-style: italic;
  font-size: 14px;
  color: var(--fg-3);
  letter-spacing: 0.12em;
  animation: bob 2.8s ease-in-out infinite;
}
@keyframes bob {
  0%,100% { opacity: 0.4; transform: translateY(0); }
  50%     { opacity: 0.85; transform: translateY(6px); }
}

/* ========================================================================
   MARQUEE
   ======================================================================== */
.marquee {
  position: relative;
  z-index: 2;
  overflow: hidden;
  border-top: 1px solid var(--rule-strong);
  border-bottom: 1px solid var(--rule-strong);
  background:
    linear-gradient(90deg, var(--gold-soft), transparent 45%, var(--gold-soft));
  padding: 22px 0;
}
.marquee-track {
  display: flex;
  white-space: nowrap;
  animation: slide 48s linear infinite;
}
.marquee-group {
  display: inline-flex;
  align-items: center;
  gap: 38px;
  padding-right: 38px;
  font-family: 'Cormorant Garamond', 'Songti SC', Georgia, serif;
  font-size: 30px;
  font-style: italic;
  color: var(--gold);
  letter-spacing: 0.02em;
}
.marquee-group .dot { font-size: 12px; color: var(--gold-deep); opacity: 0.55; }
@keyframes slide {
  from { transform: translateX(0); }
  to   { transform: translateX(-50%); }
}

/* ========================================================================
   LIVE COUNTER
   ======================================================================== */
.live {
  position: relative;
  z-index: 2;
  padding: 110px 32px;
  text-align: center;
  background: radial-gradient(ellipse at center top, var(--gold-soft), transparent 70%);
}
.live-inner { max-width: 1100px; margin: 40px auto 0; }
.live-intro {
  font-family: 'Cormorant Garamond', 'Songti SC', serif;
  font-style: italic;
  font-size: 20px;
  color: var(--fg-3);
  margin-bottom: 20px;
}
.live-number {
  font-family: 'Cormorant Garamond', 'Songti SC', Georgia, serif;
  font-size: clamp(56px, 10vw, 156px);
  font-weight: 500;
  line-height: 1;
  letter-spacing: -0.03em;
  background: linear-gradient(180deg, var(--gold-bright) 0%, var(--gold) 55%, var(--gold-deep) 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  font-variant-numeric: tabular-nums;
  text-shadow: 0 0 60px var(--gold-glow);
}
.live-unit {
  margin-top: 18px;
  font-size: 22px;
  color: var(--fg-2);
  font-family: 'Cormorant Garamond', 'Songti SC', serif;
  font-style: italic;
}
.live-sub {
  margin-top: 22px;
  font-size: 14px;
  line-height: 1.7;
  color: var(--fg-3);
}
.live-sub strong { color: var(--gold); font-weight: 500; font-variant-numeric: tabular-nums; }

/* ========================================================================
   TRUST
   ======================================================================== */
.trust {
  position: relative;
  z-index: 2;
  padding: 70px 32px;
  background: var(--bg);
  border-top: 1px solid var(--rule);
  border-bottom: 1px solid var(--rule);
}
.trust-inner { max-width: 1100px; margin: 0 auto; text-align: center; }
.trust-label {
  font-size: 12px;
  letter-spacing: 0.32em;
  color: var(--fg-3);
  margin-bottom: 18px;
  text-transform: uppercase;
}
.trust-headline {
  margin: 0 0 14px;
  font-family: 'Cormorant Garamond', 'Songti SC', 'Noto Serif CJK SC', Georgia, serif;
  font-size: clamp(30px, 4.4vw, 52px);
  font-weight: 500;
  line-height: 1.16;
  letter-spacing: -0.01em;
  color: var(--fg);
}
.trust-headline .gold-ital {
  font-size: 1.12em;
  padding: 0 0.05em;
}
.trust-reason {
  margin: 0 auto 32px;
  max-width: 640px;
  font-family: 'Cormorant Garamond', 'Songti SC', serif;
  font-style: italic;
  font-size: clamp(16px, 1.8vw, 20px);
  line-height: 1.6;
  color: var(--fg-2);
}
.trust-row {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
}
.tag {
  padding: 10px 22px;
  font-size: 15px;
  color: var(--fg);
  background: var(--gold-soft);
  border: 1px solid var(--rule);
  border-radius: 999px;
  font-family: 'Cormorant Garamond', Georgia, serif;
  font-style: italic;
  letter-spacing: 0.02em;
}
.tag-soon { color: var(--fg-3); background: transparent; }

/* ========================================================================
   FINAL CTA
   ======================================================================== */
.final {
  position: relative;
  z-index: 2;
  padding: 140px 32px 120px;
  text-align: center;
  background:
    radial-gradient(ellipse at center, var(--gold-soft), transparent 60%),
    var(--bg);
}
.final-inner { max-width: 960px; margin: 44px auto 0; }
.final-title {
  font-family: 'Cormorant Garamond', 'Songti SC', Georgia, serif;
  font-size: clamp(42px, 6.4vw, 92px);
  font-weight: 500;
  line-height: 1.1;
  letter-spacing: -0.02em;
  color: var(--fg);
}
.final-title .gold-ital {
  font-size: 1.18em;
  padding: 0 0.06em;
  display: inline-block;
  transform: translate(-0.08em, 0.08em);
}
.final-title .soft {
  display: inline-block;
  margin-top: 8px;
  font-size: 0.56em;
  color: var(--fg-3);
  font-style: italic;
}

.final-cta {
  margin-top: 52px;
  display: inline-flex;
  align-items: center;
  gap: 12px;
  padding: 20px 42px;
  font-size: 17px;
  font-weight: 500;
  color: var(--cta-ink);
  background: linear-gradient(180deg, var(--cta-gold-a) 0%, var(--cta-gold-b) 100%);
  border-radius: 999px;
  transition: all 0.3s cubic-bezier(.2,.8,.2,1);
  box-shadow:
    0 20px 60px -15px var(--gold-glow),
    inset 0 1px 0 rgba(255,255,255,0.5);
}
.final-cta svg { width: 20px; height: 20px; transition: transform 0.3s; }
.final-cta:hover {
  transform: translateY(-3px) scale(1.02);
  box-shadow:
    0 30px 80px -15px var(--gold-glow),
    inset 0 1px 0 rgba(255,255,255,0.5);
}
.final-cta:hover svg { transform: translateX(5px); }

.final-foot {
  margin-top: 30px;
  font-size: 13px;
  color: var(--fg-3);
  letter-spacing: 0.02em;
}

/* ========================================================================
   FOOTER
   ======================================================================== */
.foot {
  position: relative;
  z-index: 2;
  padding: 30px 32px;
  border-top: 1px solid var(--rule);
}
.foot-inner {
  max-width: 1320px;
  margin: 0 auto;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--fg-3);
  letter-spacing: 0.04em;
}
.foot-links { display: flex; gap: 22px; }
.foot-links a {
  color: var(--fg-3);
  transition: color 0.2s;
}
.foot-links a:hover { color: var(--gold); }

/* ========================================================================
   RESPONSIVE NUDGES
   ======================================================================== */
@media (max-width: 768px) {
  .nav { padding: 16px 20px; }
  .brand-name { font-size: 18px; }
  .hero { padding: 32px 20px 70px; }
  .contrast, .final, .live { padding-left: 20px; padding-right: 20px; }
  .contrast, .live { padding-top: 90px; padding-bottom: 90px; }
  .final { padding-top: 100px; padding-bottom: 90px; }
  .hero-foot { gap: 28px; }
  .stat-num { font-size: 26px; }
  .marquee-group { font-size: 22px; gap: 26px; padding-right: 26px; }
}

/* ──────────────────────────────────────────────────────────────────────
   GLOBE SECTION — embedded live world map
   Tonally consistent with the rest of the landing: editorial centred
   headline, dark stage, gold-italic accent on the noun. Stage is 70vh
   tall on desktop so the globe always reads as a proper "moment", not
   a thumbnail.
   ────────────────────────────────────────────────────────────────────── */
.globe-section {
  padding: 160px 24px 120px;
  background: linear-gradient(to bottom, transparent 0%, #03070d 18%, #03070d 82%, transparent 100%);
  position: relative;
}
.globe-section-headline {
  max-width: 920px;
  margin: 32px auto 56px;
  text-align: center;
}
.globe-h2 {
  font-family: 'Cormorant Garamond', 'Source Han Serif SC', 'Songti SC', Georgia, serif;
  font-size: clamp(32px, 4.4vw, 64px);
  line-height: 1.1;
  font-weight: 400;
  color: #fff;
  margin: 0 0 18px;
  letter-spacing: -0.01em;
}
.globe-h2 .gold-ital {
  font-style: italic;
  color: #ffd87a;
  letter-spacing: 0.04em;
  font-weight: 300;
}
.globe-sub {
  font-family: 'Inter', sans-serif;
  font-size: 13px;
  color: rgba(184, 200, 220, 0.7);
  letter-spacing: 0.04em;
}
.globe-sub .dot-sep { margin: 0 12px; color: rgba(255,255,255,0.25); }

.globe-stage {
  position: relative;
  max-width: 1280px;
  margin: 0 auto;
  height: 70vh;
  min-height: 520px;
  border: 1px solid rgba(255,255,255,0.06);
  background: #02060c;
  overflow: hidden;
}
.globe-canvas-wrap {
  position: absolute !important;
  inset: 0;
}
.globe-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: radial-gradient(ellipse at 50% 55%, #06121e 0%, #02060c 60%, #000 100%);
}
.globe-placeholder-orb {
  width: 280px;
  height: 280px;
  border-radius: 50%;
  background: radial-gradient(circle at 50% 40%, rgba(95,199,255,0.05) 0%, transparent 70%);
  border: 1px solid rgba(95,199,255,0.08);
  animation: globe-placeholder-pulse 2.4s ease-in-out infinite;
}
@keyframes globe-placeholder-pulse {
  0%, 100% { opacity: 0.4; transform: scale(0.96); }
  50% { opacity: 0.8; transform: scale(1.0); }
}
.globe-overlay-meta {
  position: absolute;
  top: 24px;
  left: 24px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  font-family: 'JetBrains Mono', 'IBM Plex Mono', ui-monospace, monospace;
  font-size: 10.5px;
  letter-spacing: 0.22em;
  z-index: 4;
}
.globe-overlay-meta .meta-row {
  display: flex;
  align-items: baseline;
  gap: 14px;
}
.globe-overlay-meta .meta-k {
  color: rgba(184, 200, 220, 0.45);
  min-width: 110px;
}
.globe-overlay-meta .meta-v {
  color: #fff;
  font-feature-settings: 'tnum' 1;
  font-size: 14px;
}
.globe-cta-link {
  position: absolute;
  bottom: 24px;
  right: 24px;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 10px 18px;
  border: 1px solid rgba(255,216,122,0.4);
  color: #ffd87a;
  text-decoration: none;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  letter-spacing: 0.22em;
  background: rgba(2,6,12,0.5);
  backdrop-filter: blur(4px);
  transition: all 0.2s ease;
  z-index: 4;
}
.globe-cta-link:hover {
  background: rgba(255,216,122,0.08);
  border-color: rgba(255,216,122,0.7);
}
.globe-cta-link svg { width: 14px; height: 14px; }

@media (max-width: 768px) {
  .globe-section { padding: 100px 14px 80px; }
  .globe-stage { height: 60vh; min-height: 420px; }
  .globe-overlay-meta { top: 14px; left: 14px; font-size: 9px; }
  .globe-overlay-meta .meta-k { min-width: 80px; }
  .globe-overlay-meta .meta-v { font-size: 12px; }
  .globe-cta-link { bottom: 14px; right: 14px; padding: 8px 12px; font-size: 10px; }
}
</style>
