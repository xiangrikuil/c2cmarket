import { defineNuxtPlugin } from '#app'
import { initializeAppTheme } from '@/theme/appThemes'
import { installUmamiScript } from '@/lib/umamiLoader'

export default defineNuxtPlugin(() => {
  initializeAppTheme()
  installUmamiScript()
})
