<template>
  <AppLayout>
    <div class="flex min-h-[55vh] items-center justify-center px-4 py-10">
      <section class="w-full max-w-md rounded-lg border border-gray-200 bg-white p-6 text-center shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-lg bg-sky-100 text-sky-600 dark:bg-sky-900/30 dark:text-sky-300">
          <Icon name="chat" size="lg" />
        </div>
        <h1 class="mt-4 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('chatLaunch.title') }}
        </h1>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
          {{ isOpening ? t('chatLaunch.opening') : t('chatLaunch.description') }}
        </p>
        <button
          type="button"
          class="btn-primary mt-5 w-full justify-center"
          :disabled="isOpening"
          @click="openChat"
        >
          {{ isOpening ? t('chatLaunch.openingButton') : t('chatLaunch.openButton') }}
        </button>
        <p v-if="hasError" class="mt-3 text-sm text-red-600 dark:text-red-400">
          {{ t('chatLaunch.failed') }}
        </p>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { chatAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()
const isOpening = ref(false)
const hasError = ref(false)

async function openChat() {
  if (isOpening.value) return
  isOpening.value = true
  hasError.value = false

  try {
    const { url } = await chatAPI.launch()
    if (!url) throw new Error('Missing chat launch URL')
    window.location.assign(url)
  } catch {
    isOpening.value = false
    hasError.value = true
    appStore.showError(t('dashboard.chatOpenFailed'))
  }
}

onMounted(openChat)
</script>
