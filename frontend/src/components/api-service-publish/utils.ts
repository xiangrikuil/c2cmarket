import type { ModelCatalogItem } from '@/lib/api'
import {
  apiPaymentMethodLabels,
  createDefaultApiPaymentOptions,
  defaultApiPaymentWindowMinutes,
  enabledApiPaymentOptions,
} from '@/lib/apiPaymentSettings'
import type { ApiProviderCategory, ApiServicePaymentOption, ApiServicePublishForm, BillingMode, CatalogById, DistributionSystem, PublishPaymentMethod, WarrantyConfig } from './types'

export const distributionLabels: Record<DistributionSystem, string> = {
  sub2api: 'Sub2API',
  new_api_proxy: '其他 API 接入',
  other: '其他 API 接入',
}

export const publishDistributionOptions = [
  {
    value: 'sub2api',
    title: 'Sub2API',
    description: '文本模型倍率和生图倍率固定 1.00x。',
    detail: '商户配置额度售价、模型、库存、有效期和交易承诺。',
  },
  {
    value: 'other',
    title: '其他 API 接入',
    description: '适用于 NewAPI、自建中转、固定套餐或手工核对额度。',
    detail: '进入人工审核，接入细节由双方站外确认。',
  },
] satisfies Array<{
  value: Exclude<DistributionSystem, 'new_api_proxy'>
  title: string
  description: string
  detail: string
}>

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

export const simplifiedApiQuotaRules = {
  minimumPurchaseCny: 20,
  maximumPurchaseCny: 300,
  validityDays: 30,
} as const

export const defaultPaymentWindowMinutes = defaultApiPaymentWindowMinutes
export const paymentMethodLabels: Record<PublishPaymentMethod, string> = apiPaymentMethodLabels

export const apiQuotaDefaultRuleText = `默认：最低意向 ¥${simplifiedApiQuotaRules.minimumPurchaseCny}，单笔最高 ¥${simplifiedApiQuotaRules.maximumPurchaseCny}，站外确认后 ${simplifiedApiQuotaRules.validityDays} 天有效。`

export const apiQuotaBoundaryNotice = 'C2CMarket 仅提供信息撮合，不托管支付、不保存 API Key、不担保交付、不代赔。买家提交意向后，双方站外确认接入细节和售后处理。'

export const merchantNoteTemplate = [
  '接入方式：提交意向后站外确认接入细节。',
  '用量核对：用量由商户说明，买家自行核对。',
  '限速规则：请勿高并发压测或滥用。',
  '可用时间：高峰期可能响应变慢，部分模型可能临时维护。',
  '售后口径：如遇不可用，请先联系商户协商处理。',
].join('\n')

export const merchantNoteQuickInserts = [
  '建议首次提交 ¥20 意向测试',
  '提交意向后站外确认接入细节',
  '用量由商户说明，买家自行核对',
  '高峰期响应可能变慢',
  '部分模型可能临时维护',
  '滥用或高并发压测不在服务范围内',
  '平台不担保、不代赔',
] as const

export function createDefaultPaymentOptions(): ApiServicePaymentOption[] {
  return createDefaultApiPaymentOptions()
}

export function enabledPaymentOptions(form: ApiServicePublishForm) {
  return enabledApiPaymentOptions(form)
}

export function applySimplifiedApiQuotaDefaults(form: ApiServicePublishForm) {
  form.distributionSystem = 'sub2api'
  form.distributionSystemNote = 'Sub2API 标准美元额度，接入细节由双方站外确认。'
  form.billingMode = 'metered_credit'
  form.deliveryModes = ['api_key_endpoint']
  form.usageVisibility = 'merchant_confirmed'
  form.defaultMultiplier = sub2ApiPricingPolicy.textModelMultiplier
  form.minimumPurchaseCny = simplifiedApiQuotaRules.minimumPurchaseCny
  form.maximumPurchaseCny = simplifiedApiQuotaRules.maximumPurchaseCny
  form.validity = {
    mode: 'days',
    days: simplifiedApiQuotaRules.validityDays,
    startsAt: 'delivered_at',
  }
  form.manualBillingNote = ''
  form.packages = []
  form.imageCapability = {
    enabled: false,
    supportsTextToImage: false,
    supportsImageToImage: false,
    pricingMode: 'same_multiplier',
    customMultiplier: null,
    note: '',
  }
  form.warranty = {
    mode: 'no_warranty',
    warrantyDays: null,
    coverage: null,
    compensation: null,
    exclusions: null,
    refundNote: null,
  }
}

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
  if (form.distributionSystem === 'sub2api') return `${providerSummary} · API 美元额度`
  return `${providerSummary} · 其他 API 接入 ${form.billingMode === 'fixed_package' ? '固定套餐' : '手工核对额度'}`
}

export function warrantyLabel(warranty: WarrantyConfig) {
  if (warranty.mode === 'upstream_refund_only') return '上游退款跟随'
  if (warranty.mode === 'merchant_warranty') return `商户承诺 ${warranty.warrantyDays ?? 0} 天`
  return '不作承诺'
}
