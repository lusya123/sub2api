<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import ModelCard from '@/components/status/ModelCard.vue'
import { publicStatusApi } from '@/api/publicStatus'
import type { StatusModel } from '@/types'

// TODO(i18n): child components (ModelCard / GroupSection / ChannelRow /
// HeartbeatBar) still contain hardcoded Chinese strings (e.g. "可用率",
// "负载", "已复制"). Migrate them to i18n in a follow-up task to keep
// this PR focused on the /status route wiring.

const { t } = useI18n()
const models = ref<StatusModel[]>([])
const loading = ref(true)
const lastUpdated = ref<Date | null>(null)
let timer: number | undefined

const load = async (silent = false) => {
  try {
    if (!silent) loading.value = true
    const res = await publicStatusApi.listModels()
    const list = res.data.models
    const details = await Promise.all(
      list.map((m) => publicStatusApi.getModel(m.name).then((r) => r.data))
    )
    models.value = details
    lastUpdated.value = new Date()
  } catch (err) {
    // Intentionally swallow — keep last known good data on screen.
    // Full error UX is deferred to the E2E task.
    console.error('[StatusView] failed to load status data', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
  timer = window.setInterval(() => load(true), 60_000)
})
onUnmounted(() => {
  if (timer !== undefined) {
    clearInterval(timer)
    timer = undefined
  }
})
</script>

<template>
  <main class="dark min-h-screen bg-neutral-950 text-neutral-100 p-6">
    <header class="max-w-7xl mx-auto mb-6">
      <h1 class="text-xl font-semibold">{{ t('status.title') }}</h1>
      <p class="text-sm text-neutral-500 mt-1">{{ t('status.subtitle') }}</p>
      <div v-if="lastUpdated" class="text-xs text-neutral-600 mt-2">
        {{ t('status.lastUpdated') }}: {{ lastUpdated.toLocaleTimeString() }}
      </div>
    </header>

    <div class="max-w-7xl mx-auto">
      <div
        v-if="loading"
        class="grid gap-4 md:grid-cols-2 xl:grid-cols-3"
        data-testid="status-loading"
      >
        <div
          v-for="i in 3"
          :key="i"
          class="h-96 rounded-xl bg-neutral-900 animate-pulse"
        />
      </div>
      <div
        v-else-if="models.length === 0"
        class="text-center text-neutral-500 py-20"
        data-testid="status-empty"
      >
        <svg
          class="mx-auto h-12 w-12 text-neutral-700 mb-3"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          aria-hidden="true"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M9 17.25v1.007a3 3 0 0 1-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0 1 15 18.257V17.25m6-12V15a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 15V5.25m18 0A2.25 2.25 0 0 0 18.75 3H5.25A2.25 2.25 0 0 0 3 5.25m18 0V12a2.25 2.25 0 0 1-2.25 2.25H5.25A2.25 2.25 0 0 1 3 12V5.25"
          />
        </svg>
        {{ t('status.empty') }}
      </div>
      <div v-else class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <ModelCard v-for="m in models" :key="m.name" :model="m" />
      </div>
    </div>
  </main>
</template>
