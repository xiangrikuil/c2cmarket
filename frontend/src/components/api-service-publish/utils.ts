import type { ModelCatalogItem } from '@/lib/api'
import type { ApiProviderCategory, ApiServicePublishForm, BillingMode, CatalogById, DistributionSystem, PublishDeliveryMode, UsageVisibility, WarrantyConfig } from './types'

export const distributionLabels: Record<DistributionSystem, string> = {
  sub2api: 'Sub2API',
  new_api_proxy: 'NewAPI Proxy',
  other: '其他',
}

export const providerCategoryLabels: Record<ApiProviderCategory, string> = {
  gpt: 'GPT',
  claude: 'Claude',
  other: '其他',
}

export const billingLabels: Record<BillingMode, string> = {
  metered_credit: '精确额度计费',
  manual_credit: '商户手工核对额度',
  fixed_package: '固定套餐',
}

export const deliveryLabels: Record<PublishDeliveryMode, string> = {
  api_key_endpoint: 'API 请求地址接入说明',
  sub2api_panel_account: 'Sub2API 面板接入说明',
}

export const usageLabels: Record<UsageVisibility, string> = {
  panel_realtime: '面板实时可见',
  panel_balance_only: '仅面板余额可见',
  merchant_confirmed: '商户确认用量',
  fixed_package_only: '固定套餐',
  not_available: '不展示用量',
}

export const sub2ApiPricingPolicy = {
  textModelMultiplier: 1,
  imageMultiplier: 1,
  imagePrices: {
    resolution1k: 0.134,
    resolution2k: 0.201,
    resolution4k: 0.268,
  },
  minimumCnyPerUsdCredit: 0.01,
  maximumCnyPerUsdCredit: 100,
} as const

export function modelProviderCategory(provider: ModelCatalogItem['provider']): ApiProviderCategory {
  if (provider === 'openai') return 'gpt'
  if (provider === 'anthropic') return 'claude'
  return 'other'
}

export function providerLabel(provider: ModelCatalogItem['provider']) {
  if (provider === 'openai') return 'OpenAI / GPT'
  if (provider === 'anthropic') return 'Anthropic / Claude'
  return '其他'
}

export function capabilityLabel(value: ModelCatalogItem['capabilities'][number]) {
  const labels: Record<ModelCatalogItem['capabilities'][number], string> = {
    chat: '对话',
    vision: '视觉',
    image_generation: '文生图',
    image_edit: '图生图',
    reasoning: '推理',
  }
  return labels[value]
}

export function formatMultiplier(value: number) {
  return `${value.toFixed(2)}x`
}

export function formatUsdQuotaForCny(cnyPerUsdCredit: number | null, cny: number) {
  if (!cnyPerUsdCredit || cnyPerUsdCredit <= 0) return '—'
  const quota = cny / cnyPerUsdCredit
  return `$${quota.toFixed(2).replace(/\.?0+$/, '')} 美元额度`
}

export function formatPrice(value: number | null) {
  if (value === null) return '—'
  const digits = value < 1 ? 3 : 2
  return `$${value.toFixed(digits).replace(/\.?0+$/, '')}`
}

export function formatActualPrice(value: number | null, multiplier: number) {
  if (value === null) return '—'
  const actual = value * multiplier
  const digits = actual < 1 ? 3 : 2
  return `¥${actual.toFixed(digits).replace(/\.?0+$/, '')}`
}

export function selectedCatalogItems(form: ApiServicePublishForm, catalogById: CatalogById) {
  return form.selectedModels
    .filter(item => item.enabled)
    .map(item => catalogById.get(item.modelId))
    .filter((item): item is ModelCatalogItem => Boolean(item))
}

export function generatedTitle(form: ApiServicePublishForm, catalogById: CatalogById) {
  const providerSummary = providerCategoryLabels[form.providerCategory]
  if (form.distributionSystem === 'sub2api') return `${providerSummary} · Sub2API 标准额度`
  if (form.distributionSystem === 'new_api_proxy') return `${providerSummary} · NewAPI Proxy ${form.billingMode === 'fixed_package' ? '固定套餐' : '商户确认用量'}`
  return `${providerSummary} · 其他 API 服务`
}

export function warrantyLabel(warranty: WarrantyConfig) {
  if (warranty.mode === 'upstream_refund_only') return '上游退款跟随'
  if (warranty.mode === 'merchant_warranty') return `商户承诺 ${warranty.warrantyDays ?? 0} 天`
  return '不作承诺'
}
