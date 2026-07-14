import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import { formatBeijingDateTime, formatCnyPerUsdQuota, formatModelSummary, formatMultiplierRange } from '../utils.ts'

test('formats an API service model summary', () => {
  assert.equal(formatModelSummary([]), '暂未声明模型')
  assert.equal(formatModelSummary(['GPT-4o']), 'GPT-4o')
  assert.equal(formatModelSummary(['GPT-4o', 'GPT-4o mini', 'o3']), 'GPT-4o 等 3 个模型')
})

test('formats a single API service multiplier', () => {
  assert.equal(formatMultiplierRange({
    defaultMultiplier: 1.25,
    modelPriceRows: [],
  }), '1.25x')
})

test('formats the Sub2API price for each dollar of merchant-declared quota', () => {
  assert.equal(formatCnyPerUsdQuota({ creditPerCny: 1.25 }), '¥0.80 / $1')
  assert.equal(formatCnyPerUsdQuota({ creditPerCny: 1.25, cnyPerUsdAllowance: '0.8000' }), '¥0.80 / $1')
})

test('formats the actual API service model multiplier range', () => {
  assert.equal(formatMultiplierRange({
    defaultMultiplier: 1,
    modelPriceRows: [
      { merchantMultiplier: 1.3 },
      { merchantMultiplier: 1.1 },
      { merchantMultiplier: 1.2 },
    ],
  }), '1.10x–1.30x')
})

test('formats API service timestamps in Beijing time', () => {
  assert.equal(formatBeijingDateTime('2026-07-10T17:41:28Z'), '2026-07-11 01:41')
})

test('renders the home API entry list from publicly orderable service fields', () => {
  const homeSource = readFileSync(new URL('../../../pages/HomePage.vue', import.meta.url), 'utf8')

  assert.match(homeSource, /filter\(item => item\.publiclyOrderable\)/)
  assert.match(homeSource, /formatModelSummary\(item\.models\)/)
  assert.match(homeSource, /formatCnyPerUsdQuota\(item\)/)
  assert.match(homeSource, /getApiMerchantDisplayName\(item\)/)
  assert.match(homeSource, /当前可购买 API 服务/)
  assert.doesNotMatch(homeSource, /2\.5M Tokens|500K Tokens|1M Tokens|\/ 1K Tokens/)
})
