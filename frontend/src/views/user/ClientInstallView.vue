<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
      </div>

      <EmptyState
        v-else-if="supportedKeys.length === 0"
        :title="t('clientInstallPage.noKeysTitle')"
        :description="t('clientInstallPage.noKeysDescription')"
      />

      <div v-else class="grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
        <section class="card p-4">
          <div class="space-y-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ t('clientInstallPage.selectKeyTitle') }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('clientInstallPage.selectKeyDescription') }}
              </p>
            </div>

            <SearchInput
              v-model="search"
              :placeholder="t('clientInstallPage.searchPlaceholder')"
            />

            <div class="max-h-[70vh] space-y-2 overflow-y-auto pr-1">
              <button
                v-for="keyItem in filteredKeys"
                :key="keyItem.id"
                type="button"
                :class="[
                  'w-full rounded-xl border p-4 text-left transition-colors',
                  selectedKey?.id === keyItem.id
                    ? 'border-primary-500 bg-primary-50 dark:border-primary-500 dark:bg-primary-900/20'
                    : 'border-gray-200 hover:border-primary-300 dark:border-dark-600 dark:hover:border-primary-600'
                ]"
                @click="selectedKey = keyItem"
              >
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-medium text-gray-900 dark:text-white">
                      {{ keyItem.name }}
                    </p>
                    <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                      {{ maskKey(keyItem.key) }}
                    </p>
                  </div>
                  <GroupBadge
                    :name="keyItem.group?.name || `#${keyItem.group_id}`"
                    :platform="keyItem.group?.platform || 'anthropic'"
                    :subscription-type="keyItem.group?.subscription_type"
                  />
                </div>
              </button>
            </div>
          </div>
        </section>

        <section class="card p-5">
          <div v-if="selectedKey" class="space-y-4">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                {{ selectedKey.name }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ t('clientInstallPage.panelDescription', { group: selectedKey.group?.name || `#${selectedKey.group_id}` }) }}
              </p>
            </div>

            <ClientInstallPanel
              :api-key="selectedKey.key"
              :base-url="publicSettings?.api_base_url || ''"
              :platform="selectedKey.group?.platform || null"
            />
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { authAPI, keysAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import ClientInstallPanel from '@/components/keys/ClientInstallPanel.vue'
import type { ApiKey, PublicSettings } from '@/types'

const { t } = useI18n()
const route = useRoute()

const loading = ref(false)
const search = ref('')
const apiKeys = ref<ApiKey[]>([])
const selectedKey = ref<ApiKey | null>(null)
const publicSettings = ref<PublicSettings | null>(null)

const supportedKeys = computed(() =>
  apiKeys.value.filter((item) => item.group?.platform === 'anthropic' || item.group?.platform === 'antigravity')
)

const filteredKeys = computed(() => {
  const query = search.value.trim().toLowerCase()
  if (!query) return supportedKeys.value
  return supportedKeys.value.filter((item) => {
    const name = item.name.toLowerCase()
    const groupName = (item.group?.name || '').toLowerCase()
    const keyText = item.key.toLowerCase()
    return name.includes(query) || groupName.includes(query) || keyText.includes(query)
  })
})

function maskKey(value: string): string {
  if (value.length <= 12) return value
  return `${value.slice(0, 8)}...${value.slice(-4)}`
}

function syncSelectedKey() {
  const raw = route.query.keyId
  const id = typeof raw === 'string' ? Number(raw) : Number(Array.isArray(raw) ? raw[0] : 0)
  if (Number.isFinite(id) && id > 0) {
    const matched = supportedKeys.value.find((item) => item.id === id)
    if (matched) {
      selectedKey.value = matched
      return
    }
  }

  if (!selectedKey.value && supportedKeys.value.length > 0) {
    selectedKey.value = supportedKeys.value[0]
  }
}

async function loadData() {
  loading.value = true
  try {
    const [keys, settings] = await Promise.all([
      keysAPI.list(1, 200),
      authAPI.getPublicSettings()
    ])
    apiKeys.value = keys.items || []
    publicSettings.value = settings
    syncSelectedKey()
  } finally {
    loading.value = false
  }
}

watch(() => route.query.keyId, () => {
  syncSelectedKey()
})

onMounted(() => {
  loadData()
})
</script>
