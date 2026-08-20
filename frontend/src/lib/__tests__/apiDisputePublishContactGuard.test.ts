import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const apiMarketBackendSource = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')
const carpoolBackendSource = readFileSync(new URL('../carpoolBackend.ts', import.meta.url), 'utf8')
const apiFacadeSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const profileBackendSource = readFileSync(new URL('../profileBackend.ts', import.meta.url), 'utf8')
const servicePublishSource = readFileSync(new URL('../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')
const rushPublishSource = readFileSync(new URL('../../pages/ApiQuotaRushPublishPage.vue', import.meta.url), 'utf8')
const orderDetailSource = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')
const commerceStatusPanelSource = readFileSync(new URL('../../components/api-order/SellerCommerceStatusPanel.vue', import.meta.url), 'utf8')
const disputePanelSource = readFileSync(new URL('../../components/api-order/ApiOrderDisputePanel.vue', import.meta.url), 'utf8')

function sourceBetween(source: string, startMarker: string, endMarker: string) {
  const start = source.indexOf(startMarker)
  const end = source.indexOf(endMarker, start + 1)
  expect(start).toBeGreaterThanOrEqual(0)
  expect(end).toBeGreaterThan(start)
  return source.slice(start, end)
}

describe('API 纠纷发布与身份联系方式约束', () => {
  it('API 购买与拼车显式提交当前交易选择的联系方式', () => {
    expect(sourceBetween(apiMarketBackendSource, 'export async function backendCreateAPIQuotaOrder', 'export async function backendOwnerAPIQuotaBatches'))
      .toContain('buyerContactMethodId: payload.buyerContactMethodId')
    expect(sourceBetween(apiMarketBackendSource, 'export async function backendCreateAPIPurchaseIntent', 'export async function backendCreateAPIOrderFromIntent'))
      .toContain('buyerContactMethodId: payload.buyerContactMethodId')
    expect(sourceBetween(carpoolBackendSource, 'export async function backendSubmitCarpool', 'export async function backendUpdateOwnerCarpool'))
      .toContain('payload.ownerContactMethodId')
    expect(sourceBetween(carpoolBackendSource, 'export async function backendCreateCarpoolApplication', 'async function ownerApplication'))
      .toContain('buyerContactMethodId: payload.buyerContactMethodId')
    expect(apiMarketBackendSource).not.toContain('backendBuyerContactMethod')
    expect(carpoolBackendSource).not.toContain('backendEnabledWechatContactMethod')
    const mockCarpoolContacts = sourceBetween(apiFacadeSource, 'export async function getCarpoolApplicationContacts', 'export async function createContactReport')
    expect(mockCarpoolContacts).not.toContain("type: 'linuxdo'")
    expect(mockCarpoolContacts).not.toContain('application.ownerUsername')
    expect(apiFacadeSource).toContain("buyerContacts: [mockContactSnapshotItem(buyerContact, 'buyer')]")
    expect(apiFacadeSource).toContain("contactSnapshot.sellerContacts = [mockContactSnapshotItem(ownerContact, 'carpool_owner')]")
  })

  it('两个发布入口都使用后端经营等级决定是否可以开启接单', () => {
    for (const source of [servicePublishSource, rushPublishSource]) {
      expect(source).toContain('useSellerCommerceStatus')
      expect(source).toContain('SellerCommerceStatusPanel')
      expect(source).toContain('disputePublishBlocked')
      expect(source).toContain("level === 'account_limited'")
    }
    expect(rushPublishSource).toContain('affectedServiceIds.includes(selectedServiceId.value)')
    expect(servicePublishSource).toContain('编辑和保存草稿仍然可用')
    expect(commerceStatusPanelSource).toContain('活动纠纷只冻结对应订单')
    expect(commerceStatusPanelSource).toContain('已成立订单仍可继续履约')
  })

  it('纠纷确认区区分双方责任且不宣称平台核验站外退款', () => {
    expect(disputePanelSource).toContain('已完成当前处理，无需操作')
    expect(disputePanelSource).toContain('平台未核验站外退款是否到账')
    expect(disputePanelSource).toContain('confirmRemedyDialogOpen')
    expect(disputePanelSource).toContain('fixed inset-x-0 bottom-0')
  })

  it('活动纠纷会隐藏订单普通交易动作和付款资料读取', () => {
    expect(orderDetailSource).toContain('const ordinaryActionsPaused = computed')
    expect(orderDetailSource).toContain("order.value.status === 'pending_payment' && !ordinaryActionsPaused.value")
    for (const action of ['canSubmitPayment', 'canResubmitPayment', 'canConfirmPayment', 'canReportPaymentIssue', 'canSubmitDelivery', 'canCancelOrder']) {
      expect(orderDetailSource).toContain(`const ${action} = computed(() => !ordinaryActionsPaused.value`)
    }
    expect(orderDetailSource).not.toContain('canConfirmComplete')
    expect(orderDetailSource).toContain('付款、取消、核款、交付及自动超时流程均已暂停')
  })

  it('车源更新和联系方式配置为每次命令生成幂等键', () => {
    expect(sourceBetween(carpoolBackendSource, 'export async function backendUpdateOwnerCarpool', 'export async function backendCreateCarpoolApplication'))
      .toContain("idempotencyPrefix: 'carpool-update'")
    for (const prefix of [
      'profile-contact-update',
      'profile-contact-delete',
      'profile-contact-default',
      'profile-contact-email-confirm',
    ]) {
      expect(profileBackendSource).toContain(`idempotencyPrefix: '${prefix}'`)
    }
  })
})
