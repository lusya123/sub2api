<template>
  <div class="globe-stage-root">
    <!-- The two visualisations share the same data feed. We keep both alive
         under <KeepAlive> so the toggle is instant — three.js scenes don't
         re-initialise (which is expensive), and the 2D canvas keeps its
         arc state warm. Only the *active* view receives snapshot props. -->
    <!--
      KeepAlive caches by component definition, so the LiveGlobe and
      LiveMap2D instances each persist across toggles — switching back to
      3D is instant (three.js scene preserved) and 2D keeps its arc state.
      Do NOT add :key here — it would defeat the cache.
    -->
    <KeepAlive>
      <component
        :is="mode === '3d' ? LiveGlobe : LiveMap2D"
        :snapshot="snapshot"
        :server-point="serverPoint"
        :detail="detail"
        :interactive="interactive"
      />
    </KeepAlive>

    <div v-if="!hideToggle" class="stage-toggle" :class="togglePosition">
      <button
        type="button"
        class="toggle-btn"
        :class="{ active: mode === '3d' }"
        @click="setMode('3d')"
        :title="t3DTooltip"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4">
          <circle cx="12" cy="12" r="9" />
          <ellipse cx="12" cy="12" rx="9" ry="3.2" />
          <ellipse cx="12" cy="12" rx="3.2" ry="9" />
        </svg>
        <span>3D</span>
      </button>
      <span class="toggle-sep"></span>
      <button
        type="button"
        class="toggle-btn"
        :class="{ active: mode === '2d' }"
        @click="setMode('2d')"
        :title="t2DTooltip"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.4">
          <rect x="3" y="6" width="18" height="12" rx="1.5" />
          <path d="M3 11h18M9 6v12M15 6v12" />
        </svg>
        <span>2D</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * GlobeStage — container that lets visitors flip between the 3D globe and
 * the 2D world-map view of the same live data feed.
 *
 * Storage rule
 * ────────────
 * The chosen mode is persisted in localStorage under `sub2api.globe.mode`,
 * so a customer landing on /globe gets back the mode they last picked. We
 * also accept a `?view=2d|3d` query param that wins over storage — handy
 * for screenshots, link sharing, and the wall-display setup.
 *
 * Sharing data, not view state
 * ────────────────────────────
 * Both child components are kept alive across mode switches via <KeepAlive>
 * so the three.js scene + canvas arc arrays don't have to be re-initialised
 * on every flip. Only the *active* component pulls live data — the inactive
 * one keeps its last frame.
 */

import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import LiveGlobe from './LiveGlobe.vue'
import LiveMap2D from './LiveMap2D.vue'
import type { GlobeSnapshot, ServerPoint } from '@/composables/useGlobeStream'

type Mode = '3d' | '2d'

const props = withDefaults(
  defineProps<{
    snapshot: GlobeSnapshot | null
    serverPoint?: ServerPoint
    detail?: 'public' | 'admin'
    interactive?: boolean
    /** Where the toggle floats; defaults to top-right. */
    togglePosition?: 'top-right' | 'top-left' | 'top-center' | 'bottom-right' | 'bottom-left' | 'bottom-center'
    /** Set to true to hide the built-in toggle UI when the parent renders its own. */
    hideToggle?: boolean
    /** Initial mode if storage / query param don't override. */
    defaultMode?: Mode
    /**
     * Whether to mirror the chosen mode to ?view= in the URL. Useful for the
     * dedicated /globe and /admin/globe pages (link sharing, deep linking),
     * but should be off when the stage is embedded as a section on a larger
     * page — toggling shouldn't pollute the parent's URL.
     */
    syncToUrl?: boolean
  }>(),
  {
    detail: 'public',
    interactive: false,
    togglePosition: 'top-right',
    defaultMode: '3d',
    syncToUrl: false,
  },
)

const route = useRoute()
const router = useRouter()

const STORAGE_KEY = 'sub2api.globe.mode'
const mode = ref<Mode>(props.defaultMode)

const t3DTooltip = computed(() => '立体地球视图')
const t2DTooltip = computed(() => '平面地图视图')

onMounted(() => {
  // Query param wins — but only when this stage is the page's main artefact.
  if (props.syncToUrl) {
    const q = route.query.view
    if (q === '2d' || q === '3d') {
      mode.value = q
      return
    }
  }
  // Storage second.
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === '2d' || saved === '3d') {
      mode.value = saved
      return
    }
  } catch {
    /* storage may be blocked; default applies */
  }
  mode.value = props.defaultMode
})

function setMode(next: Mode) {
  if (mode.value === next) return
  mode.value = next
  try {
    localStorage.setItem(STORAGE_KEY, next)
  } catch {
    /* storage may be blocked */
  }
  if (props.syncToUrl && route.query.view !== next) {
    router.replace({ query: { ...route.query, view: next } }).catch(() => {})
  }
}

// Reflect external query-param changes (e.g. user edits URL) back into mode —
// only when we own the URL, otherwise an unrelated `view=` param on the
// homepage shouldn't drive the embed.
watch(
  () => route.query.view,
  (v) => {
    if (!props.syncToUrl) return
    if (v === '2d' || v === '3d') mode.value = v
  },
)
</script>

<style scoped>
.globe-stage-root {
  position: relative;
  width: 100%;
  height: 100%;
}

/* Toggle pill — floats over the visualisation. Same gold-on-dark idiom as
   the other chrome on the showcase page so it doesn't introduce a third
   color. Position is configurable so views with chrome elsewhere can move
   it out of the way. */
.stage-toggle {
  position: absolute;
  display: inline-flex;
  align-items: center;
  background: rgba(2, 6, 12, 0.55);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  padding: 4px;
  z-index: 6;
  font-family: 'JetBrains Mono', ui-monospace, monospace;
}
.stage-toggle.top-right { top: 24px; right: 24px; }
.stage-toggle.top-left { top: 24px; left: 24px; }
.stage-toggle.top-center { top: 24px; left: 50%; transform: translateX(-50%); }
.stage-toggle.bottom-right { bottom: 24px; right: 24px; }
.stage-toggle.bottom-left { bottom: 24px; left: 24px; }
.stage-toggle.bottom-center { bottom: 24px; left: 50%; transform: translateX(-50%); }

.toggle-btn {
  appearance: none;
  background: transparent;
  border: none;
  color: rgba(184, 200, 220, 0.6);
  padding: 8px 14px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  letter-spacing: 0.22em;
  cursor: pointer;
  transition: color 0.2s ease;
  font-family: inherit;
  text-transform: uppercase;
}
.toggle-btn:hover {
  color: #fff;
}
.toggle-btn.active {
  color: #ffd87a;
}
.toggle-btn svg {
  width: 14px;
  height: 14px;
}
.toggle-sep {
  width: 1px;
  height: 18px;
  background: rgba(255, 255, 255, 0.1);
}

@media (max-width: 768px) {
  .stage-toggle.top-right,
  .stage-toggle.top-left {
    top: 14px;
  }
  .stage-toggle.top-right { right: 14px; }
  .stage-toggle.top-left { left: 14px; }
  .toggle-btn { padding: 6px 10px; font-size: 10px; }
}
</style>
