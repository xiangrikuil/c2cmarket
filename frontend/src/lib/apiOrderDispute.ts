export type ApiOrderDisputeStatus =
  | 'none'
  | 'negotiating'
  | 'open'
  | 'awaiting_fulfillment'
  | 'fulfillment_confirmation'
  | 'closed'

export type ApiOrderDisputeIssueCode =
  | 'service_unavailable'
  | 'description_mismatch'
  | 'quota_shortage'
  | 'expired_early'
  | 'not_delivered'
  | 'refund_not_received'
  | 'payment_dispute'
  | 'other'

export type ApiOrderDisputeResolution = 'full_refund' | 'partial_refund' | 'continue_fulfillment' | 'other'
export type ApiOrderDisputeRemedyStatus = 'pending' | 'claimed_fulfilled' | 'confirmed' | 'contested' | 'confirmation_expired' | 'overdue' | 'cancelled'

export const apiMerchantRefundPolicyVersion = 'api-merchant-refund-v1'
export const apiMerchantRefundPolicyApplicability = '服务有效期内未交付、订单事实（号池、模型或额度）不符，或交付后连续不可用超过 1 小时。'
export const apiMerchantRefundPolicyExclusions = '买家违规、超出商户声明最大并发、额度正常耗尽、正常上游限流或买家网络问题不适用。'
export const apiOrderPlatformTradeBoundary = '售后由双方站外确认；平台不代收、不托管、不担保、不代赔。'

export type OpenApiOrderDisputeInput = {
  issueCode: ApiOrderDisputeIssueCode
  requestedResolution: ApiOrderDisputeResolution
  requestedAmountCny: string | null
	issueOccurredAt?: string | null
  reason: string
}

export const apiOrderDisputeIssueLabels: Record<ApiOrderDisputeIssueCode, string> = {
  service_unavailable: '无法使用',
  description_mismatch: '与商品描述不符',
  quota_shortage: '额度不足',
  expired_early: '提前失效',
  not_delivered: '卖家未交付',
  refund_not_received: '退款未到账',
  payment_dispute: '付款争议',
  other: '其他问题',
}

export const apiOrderDisputeResolutionLabels: Record<ApiOrderDisputeResolution, string> = {
  full_refund: '全额退款',
  partial_refund: '部分退款',
  continue_fulfillment: '继续履约',
  other: '其他方案',
}

export const apiOrderDisputeRemedyStatusLabels: Record<ApiOrderDisputeRemedyStatus, string> = {
  pending: '等待责任方履行',
  claimed_fulfilled: '履行待对方确认',
  confirmed: '对方已确认',
  contested: '平台重新审核中',
  confirmation_expired: '确认超时中性结案',
  overdue: '平台已确认逾期',
  cancelled: '整改已终止',
}

export function normalizeApiOrderDisputeStatus(value: unknown): ApiOrderDisputeStatus {
  if (value === undefined || value === null || value === '') return 'none'
  if (typeof value !== 'string') throw new Error(`Unsupported API order dispute status: ${String(value)}`)
  switch (value) {
    case 'none':
    case 'negotiating':
    case 'open':
    case 'awaiting_fulfillment':
    case 'fulfillment_confirmation':
    case 'closed':
      return value as ApiOrderDisputeStatus
    default:
      throw new Error(`Unsupported API order dispute status: ${String(value)}`)
  }
}

export function isApiOrderDisputeActive(value: unknown): boolean {
  const status = normalizeApiOrderDisputeStatus(value)
  return status !== 'none' && status !== 'closed'
}

export function canOpenApiOrderDispute(value: unknown): boolean {
  return normalizeApiOrderDisputeStatus(value) === 'none'
}

export function getApiOrderDisputeStatusLabel(value: unknown): string {
  const labels: Record<ApiOrderDisputeStatus, string> = {
    none: '无纠纷',
    negotiating: '协商中',
    open: '平台审核中',
    awaiting_fulfillment: '已裁决待履行',
    fulfillment_confirmation: '履行待确认',
    closed: '已结案',
  }
  return labels[normalizeApiOrderDisputeStatus(value)]
}

export function getApiOrderDisputeStatusDescription(value: unknown): string {
  const descriptions: Record<ApiOrderDisputeStatus, string> = {
    none: '',
    negotiating: '双方正在订单内协商处理，请围绕同一诉求补充脱敏事实。',
    open: '纠纷已提交平台审核。平台由管理员非实时处理，请勿重复提交。',
    awaiting_fulfillment: '平台已经作出裁决，正在等待责任方按裁决要求履行。',
    fulfillment_confirmation: '责任方已提交履行结果，正在等待整改受益方反馈。',
    closed: '该订单纠纷已经处理完毕，纠纷记录已结案。',
  }
  return descriptions[normalizeApiOrderDisputeStatus(value)]
}
