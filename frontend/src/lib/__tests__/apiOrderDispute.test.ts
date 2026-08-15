import { readFileSync } from 'node:fs'
import { describe, expect, test } from 'vitest'
import {
  canOpenApiOrderDispute,
  apiOrderDisputeRemedyLatenessLabels,
  apiOrderDisputeRemedyStatusLabels,
  getApiOrderDisputeStatusLabel,
  isApiOrderDisputeActive,
  normalizeApiOrderDisputeStatus,
} from '@/lib/apiOrderDispute'
import { getDisputeCaseStatusLabel } from '@/lib/disputeCase'

const orderDetailSource = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')
const disputePanelSource = readFileSync(new URL('../../components/api-order/ApiOrderDisputePanel.vue', import.meta.url), 'utf8')
const disputePageSource = readFileSync(new URL('../../pages/MyApiOrderDisputePage.vue', import.meta.url), 'utf8')
const routerSource = readFileSync(new URL('../../router.ts', import.meta.url), 'utf8')
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

  test('maps every dispute-case status and uses the case helper in the participant panel', () => {
    expect(([
      'open',
      'waiting_info',
      'resolved',
      'closed',
      'withdrawn',
      'self_resolved',
    ] as const).map(getDisputeCaseStatusLabel)).toEqual([
      '处理中',
      '需要补充信息',
      '已处理',
      '已关闭',
      '已撤回',
      '线下已解决',
    ])
    expect(disputePanelSource).toContain('getDisputeCaseStatusLabel(dispute.status)')
  })

  test('opens platform handling directly and exposes only formal participant actions', () => {
    expect(orderDetailSource).toContain('v-model="disputeIssueCode"')
    expect(orderDetailSource).toContain('v-model="disputeRequestedResolution"')
    expect(orderDetailSource).toContain('evidenceAssetIds: disputeEvidence.value.map(item => item.id)')
    expect(orderDetailSource).toContain('提交后直接进入平台处理。被申请方可提交一次正式答复')

    expect(disputePanelSource).toContain("const canRespond = computed(() => dispute.value?.active")
    expect(disputePanelSource).toContain("dispute.value.status === 'open'")
    expect(disputePanelSource).toContain('正式答复只能提交一次，提交后不可修改。')
    expect(disputePanelSource).toContain('responseEvidence')
    expect(disputePanelSource).toContain('const canApplicantFinish = computed')
    expect(disputePanelSource).toContain('撤回申请')
    expect(disputePanelSource).toContain('确认线下解决')
    expect(disputePanelSource).toContain('平台定向补件')
    expect(disputePanelSource).toContain('不会开启双方站内协商')
    expect(disputePanelSource).not.toContain('pendingFromMe')
    expect(disputePanelSource).not.toContain('结束协商并申请平台介入')
    expect(disputePanelSource).not.toContain('const canMessage')
    expect(disputePanelSource).not.toMatch(/诉求不成立|关闭纠纷|单方面关闭/)
  })

  test('uses a standalone dispute page and keeps legacy negotiation records read-only', () => {
    expect(routerSource).toContain("path: '/my/disputes/:id'")
    expect(disputePageSource).toContain('<ApiOrderDisputePanel :dispute-id="disputeId" />')
    expect(orderDetailSource).toContain('进入纠纷处理')
    expect(orderDetailSource).toContain('`/my/disputes/${disputePanelId}`')
    expect(orderDetailSource).not.toContain('<ApiOrderDisputePanel')
    expect(disputePanelSource).toContain('旧流程历史记录')
    expect(disputePanelSource).toContain('仅供查看，不能继续留言或处理方案')
    expect(disputePanelSource).toContain('historicalMessages')
    expect(disputePanelSource).toContain('historicalProposals')
  })

  test('renders every remedy state and keeps claim separate from closure', () => {
    expect(Object.values(apiOrderDisputeRemedyStatusLabels)).toEqual([
      '等待责任方履行',
      '履行待对方确认',
      '对方已确认',
      '平台重新审核中',
      '确认超时中性结案',
      '整改已终止',
    ])
    expect(Object.values(apiOrderDisputeRemedyLatenessLabels)).toEqual([
      '尚未到期',
      '按时声明履行',
      '迟到待平台裁定',
      '平台已确认迟到',
      '平台已豁免迟到',
    ])
    expect(disputePanelSource).toContain('声明已履行')
    expect(disputePanelSource).toContain('canConfirmRemedy')
    expect(disputePanelSource).toContain('确认完成')
    expect(disputePanelSource).toContain('申请复核')
    expect(disputePanelSource).toContain('remedyContestEvidence')
    expect(disputePanelSource).not.toMatch(/声明已履行并结案|平台已确认退款到账|自动确认退款成功/)
  })

  test('requires an explicit admin remedy decision and keeps lateness independent', () => {
    expect(adminDialogSource).toContain('无需履行，直接结案')
    expect(adminDialogSource).toContain('remedyForm.responsibleUserId')
    expect(adminDialogSource).toContain('remedyForm.dueAt')
    expect(adminDialogSource).toContain('责任方声明履行后不会自动结案')
    expect(adminDialogSource).toContain('确认迟到')
    expect(adminDialogSource).toContain('豁免迟到')
    expect(adminDialogSource).toContain('canDecideRemedyLateness')
    expect(adminDialogSource).toContain('不改变当前履行进度')
    expect(adminDialogSource).toContain('只有“确认迟到”可作为后续责任或限制依据')
  })
})
