<template>
  <section class="group-section">
    <header class="group-head">
      <span class="group-name">{{ group.name }}</span>
      <div class="group-load">
        <span>负载</span>
        <div class="load-track">
          <div class="load-fill" :style="{ width: loadWidth }" />
        </div>
        <strong>{{ loadPct }}%</strong>
      </div>
    </header>
    <div class="channel-list">
      <ChannelRow
        v-for="(ch, idx) in group.channels"
        :key="`${ch.name}-${idx}`"
        :channel="ch"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { StatusGroup } from '@/types'
import ChannelRow from './ChannelRow.vue'

interface Props {
  group: StatusGroup
}

const props = defineProps<Props>()

const loadPct = computed(() => {
  const v = props.group.load_pct ?? 0
  return Math.max(0, Math.min(100, Math.round(v)))
})

const loadWidth = computed(() => `${loadPct.value}%`)
</script>

<style scoped>
.group-section {
  min-width: 0;
  padding: 10px 0;
  overflow: hidden;
}

.group-head {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  margin-bottom: 8px;
}

.group-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: rgba(244, 231, 200, 0.62);
  font-size: 11px;
  font-weight: 850;
  letter-spacing: 0.14em;
}

.group-load {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: rgba(244, 231, 200, 0.48);
  font-size: 11px;
  font-weight: 700;
}

.load-track {
  width: 68px;
  height: 4px;
  overflow: hidden;
  background: rgba(244, 231, 200, 0.12);
}

.load-fill {
  height: 100%;
  background: linear-gradient(90deg, #22c55e, #f5c542);
}

.group-load strong {
  width: 34px;
  color: rgba(244, 231, 200, 0.68);
  text-align: right;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
}

.channel-list {
  display: grid;
  gap: 4px;
  min-width: 0;
  overflow: hidden;
}
</style>
