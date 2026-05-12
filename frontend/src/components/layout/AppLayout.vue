<template>
  <div class="min-h-screen bg-gray-50 dark:bg-dark-950">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar v-if="!hideSidebar" />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="mainAreaClass"
    >
      <!-- Header -->
      <AppHeader :hide-sidebar="hideSidebar" />

      <!-- Main Content -->
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
