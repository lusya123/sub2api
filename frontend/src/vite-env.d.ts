/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string
  readonly VITE_PUBLIC_API_BASE_URL: string
  readonly VITE_MANUS_BASE_URL?: string
  readonly VITE_MANUS_ALLOWED_REDIRECT_ORIGINS?: string
  readonly VITE_MODEL_MARKETPLACE_USD_CNY_RATE?: string
  readonly BASE_URL: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
