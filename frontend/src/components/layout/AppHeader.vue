<template>
  <header class="app-topbar">
    <div class="app-topbar__inner">
      <!-- Left: Mobile menu button (below lg) + Brand -->
      <div class="app-topbar__left">
        <button
          v-if="!hideSidebar"
          type="button"
          class="app-topbar-menu-btn"
          aria-label="Toggle menu"
          @click="toggleMobileSidebar"
        >
          <Icon name="menu" size="sm" />
        </button>

        <router-link
          :to="consolePath"
          class="app-brand"
          :title="siteName"
        >
          <span class="app-brand__logo">
            <img v-if="settingsLoaded" :src="siteLogo || '/logo.png'" :alt="siteName" />
          </span>
          <span class="app-brand__name">{{ siteName }}</span>
        </router-link>
      </div>

      <!-- Center: Primary navigation (workspace tabs) -->
      <nav
        v-if="user"
        ref="navRef"
        class="app-topbar-nav"
        aria-label="Primary navigation"
      >
        <router-link
          v-for="item in primaryTopNavItems"
          :key="item.path"
          :to="item.path"
          class="app-topbar-nav__item"
          :class="isPrimaryTopNavActive(item.key)
            ? 'app-topbar-nav__item--active'
            : 'app-topbar-nav__item--idle'"
        >
          {{ item.label }}
        </router-link>
        <span
          class="app-topbar-nav__indicator"
          :style="indicatorStyle"
          aria-hidden="true"
        ></span>
      </nav>

      <!-- Right: Announcements + Docs + Language + Subscriptions + Balance + User Dropdown -->
      <div class="app-topbar__actions">
        <!-- Announcement Bell -->
        <AnnouncementBell v-if="user" />

        <!-- Docs Link -->
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="app-topbar__docs flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
        >
          <Icon name="book" size="sm" />
          <span class="app-topbar__docs-label hidden sm:inline">{{ t('nav.docs') }}</span>
        </a>

        <!-- Language Switcher -->
        <div class="app-topbar__locale">
          <LocaleSwitcher />
        </div>

        <!-- Subscription Progress (for users with active subscriptions) -->
        <div class="app-topbar__subscription">
          <SubscriptionProgressMini v-if="user" />
        </div>

        <!-- Balance Display -->
        <div
          v-if="user"
          class="app-topbar__balance hidden items-center gap-2 rounded-xl bg-primary-50 px-3 py-1.5 dark:bg-primary-900/20 sm:flex"
          :title="`$${user.balance?.toFixed(2) || '0.00'}`"
        >
          <svg
            class="h-4 w-4 text-primary-600 dark:text-primary-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z"
            />
          </svg>
          <span class="app-topbar__balance-value text-sm font-semibold text-primary-700 dark:text-primary-300">
            ${{ user.balance?.toFixed(2) || '0.00' }}
          </span>
        </div>

        <!-- User Dropdown -->
        <div v-if="user" class="app-topbar__user-menu relative" ref="dropdownRef">
          <button
            @click="toggleDropdown"
            class="app-topbar__user-button flex items-center gap-2 rounded-xl p-1.5 transition-colors hover:bg-gray-100 dark:hover:bg-dark-800"
            aria-label="User Menu"
          >
            <div class="flex h-8 w-8 items-center justify-center overflow-hidden rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 text-sm font-medium text-white shadow-sm">
              <img
                v-if="avatarUrl"
                :src="avatarUrl"
                :alt="displayName"
                class="h-full w-full object-cover"
              >
              <span v-else>{{ userInitials }}</span>
            </div>
            <div class="app-topbar__user-identity hidden text-left md:block">
              <div class="text-sm font-medium text-gray-900 dark:text-white">
                {{ displayName }}
              </div>
              <div class="text-xs capitalize text-gray-500 dark:text-dark-400">
                {{ user.role }}
              </div>
            </div>
            <Icon name="chevronDown" size="sm" class="app-topbar__user-chevron hidden text-gray-400 md:block" />
          </button>

          <!-- Dropdown Menu -->
          <transition name="dropdown">
            <div v-if="dropdownOpen" class="dropdown right-0 mt-2 w-56">
              <!-- User Info -->
              <div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700">
                <div class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ displayName }}
                </div>
                <div class="text-xs text-gray-500 dark:text-dark-400">{{ user.email }}</div>
              </div>

              <!-- Balance (mobile only) -->
              <div class="border-b border-gray-100 px-4 py-2 dark:border-dark-700 sm:hidden">
                <div class="text-xs text-gray-500 dark:text-dark-400">
                  {{ t('common.balance') }}
                </div>
                <div class="text-sm font-semibold text-primary-600 dark:text-primary-400">
                  ${{ user.balance?.toFixed(2) || '0.00' }}
                </div>
              </div>

              <div class="py-1">
                <router-link to="/profile" @click="closeDropdown" class="dropdown-item">
                  <Icon name="user" size="sm" />
                  {{ t('nav.profile') }}
                </router-link>

                <router-link to="/keys" @click="closeDropdown" class="dropdown-item">
                  <Icon name="key" size="sm" />
                  {{ t('nav.apiKeys') }}
                </router-link>

                <a
                  v-if="authStore.isAdmin"
                  href="https://github.com/Wei-Shaw/sub2api"
                  target="_blank"
                  rel="noopener noreferrer"
                  @click="closeDropdown"
                  class="dropdown-item"
                >
                  <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
                    <path
                      fill-rule="evenodd"
                      clip-rule="evenodd"
                      d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.17 6.839 9.49.5.092.682-.217.682-.482 0-.237-.008-.866-.013-1.7-2.782.604-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.464-1.11-1.464-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.831.092-.646.35-1.086.636-1.336-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836c.85.004 1.705.114 2.504.336 1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.167 22 16.418 22 12c0-5.523-4.477-10-10-10z"
                    />
                  </svg>
                  {{ t('nav.github') }}
                </a>

              </div>

              <!-- Contact Support (only show if configured) -->
              <div
                v-if="contactInfo"
                class="border-t border-gray-100 px-4 py-2.5 dark:border-dark-700"
              >
                <div class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                  <svg
                    class="h-3.5 w-3.5 flex-shrink-0"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M20.25 8.511c.884.284 1.5 1.128 1.5 2.097v4.286c0 1.136-.847 2.1-1.98 2.193-.34.027-.68.052-1.02.072v3.091l-3-3c-1.354 0-2.694-.055-4.02-.163a2.115 2.115 0 01-.825-.242m9.345-8.334a2.126 2.126 0 00-.476-.095 48.64 48.64 0 00-8.048 0c-1.131.094-1.976 1.057-1.976 2.192v4.286c0 .837.46 1.58 1.155 1.951m9.345-8.334V6.637c0-1.621-1.152-3.026-2.76-3.235A48.455 48.455 0 0011.25 3c-2.115 0-4.198.137-6.24.402-1.608.209-2.76 1.614-2.76 3.235v6.226c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.14 1.74.194V21l4.155-4.155"
                    />
                  </svg>
                  <span>{{ t('common.contactSupport') }}:</span>
                  <span class="font-medium text-gray-700 dark:text-gray-300">{{
                    contactInfo
                  }}</span>
                </div>
              </div>

              <div v-if="showOnboardingButton" class="border-t border-gray-100 py-1 dark:border-dark-700">
                <button @click="handleReplayGuide" class="dropdown-item w-full">
                  <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24">
                    <path
                      d="M12 2a10 10 0 100 20 10 10 0 000-20zm0 14a1 1 0 110 2 1 1 0 010-2zm1.07-7.75c0-.6-.49-1.25-1.32-1.25-.7 0-1.22.4-1.43 1.02a1 1 0 11-1.9-.62A3.41 3.41 0 0111.8 5c2.02 0 3.25 1.4 3.25 2.9 0 2-1.83 2.55-2.43 3.12-.43.4-.47.75-.47 1.23a1 1 0 01-2 0c0-1 .16-1.82 1.1-2.7.69-.64 1.82-1.05 1.82-2.06z"
                    />
                  </svg>
                  {{ $t('onboarding.restartTour') }}
                </button>
              </div>

              <div class="border-t border-gray-100 py-1 dark:border-dark-700">
                <button
                  @click="handleLogout"
                  class="dropdown-item w-full text-red-600 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
                >
                  <svg
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15M12 9l-3 3m0 0l3 3m-3-3h12.75"
                    />
                  </svg>
                  {{ t('nav.logout') }}
                </button>
              </div>
            </div>
          </transition>
        </div>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import Icon from '@/components/icons/Icon.vue'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const props = withDefaults(defineProps<{
  hideSidebar?: boolean
}>(), {
  hideSidebar: false,
})

const hideSidebar = computed(() => props.hideSidebar)

const appStore = useAppStore()
const authStore = useAuthStore()
const onboardingStore = useOnboardingStore()

const user = computed(() => authStore.user)
const dropdownOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const contactInfo = computed(() => appStore.contactInfo)
const docUrl = computed(() => appStore.docUrl)
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.siteLogo)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')

const consolePath = computed(() => {
  return authStore.canAccessAdmin ? '/admin/dashboard' : '/dashboard'
})

const primaryTopNavItems = computed(() => {
  const items: Array<{ key: string; path: string; label: string; icon: 'home' | 'grid' | 'chat' }> = [{
    key: 'console',
    path: consolePath.value,
    label: t('nav.console'),
    icon: 'home' as const,
  }]

  if (isFeatureFlagEnabled(FeatureFlags.modelMarketplace)) {
    items.push({
      key: 'marketplace',
      path: '/model-marketplace',
      label: t('nav.modelMarketplace'),
      icon: 'grid' as const,
    })
  }

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

// Sliding underline indicator — measures the active tab's offsetLeft/Width and
// animates a single absolutely-positioned element between tabs. The motion is
// the only "signature" — everything else stays quiet.
const navRef = ref<HTMLElement | null>(null)
const indicatorStyle = ref<Record<string, string>>({ opacity: '0' })

function updateIndicator() {
  const nav = navRef.value
  if (!nav) return
  const active = nav.querySelector<HTMLElement>('.app-topbar-nav__item--active')
  if (!active) {
    indicatorStyle.value = { ...indicatorStyle.value, opacity: '0' }
    return
  }
  indicatorStyle.value = {
    opacity: '1',
    transform: `translate3d(${active.offsetLeft}px, 0, 0)`,
    width: `${active.offsetWidth}px`,
  }
}

// Watch both route AND user: when the nav mounts later (auth fetch resolves
// after the layout), navRef goes from null to an element and the indicator
// must measure once it exists. Without the user watch the indicator stays
// at opacity 0 on first load.
watch(
  [() => route.path, user, () => primaryTopNavItems.value.length],
  () => { nextTick(updateIndicator) },
  { flush: 'post' }
)

// 只在标准模式的管理员下显示新手引导按钮
const showOnboardingButton = computed(() => {
  return !authStore.isSimpleMode && user.value?.role === 'admin'
})

const userInitials = computed(() => {
  if (!user.value) return ''
  // Prefer username, fallback to email
  if (user.value.username) {
    return user.value.username.substring(0, 2).toUpperCase()
  }
  if (user.value.email) {
    // Get the part before @ and take first 2 chars
    const localPart = user.value.email.split('@')[0]
    return localPart.substring(0, 2).toUpperCase()
  }
  return ''
})

const displayName = computed(() => {
  if (!user.value) return ''
  return user.value.username || user.value.email?.split('@')[0] || ''
})

// Page title / description are no longer rendered in the header (the active
// sidebar item carries that context). The brand on the left identifies the app.

function toggleMobileSidebar() {
  appStore.toggleMobileSidebar()
}

function toggleDropdown() {
  dropdownOpen.value = !dropdownOpen.value
}

function closeDropdown() {
  dropdownOpen.value = false
}

async function handleLogout() {
  closeDropdown()
  try {
    await authStore.logout()
  } catch (error) {
    // Ignore logout errors - still redirect to login
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function handleReplayGuide() {
  closeDropdown()
  onboardingStore.replay()
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  nextTick(updateIndicator)
  window.addEventListener('resize', updateIndicator)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('resize', updateIndicator)
})
</script>

<style scoped>
.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}

/* ============ Global Top Bar — whisper-quiet, naturally lively ============
   Design intent: the bar itself feels like air — no hard border, no shadow.
   The only motion that exists is one signature element: a single soft
   underline that GLIDES between tabs on route change. Everything else
   stays out of the way. */
.app-topbar {
  position: fixed;
  inset-inline: 0;
  top: 0;
  z-index: 50;
  height: 3rem;
  background: var(--app-shell-topbar-bg);
  backdrop-filter: blur(20px) saturate(1.4);
  -webkit-backdrop-filter: blur(20px) saturate(1.4);
  transition: background 0.25s ease;
}

/* Hairline that fades in from the edges — barely visible, no hard cut. */
.app-topbar::after {
  content: '';
  position: absolute;
  inset-inline: 0;
  bottom: 0;
  height: 1px;
  background: var(--app-shell-hairline);
  pointer-events: none;
  transition: background 0.25s ease;
}

.app-topbar__inner {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
  gap: 0.5rem;
  height: 3rem;
  padding: 0 0.5rem;
}

@media (min-width: 640px) {
  .app-topbar__inner {
    padding-inline: 0.75rem;
  }
}

@media (min-width: 768px) {
  .app-topbar__inner {
    padding-inline: 1rem;
  }
}

.app-topbar__left,
.app-topbar__actions {
  min-width: 0;
  display: flex;
  align-items: center;
}

.app-topbar__left {
  grid-column: 1;
  justify-self: start;
  gap: 0.25rem;
  max-width: 100%;
}

.app-topbar__actions {
  grid-column: 3;
  justify-self: end;
  justify-content: flex-end;
  gap: 0.75rem;
  max-width: 100%;
}

.app-topbar__subscription {
  display: contents;
}

/* ============ Mobile / tablet menu button ============
   Strictly hidden at lg (≥ 1024px) so it cannot leak to desktop the way the
   old Tailwind lg:hidden did. Below lg, opens the sidebar overlay drawer. */
.app-topbar-menu-btn {
  display: none;
}

@media (max-width: 1023px) {
  .app-topbar-menu-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: 2rem;
    width: 2rem;
    border-radius: 0.5rem;
    color: var(--app-shell-muted-strong);
    transition: background-color 0.16s ease, color 0.16s ease;
    -webkit-tap-highlight-color: transparent;
  }
}

.app-topbar-menu-btn:hover {
  background: var(--app-shell-hover-bg);
  color: var(--app-shell-text);
}

.app-topbar-menu-btn:active {
  background: var(--app-shell-active-bg);
}

/* ============ Brand — quiet, no hover pill ============ */
.app-brand {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  height: 2rem;
  padding: 0 0.25rem;
  min-width: 0;
  transition: opacity 0.2s ease;
}

.app-brand:hover {
  opacity: 0.68;
}

.app-brand__logo {
  display: inline-flex;
  flex: 0 0 auto;
  height: 1.375rem;
  width: 1.375rem;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 0.375rem;
}

.app-brand__logo img {
  height: 100%;
  width: 100%;
  object-fit: contain;
}

.app-brand__name {
  font-size: 0.875rem;
  font-weight: 600;
  letter-spacing: 0;
  color: var(--app-shell-text);
  white-space: nowrap;
  max-width: 12rem;
  overflow: hidden;
  text-overflow: ellipsis;
  transition: color 0.22s ease;
}

@media (max-width: 639px) {
  .app-brand__name {
    display: none;
  }
}

/* ============ Workspace tabs — text only, JS-driven sliding underline ============ */
.app-topbar-nav {
  grid-column: 2;
  position: relative;
  align-self: stretch;
  display: flex;
  align-items: stretch;
  justify-self: center;
  gap: 0;
  min-width: 0;
  max-width: 100%;
  overflow-x: auto;
  scrollbar-width: none;
}

.app-topbar-nav::-webkit-scrollbar {
  display: none;
}

.app-topbar-nav__item {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 0 1rem;
  font-size: 0.8125rem;
  font-weight: 500;
  line-height: 1;
  white-space: nowrap;
  color: var(--app-shell-muted);
  transition: color 0.22s ease;
}

.app-topbar-nav__item:hover {
  color: var(--app-shell-text);
}

.app-topbar-nav__item--active,
.app-topbar-nav__item--active:hover {
  color: var(--app-shell-text);
  font-weight: 600;
}

/* The signature: one element that glides between tabs on route change.
   Spring-curve easing creates the "灵动" moment. The gradient softens the
   ends so it feels like brushed light rather than a hard rule. */
.app-topbar-nav__indicator {
  position: absolute;
  bottom: 0;
  left: 0;
  height: 2px;
  width: 0;
  pointer-events: none;
  will-change: transform, width;
  background: var(--app-shell-indicator);
  border-radius: 2px;
  transition:
    transform 0.44s cubic-bezier(0.32, 0.72, 0, 1),
    width 0.44s cubic-bezier(0.32, 0.72, 0, 1),
    opacity 0.22s ease;
}

/* On mobile (< md) the workspace tabs move to the fixed bottom nav
   rendered by AppBottomNav.vue. Hide them from the top bar entirely. */
@media (max-width: 767px) {
  .app-topbar-nav {
    display: none;
  }
}

@media (max-width: 1279px) {
  .app-topbar__user-identity,
  .app-topbar__user-chevron {
    display: none;
  }

  .app-topbar__actions {
    gap: 0.5rem;
  }

  .app-topbar__docs {
    padding-inline: 0.5rem;
  }

  .app-topbar__docs-label {
    display: none;
  }

  .app-topbar__locale :deep(button) {
    gap: 0.25rem;
    padding-inline: 0.5rem;
  }

  .app-topbar__locale :deep(button > span:nth-of-type(2)) {
    display: none;
  }

  .app-topbar__balance {
    padding-inline: 0.5rem;
  }

  .app-topbar__balance-value {
    display: none;
  }
}

@media (max-width: 1023px) {
  .app-topbar__inner {
    gap: 0.375rem;
  }
}

@media (max-width: 899px) {
  .app-brand__name {
    max-width: 7.5rem;
  }

  .app-topbar-nav__item {
    padding-inline: 0.75rem;
  }

  .app-topbar__actions {
    gap: 0.375rem;
  }
}
</style>
