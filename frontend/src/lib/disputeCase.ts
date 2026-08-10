import type { DisputeCase } from '@/api/generated/openapi'

export type DisputeCaseStatus = DisputeCase['status']

const disputeCaseStatusLabels: Record<DisputeCaseStatus, string> = {
  negotiating: '协商中',
  open: '处理中',
  waiting_info: '需要补充信息',
  resolved: '已处理',
  closed: '已关闭',
}

export function getDisputeCaseStatusLabel(status: DisputeCaseStatus): string {
  return disputeCaseStatusLabels[status]
}
