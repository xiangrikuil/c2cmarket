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
      sellerUserId: 'seller-user-id',
      status: 'delivery_submitted',
      serviceTitleSnapshot: 'GPT 服务',
      amount: '10.00',
      requestedUsdAllowanceSnapshot: '12.500000',
      cnyPerUsdAllowanceSnapshot: '0.8000',
      pricingSnapshot: '{"rate":"0.8000"}',
      currency: 'CNY',
      selectedPaymentMethod: 'wechat',
      paymentWindowMinutesSnapshot: 10,
      paymentExpiresAt: '2026-07-12T01:00:00Z',
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
    expect(row.detailItems).toContainEqual({ label: '购买额度', value: '12.500000 美元额度' })
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
      sellerUserId: 'seller-user-id',
      status: 'completed',
      disputeStatus: 'none',
      serviceTitleSnapshot: '额度包服务',
      amount: '20.00',
      currency: 'CNY',
      selectedPaymentMethod: 'alipay',
      paymentWindowMinutesSnapshot: 10,
      paymentExpiresAt: '2026-07-12T00:10:00Z',
      deliverySubmittedAt: '2026-07-12T00:40:00Z',
      deliveryReviewExpiresAt: '2026-07-13T00:40:00Z',
      completionSource: 'auto_completed',
      completedAt: '2026-07-13T00:40:00Z',
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
    expect(detail.sellerUserId).toBe('seller-user-id')
    expect(detail.completionSource).toBe('auto_completed')
    expect(JSON.stringify(detail)).not.toContain('must-not-leak')
    expect(detail).not.toHaveProperty('deliveryCredential')
  })
})
