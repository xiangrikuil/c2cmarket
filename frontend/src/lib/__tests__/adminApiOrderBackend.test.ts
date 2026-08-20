import { describe, expect, it } from 'vitest'
import { mapBackendAdminAPIOrder, mapBackendAdminAPIOrderDetail, type BackendAPIOrder } from '@/lib/apiMarketBackend'

describe('管理员 API 订单适配', () => {
  it('展示订单十进制快照且不传播原始交付凭证', () => {
    const order: BackendAPIOrder = {
      id: 'order-1',
      orderNo: 'API-20260802-K7M4P9Q2XZ',
      purchaseKind: 'api_service',
      apiPurchaseIntentId: 'intent-1',
      apiServiceId: 'service-1',
      buyerUserId: 'buyer-user-id',
      buyerUsername: 'lin_buyer',
      sellerUserId: 'seller-user-id',
      sellerUsername: 'api_merchant',
      status: 'delivery_submitted',
      disputeStatus: 'awaiting_fulfillment',
      disputeCaseId: 'dispute-1',
      latestDisputeCaseId: 'dispute-1',
      hasDisputeHistory: true,
      activeRemedyAction: 'full_refund',
      serviceTitleSnapshot: 'GPT 服务',
      amount: '10.00',
      requestedUsdAllowanceSnapshot: '12.500000',
      cnyPerUsdAllowanceSnapshot: '0.8000',
      pricingSnapshot: '{"rate":"0.8000"}',
      currency: 'CNY',
      selectedPaymentMethod: 'wechat',
      paymentWindowMinutesSnapshot: 10,
      paymentExpiresAt: '2026-07-12T01:00:00Z',
      merchantConfirmOverdue: true,
      merchantConfirmOverdueAt: '2026-07-12T00:21:00Z',
      deliveryOverdue: true,
      deliveryOverdueAt: '2026-07-12T00:31:00Z',
      commercialOutcome: 'pending',
      quotaValidityIssueAt: '2026-07-12T00:35:00Z',
      quotaValidityIssueReason: 'delivery_insufficient',
      deliverySubmittedAt: '2026-07-12T00:40:00Z',
      deliveryReviewExpiresAt: '2026-07-13T00:40:00Z',
      deliveryCredential: {
        deliveryKind: 'api_key_endpoint',
        apiKey: 'must-not-leak',
        submittedAt: '2026-07-12T00:40:00Z',
      },
      version: 4,
      createdAt: '2026-07-12T00:00:00Z',
      updatedAt: '2026-07-12T00:40:00Z',
    }

    const row = mapBackendAdminAPIOrder(order)

    expect(row.primary).toBe('GPT 服务 API 订单')
    expect(row.secondary).toContain('API-20260802-K7M4P9Q2XZ')
    expect(row.detailItems).toContainEqual({ label: '订单号', value: 'API-20260802-K7M4P9Q2XZ' })
    expect(row.secondary).toContain('订单金额 ¥10.00')
    expect(row.owner).toBe('买家 @lin_buyer / 商户 @api_merchant')
    expect(row.detailItems).toContainEqual({ label: '购买额度', value: '12.500000 美元额度' })
    expect(row.detailItems).toContainEqual({ label: '商业结果', value: '商业结果待确认' })
    expect(row.detailItems).toContainEqual({ label: '历史纠纷', value: '有历史案件' })
    expect(row.detailItems).toContainEqual({ label: '额度有效期', value: '首次交付时剩余不足 60 分钟' })
    expect(JSON.stringify(row)).not.toContain('must-not-leak')
    expect(row.targetType).toBe('api-order')
    expect(row.targetTo).toBe('/admin/api-orders/order-1')
  })

  it('maps a read-only admin detail without credential fields', () => {
    const order: BackendAPIOrder = {
      id: 'order-2',
      purchaseKind: 'limited_quota_offer',
      apiPurchaseIntentId: 'intent-2',
      apiServiceId: 'service-2',
      buyerUserId: 'buyer-user-id',
      buyerUsername: 'lin_buyer',
      sellerUserId: 'seller-user-id',
      sellerUsername: 'api_merchant',
      status: 'completed',
      disputeStatus: 'none',
      latestDisputeCaseId: 'dispute-closed-1',
      hasDisputeHistory: true,
      serviceTitleSnapshot: '额度包服务',
      amount: '20.00',
      currency: 'CNY',
      selectedPaymentMethod: 'alipay',
      paymentWindowMinutesSnapshot: 10,
      paymentExpiresAt: '2026-07-12T00:10:00Z',
      merchantConfirmOverdue: false,
      deliveryOverdue: false,
      deliverySubmittedAt: '2026-07-12T00:40:00Z',
      deliveryReviewExpiresAt: '2026-07-13T00:40:00Z',
      completionSource: 'auto_completed',
      completedAt: '2026-07-13T00:40:00Z',
      commercialOutcome: 'normal_fulfillment',
      commercialOutcomeUpdatedAt: '2026-07-13T00:40:00Z',
      deliveryCredential: {
        deliveryKind: 'api_key_endpoint',
        apiKey: 'must-not-leak',
        submittedAt: '2026-07-12T00:40:00Z',
      },
      version: 5,
      createdAt: '2026-07-12T00:00:00Z',
      updatedAt: '2026-07-13T00:40:00Z',
    }

    const detail = mapBackendAdminAPIOrderDetail(order)

    expect(detail.buyerUserId).toBe('buyer-user-id')
    expect(detail.buyerUsername).toBe('lin_buyer')
    expect(detail.sellerUserId).toBe('seller-user-id')
    expect(detail.sellerUsername).toBe('api_merchant')
    expect(detail.completionSource).toBe('auto_completed')
    expect(detail.latestDisputeCaseId).toBe('dispute-closed-1')
    expect(detail.hasDisputeHistory).toBe(true)
    expect(detail.commercialOutcome).toBe('normal_fulfillment')
    expect(JSON.stringify(detail)).not.toContain('must-not-leak')
    expect(detail).not.toHaveProperty('deliveryCredential')
  })

  it('falls back to shortened participant IDs when usernames are unavailable', () => {
    const order = {
      id: 'order-fallback',
      orderNo: 'API-20260802-K7M4P9Q2XZ',
      purchaseKind: 'api_service',
      apiPurchaseIntentId: 'intent-fallback',
      apiServiceId: 'service-fallback',
      buyerUserId: 'buyer-12345678',
      sellerUserId: 'seller-12345678',
      status: 'pending_payment',
      hasDisputeHistory: false,
      serviceTitleSnapshot: 'Fallback API',
      amount: '10.00',
      currency: 'CNY',
      selectedPaymentMethod: 'wechat',
      paymentWindowMinutesSnapshot: 10,
      paymentExpiresAt: '2026-08-20T01:00:00Z',
      merchantConfirmOverdue: false,
      deliveryOverdue: false,
      commercialOutcome: 'pending',
      version: 1,
      createdAt: '2026-08-20T00:00:00Z',
      updatedAt: '2026-08-20T00:00:00Z',
    } satisfies BackendAPIOrder

    expect(mapBackendAdminAPIOrder(order).owner).toBe('买家 buyer-12 / 商户 seller-1')
  })
})
