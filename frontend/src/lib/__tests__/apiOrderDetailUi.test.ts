import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')

describe('API 订单详情 UI 契约', () => {
  it('使用快照投影并区分商户售后与平台边界', () => {
    expect(source).toContain('{{ orderModelSnapshotLabel }}')
    expect(source).toContain('历史订单未冻结模型信息')
    expect(source).toContain('商户售后说明')
    expect(source).toContain('平台交易边界')
    expect(source).not.toContain("order.intentSnapshot.models.join(' / ')")
  })

  it('为五步流程提供可见且有状态的连接线', () => {
    expect(source).toMatch(/<StepperSeparator[\s\S]*?h-0\.5[\s\S]*?group-data-\[state=completed\]:bg-primary\/60/)
    expect(source).toContain('min-w-[112px]')
    expect(source).toContain('overflow-x-auto')
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
