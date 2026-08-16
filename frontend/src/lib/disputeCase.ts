import type { DisputeCase } from '@/api/generated/openapi'

export type DisputeCaseStatus = DisputeCase['status']

const disputeCaseStatusLabels: Record<DisputeCaseStatus, string> = {
  pending_seller_response: '等待卖家处理',
  pending_applicant_decision: '等待买家决定',
  voluntary_fulfillment: '卖家处理中',
  open: '处理中',
  waiting_info: '需要补充信息',
  resolved: '已处理',
  closed: '已关闭',
  withdrawn: '已撤回',
  self_resolved: '线下已解决',
}

export function getDisputeCaseStatusLabel(status: DisputeCaseStatus): string {
  return disputeCaseStatusLabels[status]
}
