import { defineNuxtPlugin, useRouter, useState } from '#app'
import {
  QueryClient,
  VueQueryPlugin,
  dehydrate,
  hydrate,
  type DehydratedState,
} from '@tanstack/vue-query'
import { createLoginRedirectCoordinator } from '@/lib/authNavigation'
import { subscribeToBackendSessionInvalidation } from '@/lib/backendClient'

export default defineNuxtPlugin((nuxtApp) => {
  const vueQueryState = useState<DehydratedState | null>('vue-query', () => null)
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 60_000,
        refetchOnWindowFocus: false,
        retry: import.meta.server ? 0 : 1,
      },
    },
  })

  nuxtApp.vueApp.use(VueQueryPlugin, { queryClient })

  if (import.meta.server) {
    nuxtApp.hooks.hook('app:rendered', () => {
      vueQueryState.value = dehydrate(queryClient)
    })
  }

  if (import.meta.client) {
    const router = useRouter()
    const redirectToLogin = createLoginRedirectCoordinator(async (location) => {
      queryClient.clear()
      await router.replace(location)
    })
    const unsubscribe = subscribeToBackendSessionInvalidation(() => {
      if (router.currentRoute.value.path === '/login') return
      void redirectToLogin(router.currentRoute.value.fullPath)
    })
    nuxtApp.vueApp.onUnmount(unsubscribe)
    import.meta.hot?.dispose(unsubscribe)

    nuxtApp.hooks.hook('app:created', () => {
      if (vueQueryState.value) hydrate(queryClient, vueQueryState.value)
    })
  }
})
