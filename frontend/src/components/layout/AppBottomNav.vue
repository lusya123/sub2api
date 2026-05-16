<template>
  <nav
    v-if="user"
    ref="navRef"
    class="app-bottom-nav"
    aria-label="Primary navigation"
  >
    <router-link
      v-for="item in primaryTopNavItems"
      :key="item.path"
      :to="item.path"
      class="app-bottom-nav__item"
      :class="{ 'app-bottom-nav__item--active': isPrimaryTopNavActive(item.key) }"
    >
      <Icon :name="item.icon" size="md" class="app-bottom-nav__icon" />
      <span class="app-bottom-nav__label">{{ item.label }}</span>
    </router-link>
    <span
      class="app-bottom-nav__indicator"
      :style="indicatorStyle"
      aria-hidden="true"
    ></span>
  </nav>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const route = useRoute()
const { t } = useI18n()
const authStore = useAuthStore()

const user = computed(() => authStore.user)
const consolePath = computed(() => (authStore.canAccessAdmin ? '/admin/dashboard' : '/dashboard'))

const primaryTopNavItems = computed(() => {
  const items: Array<{ key: string; path: string; label: string; icon: 'home' | 'grid' | 'chat' }> = [{
    key: 'console',
    path: consolePath.value,
    label: t('nav.console'),
    icon: 'home' as const,
  },
  {
    key: 'marketplace',
    path: '/model-marketplace',
    label: t('nav.modelMarketplace'),
    icon: 'grid' as const,
  },
  ]

  if (isFeatureFlagEnabled(FeatureFlags.chatPage) || isFeatureFlagEnabled(FeatureFlags.agentPage)) {
    items.push({
      key: 'use-token',
      path: '/chat',
      label: t('nav.useToken'),
      icon: 'chat' as const,
    })
  }

  return items
})

function isPrimaryTopNavActive(key: string): boolean {
  if (key === 'marketplace') return route.path === '/model-marketplace'
  if (key === 'use-token') return route.path === '/chat' || route.path === '/use-token'
  return route.path !== '/model-marketplace' && route.path !== '/chat' && route.path !== '/use-token'
}

// Sliding top indicator — same signature motion as the desktop top bar.
const navRef = ref<HTMLElement | null>(null)
const indicatorStyle = ref<Record<string, string>>({ opacity: '0' })

function updateIndicator() {
  const nav = navRef.value
  if (!nav) return
  const active = nav.querySelector<HTMLElement>('.app-bottom-nav__item--active')
  if (!active) {
    indicatorStyle.value = { ...indicatorStyle.value, opacity: '0' }
    return
  }
  const labelTarget = active.offsetWidth * 0.36
  const offsetX = active.offsetLeft + (active.offsetWidth - labelTarget) / 2
  indicatorStyle.value = {
    opacity: '1',
    transform: `translate3d(${offsetX}px, 0, 0)`,
    width: `${labelTarget}px`,
  }
}

// Watch both route AND user: the nav only renders once user is loaded, so a
// late auth fetch must still trigger the first measurement.
watch(
  [() => route.path, user, () => primaryTopNavItems.value.length],
  () => { nextTick(updateIndicator) },
  { flush: 'post' }
)

onMounted(() => {
  nextTick(updateIndicator)
  window.addEventListener('resize', updateIndicator)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateIndicator)
})
</script>

<style scoped>
/* ============ Mobile bottom workspace nav ============
   Shows below md (768px). On larger screens the top bar carries the tabs
   instead, so this is hidden. Matches the top bar's glass + sliding-line
   signature, mirrored to the top edge of the bar. */
.app-bottom-nav {
  position: fixed;
  bottom: 0;
  inset-inline: 0;
  /* Sits above page content (auto z) but BELOW the sidebar overlay (z-40) and
     its backdrop (z-30) so the slide-in drawer cleanly covers it on mobile. */
  z-index: 20;
  display: flex;
  align-items: stretch;
  justify-content: space-around;
  height: calc(3.5rem + env(safe-area-inset-bottom));
  padding-bottom: env(safe-area-inset-bottom);
  background: var(--app-shell-bottom-nav-bg);
  backdrop-filter: blur(20px) saturate(1.4);
  -webkit-backdrop-filter: blur(20px) saturate(1.4);
  transition: background 0.25s ease;
}

/* Hairline at the top edge of the bar — fades from the sides, never hard. */
.app-bottom-nav::before {
  content: '';
  position: absolute;
  inset-inline: 0;
  top: 0;
  height: 1px;
  background: var(--app-shell-hairline);
  pointer-events: none;
  transition: background 0.25s ease;
}

@media (min-width: 768px) {
  .app-bottom-nav {
    display: none;
  }
}

/* ============ Tab items ============ */
.app-bottom-nav__item {
  position: relative;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.1875rem;
  padding: 0.5rem 0.25rem;
  color: var(--app-shell-muted);
  transition:
    color 0.22s ease,
    transform 0.18s ease;
  -webkit-tap-highlight-color: transparent;
}

.app-bottom-nav__item:active {
  transform: scale(0.94);
}

.app-bottom-nav__item--active {
  color: var(--app-shell-text);
}

.app-bottom-nav__icon {
  width: 1.375rem;
  height: 1.375rem;
  transition: transform 0.22s cubic-bezier(0.32, 0.72, 0, 1);
}

.app-bottom-nav__item--active .app-bottom-nav__icon {
  transform: translateY(-1px);
}

.app-bottom-nav__label {
  font-size: 0.6875rem;
  font-weight: 500;
  line-height: 1;
  letter-spacing: 0;
}

.app-bottom-nav__item--active .app-bottom-nav__label {
  font-weight: 600;
}

/* Sliding signature indicator (mirrored to the top edge of the bar). */
.app-bottom-nav__indicator {
  position: absolute;
  top: 0;
  left: 0;
  height: 2px;
  width: 0;
  pointer-events: none;
  will-change: transform, width;
  background: var(--app-shell-indicator);
  border-radius: 0 0 2px 2px;
  transition:
    transform 0.42s cubic-bezier(0.32, 0.72, 0, 1),
    width 0.42s cubic-bezier(0.32, 0.72, 0, 1),
    opacity 0.22s ease;
}
</style>
