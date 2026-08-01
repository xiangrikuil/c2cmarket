import type { ConcreteProductCategoryKey } from '@/lib/productCategories'
import type { ReputationSummary } from '@/types/reputation'

export type ApiFreeServiceCardData = {
  title: string
  delivery: string
  models: string[]
  category: ConcreteProductCategoryKey
  categoryLabel: string
  iconSrc?: string | null
  cnyPerUsdAllowance: string | number
  minimumPurchaseCny: string | number
  availableUsdAllowance: string | number
  maximumPurchaseCny: string | number
  multiplier: string
  ttftLabel: string
  recommendedConcurrency: string | number
  paymentWindowMinutes: number
  merchantName: string
  merchantType: string
  expiresAt: string
  sellerReputation?: ReputationSummary | null
  actionHref?: string
}

export function compactApiServiceModels(models: string[]) {
  return {
    visibleModels: models.slice(0, 2),
    hiddenModelCount: Math.max(0, models.length - 2),
  }
}
