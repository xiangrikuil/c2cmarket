import { describe, expect, it } from 'vitest'
import {
  apiQuotaUsagePolicyInputError,
  normalizeHistoricalApiQuotaUsagePolicy,
  parseApiQuotaUsagePolicy,
  toApiQuotaUsagePolicyInput,
} from '@/lib/apiQuotaPolicy'

const fixedPolicyFields = {
  scope: 'per_buyer_credential',
  dailyReset: 'utc_plus_8_calendar_day',
} as const

describe('API 额度使用规则', () => {
  it.each([
    ['limited', { mode: 'limited', amountUsd: '12.340000' }, { mode: 'limited', amountUsd: '12.34' }],
    ['unlimited', { mode: 'unlimited', amountUsd: null }, { mode: 'unlimited', amountUsd: null }],
    ['unspecified', { mode: 'unspecified', amountUsd: null }, { mode: 'unspecified', amountUsd: null }],
  ] as const)('严格读取 %s 模式', (_mode, input, expected) => {
    expect(parseApiQuotaUsagePolicy({
      fiveHour: input,
      daily: input,
      ...fixedPolicyFields,
    })).toEqual({
      fiveHour: expected,
      daily: expected,
      ...fixedPolicyFields,
    })
  })

  it.each(['0', '-1', '0.000000', '1.0000001', 'not-a-number'])('拒绝非法 limited 金额 %s', (amountUsd) => {
    const policy = {
      fiveHour: { mode: 'limited', amountUsd },
      daily: { mode: 'unlimited', amountUsd: null },
      ...fixedPolicyFields,
    }
    expect(() => parseApiQuotaUsagePolicy(policy)).toThrow('Invalid five-hour quota limit amount')
    expect(apiQuotaUsagePolicyInputError(policy)).toContain('大于 0、最多 6 位小数')
  })

  it('拒绝在新写请求中提交 unspecified', () => {
    const policy = {
      fiveHour: { mode: 'unspecified', amountUsd: null },
      daily: { mode: 'unlimited', amountUsd: null },
    }
    expect(apiQuotaUsagePolicyInputError(policy)).toBe('5h 限额必须选择金额上限或不限。')
    expect(() => toApiQuotaUsagePolicyInput(policy)).toThrow('5h 限额必须选择金额上限或不限。')
  })

  it.each([undefined, null])('只把缺失历史值 %s 规范为未说明', (value) => {
    expect(normalizeHistoricalApiQuotaUsagePolicy(value)).toEqual({
      fiveHour: { mode: 'unspecified', amountUsd: null },
      daily: { mode: 'unspecified', amountUsd: null },
      ...fixedPolicyFields,
    })
  })

  it.each([
    {},
    { fiveHour: { mode: 'limited', amountUsd: '1' } },
    {
      fiveHour: { mode: 'unlimited', amountUsd: '1' },
      daily: { mode: 'unlimited', amountUsd: null },
      ...fixedPolicyFields,
    },
    {
      fiveHour: { mode: 'unlimited', amountUsd: null },
      daily: { mode: 'mystery', amountUsd: null },
      ...fixedPolicyFields,
    },
  ])('拒绝静默修复已提供但格式错误的历史规则 %#', (value) => {
    expect(() => normalizeHistoricalApiQuotaUsagePolicy(value)).toThrow()
  })
})
