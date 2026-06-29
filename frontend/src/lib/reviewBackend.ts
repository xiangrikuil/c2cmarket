import type { PublicReviewRecord } from '@/data/mock'
import type { ReviewCenterRow, SubmitReviewPayload } from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'

type ListResponse<T> = {
  items: T[]
  nextCursor?: string | null
}

type BackendReviewCenterStatus = 'reviewable' | 'reviewed'

type BackendReviewCenterRow = {
  id: string
  sourceType: 'carpool_membership'
  sourceId: string
  target: string
  counterpartyUsername: string
  counterpartyName: string
  status: BackendReviewCenterStatus
  rating: number
  tags: string[]
  note: string
  createdAt: string
  updatedAt: string
}

type BackendPublicReview = {
  id: string
  username: string
  date: string
  serviceType: string
  rating: number
  tags: string[]
  note: string
  verified: boolean
}

function mapReviewStatus(status: BackendReviewCenterStatus): ReviewCenterRow['status'] {
  return status === 'reviewed' ? '已评价' : '可评价'
}

function mapReviewCenterRow(row: BackendReviewCenterRow): ReviewCenterRow {
  return {
    id: row.id,
    sourceType: 'carpool',
    sourceId: row.sourceId,
    target: row.target,
    counterparty: row.counterpartyName || row.counterpartyUsername,
    status: mapReviewStatus(row.status),
    rating: row.rating,
    tags: row.tags,
    note: row.note,
    createdAt: row.updatedAt || row.createdAt,
  }
}

function mapPublicReview(row: BackendPublicReview): PublicReviewRecord {
  return {
    id: row.id,
    username: row.username,
    date: row.date,
    serviceType: row.serviceType,
    rating: row.rating,
    tags: row.tags,
    note: row.note,
    verified: row.verified,
  }
}

export async function backendReviewCenterRows(): Promise<ReviewCenterRow[]> {
  await ensureBackendSession('buyer', false)
  const response = await backendRequest<ListResponse<BackendReviewCenterRow>>('/api/v1/me/reviews')
  return response.items.map(mapReviewCenterRow)
}

export async function backendSubmitReview(payload: SubmitReviewPayload): Promise<ReviewCenterRow> {
  await ensureBackendSession('buyer', false)
  if (payload.sourceType !== 'carpool') {
    throw new Error('当前仅支持拼车成员关系评价。')
  }
  const response = await backendMutation<BackendReviewCenterRow>(
    `/api/v1/me/reviews/carpool-memberships/${encodeURIComponent(payload.sourceId)}`,
    {
      rating: payload.rating,
      tags: payload.tags,
      note: payload.note,
    },
    {
      method: 'PUT',
      idempotencyPrefix: 'review-put',
    },
  )
  return mapReviewCenterRow(response)
}

export async function backendPublicUserReviews(username: string): Promise<PublicReviewRecord[]> {
  const response = await backendRequest<ListResponse<BackendPublicReview>>(`/api/v1/users/${encodeURIComponent(username)}/reviews`)
  return response.items.map(mapPublicReview)
}
