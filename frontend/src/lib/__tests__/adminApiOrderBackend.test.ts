import { describe, expect, it } from 'vitest'
import { mapBackendAdminAPIOrder, type BackendAPIOrder } from '@/lib/apiMarketBackend'

describe('管理员 API 订单适配', () => {
  it('展示订单十进制快照且不传播原始交付凭证', () => {
    const order: BackendAPIOrder = {
      id: 'order-1',
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
    expect(row.secondary).toContain('订单金额 ¥10.00')
    expect(row.detailItems).toContainEqual({ label: '购买额度', value: '12.500000 美元额度' })
    expect(JSON.stringify(row)).not.toContain('must-not-leak')
    expect(row.targetType).toBe('api-order')
  })
})
