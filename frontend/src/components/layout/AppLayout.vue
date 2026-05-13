<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Global Top Bar (full width, above sidebar and content) -->
    <AppHeader :hide-sidebar="hideSidebar" />

    <!-- Sidebar (drops in under the top bar) -->
    <AppSidebar v-if="!hideSidebar" />

    <!-- Mobile bottom workspace nav (hidden on md+) -->
    <AppBottomNav />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen pt-12 transition-all duration-300 app-main-area"
      :class="mainAreaClass"
    >
      <main :class="contentClass || 'p-4 md:p-6 lg:p-8'">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import AppBottomNav from './AppBottomNav.vue'

const props = withDefaults(defineProps<{
  hideSidebar?: boolean
  contentClass?: string
}>(), {
  hideSidebar: false,
  contentClass: '',
})

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const canAccessAdmin = computed(() => authStore.canAccessAdmin)
const hideSidebar = computed(() => props.hideSidebar)
const contentClass = computed(() => props.contentClass)
const mainAreaClass = computed(() => {
  if (hideSidebar.value) return 'w-full'
  return sidebarCollapsed.value
    ? 'lg:ml-[72px] lg:w-[calc(100%_-_72px)]'
    : 'lg:ml-64 lg:w-[calc(100%_-_16rem)]'
})

const { replayTour } = useOnboardingTour({
  storageKey: canAccessAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
/* Leave room for the fixed bottom nav on mobile (visible below md).
   Includes safe-area inset so the last row never sits behind the iPhone
   home indicator. On md+ the bottom nav is hidden so no extra padding. */
@media (max-width: 767px) {
  .app-main-area {
    padding-bottom: calc(3.5rem + env(safe-area-inset-bottom) + 0.5rem);
  }
}
</style>
