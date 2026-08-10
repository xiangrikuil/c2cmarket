export type QuotaPeriod = 'monthly'

export type QuotaDisplayInput = {
  amount?: number | null
  dailyQuotaAmount?: number | null
  weeklyQuotaAmount?: number | null
  label?: string | null
  quotaLabel?: string | null
  unit?: string | null
  quotaUnit?: string | null
  period?: string | null
  quotaPeriod?: string | null
}

export const defaultQuotaLabel = '额度'
export const defaultQuotaUnit = 'USD'
export const defaultQuotaPeriod: QuotaPeriod = 'monthly'

export function quotaPeriodText(period?: string | null) {
  return period === defaultQuotaPeriod ? '每月' : '每月'
}

export function quotaFieldLabel(input?: QuotaDisplayInput | null) {
  return `${quotaPeriodText(input?.period ?? input?.quotaPeriod)}${(input?.label ?? input?.quotaLabel)?.trim() || defaultQuotaLabel}`
}

export function formatQuotaAmount(value: number) {
  return value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

export function formatMonthlyQuota(input: QuotaDisplayInput, fallback = '额度待补充') {
  const amount = input.amount ?? input.weeklyQuotaAmount
  if (!Number.isFinite(amount) || !amount || amount <= 0) return fallback
  const unit = (input.unit ?? input.quotaUnit)?.trim() || defaultQuotaUnit
  return `${quotaPeriodText(input.period ?? input.quotaPeriod)} ${formatQuotaAmount(amount)} ${unit}`
}

export function formatDailyWeeklyQuota(input: QuotaDisplayInput, fallback = '未声明') {
  const unit = (input.unit ?? input.quotaUnit)?.trim() || defaultQuotaUnit
  const daily = input.dailyQuotaAmount
  const weekly = input.weeklyQuotaAmount ?? input.amount
  const dailyText = Number.isFinite(daily) && daily && daily > 0
    ? `${formatQuotaAmount(daily)} ${unit}`
    : fallback
  const weeklyText = Number.isFinite(weekly) && weekly && weekly > 0
    ? `${formatQuotaAmount(weekly)} ${unit}`
    : fallback
  return `日 ${dailyText} · 周 ${weeklyText}`
}
