import { BackendProblemError } from '@/lib/backendClient'
import type { PublicApiQuotaOffer } from '@/lib/api'
import { LIMITED_API_QUOTA_OFFERS_ENABLED } from '@/lib/featureFlags'

export type ApiMarketView = 'limited' | 'packages' | 'free'

export function apiMarketViewFromQuery(value: unknown): ApiMarketView {
  if (value === 'free' || value === 'packages') return value
  if (value === 'limited' && LIMITED_API_QUOTA_OFFERS_ENABLED) return value
  return 'free'
}

export function withApiMarketViewQuery<T extends Record<string, unknown>>(query: T, view: ApiMarketView) {
  return { ...query, view }
}

export function apiQuotaDurationLabel(value: string, now = Date.now()) {
  const diff = Date.parse(value) - now
  if (!Number.isFinite(diff) || diff <= 0) return '已结束'
  const totalMinutes = Math.floor(diff / 60_000)
  const days = Math.floor(totalMinutes / 1440)
  const hours = Math.floor((totalMinutes % 1440) / 60)
  const minutes = totalMinutes % 60
  if (days > 0) return `${days} 天 ${hours} 小时`
  if (hours > 0) return `${hours} 小时 ${minutes} 分`
  return `${Math.max(1, minutes)} 分钟`
}

export function apiQuotaOfferCountdown(item: PublicApiQuotaOffer, now = Date.now()) {
  if (item.isOrderable && item.currentRound) return `本轮 ${apiQuotaDurationLabel(item.currentRound.endsAt, now)} 后结束`
  if (item.nextRound) return `${apiQuotaDurationLabel(item.nextRound.startsAt, now)} 后开售`
  if (item.isOrderable) return `${apiQuotaDurationLabel(item.saleCutoffAt, now)} 后停售`
  return item.orderabilityReason
}

export function apiQuotaOfferErrorMessage(error: unknown) {
  if (!(error instanceof BackendProblemError)) return error instanceof Error ? error.message : '创建额度包订单失败。'
  const messages: Record<string, string> = {
    API_QUOTA_NOT_STARTED: '本轮尚未开始，请按页面倒计时再试。',
    API_QUOTA_ROUND_ENDED: '本轮已经结束，请等待下一轮。',
    API_QUOTA_SOLD_OUT: '本轮额度包已经售罄。',
    API_QUOTA_BUYER_ROUND_LIMIT: '你本轮已经抢到过 1 份，取消后也不能再次抢购。',
    API_QUOTA_CREDENTIAL_UNAVAILABLE: '当前买家专属交付凭据不足。',
    API_QUOTA_BATCH_EXPIRED: '该额度批次已经失效。',
  }
  return messages[error.code] ?? error.detail ?? error.message
}
