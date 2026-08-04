import type { ConcreteProductCategoryKey } from '@/lib/productCategories'
import type { ReputationSummary } from '@/types/reputation'
import type { ApiMerchantBadge } from '@/lib/apiMerchantBadges'
import type { ApiQuotaUsagePolicy, ApiQuotaUsagePolicyInput } from '@/types/apiQuota'

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
  quotaUsagePolicy: ApiQuotaUsagePolicy | ApiQuotaUsagePolicyInput
  maximumPurchaseCny: string | number
  multiplier: string
  declaredMaxConcurrency: string | number
  paymentWindowMinutes: number
  merchantName: string
  merchantType: string
  expiresAt: string
  accountPoolLabel: string
  merchantRefundCommitment: boolean
  merchantBadges: ApiMerchantBadge[]
  sellerReputation?: ReputationSummary | null
  actionHref?: string
}

export function compactApiServiceModels(models: string[]) {
  return {
    visibleModels: models.slice(0, 2),
    hiddenModelCount: Math.max(0, models.length - 2),
  }
}
