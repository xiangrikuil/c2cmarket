import { defineNuxtPlugin, useRuntimeConfig } from '#app'
import { setBackendRuntimeConfig } from '@/lib/backendClient'

export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig()
  setBackendRuntimeConfig({
    apiMode: String(config.public.apiMode ?? ''),
    apiBaseUrl: import.meta.server
      ? String(config.apiBaseUrl ?? '')
      : String(config.public.apiBaseUrl ?? ''),
  })
})
