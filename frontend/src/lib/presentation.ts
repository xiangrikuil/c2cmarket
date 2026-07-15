export type StatusTone = 'brand' | 'success' | 'waiting' | 'warning' | 'risk' | 'complete' | 'neutral'

const statusToneGroups: Record<StatusTone, string[]> = {
  brand: ['active', 'online', '可上车', '进行中'],
  success: ['eligible', 'completed', 'approved', 'published', '已完成', '已交付', '已验证', '可创建订单'],
  waiting: ['pending_payment', 'payment_submitted', 'payment_issue', 'paid_confirmed', 'delivery_submitted', 'pending_owner', 'accepted_reserved', '待付款', '待确认', '付款待补充', '待交付', '待验收', '待处理'],
  warning: ['paused', 'changes_requested', 'owner_action_required', '待复核', '暂停', '即将超时'],
  risk: ['credential_risk', 'disputed', 'open', 'rejected', 'removed', '纠纷处理中', '风险', '已拒绝', '已下架'],
  complete: ['cancelled', 'closed', 'expired', '已取消', '已关闭', '已过期'],
  neutral: [],
}

export function statusTone(status: string): StatusTone {
  const normalized = status.trim().toLowerCase()
  for (const [tone, values] of Object.entries(statusToneGroups) as Array<[StatusTone, string[]]>) {
    if (values.some(value => normalized === value.toLowerCase() || normalized.includes(value.toLowerCase()))) return tone
  }
  return 'neutral'
}

export function shortId(value: string, prefix = '') {
  const compact = value.replace(/[^a-zA-Z0-9]/g, '')
  const suffix = (compact.slice(-6) || value.slice(-6)).toUpperCase()
  return prefix ? `${prefix}-${suffix}` : suffix
}

export function formatLocalDateTime(value: string | Date, includeSeconds = false) {
  const date = value instanceof Date ? value : new Date(value)
  if (!Number.isFinite(date.getTime())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: includeSeconds ? '2-digit' : undefined,
    hour12: false,
  }).format(date).replaceAll('/', '-')
}
