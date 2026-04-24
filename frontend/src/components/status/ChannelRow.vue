<template>
  <div class="grid grid-cols-[64px_1fr_64px] items-center gap-3 py-1">
    <div class="text-xs uppercase tracking-wide text-neutral-500 truncate">
      {{ channel.name }}
    </div>
    <div class="min-w-0">
      <HeartbeatBar :beats="channel.heartbeats || []" />
    </div>
    <div :class="['font-mono text-xs text-right', pctClass]">
      {{ pctDisplay }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { StatusChannel } from '@/types'
import HeartbeatBar from './HeartbeatBar.vue'

interface Props {
  channel: StatusChannel
}

const props = defineProps<Props>()

const hasNoData = computed(() => {
  const hb = props.channel.heartbeats
  return (!hb || hb.length === 0) && (props.channel.availability_pct ?? 0) === 0
})

const pctDisplay = computed(() => {
  if (hasNoData.value) return 'N/A'
  const pct = props.channel.availability_pct ?? 0
  return `${pct.toFixed(2)}%`
})

const pctClass = computed(() => {
  if (hasNoData.value) return 'text-neutral-500'
  const pct = props.channel.availability_pct ?? 0
  if (pct >= 99) return 'text-green-400'
  if (pct >= 90) return 'text-orange-400'
  return 'text-red-400'
})
</script>
