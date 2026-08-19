import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const packageJson = JSON.parse(source('../../../package.json')) as {
  dependencies: Record<string, string>
}
const nuxtConfig = source('../../../nuxt.config.ts')
const styles = source('../../styles.css')
const motion = source('../motion.ts')
const softTable = source('../../components/market/SoftTable.vue')
const purchaseDialog = source('../../components/api-service-detail/PurchaseConfirmDialog.vue')
const carpoolDetail = source('../../pages/CarpoolDetailPage.vue')
const myApiOrders = source('../../pages/MyApiOrdersPage.vue')
const merchantApiOrders = source('../../pages/MerchantApiOrdersPage.vue')
const myRides = source('../../pages/MyRidesPage.vue')
const merchantApplications = source('../../pages/MerchantCarpoolApplicationsPage.vue')
const apiOrderDetail = source('../../pages/ApiPurchaseOrderDetailPage.vue')
const rideDetail = source('../../pages/CarpoolApplicationDetailPage.vue')
const carpoolPublish = source('../../pages/CarpoolPublishPage.vue')
const apiPublish = source('../../pages/ApiServicePublishPage.vue')

describe('transaction motion feedback', () => {
  it('uses one narrow runtime library and completes the existing shadcn animation contract', () => {
    expect(packageJson.dependencies['@formkit/auto-animate']).toBe('^0.10.0')
    expect(packageJson.dependencies['tw-animate-css']).toBe('^1.4.0')
    expect(packageJson.dependencies).not.toHaveProperty('motion-v')
    expect(packageJson.dependencies).not.toHaveProperty('@vueuse/motion')
    expect(packageJson.dependencies).not.toHaveProperty('gsap')
    expect(nuxtConfig).toContain("'@formkit/auto-animate/nuxt'")
    expect(styles).toContain('@import "tw-animate-css";')
  })

  it('centralizes functional timing and honors reduced motion', () => {
    expect(motion).toContain('satisfies Partial<AutoAnimateOptions>')
    expect(motion).toContain('duration: 180')
    expect(motion).not.toContain('disrespectUserMotionPreference')
    expect(styles).toContain('--motion-duration-base: 180ms')
    expect(styles).toContain('@media (prefers-reduced-motion: reduce)')
    expect(styles).toContain('transition-duration: 0.01ms !important')
  })

  it('keeps shared tables opt-in while animating the four transaction queues', () => {
    expect(softTable).toContain('animateRows?: boolean')
    expect(softTable).toContain('<TableBody v-if="animateRows" v-auto-animate="functionalMotion">')
    expect(softTable).toContain('<TableBody v-else>')
    expect(myApiOrders).toContain('v-auto-animate="functionalMotion" class="my-transaction-list"')
    expect(myRides).toContain('v-auto-animate="functionalMotion" class="my-transaction-list"')
    expect(merchantApiOrders).toContain('<SoftTable v-else animate-rows')
    expect(merchantApplications).toContain('<SoftTable v-else animate-rows')
  })

  it('uses official transaction dialogs instead of page-local overlays', () => {
    expect(purchaseDialog).toContain('<Dialog :open="open" @update:open="updateOpen">')
    expect(purchaseDialog).toContain('<DialogContent')
    expect(purchaseDialog).toContain('<Checkbox v-model="acknowledged"')
    expect(purchaseDialog).not.toContain('<Teleport')
    expect(carpoolDetail).toContain('<Dialog v-if="canApplyToCarpool" v-model:open="applyDialogOpen">')
    expect(carpoolDetail).toContain('<Checkbox v-model="rulesAccepted"')
    expect(carpoolDetail).not.toContain('v-if="applyDialogOpen" class="fixed inset-0')
  })

  it('limits status animation to local action and timeline regions', () => {
    expect(apiOrderDetail.match(/v-auto-animate="functionalMotion"/g)?.length).toBeGreaterThanOrEqual(4)
    expect(apiOrderDetail).toContain('<StatusBadge :key="`${order.status}-${order.disputeStatus}`"')
    expect(apiOrderDetail).toMatch(/class="[^"]*c2c-motion-state[^"]*h-8[^"]*w-8[^"]*"/)
    expect(rideDetail.match(/v-auto-animate="functionalMotion"/g)?.length).toBeGreaterThanOrEqual(4)
    expect(rideDetail).toContain('<Badge :key="application.status"')
    expect(rideDetail).toContain(':key="application.status" class="mt-4 grid gap-2"')
  })

  it('shows creation and publish success before cache refresh work', () => {
    const applicationSuccess = carpoolDetail.indexOf('toast.success(`申请已提交')
    const applicationRefresh = carpoolDetail.indexOf('await Promise.all([', applicationSuccess)
    expect(applicationSuccess).toBeGreaterThan(-1)
    expect(applicationRefresh).toBeGreaterThan(applicationSuccess)

    const carpoolSuccess = carpoolPublish.indexOf("toast.success(isEditMode.value ? '车源修改已提交审核。' : '车源已提交。')")
    const carpoolRefresh = carpoolPublish.indexOf('await invalidateCarpoolPublishQueries()', carpoolSuccess)
    expect(carpoolSuccess).toBeGreaterThan(-1)
    expect(carpoolRefresh).toBeGreaterThan(carpoolSuccess)

    const apiSuccess = apiPublish.indexOf('toast.success(isFixedPackageMode.value')
    const apiRefresh = apiPublish.indexOf('await invalidateApiServicePublishQueries()', apiSuccess)
    expect(apiSuccess).toBeGreaterThan(-1)
    expect(apiRefresh).toBeGreaterThan(apiSuccess)
  })
})
