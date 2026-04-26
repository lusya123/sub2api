<template>
  <div :class="['heartbeat-bar', { 'is-compact': compact }]">
    <div
      v-for="(b, i) in normalizedBeats"
      :key="i"
      data-beat
      :data-status="b.status"
      :class="['heartbeat-beat', colorOf(b.status)]"
      :title="`${formatTs(b.ts)} · ${statusLabel(b.status)}`"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { StatusBeat, StatusBeatStatus } from '@/types'

interface Props {
  beats: StatusBeat[]
  compact?: boolean
}

const props = defineProps<Props>()

const MAX_BEATS = 90

const normalizedBeats = computed<StatusBeat[]>(() => {
  const src = props.beats ?? []
  // 截断：如果超过 90 个，保留最近的 90 个（数组末尾）
  const trimmed = src.length > MAX_BEATS ? src.slice(src.length - MAX_BEATS) : src
  // 不足 90 个前面补 unknown 占位
  if (trimmed.length >= MAX_BEATS) return trimmed
  const padCount = MAX_BEATS - trimmed.length
  const pad: StatusBeat[] = Array.from({ length: padCount }, () => ({
    ts: '',
    status: 'unknown' as StatusBeatStatus
  }))
  return [...pad, ...trimmed]
})

function colorOf(status: StatusBeatStatus): string {
  switch (status) {
    case 'ok':
      return 'is-ok'
    case 'degraded':
      return 'is-degraded'
    case 'down':
      return 'is-down'
    case 'unknown':
    default:
      return 'is-unknown'
  }
}

function statusLabel(status: StatusBeatStatus): string {
  switch (status) {
    case 'ok':
      return '正常'
    case 'degraded':
      return '降级'
    case 'down':
      return '异常'
    case 'unknown':
    default:
      return '无数据'
  }
}

function formatTs(ts: string): string {
  if (!ts) return '--:--'
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
}
</script>

<style scoped>
.heartbeat-bar {
  display: grid;
  grid-template-columns: repeat(90, minmax(0, 1fr));
  align-items: stretch;
  gap: 1px;
  width: 100%;
  max-width: 100%;
  height: 16px;
  min-width: 0;
  overflow: hidden;
}

.heartbeat-bar.is-compact {
  height: 20px;
}

.heartbeat-beat {
  min-width: 0;
  width: 100%;
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.heartbeat-beat:hover {
  opacity: 0.78;
  transform: translateY(-1px);
}

.heartbeat-beat.is-ok {
  background: #22c55e;
}

.heartbeat-beat.is-degraded {
  background: #f5c542;
}

.heartbeat-beat.is-down {
  background: #ef4444;
}

.heartbeat-beat.is-unknown {
  background: rgba(244, 231, 200, 0.1);
}
</style>
