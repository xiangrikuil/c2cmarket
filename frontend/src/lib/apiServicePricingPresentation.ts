import type { ApiService, ApiServicePackage } from '@/data/mock'
import { compareDecimal, divideDecimal, formatDecimal, multiplyDecimalDown, normalizeDecimalTrimmed } from '@/lib/decimal'

type ApiServicePricingSource = Pick<
  ApiService,
  'billingMode' | 'packages' | 'creditPerCny' | 'cnyPerUsdAllowance' | 'minimumPurchaseCny' | 'maxBuy'
>

type ApiServicePurchaseSource = Pick<
  ApiService,
  'billingMode' | 'creditPerCny' | 'cnyPerUsdAllowance' | 'minimumPurchaseCny' | 'maxBuy' | 'availableUsdAllowance'
>

export type ApiServicePricePresentation = {
  fixedPackage: boolean
  label: '套餐价格' | '购买价格'
  value: string
  secondary: string
  minimumPriceCny: number | null
  packageCount: number
  stockAvailable: number
}

export function apiServiceCnyPerUsdAllowance(
  service: Pick<ApiService, 'creditPerCny' | 'cnyPerUsdAllowance'>,
) {
  if (service.cnyPerUsdAllowance) return service.cnyPerUsdAllowance
  if (service.creditPerCny <= 0) return ''
  return divideDecimal('1', String(service.creditPerCny), 4)
}

export function formatCnyPerUsdQuota(
  service: Pick<ApiService, 'creditPerCny' | 'cnyPerUsdAllowance'>,
) {
  const rate = apiServiceCnyPerUsdAllowance(service)
  if (!rate) return '—'
  return `¥${formatDecimal(rate, 2, 4)} / $1`
}

export function maximumPurchaseCnyForInventory(availableUsdAllowance: string | number, cnyPerUsdAllowance: string | number) {
  return Number(multiplyDecimalDown(availableUsdAllowance, cnyPerUsdAllowance, 2))
}

export function isApiServiceTailOrder(
  service: Pick<ApiService, 'billingMode' | 'minimumPurchaseCny' | 'maxBuy'>,
) {
  return service.billingMode === 'metered_credit'
    && Number.isFinite(service.maxBuy)
    && service.maxBuy >= 0.01
    && compareDecimal(String(service.maxBuy), String(service.minimumPurchaseCny)) < 0
}

export function requestedUsdAllowanceForApiServicePurchase(service: ApiServicePurchaseSource, requestedCnyAmount: string) {
  if (service.billingMode === 'fixed_package') return ''
  if (isApiServiceTailOrder(service)) {
    return normalizeDecimalTrimmed(service.availableUsdAllowance || '0', 6)
  }
  const rate = apiServiceCnyPerUsdAllowance(service)
  return rate ? normalizeDecimalTrimmed(divideDecimal(requestedCnyAmount, rate, 6), 6) : ''
}

export function availableApiServicePackages(
  service: Pick<ApiService, 'packages'>,
) {
  return (service.packages ?? []).filter(item => item.enabled && item.stockAvailable > 0)
}

function packagePriceValue(packages: ApiServicePackage[]) {
  if (!packages.length) return '暂无可售套餐'
  const prices = packages.map(item => item.priceCny)
  const minimum = Math.min(...prices)
  const maximum = Math.max(...prices)
  if (minimum === maximum) return `¥${formatDecimal(minimum, 0, 2)}`
  return `¥${formatDecimal(minimum, 0, 2)}–¥${formatDecimal(maximum, 0, 2)}`
}

export function getApiServicePricePresentation(
  service: ApiServicePricingSource,
  selectedPackage?: ApiServicePackage | null,
): ApiServicePricePresentation {
  if (service.billingMode !== 'fixed_package') {
    const tailOrder = isApiServiceTailOrder(service)
    return {
      fixedPackage: false,
      label: '购买价格',
      value: formatCnyPerUsdQuota(service),
      secondary: tailOrder
        ? `尾单 ¥${formatDecimal(service.maxBuy, 0, 2)} · 一次买完`
        : `最低 ¥${formatDecimal(service.minimumPurchaseCny, 0, 2)} 起购`,
      minimumPriceCny: tailOrder ? service.maxBuy : service.minimumPurchaseCny,
      packageCount: 0,
      stockAvailable: 0,
    }
  }

  const packages = availableApiServicePackages(service)
  const visiblePackages = selectedPackage ? [selectedPackage] : packages
  const stockAvailable = packages.reduce((total, item) => total + item.stockAvailable, 0)
  const minimumPriceCny = packages.length ? Math.min(...packages.map(item => item.priceCny)) : null
  const secondary = selectedPackage
    ? `${selectedPackage.durationDays} 天 · 面板额度 $${formatDecimal(selectedPackage.panelAllowance, 0, 6)}`
    : packages.length
      ? `${packages.length} 款套餐 · 剩余 ${stockAvailable} 份`
      : '当前没有可购买的套餐'

  return {
    fixedPackage: true,
    label: '套餐价格',
    value: packagePriceValue(visiblePackages),
    secondary,
    minimumPriceCny,
    packageCount: packages.length,
    stockAvailable,
  }
}
