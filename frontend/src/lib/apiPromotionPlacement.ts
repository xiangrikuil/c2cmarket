import type { ApiServicePromotion } from '@/lib/apiMarketBackend'

export type PromotionDisplayPosition = 'first' | 'middle' | 'last'

export function promotionsForBillingMode(
  promotions: ApiServicePromotion[],
  fixedPackage: boolean,
  matchesCurrentFilters: (promotion: ApiServicePromotion) => boolean = () => true,
) {
  const matching = promotions.filter(item => (
    (item.service.billingMode === 'fixed_package') === fixedPackage
    && matchesCurrentFilters(item)
  ))
  return {
    operator: matching.find(item => item.kind === 'operator'),
    reward: matching.find(item => item.kind === 'reward'),
  }
}

export function placePromotions<T extends { promotion?: ApiServicePromotion, promotionPosition?: PromotionDisplayPosition }>(
  naturalRows: T[],
  promotions: { operator?: ApiServicePromotion, reward?: ApiServicePromotion },
  resolvePromotedRow: (rows: T[], promotion: ApiServicePromotion) => T | undefined,
  serviceId: (row: T) => string,
): T[] {
  const operator = promotions.operator
  const reward = promotions.reward?.service.id === operator?.service.id ? undefined : promotions.reward
  const operatorRow = operator ? resolvePromotedRow(naturalRows, operator) : undefined
  const rewardRow = reward ? resolvePromotedRow(naturalRows, reward) : undefined
  const promotedServiceIds = new Set([
    ...(operatorRow && operator ? [operator.service.id] : []),
    ...(rewardRow && reward ? [reward.service.id] : []),
  ])
  const remaining = naturalRows.filter(row => !promotedServiceIds.has(serviceId(row)))
  const result: T[] = []

  if (operator && operatorRow) {
    result.push({ ...operatorRow, promotion: operator, promotionPosition: 'first' })
  }

  if (!reward || !rewardRow) return [...result, ...remaining]

  const insertionIndex = Math.min(3, remaining.length)
  result.push(...remaining.slice(0, insertionIndex))
  if (!(operatorRow && insertionIndex === 0)) {
    result.push({
      ...rewardRow,
      promotion: reward,
      promotionPosition: insertionIndex === remaining.length ? 'last' : 'middle',
    })
  }
  result.push(...remaining.slice(insertionIndex))
  return result
}
