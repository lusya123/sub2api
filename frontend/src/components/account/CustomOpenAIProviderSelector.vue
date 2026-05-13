<template>
  <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700/40">
    <div class="flex items-start justify-between gap-4">
      <div>
        <label class="input-label mb-0">自定义厂商</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          选择模型广场里的厂商 Logo，账号仍按 OpenAI 兼容接口调用。
        </p>
      </div>
      <button
        type="button"
        @click="$emit('update:enabled', !enabled)"
        :class="[
          'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2',
          enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
        ]"
      >
        <span
          :class="[
            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
            enabled ? 'translate-x-5' : 'translate-x-0'
          ]"
        />
      </button>
    </div>

    <div v-if="enabled" class="mt-4 space-y-3">
      <div>
        <label class="input-label">厂商</label>
        <div class="relative">
          <div class="pointer-events-none absolute inset-y-0 left-3 flex items-center">
            <ProviderIcon :provider="provider" :size="20" />
          </div>
          <select :value="provider" class="input pl-10" @change="handleProviderChange">
            <option v-for="option in customProviderOptions" :key="option.value" :value="option.value">
              {{ option.label }}
            </option>
          </select>
        </div>
        <p class="input-hint">
          会保存为 {{ selectedCustomProviderLabel }} 账号，并按所选厂商推荐相关模型。
        </p>
      </div>

      <button
        v-if="suggestedCustomProviderBaseUrl"
        type="button"
        @click="$emit('apply-suggested-base-url')"
        class="inline-flex items-center rounded-lg border border-primary-200 px-3 py-2 text-sm font-medium text-primary-700 transition-colors hover:bg-primary-50 dark:border-primary-700 dark:text-primary-300 dark:hover:bg-primary-900/20"
      >
        使用推荐 URL：{{ suggestedCustomProviderBaseUrl }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { toRef } from 'vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { useCustomOpenAIProvider } from '@/composables/useCustomOpenAIProvider'

const props = defineProps<{
  enabled: boolean
  provider: string
}>()

const emit = defineEmits<{
  'update:enabled': [value: boolean]
  'update:provider': [value: string]
  'apply-suggested-base-url': []
}>()

const {
  customProviderOptions,
  selectedCustomProviderLabel,
  suggestedCustomProviderBaseUrl
} = useCustomOpenAIProvider(toRef(props, 'provider'))

const handleProviderChange = (event: Event) => {
  emit('update:provider', (event.target as HTMLSelectElement).value)
}
</script>
