import type { DisputeCase } from '@/api/generated/openapi'

export type DisputeCaseStatus = DisputeCase['status']

const disputeCaseStatusLabels: Record<DisputeCaseStatus, string> = {
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
