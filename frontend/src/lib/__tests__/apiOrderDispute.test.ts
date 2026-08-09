import { readFileSync } from 'node:fs'
import { describe, expect, test } from 'vitest'
import {
  canOpenApiOrderDispute,
  apiOrderDisputeRemedyStatusLabels,
  getApiOrderDisputeStatusLabel,
  isApiOrderDisputeActive,
  normalizeApiOrderDisputeStatus,
} from '@/lib/apiOrderDispute'

const orderDetailSource = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')
const disputePanelSource = readFileSync(new URL('../../components/api-order/ApiOrderDisputePanel.vue', import.meta.url), 'utf8')
const adminDialogSource = readFileSync(new URL('../../components/admin/AdminDisputeResolutionDialog.vue', import.meta.url), 'utf8')

describe('API order dispute projection', () => {
  test('maps every V1 projection to stable copy', () => {
    expect([
      'none',
      'negotiating',
      'open',
      'awaiting_fulfillment',
      'fulfillment_confirmation',
      'closed',
    ].map(getApiOrderDisputeStatusLabel)).toEqual([
      '无纠纷',
      '协商中',
      '平台审核中',
      '已裁决待履行',
      '履行待确认',
      '已结案',
    ])
  })

  test('allows a new dispute only from none and blocks completion for active phases', () => {
    expect(canOpenApiOrderDispute('none')).toBe(true)
    expect(canOpenApiOrderDispute('closed')).toBe(false)
    expect(isApiOrderDisputeActive('negotiating')).toBe(true)
    expect(isApiOrderDisputeActive('open')).toBe(true)
    expect(isApiOrderDisputeActive('awaiting_fulfillment')).toBe(true)
    expect(isApiOrderDisputeActive('fulfillment_confirmation')).toBe(true)
    expect(isApiOrderDisputeActive('closed')).toBe(false)
  })

  test('normalizes legacy missing values and rejects unknown backend states', () => {
    expect(normalizeApiOrderDisputeStatus(undefined)).toBe('none')
    expect(() => normalizeApiOrderDisputeStatus('resolved')).toThrow('Unsupported API order dispute status')
  })

  test('requires bilateral settlement and keeps platform review as a separate action', () => {
    expect(orderDetailSource).toContain('v-model="disputeIssueCode"')
    expect(orderDetailSource).toContain('v-model="disputeRequestedResolution"')
    expect(orderDetailSource).toContain('提交后进入双方协商；任一方可在无法达成一致时申请平台审核。')

    expect(disputePanelSource).toContain('pendingFromMe')
    expect(disputePanelSource).toContain('等待对方确认或拒绝。')
    expect(disputePanelSource).toContain('确认方案并结案')
    expect(disputePanelSource).toContain('拒绝方案')
    expect(disputePanelSource).toContain('申请平台审核')
    expect(disputePanelSource).not.toMatch(/诉求不成立|关闭纠纷|单方面关闭/)
  })

  test('renders every remedy state and keeps claim separate from closure', () => {
    expect(Object.values(apiOrderDisputeRemedyStatusLabels)).toEqual([
      '等待责任方履行',
      '履行待对方确认',
      '对方已确认',
      '平台重新审核中',
      '确认超时中性结案',
      '平台已确认逾期',
      '整改已终止',
    ])
    expect(disputePanelSource).toContain('声明已履行')
    expect(disputePanelSource).toContain('等待对方确认。')
    expect(disputePanelSource).toContain('确认已收到或已完成')
    expect(disputePanelSource).toContain('未收到或未完成')
    expect(disputePanelSource).toContain('平台未核验退款到账或履约事实')
    expect(disputePanelSource).not.toMatch(/声明已履行并结案|平台已确认退款到账|自动确认退款成功/)
  })

  test('requires an explicit admin remedy decision and gates overdue confirmation', () => {
    expect(adminDialogSource).toContain('无需履行，直接结案')
    expect(adminDialogSource).toContain('remedyForm.responsibleUserId')
    expect(adminDialogSource).toContain('remedyForm.dueAt')
    expect(adminDialogSource).toContain('责任方声明履行后不会自动结案')
    expect(adminDialogSource).toContain('确认逾期未履行')
    expect(adminDialogSource).toContain('!remedyDeadlineReached')
    expect(adminDialogSource).toContain('后续处罚可消费的逾期事实')
  })
})
