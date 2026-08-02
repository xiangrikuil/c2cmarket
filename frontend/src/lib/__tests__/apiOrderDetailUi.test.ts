import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const detail = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')
const buyerList = readFileSync(new URL('../../pages/MyApiOrdersPage.vue', import.meta.url), 'utf8')
const merchantList = readFileSync(new URL('../../pages/MerchantApiOrdersPage.vue', import.meta.url), 'utf8')
const adminDetail = readFileSync(new URL('../../pages/AdminApiOrderDetailPage.vue', import.meta.url), 'utf8')
const router = readFileSync(new URL('../../router.ts', import.meta.url), 'utf8')

describe('API 订单角色视图', () => {
  it('gives the buyer a review window and explicit credential actions', () => {
    expect(detail).toContain("'API 购买订单'")
    expect(detail).toContain('凭证核验剩余时间')
    expect(detail).toContain('确认凭证可用')
    expect(detail).toContain('凭证存在问题')
    expect(detail).toContain('credentialProblemOptions')
    expect(detail).not.toContain('window.confirm')
    expect(buyerList).toContain("'待核验'")
  })

  it('ends seller work immediately after credential delivery', () => {
    expect(detail).toContain("'API 销售订单'")
    expect(detail).toContain('已完成交付，无需继续操作')
    expect(merchantList).toContain('提交凭证后你的履约任务即完成')
    expect(merchantList).not.toContain('等待买家确认完成')
  })

  it('registers a read-only admin detail without rendering secret properties', () => {
    expect(router).toContain("path: '/admin/api-orders/:id'")
    expect(adminDetail).toContain('API 订单监管详情')
    expect(adminDetail).toContain('订单参与方')
    expect(adminDetail).toContain('核验截止')
    expect(adminDetail).not.toContain('order.deliveryCredential')
    expect(adminDetail).not.toContain('order.apiKey')
    expect(adminDetail).not.toContain('order.password')
  })
})
