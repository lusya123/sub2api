<template>
  <div class="flex gap-[2px] h-4 items-stretch">
    <div
      v-for="(b, i) in normalizedBeats"
      :key="i"
      data-beat
      :data-status="b.status"
      :class="[
        'flex-1 min-w-[2px] rounded-[1px] transition-opacity hover:opacity-80',
        colorOf(b.status)
      ]"
      :title="`${formatTs(b.ts)} · ${statusLabel(b.status)}`"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { StatusBeat, StatusBeatStatus } from '@/types'

interface Props {
  beats: StatusBeat[]
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
      return 'bg-green-500'
    case 'degraded':
      return 'bg-orange-500'
    case 'down':
      return 'bg-red-500'
    case 'unknown':
    default:
      return 'bg-neutral-700'
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
