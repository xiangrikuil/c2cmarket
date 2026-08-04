import { formatDecimal, isPositiveDecimal, normalizeDecimalTrimmed } from '@/lib/decimal'
import type {
  ApiQuotaLimitMode,
  ApiQuotaUsageLimit,
  ApiQuotaUsageLimitInput,
  ApiQuotaUsagePolicy,
  ApiQuotaUsagePolicyInput,
} from '@/types/apiQuota'

const quotaLimitModes = new Set<ApiQuotaLimitMode>(['limited', 'unlimited', 'unspecified'])
const writableQuotaLimitModes = new Set<ApiQuotaUsageLimitInput['mode']>(['limited', 'unlimited'])
const quotaAmountPattern = /^\d{1,12}(?:\.\d{1,6})?$/

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function parseQuotaUsageLimit(value: unknown, field: string): ApiQuotaUsageLimit {
  if (!isRecord(value) || !quotaLimitModes.has(value.mode as ApiQuotaLimitMode)) {
    throw new Error(`Invalid ${field} quota limit mode`)
  }
  const mode = value.mode as ApiQuotaLimitMode
  if (mode === 'limited') {
    if (typeof value.amountUsd !== 'string' || !quotaAmountPattern.test(value.amountUsd) || !isPositiveDecimal(value.amountUsd)) {
      throw new Error(`Invalid ${field} quota limit amount`)
    }
    return { mode, amountUsd: normalizeDecimalTrimmed(value.amountUsd, 6) }
  }
  if (value.amountUsd !== null) throw new Error(`Invalid ${field} quota limit amount`)
  return { mode, amountUsd: null }
}

export function unspecifiedApiQuotaUsagePolicy(): ApiQuotaUsagePolicy {
  return {
    fiveHour: { mode: 'unspecified', amountUsd: null },
    daily: { mode: 'unspecified', amountUsd: null },
    scope: 'per_buyer_credential',
    dailyReset: 'utc_plus_8_calendar_day',
  }
}

export function parseApiQuotaUsagePolicy(value: unknown): ApiQuotaUsagePolicy {
  if (!isRecord(value)
    || value.scope !== 'per_buyer_credential'
    || value.dailyReset !== 'utc_plus_8_calendar_day') {
    throw new Error('Invalid API quota usage policy')
  }
  return {
    fiveHour: parseQuotaUsageLimit(value.fiveHour, 'five-hour'),
    daily: parseQuotaUsageLimit(value.daily, 'daily'),
    scope: 'per_buyer_credential',
    dailyReset: 'utc_plus_8_calendar_day',
  }
}

export function normalizeHistoricalApiQuotaUsagePolicy(value: unknown): ApiQuotaUsagePolicy {
  return value === undefined || value === null
    ? unspecifiedApiQuotaUsagePolicy()
    : parseApiQuotaUsagePolicy(value)
}

function quotaUsageLimitInputError(limit: unknown, label: string) {
  if (!isRecord(limit) || !writableQuotaLimitModes.has(limit.mode as ApiQuotaUsageLimitInput['mode'])) {
    return `${label}必须选择金额上限或不限。`
  }
  if (limit.mode === 'unlimited') return null
  const amount = typeof limit.amountUsd === 'string' ? limit.amountUsd.trim() : ''
  if (!quotaAmountPattern.test(amount) || !isPositiveDecimal(amount)) {
    return `${label}金额必须是大于 0、最多 6 位小数的美元金额。`
  }
  return null
}

export function apiQuotaUsagePolicyInputError(policy: unknown) {
  if (!isRecord(policy)) return '请完整填写额度使用规则。'
  return quotaUsageLimitInputError(policy.fiveHour, '5h 限额')
    ?? quotaUsageLimitInputError(policy.daily, '每日限额')
}

function toQuotaUsageLimitInput(limit: ApiQuotaUsageLimitInput): ApiQuotaUsageLimitInput {
  if (limit.mode === 'unlimited') return { mode: 'unlimited' }
  return { mode: 'limited', amountUsd: normalizeDecimalTrimmed(limit.amountUsd!.trim(), 6) }
}

export function toApiQuotaUsagePolicyInput(policy: unknown): ApiQuotaUsagePolicyInput {
  const validationError = apiQuotaUsagePolicyInputError(policy)
  if (validationError) throw new Error(validationError)
  const validated = policy as ApiQuotaUsagePolicyInput
  return {
    fiveHour: toQuotaUsageLimitInput(validated.fiveHour),
    daily: toQuotaUsageLimitInput(validated.daily),
  }
}

export function apiQuotaUsagePolicyFromInput(policy: unknown): ApiQuotaUsagePolicy {
  const input = toApiQuotaUsagePolicyInput(policy)
  return {
    fiveHour: input.fiveHour.mode === 'limited'
      ? { mode: 'limited', amountUsd: input.fiveHour.amountUsd! }
      : { mode: 'unlimited', amountUsd: null },
    daily: input.daily.mode === 'limited'
      ? { mode: 'limited', amountUsd: input.daily.amountUsd! }
      : { mode: 'unlimited', amountUsd: null },
    scope: 'per_buyer_credential',
    dailyReset: 'utc_plus_8_calendar_day',
  }
}

export function defaultApiQuotaUsagePolicyInput(): ApiQuotaUsagePolicyInput {
  return {
    fiveHour: { mode: 'unlimited' },
    daily: { mode: 'unlimited' },
  }
}

export function apiQuotaUsageLimitLabel(limit: ApiQuotaUsageLimit | ApiQuotaUsageLimitInput) {
  if (limit.mode === 'unlimited') return '不限'
  if (limit.mode === 'unspecified') return '未说明'
  return limit.amountUsd ? `$${formatDecimal(limit.amountUsd, 0, 6)}` : '待填写'
}
