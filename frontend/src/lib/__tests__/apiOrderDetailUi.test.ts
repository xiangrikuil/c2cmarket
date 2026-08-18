import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const detail = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')
const buyerList = readFileSync(new URL('../../pages/MyApiOrdersPage.vue', import.meta.url), 'utf8')
const merchantList = readFileSync(new URL('../../pages/MerchantApiOrdersPage.vue', import.meta.url), 'utf8')
const adminDetail = readFileSync(new URL('../../pages/AdminApiOrderDetailPage.vue', import.meta.url), 'utf8')
const backendAdapter = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')
const contactSelector = readFileSync(new URL('../../components/api-service-publish/MerchantContactMethodsSection.vue', import.meta.url), 'utf8')
const refundEvidence = readFileSync(new URL('../../components/api-order/ApiRefundPolicyEvidence.vue', import.meta.url), 'utf8')
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

  it('uses the shared dispute projection labels in the admin detail', () => {
    expect(adminDetail).toContain('getApiOrderDisputeStatusLabel(order.disputeStatus)')
    expect(adminDetail).toContain('getApiOrderDisputeStatusDescription(order.disputeStatus)')
    expect(adminDetail).toContain("order.disputeStatus !== 'none'")
    expect(adminDetail).not.toContain("order.disputeStatus || '无纠纷'")
  })
})

const source = detail

describe('API 订单详情 UI 契约', () => {
  it('展示完整公开业务编号且不再截取 UUID 冒充订单号', () => {
    expect(source).toContain(':value="order.orderNo" full copyable')
    expect(source).not.toContain(':value="order.id" prefix="API"')
  })

  it('使用快照投影并区分结构化商户承诺、历史售后与平台边界', () => {
    expect(source).toContain('{{ orderModelSnapshotLabel }}')
    expect(source).toContain('历史订单未冻结模型信息')
    expect(source).toContain('号池')
    expect(source).toContain('商户声明最大并发')
    expect(source).toContain('商户退款承诺')
    expect(source).toContain('退款规则版本')
    expect(source).toContain('服务有效期')
    expect(source).toContain('历史售后说明')
    expect(source).toContain('历史订单未冻结')
    expect(source).toContain('平台交易边界')
    expect(source).not.toContain("order.intentSnapshot.models.join(' / ')")
  })

  it('uses frozen contacts and the server-owned completed-order after-sales projection', () => {
    expect(contactSelector).toContain('成交时锁定当前微信')
    expect(contactSelector).toContain('微信自动用于 API 交易联系')
    expect(contactSelector).not.toContain('type="checkbox"')
    expect(backendAdapter).toContain('intent.merchantContacts.flatMap(contactToChannel)')
    expect(backendAdapter).toContain('afterSalesExpiresAt: order.afterSalesExpiresAt')
    expect(backendAdapter).toContain('canOpenDispute: order.canOpenDispute')
    expect(backendAdapter).toContain('disputeEligibilityReason: order.disputeEligibilityReason')
    expect(source).toContain('仍在 24 小时补报期内')
    expect(source).toContain('v-model="disputeIssueOccurredAt"')
    expect(source).toContain('disputeValidityExpiresAt')
  })

  it('shows the frozen refund-policy version and full evidence dialog', () => {
    expect(refundEvidence).toContain('API 商户退款规则 v1')
    expect(refundEvidence).toContain('下单时已锁定')
    expect(refundEvidence).toContain('商户售后承诺')
    expect(refundEvidence).toContain('平台交易边界')
  })

  it('为五步流程提供可见且有状态的连接线', () => {
    expect(source).toMatch(/<StepperSeparator[\s\S]*?h-0\.5[\s\S]*?group-data-\[state=completed\]:bg-primary\/60/)
    expect(source).toContain('min-w-[112px]')
    expect(source).toContain('overflow-x-auto')
  })

  it('将订单、接入凭证和冻结联系方式分区展示', () => {
    expect(source).toContain("lg:grid-cols-[minmax(0,0.95fr)_minmax(0,1.1fr)_minmax(280px,0.8fr)]")
    expect(source).toContain('API 购买订单')
    expect(source).toContain('API 销售订单')
    expect(source).toContain('接入凭证')
    expect(source).not.toContain('支付凭证')
    expect(source).toContain('apiOrderMerchantContactSnapshot(order.value)')
    expect(source).toContain('apiOrderBuyerContactSnapshot(order.value)')
    expect(source).not.toContain('useApiService')
  })

  it('默认遮罩 API Key 和初始密码并保留显示与复制动作', () => {
    expect(source).toContain('function maskCredential')
    expect(source).toContain("const apiKeyVisible = ref(false)")
    expect(source).toContain("const passwordVisible = ref(false)")
    expect(source).toContain("apiKeyVisible ? order.deliveryCredential.apiKey : maskCredential(order.deliveryCredential.apiKey)")
    expect(source).toContain("passwordVisible ? order.deliveryCredential.password : maskCredential(order.deliveryCredential.password)")
    expect(source).toContain('复制 API Key')
    expect(source).toContain('显示初始密码')
  })

  it('凭证销毁后仅展示审计事实且不承诺长期保存', () => {
    expect(source).toContain('order.deliveryCredential.destroyedAt')
    expect(source).toContain('历史凭证已按保留策略销毁')
    expect(source).toContain('保留期内可查看')
    expect(source).not.toContain('长期可查看')
  })

  it('通过 shadcn-vue RadioGroupItem 与 Label 组合单选项', () => {
    expect(source).toContain("import { Label } from '@/components/ui/label'")
    expect(source).toContain('<RadioGroup v-model="paymentIssueReason"')
    expect(source).toContain('<RadioGroup v-model="cancelReason"')
    expect(source).toContain(':for="`payment-issue-${option.value}`"')
    expect(source).toContain(':for="`cancel-reason-${option.value}`"')
    expect(source).not.toContain("paymentIssueReason === option.value ? 'border-warning/60 bg-warning/10' : ''")
  })
})
