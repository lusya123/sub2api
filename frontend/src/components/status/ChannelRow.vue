<template>
  <div class="channel-row">
    <div class="channel-name">{{ channel.name }}</div>
    <div class="channel-beats">
      <HeartbeatBar :beats="channel.heartbeats || []" compact />
    </div>
    <div :class="['channel-pct', pctClass]">
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
  return !hb || hb.length === 0 || hb.every((beat) => beat.status === 'unknown')
})

const pctDisplay = computed(() => {
  if (hasNoData.value) return 'N/A'
  const pct = props.channel.availability_pct ?? 0
  return `${pct.toFixed(2)}%`
})

const pctClass = computed(() => {
  if (hasNoData.value) return 'is-empty'
  const pct = props.channel.availability_pct ?? 0
  if (pct >= 99) return 'is-ok'
  if (pct >= 90) return 'is-degraded'
  return 'is-down'
})
</script>

<style scoped>
.channel-row {
  display: grid;
  grid-template-columns: minmax(64px, 0.55fr) minmax(0, 1fr) minmax(58px, auto);
  gap: 8px;
  align-items: center;
  min-width: 0;
  min-height: 28px;
  padding: 4px 0;
  overflow: hidden;
}

.channel-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgba(244, 231, 200, 0.72);
  font-size: 12px;
  font-weight: 800;
}

.channel-beats {
  min-width: 0;
  overflow: hidden;
}

.channel-pct {
  min-width: 0;
  overflow: hidden;
  text-align: right;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  font-weight: 850;
}

.channel-pct.is-empty {
  color: rgba(244, 231, 200, 0.42);
}

.channel-pct.is-ok {
  color: #22c55e;
}

.channel-pct.is-degraded {
  color: #f5c542;
}

.channel-pct.is-down {
  color: #ef4444;
}

@media (max-width: 560px) {
  .channel-row {
    grid-template-columns: minmax(0, 1fr) 92px;
    gap: 6px 10px;
    padding: 6px 0 8px;
  }

  .channel-pct {
    font-size: 11px;
  }

  .channel-beats {
    grid-column: 1 / -1;
    grid-row: 2;
  }
}
</style>
