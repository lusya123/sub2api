import { computed, ref, watch } from 'vue'
import { keysAPI } from '@/api/keys'
import { useAuthStore } from '@/stores/auth'
import type { ApiKey } from '@/types'

const loaded = ref(false)
const loading = ref(false)
const hasAllowedBatchImageKey = ref(false)
let accessOwnerID: number | null = null
let loadGeneration = 0
let pendingLoad: { ownerID: number; generation: number; force: boolean; promise: Promise<boolean> } | null = null
const pageSize = 100

function keyAllowsBatchImage(key: ApiKey): boolean {
  return (
    key.status === 'active' &&
    key.group?.platform === 'gemini' &&
    key.group?.allow_batch_image_generation === true
  )
}

async function loadBatchImageAccess(force = false): Promise<boolean> {
  const authStore = useAuthStore()
  const ownerID = authStore.isAuthenticated ? (authStore.user?.id ?? null) : null
  resetBatchImageAccessOwner(ownerID)

  if (ownerID === null) {
    loaded.value = true
    hasAllowedBatchImageKey.value = false
    return false
  }

  // Ensure calls can share any current request. Explicit refreshes only share
  // another refresh: a pre-mutation ensure may contain stale server state and
  // must be superseded after a successful key/group mutation.
  if (pendingLoad?.ownerID === ownerID && (!force || pendingLoad.force)) {
    return pendingLoad.promise
  }

  if (loaded.value && !force) {
    return hasAllowedBatchImageKey.value
  }

  if (force) {
    loaded.value = false
    // Keep the last known value visible while revalidating so persistent
    // navigation does not flicker. A successful response replaces it atomically.
  }

  const generation = ++loadGeneration
  loading.value = true
  const promise = (async () => {
    let page = 1
    while (true) {
      const response = await keysAPI.list(page, pageSize, {
        status: 'active',
        sort_by: 'created_at',
        sort_order: 'desc'
      })

      if (accessOwnerID !== ownerID || loadGeneration !== generation) {
        return false
      }

      if ((response.items || []).some(keyAllowsBatchImage)) {
        hasAllowedBatchImageKey.value = true
        loaded.value = true
        return true
      }

      if (page >= response.pages || (response.items || []).length === 0) {
        hasAllowedBatchImageKey.value = false
        loaded.value = true
        return false
      }

      page += 1
    }
  })()
    .catch(() => {
      if (accessOwnerID === ownerID && loadGeneration === generation) {
        // Do not cache a transient request failure as a definitive denial.
        // Keep the last known value and let a later navigation retry.
        loaded.value = false
      }
      return false
    })
    .finally(() => {
      if (pendingLoad?.ownerID === ownerID && pendingLoad.generation === generation) {
        loading.value = false
        pendingLoad = null
      }
    })

  pendingLoad = { ownerID, generation, force, promise }
  return promise
}

function resetBatchImageAccessOwner(ownerID: number | null): void {
  if (accessOwnerID === ownerID) return

  accessOwnerID = ownerID
  loadGeneration += 1
  pendingLoad = null
  loaded.value = false
  loading.value = false
  hasAllowedBatchImageKey.value = false
}

export function useBatchImageAccess() {
  const authStore = useAuthStore()
  watch(
    [() => authStore.isAuthenticated, () => authStore.user?.id],
    ([isAuthenticated, userID]) => {
      resetBatchImageAccessOwner(isAuthenticated ? (userID ?? null) : null)
    },
    { immediate: true },
  )

  const canUseBatchImage = computed(() => hasAllowedBatchImageKey.value)
  const ensureBatchImageAccess = () => loadBatchImageAccess(false)
  const refreshBatchImageAccess = () => loadBatchImageAccess(true)

  return {
    canUseBatchImage,
    batchImageAccessLoaded: computed(() => loaded.value),
    batchImageAccessLoading: computed(() => loading.value),
    ensureBatchImageAccess,
    refreshBatchImageAccess,
  }
}
