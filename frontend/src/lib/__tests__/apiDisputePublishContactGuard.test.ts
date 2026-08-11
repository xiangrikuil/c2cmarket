import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const apiMarketBackendSource = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')
const carpoolBackendSource = readFileSync(new URL('../carpoolBackend.ts', import.meta.url), 'utf8')
const servicePublishSource = readFileSync(new URL('../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')
const rushPublishSource = readFileSync(new URL('../../pages/ApiQuotaRushPublishPage.vue', import.meta.url), 'utf8')
const orderDetailSource = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')

function sourceBetween(source: string, startMarker: string, endMarker: string) {
  const start = source.indexOf(startMarker)
  const end = source.indexOf(endMarker, start + 1)
  expect(start).toBeGreaterThanOrEqual(0)
  expect(end).toBeGreaterThan(start)
  return source.slice(start, end)
}

describe('API 纠纷发布与身份联系方式约束', () => {
  it('购买和拼车交易复用账号绑定的 linux.do 联系方式', () => {
    expect(sourceBetween(apiMarketBackendSource, 'export async function backendCreateAPIQuotaOrder', 'export async function backendOwnerAPIQuotaBatches'))
      .toContain('backendBoundLinuxDoContactMethod()')
    expect(sourceBetween(apiMarketBackendSource, 'export async function backendCreateAPIPurchaseIntent', 'export async function backendCreateAPIOrderFromIntent'))
      .toContain('backendBoundLinuxDoContactMethod()')
    expect(sourceBetween(carpoolBackendSource, 'export async function backendSubmitCarpool', 'export async function backendUpdateOwnerCarpool'))
      .toContain('backendBoundLinuxDoContactMethod()')
    expect(sourceBetween(carpoolBackendSource, 'export async function backendCreateCarpoolApplication', 'async function ownerApplication'))
      .toContain('backendBoundLinuxDoContactMethod()')
    expect(apiMarketBackendSource).toContain("methods.find(method => method.enabled && method.type === 'linuxdo')")
  })

  it('两个发布入口都会提前读取活动纠纷并禁止最终提交', () => {
    for (const source of [servicePublishSource, rushPublishSource]) {
      expect(source).toContain("useMerchantApiOrders({ dispute: 'active' })")
      expect(source).toContain('发布前纠纷规则')
      expect(source).toContain('disputePublishBlocked')
      expect(source).toContain('不能发布或恢复 API 服务与额度，也不会接收新订单')
    }
  })

  it('活动纠纷会隐藏订单普通交易动作和付款资料读取', () => {
    expect(orderDetailSource).toContain('const ordinaryActionsPaused = computed')
    expect(orderDetailSource).toContain("order.value.status === 'pending_payment' && !ordinaryActionsPaused.value")
    for (const action of ['canSubmitPayment', 'canResubmitPayment', 'canConfirmPayment', 'canReportPaymentIssue', 'canSubmitDelivery', 'canConfirmComplete', 'canCancelOrder']) {
      expect(orderDetailSource).toContain(`const ${action} = computed(() => !ordinaryActionsPaused.value`)
    }
    expect(orderDetailSource).toContain('付款、取消、核款、交付、确认完成及自动超时流程均已暂停')
  })
})
