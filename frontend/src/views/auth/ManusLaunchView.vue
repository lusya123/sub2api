<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50 px-4">
    <div class="w-full max-w-md rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
      <div class="mb-4 h-2 w-24 rounded-full bg-blue-500" />
      <h1 class="text-xl font-semibold text-gray-900">正在打开 Manus</h1>
      <p class="mt-2 text-sm text-gray-600">
        {{ message }}
      </p>
      <button
        v-if="error"
        class="mt-5 rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800"
        type="button"
        @click="goHome"
      >
        返回首页
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getAuthToken, getRefreshToken, getTokenExpiresAt } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const appStore = useAppStore()

const message = ref('正在同步登录状态，请稍候。')
const error = ref(false)

function parseAllowedOrigins(): Set<string> {
  const origins = new Set<string>()
  const rawValues = [
    appStore.cachedPublicSettings?.agent_page_url,
    import.meta.env.VITE_MANUS_BASE_URL,
    import.meta.env.VITE_MANUS_ALLOWED_REDIRECT_ORIGINS,
  ].filter(Boolean)

  for (const rawValue of rawValues) {
    for (const item of String(rawValue).split(',')) {
      const value = item.trim()
      if (!value) {
        continue
      }
      try {
        origins.add(new URL(value).origin)
      } catch {
        console.warn('Invalid Manus redirect origin:', value)
      }
    }
  }

  return origins
}

function isAllowedRedirect(url: URL): boolean {
  if (import.meta.env.DEV && ['localhost', '127.0.0.1', '::1'].includes(url.hostname)) {
    return true
  }

  return parseAllowedOrigins().has(url.origin)
}

function fail(text: string): void {
  error.value = true
  message.value = text
}

function goHome(): void {
  void router.push('/dashboard')
}

onMounted(async () => {
  if (!appStore.publicSettingsLoaded) {
    await appStore.fetchPublicSettings().catch(() => null)
  }

  const redirectURI = route.query.redirect_uri
  if (typeof redirectURI !== 'string' || !redirectURI) {
    fail('缺少 Manus 回跳地址。')
    return
  }

  let target: URL
  try {
    target = new URL(redirectURI)
  } catch {
    fail('Manus 回跳地址格式不正确。')
    return
  }

  if (!['http:', 'https:'].includes(target.protocol)) {
    fail('Manus 回跳地址协议不被允许。')
    return
  }

  if (!isAllowedRedirect(target)) {
    fail('Manus 回跳域名未被允许，请检查 VITE_MANUS_BASE_URL 或 VITE_MANUS_ALLOWED_REDIRECT_ORIGINS。')
    return
  }

  const accessToken = authStore.token || getAuthToken()
  if (!accessToken) {
    void router.replace({
      path: '/login',
      query: { redirect: route.fullPath },
    })
    return
  }

  const hashValue = target.hash.startsWith('#') ? target.hash.slice(1) : target.hash
  const params = new URLSearchParams(hashValue)
  params.set('access_token', accessToken)
  params.set('token_type', 'Bearer')

  const refreshToken = getRefreshToken()
  if (refreshToken) {
    params.set('refresh_token', refreshToken)
  }

  const expiresAt = getTokenExpiresAt()
  if (expiresAt) {
    params.set('expires_in', String(Math.max(0, Math.floor((expiresAt - Date.now()) / 1000))))
  }

  target.hash = params.toString()
  window.location.replace(target.toString())
})
</script>
