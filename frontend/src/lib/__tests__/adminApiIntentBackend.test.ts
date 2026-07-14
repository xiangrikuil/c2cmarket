import { describe, expect, it } from 'vitest'
import { mapBackendAdminAPIIntent, type BackendAPIPurchaseIntent } from '@/lib/apiMarketBackend'

describe('管理员 API 购买意向适配', () => {
  it('读取真实管理员队列时不投射联系方式', () => {
    const intent: BackendAPIPurchaseIntent = {
      id: 'intent-1',
      apiServiceId: 'service-1',
      buyerUserId: 'buyer-12345678',
      ownerUserId: 'owner-12345678',
      status: 'open',
      requestedCnyAmount: '18.50',
      selectedAccessMode: 'api_key',
      serviceVersionSnapshot: 2,
      serviceTitleSnapshot: 'GPT 服务',
      distributionSystemSnapshot: 'merchant',
      billingModeSnapshot: 'fixed',
      minimumIntentCnySnapshot: '10.00',
      pricingSnapshot: '¥18.50',
      version: 3,
      createdAt: '2026-07-11T08:00:00Z',
      updatedAt: '2026-07-11T09:00:00Z',
    }

    const row = mapBackendAdminAPIIntent(intent)

    expect(row).toMatchObject({
      id: 'intent-1',
      primary: 'GPT 服务 购买意向',
      status: '待处理',
      targetType: 'api-intent',
      backendVersion: 3,
    })
    expect(row.owner).toContain('buyer-12')
    expect(row.detailItems?.map(item => item.label)).not.toContain('联系方式')
  })
})
