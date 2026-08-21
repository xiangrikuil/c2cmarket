import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  accountPoolOptionsForProviderCategory,
  isAccountPoolCompatibleWithProviderCategory,
} from '@/components/api-service-publish/utils'

const priceInventorySource = readFileSync(
  new URL('../../components/api-service-publish/PriceInventorySection.vue', import.meta.url),
  'utf8',
)
const publishPageSource = readFileSync(
  new URL('../../pages/ApiServicePublishPage.vue', import.meta.url),
  'utf8',
)
const rushPublishPageSource = readFileSync(
  new URL('../../pages/ApiQuotaRushPublishPage.vue', import.meta.url),
  'utf8',
)

describe('API 服务发布输入', () => {
  it('价格输入保留原始编辑文本，支持从 0.8 连续编辑为 0.15', () => {
    expect(priceInventorySource).toContain(':model-value="cnyPerUsdCreditInput"')
    expect(priceInventorySource).toContain('@update:model-value="updateCnyPerUsdCredit"')
    expect(priceInventorySource).not.toContain('form.cnyPerUsdCredit = Number(value)')
  })

  it('GPT 模型展示 GPT 号池和自定义号池', () => {
    expect(accountPoolOptionsForProviderCategory('gpt')).toEqual([
      'gpt_pro_20x',
      'gpt_pro_5x',
      'gpt_plus',
      'custom',
    ])
  })

  it('Grok 等非 GPT 模型不展示或保留 GPT 号池', () => {
    expect(accountPoolOptionsForProviderCategory('grok')).toEqual(['custom'])
    expect(accountPoolOptionsForProviderCategory('claude')).toEqual(['custom'])
    expect(isAccountPoolCompatibleWithProviderCategory('gpt_plus', 'grok')).toBe(false)
    expect(isAccountPoolCompatibleWithProviderCategory('custom', 'grok')).toBe(true)
  })

  it('切换模型大类时清空原模型的号池选择和名称', () => {
    for (const source of [publishPageSource, rushPublishPageSource]) {
      expect(source).toContain("accountPoolType = ''")
      expect(source).toContain("accountPoolCustomName = ''")
    }
  })

  it('普通 API 额度和限时额度发布默认使用 2 并发', () => {
    for (const source of [publishPageSource, rushPublishPageSource]) {
      expect(source).toContain('declaredMaxConcurrency: 2')
      expect(source).not.toContain('declaredMaxConcurrency: 1,')
    }
  })

  it('普通 API 额度和限时额度发布默认售价为每美元 0.15 元', () => {
    for (const source of [publishPageSource, rushPublishPageSource]) {
      expect(source).toContain('cnyPerUsdCredit: 0.15')
      expect(source).not.toContain('cnyPerUsdCredit: 0.8,')
    }
    expect(priceInventorySource).toContain('placeholder="0.15"')
    expect(priceInventorySource).toContain('例如 ¥0.15 / $1')
  })
})
