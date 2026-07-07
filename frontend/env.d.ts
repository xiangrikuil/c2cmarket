/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_UMAMI_ENABLED?: string
  readonly VITE_UMAMI_SCRIPT_URL?: string
  readonly VITE_UMAMI_WEBSITE_ID?: string
  readonly VITE_UMAMI_DOMAINS?: string
  readonly VITE_UMAMI_HOST_URL?: string
  readonly VITE_UMAMI_DEBUG?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
