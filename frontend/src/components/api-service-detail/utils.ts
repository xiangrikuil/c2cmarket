import type { ApiService } from '@/lib/api'
import { formatQuotaExpiresAtLabel } from '@/lib/apiQuotaExpiration'
import { divideDecimal, formatDecimal, type DecimalInput } from '@/lib/decimal'

export function formatMultiplier(value: number) {
  return `${value.toFixed(2)}x`
}

export function formatMultiplierRange(service: Pick<ApiService, 'defaultMultiplier'> & {
  modelPriceRows: Array<Pick<ApiService['modelPriceRows'][number], 'merchantMultiplier'>>
}) {
  const multipliers = service.modelPriceRows.length
    ? service.modelPriceRows.map(row => row.merchantMultiplier)
    : [service.defaultMultiplier]
  const minimum = Math.min(...multipliers)
  const maximum = Math.max(...multipliers)

  if (minimum === maximum) return formatMultiplier(minimum)
  return `${formatMultiplier(minimum)}–${formatMultiplier(maximum)}`
}

export function formatModelSummary(models: string[]) {
  if (!models.length) return '暂未声明模型'
  if (models.length === 1) return models[0]
  return `${models[0]} 等 ${models.length} 个模型`
}

export function formatCredit(value: DecimalInput) {
  return `$${formatDecimal(value, 2, 6)} 美元额度`
}

export function apiServiceCnyPerUsdAllowance(service: Pick<ApiService, 'creditPerCny' | 'cnyPerUsdAllowance'>) {
  if (service.cnyPerUsdAllowance) return service.cnyPerUsdAllowance
  if (service.creditPerCny <= 0) return ''
  return divideDecimal('1', String(service.creditPerCny), 4)
}

export function apiServiceAvailableUsdAllowance(service: Pick<ApiService, 'balance' | 'availableUsdAllowance'>) {
  return service.availableUsdAllowance || String(service.balance)
}

export function apiServiceMaxUsdAllowancePerOrder(service: Pick<ApiService, 'balance' | 'maxUsdAllowancePerOrder'>) {
  return service.maxUsdAllowancePerOrder || String(service.balance)
}

export function estimateUsdAllowance(amountCny: DecimalInput, service: Pick<ApiService, 'creditPerCny' | 'cnyPerUsdAllowance'>) {
  const rate = apiServiceCnyPerUsdAllowance(service)
  return rate ? divideDecimal(amountCny, rate, 6) : '0.000000'
}

export function formatCnyPerUsdQuota(service: Pick<ApiService, 'creditPerCny' | 'cnyPerUsdAllowance'>) {
  const rate = apiServiceCnyPerUsdAllowance(service)
  if (!rate) return '—'
  return `¥${formatDecimal(rate, 2, 4)} / $1`
}

export function formatCny(value: number) {
  return `¥${value}`
}

export function formatBeijingDateTime(value: string) {
  return formatQuotaExpiresAtLabel(value) || value
}

export function formatPricePerMillion(value: number | null, prefix: '$' | '¥') {
  if (value === null) return '—'
  const digits = value < 1 ? 3 : 2
  return `${prefix}${value.toFixed(digits).replace(/\.?0+$/, '')} / M`
}
