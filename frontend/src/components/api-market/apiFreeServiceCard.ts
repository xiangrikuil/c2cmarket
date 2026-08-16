import type { ConcreteProductCategoryKey } from '@/lib/productCategories'
import type { ReputationSummary } from '@/types/reputation'
import type { ApiMerchantBadge } from '@/lib/apiMerchantBadges'
import type { ApiQuotaUsagePolicy, ApiQuotaUsagePolicyInput } from '@/types/apiQuota'
import type { ApiMerchantIdentityMode } from '@/data/mock'

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
  promptAuditEnabled: boolean | null
  paymentWindowMinutes: number
  merchantName: string
  merchantIdentityMode: ApiMerchantIdentityMode
  merchantAvatarUrl?: string
  merchantType: string
  expiresAt: string
  accountPoolLabel: string
  merchantRefundCommitment: boolean
  merchantBadges: ApiMerchantBadge[]
  sellerReputation?: ReputationSummary | null
  actionHref?: string
}

type ApiModelFreshness = {
  name: string
  sortOrder?: number
  updatedAt?: string
}

export function orderSellerDeclaredApiModels(models: string[], catalog: ApiModelFreshness[]) {
  const freshnessByName = new Map(catalog.map(item => [item.name, item]))
  return models
    .map((name, index) => ({ name, index, freshness: freshnessByName.get(name) }))
    .sort((left, right) => {
      const leftOrder = left.freshness?.sortOrder
      const rightOrder = right.freshness?.sortOrder
      if (leftOrder !== undefined || rightOrder !== undefined) {
        const orderDifference = (rightOrder ?? Number.NEGATIVE_INFINITY) - (leftOrder ?? Number.NEGATIVE_INFINITY)
        if (orderDifference) return orderDifference
      }

      const leftUpdatedAt = Date.parse(left.freshness?.updatedAt ?? '')
      const rightUpdatedAt = Date.parse(right.freshness?.updatedAt ?? '')
      if (Number.isFinite(leftUpdatedAt) || Number.isFinite(rightUpdatedAt)) {
        const timeDifference = (Number.isFinite(rightUpdatedAt) ? rightUpdatedAt : Number.NEGATIVE_INFINITY)
          - (Number.isFinite(leftUpdatedAt) ? leftUpdatedAt : Number.NEGATIVE_INFINITY)
        if (timeDifference) return timeDifference
      }

      return left.index - right.index
    })
    .map(item => item.name)
}

export function compactApiServiceModels(models: string[]) {
  return {
    visibleModels: models.slice(0, 2),
    hiddenModelCount: Math.max(0, models.length - 2),
  }
}
