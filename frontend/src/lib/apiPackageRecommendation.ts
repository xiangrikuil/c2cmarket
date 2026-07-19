import type { ApiService, ApiServicePackage, ApiServicePackageModel } from '@/lib/api'

export type ApiPackageRecommendation = {
  service: ApiService
  package: ApiServicePackage
  selectedModel: ApiServicePackageModel
  declaredUnitCost: number
  score: number
  valueScore: number
  fulfillmentScore: number
  responseScore: number
  freshnessScore: number
}

type RecommendationCandidate = Omit<ApiPackageRecommendation, 'score' | 'valueScore' | 'fulfillmentScore' | 'responseScore' | 'freshnessScore'>

const finiteOr = (value: number, fallback: number) => Number.isFinite(value) ? value : fallback

const fulfillmentScore = (service: ApiService) => {
  const completed = Math.max(0, service.completed30d)
  const disputes = Math.max(0, service.unresolvedDisputes)
  return 100 * (completed + 2) / (completed + disputes + 4)
}

const responseScore = (service: ApiService) => {
  const minutes = service.recommendationResponseMedianMinutes
  if (minutes === null || minutes === undefined) return 50
  return 100 * 60 / (60 + Math.max(0, minutes))
}

const freshnessScore = (service: ApiService, now: Date) => {
  const updatedAt = new Date(service.serviceUpdatedAt ?? service.officialPricingUpdatedAt)
  if (Number.isNaN(updatedAt.getTime())) return 0
  const ageDays = Math.max(0, now.getTime() - updatedAt.getTime()) / 86_400_000
  return 100 * Math.exp(-ageDays / 30)
}

export const rankApiPackages = (
  services: ApiService[],
  modelCatalogId: string,
  durationDays: number,
  now = new Date(),
): ApiPackageRecommendation[] => {
  if (!modelCatalogId || ![1, 3, 7, 30].includes(durationDays)) return []

  const candidates: RecommendationCandidate[] = []
  for (const service of services) {
    if (!service.publiclyOrderable || service.billingMode !== 'fixed_package') continue
    for (const item of service.packages ?? []) {
      const selectedModel = item.models.find(model => model.modelCatalogId === modelCatalogId)
      if (!item.enabled || item.stockAvailable <= 0 || item.durationDays !== durationDays || !selectedModel || item.panelAllowance <= 0) continue
      candidates.push({
        service,
        package: item,
        selectedModel,
        declaredUnitCost: item.priceCny * selectedModel.merchantMultiplier / item.panelAllowance,
      })
    }
  }
  if (!candidates.length) return []

  const bestUnitCost = Math.min(...candidates.map(item => item.declaredUnitCost))
  return candidates.map(item => {
    const valueScore = 100 * bestUnitCost / item.declaredUnitCost
    const fulfillment = fulfillmentScore(item.service)
    const response = responseScore(item.service)
    const freshness = freshnessScore(item.service, now)
    return {
      ...item,
      valueScore,
      fulfillmentScore: fulfillment,
      responseScore: response,
      freshnessScore: freshness,
      score: finiteOr(0.60 * valueScore + 0.25 * fulfillment + 0.10 * response + 0.05 * freshness, 0),
    }
  }).sort((left, right) =>
    right.score - left.score
    || left.declaredUnitCost - right.declaredUnitCost
    || right.package.stockAvailable - left.package.stockAvailable
    || new Date(right.service.serviceUpdatedAt ?? 0).getTime() - new Date(left.service.serviceUpdatedAt ?? 0).getTime()
    || left.package.id.localeCompare(right.package.id),
  )
}
