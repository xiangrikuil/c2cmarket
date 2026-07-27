import type { ApiQuotaBatch, ApiQuotaOffer, ApiQuotaRound } from '@/lib/api'

export type ApiQuotaStatusTone = 'success' | 'waiting' | 'warning' | 'neutral'

export type ApiQuotaStatusPresentation = {
  label: string
  tone: ApiQuotaStatusTone
}

const batchStatuses: Record<ApiQuotaBatch['status'], ApiQuotaStatusPresentation> = {
  draft: { label: '草稿', tone: 'neutral' },
  published: { label: '销售中', tone: 'success' },
  paused: { label: '已暂停', tone: 'warning' },
  archived: { label: '已归档', tone: 'neutral' },
}

const offerStatuses: Record<ApiQuotaOffer['status'], ApiQuotaStatusPresentation> = {
  draft: { label: '草稿', tone: 'neutral' },
  published: { label: '销售中', tone: 'success' },
  paused: { label: '已暂停', tone: 'warning' },
  archived: { label: '已归档', tone: 'neutral' },
}

export function getApiQuotaBatchStatus(status: ApiQuotaBatch['status']) {
  return batchStatuses[status]
}

export function getApiQuotaOfferStatus(status: ApiQuotaOffer['status']) {
  return offerStatuses[status]
}

export function getApiQuotaRoundStatus(
  round: Pick<ApiQuotaRound, 'status' | 'startsAt' | 'endsAt'>,
  now = Date.now(),
): ApiQuotaStatusPresentation {
  if (round.status === 'cancelled') return { label: '已取消', tone: 'neutral' }
  if (round.status === 'closed') return { label: '已完成', tone: 'neutral' }

  const startsAt = Date.parse(round.startsAt)
  const endsAt = Date.parse(round.endsAt)
  if (Number.isFinite(endsAt) && now >= endsAt) return { label: '已完成', tone: 'neutral' }
  if (Number.isFinite(startsAt) && now >= startsAt) return { label: '进行中', tone: 'success' }
  return { label: '待开始', tone: 'waiting' }
}

export function findNextApiQuotaRound(
  rounds: Array<Pick<ApiQuotaRound, 'startsAt' | 'status'>>,
  now = Date.now(),
) {
  return rounds
    .filter(round => round.status === 'scheduled' && Date.parse(round.startsAt) > now)
    .sort((left, right) => Date.parse(left.startsAt) - Date.parse(right.startsAt))[0] ?? null
}
