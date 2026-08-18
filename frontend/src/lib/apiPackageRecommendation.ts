import type { ApiService, ApiServicePackage, ApiServicePackageModel } from '@/lib/api'

export type ApiPackageRecommendation = {
  service: ApiService
  package: ApiServicePackage
  selectedModel: ApiServicePackageModel
  matchedModels: ApiServicePackageModel[]
  recommendationEligible: boolean
  declaredUnitCost: number | null
  score: number | null
  valueScore: number | null
  fulfillmentScore: number | null
  responseScore: number | null
  freshnessScore: number | null
}

type RecommendationCandidate = Omit<ApiPackageRecommendation, 'score' | 'valueScore' | 'fulfillmentScore' | 'responseScore' | 'freshnessScore'> & { declaredUnitCost: number }

const finiteOr = (value: number, fallback: number) => Number.isFinite(value) ? value : fallback

const fulfillmentScore = (service: ApiService) => {
  if (service.completed30d === null || service.unresolvedDisputes === null) return 50
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
  modelCatalogIds: string | string[],
  durationDays: number,
  now = new Date(),
  multiplierMax?: number,
): ApiPackageRecommendation[] => {
  const selectedIds = new Set((Array.isArray(modelCatalogIds) ? modelCatalogIds : [modelCatalogIds]).filter(Boolean))
  const selectedDuration = [1, 3, 7, 30].includes(durationDays) ? durationDays : null
  const recommendationEligible = selectedIds.size === 1 && selectedDuration !== null

  const candidates: RecommendationCandidate[] = []
  for (const service of services) {
    if (!service.publiclyOrderable || service.billingMode !== 'fixed_package') continue
    for (const item of service.packages ?? []) {
      const matchedModels = selectedIds.size
        ? item.models.filter(model => selectedIds.has(model.modelCatalogId) && (multiplierMax === undefined || model.merchantMultiplier <= multiplierMax))
        : item.models.filter(model => multiplierMax === undefined || model.merchantMultiplier <= multiplierMax)
      const selectedModel = matchedModels[0]
      if (!item.enabled || item.stockAvailable <= 0 || (selectedDuration !== null && item.durationDays !== selectedDuration) || !selectedModel || item.panelAllowance <= 0) continue
      candidates.push({
        service,
        package: item,
        selectedModel,
        matchedModels,
        recommendationEligible,
        declaredUnitCost: recommendationEligible ? item.priceCny * selectedModel.merchantMultiplier / item.panelAllowance : 0,
      })
    }
  }
  if (!candidates.length) return []

  if (!recommendationEligible) {
    return candidates.map(item => ({
      ...item,
      declaredUnitCost: null,
      score: null,
      valueScore: null,
      fulfillmentScore: null,
      responseScore: null,
      freshnessScore: null,
    })).sort((left, right) =>
      new Date(right.service.serviceUpdatedAt ?? 0).getTime() - new Date(left.service.serviceUpdatedAt ?? 0).getTime()
      || left.package.sortOrder - right.package.sortOrder
      || left.package.id.localeCompare(right.package.id),
    )
  }

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
