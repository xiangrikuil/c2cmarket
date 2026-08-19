import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'

function componentSource(name: string) {
  return readFileSync(new URL(`../${name}.vue`, import.meta.url), 'utf8')
}

function pageSource(name: string) {
  return readFileSync(new URL(`../../../pages/${name}.vue`, import.meta.url), 'utf8')
}

test('keeps merchant trust signals in the purchase card only', () => {
  const header = componentSource('ApiServiceHeader')
  const panel = componentSource('ApiPurchasePanel')

  assert.doesNotMatch(header, /近期完成|首字响应|getApiMerchantDisplayName/)
  assert.match(panel, /ReputationInlineSummary/)
  assert.match(panel, /ApiMerchantBadges/)
  assert.match(panel, /可验证完成[\s\S]*商户声明最大并发/)
  assert.doesNotMatch(panel, /首字响应|getApiTTFTBandLabel|性能为商户自报|平台未测速/)
  assert.match(panel, /getApiMerchantDisplayName/)
})

test('prioritizes the actual quota price over the merchant multiplier', () => {
  const summary = componentSource('ApiServiceSummary')
  const priceIndex = summary.indexOf('购买价格')
  const multiplierIndex = summary.indexOf('模型计费倍率')

  assert.ok(priceIndex >= 0)
  assert.ok(multiplierIndex > priceIndex)
  assert.match(summary, /可售额度/)
  assert.match(summary, /服务有效期/)
  assert.match(summary, /接入类型/)
  assert.match(summary, /<ApiServiceHealthPanel :summary="service\.healthSummary"/)
  assert.match(summary, /商户声明最大并发/)
  assert.match(summary, /付款窗口/)
  assert.match(summary, /号池/)
  assert.match(summary, /商户退款承诺/)
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
  assert.match(dialog, /<Checkbox v-model="acknowledged"/)
  assert.match(dialog, /submitting \|\| !acknowledged/)
})

test('does not rewrite mixed query families after creating an order', () => {
  const page = pageSource('ApiServiceDetailPage')
  const panel = componentSource('ApiPurchasePanel')

  assert.doesNotMatch(page, /setQueriesData/)
  assert.match(page, /setQueryData\(\['api-orders', 'buyer', order\.id\], order\)/)
  assert.match(page, /invalidateQueries\(\{ queryKey: \['api-purchase-intents'\] \}\)/)
  assert.match(panel, /import \{ Badge \} from '@\/components\/ui\/badge'/)
})

test('keeps summary and secondary information visible in one continuous left column', () => {
  const details = componentSource('ApiServiceDetailsTabs')
  const page = pageSource('ApiServiceDetailPage')

  assert.match(details, /模型价格/)
  assert.match(details, /购买须知/)
  assert.match(details, /平台健康数据只反映当前探测模型与平台单节点/)
  assert.match(details, /最大并发仍为商户声明/)
  assert.doesNotMatch(details, /首字响应|平台未进行统一测速/)
  assert.doesNotMatch(details, /role="tablist"|aria-selected/)
  assert.match(page, /<div class="min-w-0 space-y-4">[\s\S]*ApiServiceSummary[\s\S]*ApiServiceDetailsTabs/)
})

test('formats visible backend timestamps as Beijing time', () => {
  const prices = componentSource('ModelPriceTable')

  assert.match(prices, /formatBeijingDateTime\(service\.officialPricingUpdatedAt\)/)
  assert.doesNotMatch(prices, /最终由双方站外确认/)
})

test('does not render a buyer-only sold-out detail fallback', () => {
  const page = pageSource('ApiServiceDetailPage')
  const panel = componentSource('ApiPurchasePanel')

  assert.doesNotMatch(page, /packageSoldOut|package_sold_out|短期流量包已售罄/)
  assert.doesNotMatch(panel, /packageSoldOut|所有套餐已售罄|暂时售罄/)
})
