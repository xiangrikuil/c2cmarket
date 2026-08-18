import { defineNuxtPlugin, useRouter, useRuntimeConfig } from '#app'
import { setAnalyticsRuntimeConfig, trackAnalytics } from '@/lib/analytics'
import { initializeAppTheme } from '@/theme/appThemes'
import { buildUmamiScriptConfig, installUmamiScript } from '@/lib/umamiLoader'
import { captureRegistrationAttribution, clearRegistrationAttribution } from '@/lib/registrationAttribution'
import { clearReferralCapture } from '@/lib/referralCapture'
import { wechatOnboardingRoute } from '@/lib/authNavigation'

export default defineNuxtPlugin((nuxtApp) => {
  const config = useRuntimeConfig().public
  const router = useRouter()
  const umamiConfig = buildUmamiScriptConfig({
    enabled: config.umamiEnabled,
    scriptUrl: config.umamiScriptUrl,
    websiteId: config.umamiWebsiteId,
    domains: config.umamiDomains,
    hostUrl: config.umamiHostUrl,
  })

  initializeAppTheme()
  setAnalyticsRuntimeConfig({
    enabled: config.umamiEnabled,
    debug: config.umamiDebug,
  })
  captureRegistrationAttribution()
  installUmamiScript(umamiConfig)

  let lastTrackedPath = ''
  const consumedAuthOutcomes = new Set<string>()
  const processRoute = (route: typeof router.currentRoute.value) => {
    if (route.path !== lastTrackedPath) {
      lastTrackedPath = route.path
      trackAnalytics('normalized_page_view', { path: route.path })
    }

    const rawOutcome = Array.isArray(route.query.authOutcome)
      ? route.query.authOutcome[0]
      : route.query.authOutcome
		if (rawOutcome === 'restricted_business') {
			void router.replace({ path: '/restricted-business' })
			return
		}
		if ((rawOutcome !== 'registered' && rawOutcome !== 'logged_in' && rawOutcome !== 'admin_reauthenticated') || consumedAuthOutcomes.has(route.fullPath)) return

    consumedAuthOutcomes.add(route.fullPath)
		if (rawOutcome !== 'admin_reauthenticated') trackAnalytics(rawOutcome === 'registered' ? 'registration_success' : 'login_success', {
      method: 'oauth_linux_do',
      source_route: route.path,
		})
    clearRegistrationAttribution()
    clearReferralCapture()

    const query = { ...route.query }
    delete query.authOutcome
    const cleanRoute = { path: route.path, query, hash: route.hash }
    if (rawOutcome === 'registered') {
      void router.replace(wechatOnboardingRoute(router.resolve(cleanRoute).fullPath))
      return
    }
    void router.replace(cleanRoute)
  }

  const removeAfterEach = router.afterEach(to => processRoute(to))
  nuxtApp.hooks.hook('app:mounted', () => processRoute(router.currentRoute.value))
  nuxtApp.vueApp.onUnmount(removeAfterEach)
  import.meta.hot?.dispose(removeAfterEach)
})
