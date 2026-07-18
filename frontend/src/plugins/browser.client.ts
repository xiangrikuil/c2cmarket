import { defineNuxtPlugin, useRuntimeConfig } from '#app'
import { setAnalyticsRuntimeConfig } from '@/lib/analytics'
import { initializeAppTheme } from '@/theme/appThemes'
import { buildUmamiScriptConfig, installUmamiScript } from '@/lib/umamiLoader'

export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig().public
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
  installUmamiScript(umamiConfig)
})
