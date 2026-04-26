<template>
  <article class="model-card">
    <header class="model-head">
      <div class="model-mark" aria-hidden="true">
        <svg viewBox="0 0 48 48">
          <path d="M24 3l2.7 14.2L38 8.1l-8.1 12.7L45 18l-14.2 6L45 30l-15.1-2.8L38 39.9l-11.3-9.1L24 45l-2.7-14.2L10 39.9l8.1-12.7L3 30l14.2-6L3 18l15.1 2.8L10 8.1l11.3 9.1L24 3z" />
        </svg>
      </div>
      <div class="model-title">
        <div class="model-name-row">
          <h2>{{ model.name }}</h2>
          <button
            type="button"
            class="copy-btn"
            :title="copied ? '已复制' : '复制模型名'"
            @click="copyName"
          >
            <svg
              v-if="!copied"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
              <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
            </svg>
            <svg
              v-else
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2.5"
              stroke-linecap="round"
              stroke-linejoin="round"
              aria-hidden="true"
            >
              <polyline points="20 6 9 17 4 12" />
            </svg>
          </button>
        </div>
        <div class="model-meta">
          <span>{{ model.provider }}</span>
          <span v-if="model.release_date">{{ model.release_date }}</span>
        </div>
      </div>
    </header>

    <div class="model-badges">
      <span v-if="model.prompt_caching" class="cache-badge">
        <span class="cache-bolt" aria-hidden="true"></span>
        Prompt Caching
      </span>
    </div>

    <div v-if="model.note" class="model-note">{{ model.note }}</div>

    <div class="price-grid">
      <div>
        <span>输入</span>
        <strong>${{ formatPrice(model.pricing.input_per_mtok) }}</strong>
      </div>
      <div>
        <span>输出</span>
        <strong>${{ formatPrice(model.pricing.output_per_mtok) }}</strong>
      </div>
      <div>
        <span>缓存写入</span>
        <strong>${{ formatPrice(model.pricing.cache_write) }}</strong>
      </div>
      <div>
        <span>缓存读取</span>
        <strong>${{ formatPrice(model.pricing.cache_read) }}</strong>
      </div>
    </div>
    <div class="price-unit">USD 每百万 TOKENS</div>

    <section class="availability">
      <div class="availability-line">
        <span :class="['availability-dot', availabilityDotClass]" aria-hidden="true"></span>
        <strong>{{ availabilityDisplay }}</strong>
        <span>可用率</span>
      </div>
      <HeartbeatBar :beats="model.heartbeats || []" />
      <div class="time-axis">
        <span>90 min ago</span>
        <span>现在</span>
      </div>
    </section>

    <section class="channels">
      <button type="button" class="channels-toggle" @click="expanded = !expanded">
        <svg :class="{ 'is-open': expanded }" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M8 10l4 4 4-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <span>{{ channelCount }} 个公开渠道</span>
      </button>
      <div v-if="expanded" class="group-list">
        <GroupSection
          v-for="(g, idx) in model.groups"
          :key="`${g.name}-${idx}`"
          :group="g"
        />
      </div>
    </section>
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { StatusModel } from '@/types'
import HeartbeatBar from './HeartbeatBar.vue'
import GroupSection from './GroupSection.vue'

interface Props {
  model: StatusModel
}

const props = defineProps<Props>()

const expanded = ref(true)
const copied = ref(false)

const channelCount = computed(() => {
  const groups = props.model.groups ?? []
  return groups.reduce((sum, g) => sum + (g.channels?.length ?? 0), 0)
})

const hasNoData = computed(() => {
  const beats = props.model.heartbeats ?? []
  return beats.length === 0 || beats.every((beat) => beat.status === 'unknown')
})

const availabilityDisplay = computed(() => {
  if (hasNoData.value) return 'N/A'
  return `${(props.model.availability_pct ?? 0).toFixed(2)}%`
})

const availabilityDotClass = computed(() => {
  if (hasNoData.value) return 'is-empty'
  const pct = props.model.availability_pct ?? 0
  if (pct >= 99) return 'is-ok'
  if (pct >= 90) return 'is-degraded'
  return 'is-down'
})

function formatPrice(v: number): string {
  if (v == null || Number.isNaN(v)) return '0'
  return Number(v)
    .toFixed(4)
    .replace(/\.?0+$/, '')
}

async function copyName() {
  const name = props.model.name
  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(name)
    } else {
      const ta = document.createElement('textarea')
      ta.value = name
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    copied.value = false
  }
}
</script>

<style scoped>
.model-card {
  border: 1px solid rgba(244, 231, 200, 0.14);
  background:
    linear-gradient(180deg, rgba(245, 197, 66, 0.055), transparent 180px),
    rgba(12, 10, 7, 0.9);
  box-shadow: 0 22px 70px rgba(0, 0, 0, 0.3);
  padding: 22px;
  min-width: 0;
  overflow: hidden;
}

.model-head {
  display: flex;
  gap: 14px;
  min-width: 0;
}

.model-mark {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  flex: 0 0 auto;
  color: #ff7a2e;
}

.model-mark svg {
  width: 34px;
  height: 34px;
  fill: currentColor;
  filter: drop-shadow(0 0 18px rgba(255, 122, 46, 0.28));
}

.model-title {
  min-width: 0;
  flex: 1;
}

.model-name-row {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.model-name-row h2 {
  min-width: 0;
  color: #f4e7c8;
  font-size: 18px;
  line-height: 1.2;
  font-weight: 850;
  letter-spacing: 0;
  overflow-wrap: anywhere;
}

.copy-btn {
  display: inline-grid;
  place-items: center;
  width: 24px;
  height: 24px;
  flex: 0 0 auto;
  color: rgba(244, 231, 200, 0.46);
  border: 1px solid rgba(244, 231, 200, 0.12);
  background: rgba(244, 231, 200, 0.035);
  transition: color 0.18s ease, border-color 0.18s ease, background 0.18s ease;
}

.copy-btn:hover {
  color: #f5c542;
  border-color: rgba(245, 197, 66, 0.34);
  background: rgba(245, 197, 66, 0.08);
}

.copy-btn svg {
  width: 14px;
  height: 14px;
}

.model-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 7px;
  color: rgba(244, 231, 200, 0.5);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.14em;
}

.model-badges {
  min-height: 30px;
  margin-top: 18px;
}

.cache-badge {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: 1px solid rgba(34, 197, 94, 0.28);
  background: rgba(34, 197, 94, 0.1);
  color: #4ade80;
  padding: 7px 11px;
  font-size: 12px;
  font-weight: 800;
}

.cache-bolt {
  width: 8px;
  height: 14px;
  background: currentColor;
  clip-path: polygon(60% 0, 12% 55%, 46% 55%, 30% 100%, 88% 42%, 54% 42%);
}

.model-note {
  margin-top: 10px;
  padding: 12px 13px;
  background: rgba(244, 231, 200, 0.045);
  color: rgba(244, 231, 200, 0.5);
  font-size: 13px;
}

.price-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  margin-top: 18px;
  background: rgba(244, 231, 200, 0.1);
}

.price-grid > div {
  min-width: 0;
  padding: 14px 0;
  background: #0c0a07;
}

.price-grid span {
  display: block;
  color: rgba(244, 231, 200, 0.52);
  font-size: 12px;
}

.price-grid strong {
  display: block;
  margin-top: 6px;
  color: #f4e7c8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 24px;
  line-height: 1;
}

.price-unit {
  margin-top: 11px;
  padding-top: 12px;
  border-top: 1px solid rgba(244, 231, 200, 0.12);
  color: rgba(244, 231, 200, 0.48);
  text-align: right;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.18em;
}

.availability {
  margin-top: 18px;
  padding-top: 18px;
  border-top: 1px solid rgba(244, 231, 200, 0.12);
  min-width: 0;
  overflow: hidden;
}

.availability-line {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.availability-line strong {
  color: #f4e7c8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 24px;
  line-height: 1;
}

.availability-line span:last-child {
  color: rgba(244, 231, 200, 0.55);
  font-size: 13px;
}

.availability-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
}

.availability-dot.is-ok {
  background: #22c55e;
  box-shadow: 0 0 18px rgba(34, 197, 94, 0.48);
}

.availability-dot.is-empty {
  background: rgba(244, 231, 200, 0.26);
}

.availability-dot.is-degraded {
  background: #f5c542;
  box-shadow: 0 0 18px rgba(245, 197, 66, 0.4);
}

.availability-dot.is-down {
  background: #ef4444;
  box-shadow: 0 0 18px rgba(239, 68, 68, 0.42);
}

.time-axis {
  display: flex;
  justify-content: space-between;
  margin-top: 8px;
  color: rgba(244, 231, 200, 0.46);
  font-size: 11px;
}

.channels {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid rgba(244, 231, 200, 0.12);
  min-width: 0;
  overflow: hidden;
}

.channels-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: rgba(244, 231, 200, 0.66);
  font-size: 13px;
  font-weight: 700;
}

.channels-toggle:hover {
  color: #f5c542;
}

.channels-toggle svg {
  width: 18px;
  height: 18px;
  transform: rotate(-90deg);
  transition: transform 0.18s ease;
}

.channels-toggle svg.is-open {
  transform: rotate(0deg);
}

.group-list {
  margin-top: 12px;
  display: grid;
  gap: 12px;
  min-width: 0;
  overflow: hidden;
}
</style>
