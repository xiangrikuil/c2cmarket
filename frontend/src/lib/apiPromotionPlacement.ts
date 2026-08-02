import type { ApiServicePromotion } from '@/lib/apiMarketBackend'

export function firstPromotionForBillingMode(
  promotions: ApiServicePromotion[],
  fixedPackage: boolean,
  matchesCurrentFilters: (promotion: ApiServicePromotion) => boolean = () => true,
) {
  return promotions.find(item => (item.service.billingMode === 'fixed_package') === fixedPackage && matchesCurrentFilters(item))
}

export function placePromotionFirst<T extends { promotion?: ApiServicePromotion }>(
  naturalRows: T[],
  promotion: ApiServicePromotion | undefined,
  resolvePromotedRow: (rows: T[], promotion: ApiServicePromotion) => T | undefined,
  serviceId: (row: T) => string,
): T[] {
  if (!promotion) return naturalRows
  const promotedRow = resolvePromotedRow(naturalRows, promotion)
  if (!promotedRow) return naturalRows
  return [
    { ...promotedRow, promotion },
    ...naturalRows.filter(row => serviceId(row) !== promotion.service.id),
  ]
}
