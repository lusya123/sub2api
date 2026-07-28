import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick, reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

const { list } = vi.hoisted(() => ({
  list: vi.fn(),
}))

const authState = reactive({
  isAuthenticated: true,
  user: { id: 1 },
})

vi.mock('@/api/keys', () => ({
  keysAPI: { list },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

function response(items: unknown[]) {
  return {
    items,
    total: items.length,
    page: 1,
    page_size: 100,
    pages: items.length > 0 ? 1 : 0,
  }
}

function allowedKey(id: number) {
  return {
    id,
    status: 'active',
    group: {
      platform: 'gemini',
      allow_batch_image_generation: true,
    },
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

describe('useBatchImageAccess', () => {
  let nextUserID = 100

  beforeEach(() => {
    list.mockReset()
    authState.isAuthenticated = true
    authState.user = { id: nextUserID++ }
  })

  it('caches ensure calls but explicitly refreshes permission changes for the same user', async () => {
    let access!: ReturnType<typeof useBatchImageAccess>
    const wrapper = mount(defineComponent({
      setup() {
        access = useBatchImageAccess()
        return () => null
      },
    }))

    list.mockResolvedValueOnce(response([allowedKey(1)]))
    await expect(access.ensureBatchImageAccess()).resolves.toBe(true)
    expect(access.canUseBatchImage.value).toBe(true)

    await expect(access.ensureBatchImageAccess()).resolves.toBe(true)
    expect(list).toHaveBeenCalledTimes(1)

    const refreshedPermission = deferred<ReturnType<typeof response>>()
    list.mockImplementationOnce(() => refreshedPermission.promise)
    const refresh = access.refreshBatchImageAccess()
    expect(access.canUseBatchImage.value).toBe(true)
    refreshedPermission.resolve(response([]))
    await expect(refresh).resolves.toBe(false)
    expect(access.canUseBatchImage.value).toBe(false)
    expect(list).toHaveBeenCalledTimes(2)

    authState.user = { id: nextUserID++ }
    await nextTick()
    expect(access.canUseBatchImage.value).toBe(false)
    expect(access.batchImageAccessLoaded.value).toBe(false)

    list.mockResolvedValueOnce(response([allowedKey(2)]))
    await expect(access.ensureBatchImageAccess()).resolves.toBe(true)
    expect(access.canUseBatchImage.value).toBe(true)
    expect(list).toHaveBeenCalledTimes(3)

    wrapper.unmount()
  })

  it('does not let a previous user request overwrite the next user cache', async () => {
    let access!: ReturnType<typeof useBatchImageAccess>
    const wrapper = mount(defineComponent({
      setup() {
        access = useBatchImageAccess()
        return () => null
      },
    }))
    const firstUserResponse = deferred<ReturnType<typeof response>>()
    list.mockImplementationOnce(() => firstUserResponse.promise)

    const firstUserLoad = access.ensureBatchImageAccess()
    await flushPromises()

    authState.user = { id: nextUserID++ }
    await nextTick()
    list.mockResolvedValueOnce(response([]))
    await expect(access.ensureBatchImageAccess()).resolves.toBe(false)

    firstUserResponse.resolve(response([allowedKey(2)]))
    await expect(firstUserLoad).resolves.toBe(false)
    expect(access.canUseBatchImage.value).toBe(false)
    expect(access.batchImageAccessLoaded.value).toBe(true)

    wrapper.unmount()
  })

  it('does not cache a transient API failure as a permanent denial', async () => {
    let access!: ReturnType<typeof useBatchImageAccess>
    const wrapper = mount(defineComponent({
      setup() {
        access = useBatchImageAccess()
        return () => null
      },
    }))

    list.mockRejectedValueOnce(new Error('temporary network failure'))
    await expect(access.ensureBatchImageAccess()).resolves.toBe(false)
    expect(access.batchImageAccessLoaded.value).toBe(false)

    list.mockResolvedValueOnce(response([allowedKey(3)]))
    await expect(access.ensureBatchImageAccess()).resolves.toBe(true)
    expect(access.canUseBatchImage.value).toBe(true)
    expect(list).toHaveBeenCalledTimes(2)

    wrapper.unmount()
  })

  it('deduplicates concurrent explicit refreshes for the same user', async () => {
    let access!: ReturnType<typeof useBatchImageAccess>
    const wrapper = mount(defineComponent({
      setup() {
        access = useBatchImageAccess()
        return () => null
      },
    }))
    const pendingResponse = deferred<ReturnType<typeof response>>()
    list.mockImplementationOnce(() => pendingResponse.promise)

    const first = access.refreshBatchImageAccess()
    const second = access.refreshBatchImageAccess()
    expect(list).toHaveBeenCalledTimes(1)

    pendingResponse.resolve(response([allowedKey(4)]))
    await expect(Promise.all([first, second])).resolves.toEqual([true, true])
    expect(list).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('supersedes an in-flight ensure when a post-mutation refresh starts', async () => {
    let access!: ReturnType<typeof useBatchImageAccess>
    const wrapper = mount(defineComponent({
      setup() {
        access = useBatchImageAccess()
        return () => null
      },
    }))
    const staleEnsureResponse = deferred<ReturnType<typeof response>>()
    list.mockImplementationOnce(() => staleEnsureResponse.promise)

    const staleEnsure = access.ensureBatchImageAccess()
    await flushPromises()

    // Simulate a key creation while the pre-mutation ensure request is still
    // in flight. The explicit refresh must issue a second, authoritative read.
    list.mockResolvedValueOnce(response([allowedKey(5)]))
    await expect(access.refreshBatchImageAccess()).resolves.toBe(true)
    expect(list).toHaveBeenCalledTimes(2)
    expect(access.canUseBatchImage.value).toBe(true)

    staleEnsureResponse.resolve(response([]))
    await expect(staleEnsure).resolves.toBe(false)
    expect(access.canUseBatchImage.value).toBe(true)
    expect(access.batchImageAccessLoaded.value).toBe(true)

    wrapper.unmount()
  })
})
