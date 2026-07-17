import type { ApiService } from '@/lib/api'

export type ApiMerchantBadgeKind = 'quality' | 'fast_response'

export type ApiMerchantBadge = {
  kind: ApiMerchantBadgeKind
  label: string
  description: string
}

type ApiMerchantBadgeSource = Pick<ApiService,
  | 'completed30d'
  | 'publiclyOrderable'
  | 'recommendationResponseMedianMinutes'
  | 'responseMedianMinutes'
  | 'trustLevel'
  | 'unresolvedDisputes'
>

const qualityCompleted30dMinimum = 10
const fastResponseMaximumMinutes = 10

function responseMedianMinutes(service: ApiMerchantBadgeSource) {
  if (service.recommendationResponseMedianMinutes === null) return null
  return service.recommendationResponseMedianMinutes ?? service.responseMedianMinutes
}

/**
 * 商家徽章只由公开履约数据计算，避免把卖家自述或缺失数据展示成平台背书。
 */
export function getApiMerchantBadges(service: ApiMerchantBadgeSource): ApiMerchantBadge[] {
  const badges: ApiMerchantBadge[] = []

  if (
    service.trustLevel >= 3
    && service.completed30d >= qualityCompleted30dMinimum
    && service.unresolvedDisputes === 0
  ) {
    badges.push({
      kind: 'quality',
      label: '优质商家',
      description: `信任等级 3+、近 30 天完成 ${qualityCompleted30dMinimum} 单以上且无未解决纠纷`,
    })
  }

  const responseMinutes = responseMedianMinutes(service)
  if (service.publiclyOrderable && responseMinutes !== null && responseMinutes <= fastResponseMaximumMinutes) {
    badges.push({
      kind: 'fast_response',
      label: '快速响应',
      description: `响应中位数不超过 ${fastResponseMaximumMinutes} 分钟`,
    })
  }

  return badges
}
