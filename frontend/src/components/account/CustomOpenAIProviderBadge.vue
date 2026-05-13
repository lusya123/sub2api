<template>
  <span
    v-if="provider"
    class="inline-flex items-center gap-1 rounded bg-sky-50 px-1.5 py-0.5 text-[10px] font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-300"
  >
    <ProviderIcon :provider="provider" :size="12" />
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'

const props = defineProps<{
  platform: string
  type: string
  extra?: Record<string, unknown> | null
}>()

const provider = computed(() => {
  if (props.platform !== 'openai' || props.type !== 'apikey') return ''
  return typeof props.extra?.custom_provider === 'string' ? props.extra.custom_provider : ''
})

const label = computed(() => {
  if (!provider.value) return ''
  return typeof props.extra?.custom_provider_label === 'string' ? props.extra.custom_provider_label : provider.value
})
</script>
