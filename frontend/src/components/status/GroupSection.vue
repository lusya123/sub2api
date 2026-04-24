<template>
  <section class="py-2">
    <header class="flex items-center justify-between mb-1.5">
      <span class="text-[10px] uppercase tracking-widest text-neutral-500">
        {{ group.name }}
      </span>
      <div class="flex items-center gap-2">
        <span class="text-[10px] uppercase tracking-wide text-neutral-500">负载</span>
        <div class="w-16 h-[3px] rounded-full bg-neutral-800 overflow-hidden">
          <div
            class="h-full bg-green-500 rounded-full"
            :style="{ width: loadWidth }"
          />
        </div>
        <span class="font-mono text-xs text-neutral-300">
          {{ loadPct }}%
        </span>
      </div>
    </header>
    <div class="space-y-0.5">
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
