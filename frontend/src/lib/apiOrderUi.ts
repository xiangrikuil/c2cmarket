export type ApiOrderCancelResponsibility = 'merchant' | 'buyer'

export type ApiOrderCancelOption = {
  value: string
  label: string
  responsibility: ApiOrderCancelResponsibility
  responsibilityLabel: string
  requiresNote?: boolean
}

export const API_ORDER_CANCEL_OPTIONS: ApiOrderCancelOption[] = [
  { value: 'merchant_unavailable_quota', label: '商家告知暂无可用额度', responsibility: 'merchant', responsibilityLabel: '商家原因' },
  { value: 'merchant_payment_unavailable', label: '商家无法正常收款', responsibility: 'merchant', responsibilityLabel: '商家原因' },
  { value: 'merchant_unresponsive', label: '商家长时间未响应', responsibility: 'merchant', responsibilityLabel: '商家原因' },
  { value: 'buyer_no_longer_needed', label: '我不再需要该服务', responsibility: 'buyer', responsibilityLabel: '个人原因' },
  { value: 'buyer_wrong_order', label: '我选错了服务或金额', responsibility: 'buyer', responsibilityLabel: '个人原因' },
  { value: 'buyer_cannot_pay_in_time', label: '我无法在规定时间内付款', responsibility: 'buyer', responsibilityLabel: '个人原因' },
  { value: 'buyer_other', label: '其他个人原因', responsibility: 'buyer', responsibilityLabel: '个人原因', requiresNote: true },
]

export function merchantHandlingDeadline(paymentSubmittedAt?: string, windowMinutes = 10) {
  if (!paymentSubmittedAt) return null
  const submittedAt = Date.parse(paymentSubmittedAt)
  if (!Number.isFinite(submittedAt)) return null
  return new Date(submittedAt + windowMinutes * 60_000).toISOString()
}

export function orderCountdown(deadline?: string | null, now = Date.now()) {
  const deadlineTime = deadline ? Date.parse(deadline) : Number.NaN
  if (!Number.isFinite(deadlineTime)) {
    return { totalSeconds: 0, minutes: '00', seconds: '00', label: '--:--', expired: false, urgent: false }
  }
  const totalSeconds = Math.max(0, Math.ceil((deadlineTime - now) / 1000))
  const minutes = String(Math.floor(totalSeconds / 60)).padStart(2, '0')
  const seconds = String(totalSeconds % 60).padStart(2, '0')
  return {
    totalSeconds,
    minutes,
    seconds,
    label: `${minutes}:${seconds}`,
    expired: deadlineTime <= now,
    urgent: deadlineTime > now && totalSeconds <= 120,
  }
}

export function buildApiOrderCancelReason(optionValue: string, note: string) {
  const option = API_ORDER_CANCEL_OPTIONS.find(item => item.value === optionValue)
  if (!option) throw new Error('请选择取消原因。')
  const detail = note.trim()
  if (option.requiresNote && !detail) throw new Error('请填写其他取消原因。')
  return `${option.responsibilityLabel}｜${option.label}${detail ? `｜补充说明：${detail}` : ''}`
}

export function formatOrderDateTime(value?: string | null) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

export function formatApiOrderCancelReason(value?: string | null) {
  const reason = value?.trim()
  if (!reason) return '订单已取消，无需继续操作。'
  if (reason === 'payment_timeout') return '未在付款时间内完成付款，系统已自动取消订单。'
  if (reason === 'buyer_cancelled') return '买家已在付款前取消订单。'
  return reason
}
