import type { ApiDeliveryMode, ApiService, ApiUsageVisibility } from '@/lib/api'
import { getApiDeliveryModeLabel, getApiUsageVisibilityLabel } from '@/lib/api'

export function merchantIdentityLabel(type: ApiService['merchantType']) {
  return type === '商户' ? 'API 商户' : type === '可信新车主' ? '可信新商户' : '个人商户'
}

export function formatMultiplier(value: number) {
  return `${value.toFixed(2)}x`
}

export function formatPercentMultiplier(value: number) {
  return `${Math.round(value * 100)}%`
}

export function formatCreditConversion(service: Pick<ApiService, 'creditPerCny'>) {
  return `¥1 对应 ${formatCredit(service.creditPerCny)}`
}

export function formatCredit(value: number) {
  return `$${value.toLocaleString('zh-CN')} 美元额度`
}

export function formatCnyPerUsdQuota(service: Pick<ApiService, 'creditPerCny'>) {
  if (service.creditPerCny <= 0) return '—'
  return `¥${(1 / service.creditPerCny).toFixed(2).replace(/\.?0+$/, '')} / $1`
}

export function formatCny(value: number) {
  return `¥${value}`
}

export function formatPricePerMillion(value: number | null, prefix: '$' | '¥') {
  if (value === null) return '—'
  const digits = value < 1 ? 3 : 2
  return `${prefix}${value.toFixed(digits).replace(/\.?0+$/, '')} / M`
}

export function deliveryModeLabel(mode: ApiDeliveryMode) {
  return getApiDeliveryModeLabel(mode)
}

export function usageVisibilityLabel(value: ApiUsageVisibility) {
  return getApiUsageVisibilityLabel(value)
}

export function deliveryModesLabel(modes: ApiDeliveryMode[]) {
  return modes.map(deliveryModeLabel).join(' / ')
}
