import type {
  CarpoolProductCatalogItem,
  CarpoolDistributionMethod,
  CarpoolPublishForm,
  CarpoolWarrantyConfig,
  CarpoolWarrantyMode,
  CatalogProviderCode,
  OpeningChannelOption,
  OpeningChannelCode,
  PaymentMethodCode,
  PaymentMethodOption,
  RegionOption,
} from './types'
import { formatMonthlyQuota, quotaFieldLabel } from '@/lib/quota'

export const providerLabels: Record<CatalogProviderCode, string> = {
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  other: '其他',
}

export const openingChannelLabels: Record<OpeningChannelCode, string> = {
  web: 'Web 官网',
  ios_app_store: 'iOS App Store',
  google_play: 'Google Play',
  team_seat: 'Team / Business 席位',
  other: '其他',
}

export const paymentMethodLabels: Record<PaymentMethodCode, string> = {
  credit_card: '信用卡',
  virtual_card: '虚拟卡',
  apple_pay: 'Apple Pay',
  google_pay: 'Google Pay',
  app_store_gift_card: 'App Store 礼品卡',
  google_play_gift_card: 'Google Play 礼品卡',
  paypal: 'PayPal',
  other: '其他',
}

export const distributionMethodLabels: Record<CarpoolDistributionMethod, string> = {
  sub2api: 'Sub2API',
  other: '其他分发',
}

export function distributionMethodLabel(value: CarpoolDistributionMethod | '' | undefined) {
  if (!value) return '待选择'
  return distributionMethodLabels[value]
}

export function adminAccountLabel(value: boolean | null | undefined) {
  if (value === true) return '提供管理员'
  if (value === false) return '不提供管理员'
  return '待选择'
}

export function distributionFieldsComplete(form: Pick<CarpoolPublishForm, 'distributionMethod' | 'distributionMethodNote' | 'providesAdminAccount'>) {
  if (!form.distributionMethod) return false
  if (form.distributionMethod === 'other' && !form.distributionMethodNote.trim()) return false
  return form.providesAdminAccount !== null
}

export const warrantyModeLabels: Record<CarpoolWarrantyMode, string> = {
  no_warranty: '不作补偿承诺',
  remaining_days_compensation: '车主按剩余天数补偿',
  fixed_days_warranty: '固定天数车主承诺',
}

const credentialSharingRiskPattern = /(共享|共用|转交|借用).*(账号|密码|主账号|session|cookie|token|登录态)|主账号|主 key|主key|session|cookie|refresh token|api key/i
const credentialSharingProhibitionPattern = /(不得|不能|不可|禁止|不允许|拒绝|避免|不保存|不交付|不提供|不会保存|不会交付|不会提供).{0,16}(共享|共用|转交|借用|填写|粘贴|上传|提供|交换|索要|账号|密码|主账号|session|cookie|token|登录态|api key)/i

export function availableSeats(form: Pick<CarpoolPublishForm, 'totalSeats' | 'occupiedSeats'>) {
  return Math.max(form.totalSeats - form.occupiedSeats, 0)
}

export function selectedProduct(form: CarpoolPublishForm, catalogById: Map<string, CarpoolProductCatalogItem>) {
  return catalogById.get(form.productId) ?? null
}

export function selectedRegion(form: CarpoolPublishForm, regionsByCode: Map<string, RegionOption>) {
  return regionsByCode.get(form.regionCode) ?? null
}

export function isCustomRegion(form: Pick<CarpoolPublishForm, 'regionCode'>) {
  return form.regionCode === 'other'
}

export function productDisplayName(form: CarpoolPublishForm, catalogById: Map<string, CarpoolProductCatalogItem>) {
  const product = selectedProduct(form, catalogById)
  if (!product) return form.customProductName?.trim() || '待选择产品'
  if (product.id === 'other-custom') return form.customProductName?.trim() || product.displayName
  return product.displayName
}

export function hasForbiddenCredentialSharingText(value: string) {
  return credentialSharingRiskPattern.test(value) && !credentialSharingProhibitionPattern.test(value)
}

export function requiresSubscriptionRiskAck(product: CarpoolProductCatalogItem | null, form: CarpoolPublishForm) {
  if (product) return product.riskAckRequired
  return Boolean(form.riskNoticeCode)
}

export function canPublishProduct(product: CarpoolProductCatalogItem | null) {
  return product?.publishPolicy === 'allowed'
}

export function accessArrangementComplete(form: CarpoolPublishForm, product: CarpoolProductCatalogItem | null) {
  if (product && !canPublishProduct(product)) return false
  if (form.accessArrangementMode === 'not_allowed') return false
  if (form.accessArrangementNote.trim().length < 8) return false
  if (hasForbiddenCredentialSharingText(form.accessArrangementNote)) return false
  if (requiresSubscriptionRiskAck(product, form) && !form.riskAcknowledged) return false
  return true
}

export function regionDisplayName(form: CarpoolPublishForm, regionsByCode: Map<string, RegionOption>) {
  if (isCustomRegion(form)) return form.customRegionName?.trim() || ''
  return selectedRegion(form, regionsByCode)?.displayName || '待选择地区'
}

export function previewTitle(
  form: CarpoolPublishForm,
  catalogById: Map<string, CarpoolProductCatalogItem>,
  regionsByCode: Map<string, RegionOption>,
) {
  return `${productDisplayName(form, catalogById)} · ${regionDisplayName(form, regionsByCode)}拼车`
}

export function warrantyLabel(warranty: CarpoolWarrantyConfig) {
  if (warranty.mode === 'fixed_days_warranty') {
    return warranty.fixedWarrantyDays ? `车主承诺 ${warranty.fixedWarrantyDays} 天` : '固定天数车主承诺'
  }
  return warrantyModeLabels[warranty.mode]
}

export function warrantyComplete(warranty: CarpoolWarrantyConfig) {
  if (warranty.mode === 'fixed_days_warranty') {
    return Boolean(warranty.fixedWarrantyDays && warranty.fixedWarrantyDays > 0 && warranty.compensationMethod?.trim())
  }
  if (warranty.mode === 'remaining_days_compensation') {
    return Boolean(warranty.compensationMethod?.trim())
  }
  return true
}

export function warrantyPostText(warranty: CarpoolWarrantyConfig) {
  if (warranty.mode === 'no_warranty') return '不作补偿承诺'
  const parts = [warrantyLabel(warranty)]
  if (warranty.compensationMethod?.trim()) parts.push(warranty.compensationMethod.trim())
  if (warranty.exclusions?.trim()) parts.push(`不适用情形：${warranty.exclusions.trim()}`)
  return parts.join('；')
}

export function openingChannelDisplayName(code: CarpoolPublishForm['openingChannelCode'], channelsByCode: Map<string, OpeningChannelOption>) {
  if (!code) return ''
  return channelsByCode.get(code)?.displayName ?? openingChannelLabels[code]
}

export function paymentMethodDisplayNames(codes: string[], methodsByCode: Map<string, PaymentMethodOption>) {
  return codes.map(code => methodsByCode.get(code)?.displayName ?? paymentMethodLabels[code as PaymentMethodCode]).filter(Boolean)
}

export function canBuildLinuxDoPostText(
  form: CarpoolPublishForm,
  regionsByCode: Map<string, RegionOption>,
  channelsByCode: Map<string, OpeningChannelOption>,
  methodsByCode: Map<string, PaymentMethodOption>,
) {
  return Boolean(
    form.productId
    && (form.productId !== 'other-custom' || form.customProductName?.trim())
    && regionDisplayName(form, regionsByCode)
    && form.monthlyPriceCny
    && form.monthlyPriceCny > 0
    && form.totalSeats >= 1
    && form.occupiedSeats >= 0
    && form.occupiedSeats <= form.totalSeats
    && openingChannelDisplayName(form.openingChannelCode, channelsByCode)
    && paymentMethodDisplayNames(form.paymentMethodCodes, methodsByCode).length === 1
    && distributionFieldsComplete(form)
    && form.accessArrangementMode !== 'not_allowed'
    && form.accessArrangementNote.trim().length >= 8
    && warrantyComplete(form.warranty)
    && form.rulesNote.trim(),
  )
}

export function buildLinuxDoPostText(
  form: CarpoolPublishForm,
  catalogById: Map<string, CarpoolProductCatalogItem>,
  regionsByCode: Map<string, RegionOption>,
  channelsByCode: Map<string, OpeningChannelOption>,
  methodsByCode: Map<string, PaymentMethodOption>,
  listingUrl?: string,
) {
  const productName = productDisplayName(form, catalogById)
  const regionName = regionDisplayName(form, regionsByCode)
  const remaining = availableSeats(form)
  const openingChannel = openingChannelDisplayName(form.openingChannelCode, channelsByCode) || '待确认'
  const paymentMethods = paymentMethodDisplayNames(form.paymentMethodCodes, methodsByCode).join(' / ') || '待确认'
  const distributionText = form.distributionMethod === 'other'
    ? `${distributionMethodLabel(form.distributionMethod)}：${form.distributionMethodNote.trim() || '待确认'}`
    : distributionMethodLabel(form.distributionMethod)
  const product = selectedProduct(form, catalogById)
  const quotaText = formatMonthlyQuota({
    amount: form.monthlyQuotaAmount,
    label: product?.quotaLabel,
    unit: product?.quotaUnit,
    period: product?.quotaPeriod,
  }, '待确认')
  const quotaLabel = quotaFieldLabel(product)
  const priceText = form.monthlyPriceCny ? `¥${form.monthlyPriceCny}/月` : '价格待确认'

  return [
    `【拼车】${productName} ${regionName}，剩余 ${remaining} 席，${priceText}`,
    '',
    `产品：${productName}`,
    `开通区：${regionName}`,
    `席位：总 ${form.totalSeats} 人，已上车 ${form.occupiedSeats} 人，剩余 ${remaining} 席`,
    `价格：${priceText}`,
    `倍率：${form.serviceMultiplier ?? '-'}x`,
    `${quotaLabel}：${quotaText}`,
    `开通渠道：${openingChannel}`,
    `付款方式：${paymentMethods}`,
    `分发方式：${distributionText}`,
    `管理员账号：${adminAccountLabel(form.providesAdminAccount)}`,
    `访问安排：${form.accessArrangementNote.trim() || '待确认'}`,
    `售后说明：${warrantyPostText(form.warranty)}`,
    '买家须知：',
    form.rulesNote.trim() || '待补充',
    '',
    `平台车源链接：${listingUrl || '发布后补充'}`,
  ].join('\n')
}

export function clampNumber(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) return min
  return Math.min(Math.max(Math.trunc(value), min), max)
}

export function formatConfidence(value: string | null) {
  if (value === 'high') return '高'
  if (value === 'medium') return '中'
  if (value === 'low') return '低'
  return '待确认'
}
