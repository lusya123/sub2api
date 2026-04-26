<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelCard from '@/components/status/ModelCard.vue'
import { publicStatusApi } from '@/api/publicStatus'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import type { StatusModel } from '@/types'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const router = useRouter()
const models = ref<StatusModel[]>([])
const loading = ref(true)
const error = ref('')
const lastUpdated = ref<Date | null>(null)
let timer: number | undefined

const modelCount = computed(() => models.value.length)
const channelCount = computed(() =>
  models.value.reduce((sum, model) => {
    return sum + (model.groups ?? []).reduce((n, group) => n + (group.channels?.length ?? 0), 0)
  }, 0)
)
const avgAvailability = computed(() => {
  const knownModels = models.value.filter(hasKnownStatus)
  if (knownModels.length === 0) return 0
  const total = knownModels.reduce((sum, model) => sum + (model.availability_pct ?? 0), 0)
  return total / knownModels.length
})
const healthyCount = computed(() =>
  models.value.filter((model) => hasKnownStatus(model) && (model.availability_pct ?? 0) >= 99).length
)

const load = async (silent = false) => {
  try {
    if (!silent) loading.value = true
    error.value = ''
    const res = await publicStatusApi.listModels()
    const list = res.data.models ?? []
    const details = await Promise.all(
      list.map((m) => publicStatusApi.getModel(m.name).then((r) => r.data))
    )
    models.value = details
    lastUpdated.value = new Date()
  } catch (err: any) {
    error.value = err?.message || '状态数据加载失败'
    console.error('[StatusView] failed to load status data', err)
  } finally {
    loading.value = false
  }
}

function formatAvailability(value: number) {
  return `${Math.max(0, Math.min(100, value)).toFixed(2)}%`
}

function formatTime(value: Date | null) {
  if (!value) return '--:--'
  return value.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function hasKnownStatus(model: StatusModel) {
  const beats = model.heartbeats ?? []
  return beats.some((beat) => beat.status !== 'unknown')
}

onMounted(async () => {
  const settings = await appStore.fetchPublicSettings()
  if (!authStore.canAccessAdmin && settings?.model_health_page_enabled === false) {
    router.replace(authStore.isAuthenticated ? '/dashboard' : '/home')
    return
  }

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
  <component :is="authStore.isAuthenticated ? AppLayout : 'div'">
    <main
      class="status-page"
      :class="{ 'status-page-embedded': authStore.isAuthenticated }"
    >
      <div class="status-grain" aria-hidden="true"></div>
      <section class="status-hero">
        <div class="status-kicker">
          <span>MODEL STATUS</span>
          <span class="status-rule"></span>
          <span>{{ formatTime(lastUpdated) }}</span>
        </div>
        <div class="status-hero-grid">
          <div>
            <h1>{{ t('status.title') }}</h1>
            <p>{{ t('status.subtitle') }}</p>
          </div>
          <div class="status-metrics" aria-label="status overview">
            <div>
              <span>模型</span>
              <strong>{{ modelCount }}</strong>
            </div>
            <div>
              <span>公开渠道</span>
              <strong>{{ channelCount }}</strong>
            </div>
            <div>
              <span>平均可用率</span>
              <strong>{{ formatAvailability(avgAvailability) }}</strong>
            </div>
            <div>
              <span>健康模型</span>
              <strong>{{ healthyCount }}/{{ modelCount }}</strong>
            </div>
          </div>
        </div>
      </section>

      <section class="status-board">
        <div
          v-if="loading"
          class="status-grid"
          data-testid="status-loading"
        >
          <div
            v-for="i in 3"
            :key="i"
            class="status-skeleton"
          />
        </div>

        <div
          v-else-if="error && models.length === 0"
          class="status-empty"
          data-testid="status-error"
        >
          {{ error }}
        </div>

        <div
          v-else-if="models.length === 0"
          class="status-empty"
          data-testid="status-empty"
        >
          {{ t('status.empty') }}
        </div>

        <div v-else class="status-grid">
          <ModelCard v-for="m in models" :key="m.name" :model="m" />
        </div>
      </section>
    </main>
  </component>
</template>

<style scoped>
.status-page {
  --status-bg: #080604;
  --status-panel: rgba(16, 13, 8, 0.88);
  --status-panel-2: rgba(28, 22, 13, 0.68);
  --status-ink: #f4e7c8;
  --status-muted: rgba(244, 231, 200, 0.58);
  --status-faint: rgba(244, 231, 200, 0.28);
  --status-rule: rgba(244, 231, 200, 0.12);
  --status-gold: #f5c542;
  --status-gold-soft: rgba(245, 197, 66, 0.12);
  --status-flame: #ff7a2e;

  position: relative;
  min-height: 100vh;
  overflow: hidden;
  box-sizing: border-box;
  container-type: inline-size;
  padding: 0 24px;
  background:
    linear-gradient(180deg, rgba(245, 197, 66, 0.06), transparent 260px),
    linear-gradient(90deg, rgba(255, 122, 46, 0.08), transparent 34%),
    var(--status-bg);
  color: var(--status-ink);
  font-family: Inter, -apple-system, BlinkMacSystemFont, 'PingFang SC',
    'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
}

.status-page-embedded {
  --status-panel: rgba(15, 23, 42, 0.74);
  --status-panel-2: rgba(30, 41, 59, 0.64);
  --status-ink: #f8fafc;
  --status-muted: rgba(203, 213, 225, 0.78);
  --status-faint: rgba(148, 163, 184, 0.45);
  --status-rule: rgba(148, 163, 184, 0.18);
  --status-gold: #2dd4bf;
  --status-gold-soft: rgba(45, 212, 191, 0.12);
  --status-flame: #f59e0b;

  width: 100%;
  min-height: auto;
  margin: 0;
  padding: 0;
  overflow: visible;
  background: transparent;
  color: var(--status-ink);
}

.status-grain {
  pointer-events: none;
  position: fixed;
  inset: 0;
  opacity: 0.08;
  mix-blend-mode: overlay;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='160' height='160'><filter id='n'><feTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2' seed='11'/><feColorMatrix values='0 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 0.55 0'/></filter><rect width='100%' height='100%' filter='url(%23n)'/></svg>");
}

.status-page-embedded .status-grain {
  display: none;
}

.status-hero,
.status-board {
  position: relative;
  z-index: 1;
  width: min(1580px, 100%);
  margin: 0 auto;
}

.status-hero {
  padding: 34px 0 24px;
}

.status-page-embedded .status-hero {
  padding: 0 0 20px;
}

.status-kicker {
  display: flex;
  align-items: center;
  gap: 14px;
  color: var(--status-muted);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
}

.status-page-embedded .status-kicker {
  margin-bottom: 12px;
  color: rgba(148, 163, 184, 0.82);
}

.status-rule {
  width: 68px;
  height: 1px;
  background: linear-gradient(90deg, var(--status-gold), transparent);
}

.status-hero-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(420px, 0.7fr);
  gap: 36px;
  align-items: end;
  margin-top: 28px;
}

.status-page-embedded .status-hero-grid {
  align-items: stretch;
  gap: 16px;
  margin-top: 0;
}

.status-hero h1 {
  max-width: 820px;
  color: var(--status-ink);
  font-size: clamp(44px, 6vw, 92px);
  line-height: 0.92;
  font-weight: 900;
  letter-spacing: 0;
}

.status-page-embedded .status-hero h1 {
  display: none;
}

.status-hero p {
  max-width: 620px;
  margin-top: 18px;
  color: var(--status-muted);
  font-size: 16px;
  line-height: 1.8;
}

.status-page-embedded .status-hero p {
  max-width: 760px;
  margin-top: 0;
  color: var(--status-muted);
  font-size: 14px;
  line-height: 1.7;
}

.status-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  border: 1px solid var(--status-rule);
  background: var(--status-panel);
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.28);
}

.status-page-embedded .status-metrics {
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.status-metrics > div {
  min-height: 92px;
  padding: 18px;
  border-right: 1px solid var(--status-rule);
  border-bottom: 1px solid var(--status-rule);
}

.status-page-embedded .status-metrics > div {
  min-height: 92px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 14px;
  background:
    linear-gradient(180deg, rgba(45, 212, 191, 0.08), transparent),
    rgba(15, 23, 42, 0.72);
  box-shadow: 0 18px 46px rgba(2, 6, 23, 0.18);
}

.status-page-embedded .status-metrics > div:nth-child(2n),
.status-page-embedded .status-metrics > div:nth-last-child(-n + 2) {
  border: 1px solid rgba(148, 163, 184, 0.16);
}

.status-metrics > div:nth-child(2n) {
  border-right: 0;
}

.status-metrics > div:nth-last-child(-n + 2) {
  border-bottom: 0;
}

.status-metrics span {
  display: block;
  color: var(--status-muted);
  font-size: 11px;
  letter-spacing: 0.12em;
}

.status-metrics strong {
  display: block;
  margin-top: 9px;
  color: var(--status-gold);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 26px;
  line-height: 1;
}

.status-board {
  padding-bottom: 42px;
}

.status-page-embedded .status-board {
  padding-bottom: 0;
}

.status-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 340px), 1fr));
  gap: 18px;
}

.status-page-embedded .status-grid,
.status-page-embedded .status-hero-grid {
  grid-template-columns: 1fr;
}

.status-skeleton,
.status-empty {
  min-height: 420px;
  border: 1px solid var(--status-rule);
  background: var(--status-panel);
}

.status-page-embedded .status-skeleton,
.status-page-embedded .status-empty {
  min-height: 260px;
  border: 1px dashed rgba(148, 163, 184, 0.24);
  border-radius: 16px;
  background:
    linear-gradient(135deg, rgba(45, 212, 191, 0.06), transparent 34%),
    rgba(15, 23, 42, 0.58);
}

.status-page-embedded :deep(.model-card) {
  border-color: rgba(148, 163, 184, 0.18);
  border-radius: 16px;
  background:
    linear-gradient(180deg, rgba(45, 212, 191, 0.055), transparent 180px),
    rgba(15, 23, 42, 0.76);
  box-shadow: 0 18px 46px rgba(2, 6, 23, 0.18);
}

.status-page-embedded :deep(.model-mark) {
  color: var(--status-flame);
}

.status-page-embedded :deep(.cache-badge) {
  border-color: rgba(45, 212, 191, 0.32);
  background: rgba(20, 184, 166, 0.12);
  color: #5eead4;
}

.status-skeleton {
  position: relative;
  overflow: hidden;
}

.status-skeleton::after {
  content: '';
  position: absolute;
  inset: 0;
  transform: translateX(-100%);
  background: linear-gradient(90deg, transparent, rgba(245, 197, 66, 0.08), transparent);
  animation: status-shimmer 1.4s infinite;
}

.status-empty {
  display: grid;
  place-items: center;
  color: var(--status-muted);
  font-size: 14px;
}

@keyframes status-shimmer {
  100% {
    transform: translateX(100%);
  }
}

@media (max-width: 1500px) {
  .status-grid {
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 320px), 1fr));
  }
}

@media (max-width: 1120px) {
  .status-hero-grid {
    grid-template-columns: 1fr;
  }

  .status-grid {
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 300px), 1fr));
  }
}

@container (min-width: 980px) {
  .status-page-embedded .status-hero-grid {
    grid-template-columns: minmax(280px, 0.58fr) minmax(0, 1fr);
  }

  .status-page-embedded .status-grid {
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 360px), 1fr));
  }
}

@media (max-width: 900px) {
  .status-grid {
    grid-template-columns: 1fr;
  }

  .status-page-embedded .status-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .status-page {
    padding: 0 14px;
  }

  .status-page-embedded {
    width: 100%;
  }

  .status-page-embedded .status-metrics {
    grid-template-columns: 1fr;
  }

  .status-hero {
    padding-top: 24px;
  }

  .status-hero h1 {
    font-size: 42px;
  }

  .status-metrics {
    grid-template-columns: 1fr;
  }

  .status-metrics > div,
  .status-metrics > div:nth-child(2n),
  .status-metrics > div:nth-last-child(-n + 2) {
    border-right: 0;
    border-bottom: 1px solid var(--status-rule);
  }

  .status-metrics > div:last-child {
    border-bottom: 0;
  }
}
</style>
