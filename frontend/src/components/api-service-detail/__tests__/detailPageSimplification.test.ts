import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'

function componentSource(name: string) {
  return readFileSync(new URL(`../${name}.vue`, import.meta.url), 'utf8')
}

test('keeps merchant trust signals in the purchase card only', () => {
  const header = componentSource('ApiServiceHeader')
  const panel = componentSource('ApiPurchasePanel')

  assert.doesNotMatch(header, /近 30 天完成|响应中位|getApiMerchantDisplayName/)
  assert.match(panel, /近 30 天完成.*响应中位/)
  assert.match(panel, /getApiMerchantDisplayName/)
})

test('prioritizes the actual quota price over the merchant multiplier', () => {
  const summary = componentSource('ApiServiceSummary')
  const priceIndex = summary.indexOf('美元额度售价')
  const multiplierIndex = summary.indexOf('商户倍率')

  assert.ok(priceIndex >= 0)
  assert.ok(multiplierIndex > priceIndex)
  assert.match(summary, /可售额度/)
  assert.match(summary, /API 额度有效期/)
  assert.match(summary, /接入类型/)
  assert.doesNotMatch(summary, /建议首次小额测试|官方模型价格的/)
})

test('uses direct amount entry and moves acknowledgement into the dialog', () => {
  const selector = componentSource('PurchaseAmountSelector')
  const panel = componentSource('ApiPurchasePanel')
  const dialog = componentSource('PurchaseConfirmDialog')

  assert.match(selector, /请输入订单金额/)
  assert.doesNotMatch(selector, /presets|自定义/)
  assert.match(panel, /创建订单并查看付款方式/)
  assert.doesNotMatch(panel, /type="checkbox"/)
  assert.match(dialog, /type="checkbox"/)
  assert.match(dialog, /submitting \|\| !acknowledged/)
})

test('layers secondary information behind marketplace detail tabs', () => {
  const tabs = componentSource('ApiServiceDetailsTabs')

  assert.match(tabs, /模型价格/)
  assert.match(tabs, /服务说明/)
  assert.match(tabs, /购买须知/)
  assert.match(tabs, /role="tablist"/)
  assert.match(tabs, /aria-selected/)
})

test('formats visible backend timestamps as Beijing time', () => {
  const prices = componentSource('ModelPriceTable')

  assert.match(prices, /formatBeijingDateTime\(service\.officialPricingUpdatedAt\)/)
  assert.doesNotMatch(prices, /最终由双方站外确认/)
})
