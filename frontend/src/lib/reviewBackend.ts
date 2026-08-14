import type { PublicReviewRecord } from '@/data/mock'
import type { ReviewCenterData, ReviewCenterRow, ReviewTag, SubmitReviewPayload } from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import { collectCursorPages, normalizeNextCursor, type CursorPage, type CursorPageRequest } from '@/lib/cursorPagination'

type ListResponse<T> = {
  items: T[]
  nextCursor?: string | null
}

type BackendReviewCenterRow = {
  id: string
  transactionType: 'carpool_membership' | 'api_order'
  transactionId: string
  direction: 'pending' | 'sent' | 'received'
  target: string
  counterpartyUsername: string
  counterpartyName: string
  reviewerRole: 'buyer' | 'seller'
  revieweeRole: 'buyer' | 'seller'
  status: 'reviewable' | 'expired' | 'sealed' | 'published' | 'removed'
  visibility: 'none' | 'sealed' | 'published' | 'removed'
  allowedTags: ReviewTag[]
  canCreate: boolean
  canEdit: boolean
  rating: number | null
  tags: string[]
  note: string | null
  completedAt: string
  reviewDeadlineAt: string
  submittedAt: string | null
  visibleAt: string | null
  frozenAt: string | null
  createdAt: string
  updatedAt: string
  version: number
}

type BackendReviewCenterResponse = {
  items: BackendReviewCenterRow[]
  presetTags: ReviewTag[]
  nextCursor?: string | null
}

export type ReviewCenterPage = CursorPage<ReviewCenterRow> & {
  presetTags: ReviewTag[]
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

function mapReviewCenterRow(row: BackendReviewCenterRow): ReviewCenterRow {
  return {
    id: row.id,
    transactionType: row.transactionType,
    transactionId: row.transactionId,
    direction: row.direction,
    target: row.target,
    counterparty: row.counterpartyName || row.counterpartyUsername,
    counterpartyUsername: row.counterpartyUsername,
    reviewerRole: row.reviewerRole,
    revieweeRole: row.revieweeRole,
    status: row.status,
    visibility: row.visibility,
    allowedTags: Array.isArray(row.allowedTags) ? row.allowedTags : [],
    canCreate: row.canCreate,
    canEdit: row.canEdit,
    rating: row.rating ?? null,
    tags: Array.isArray(row.tags) ? row.tags : [],
    note: row.note,
    completedAt: row.completedAt,
    reviewDeadlineAt: row.reviewDeadlineAt,
    submittedAt: row.submittedAt,
    visibleAt: row.visibleAt,
    frozenAt: row.frozenAt,
    createdAt: row.createdAt,
    updatedAt: row.updatedAt,
    version: row.version,
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

export async function backendReviewCenterPage(direction: ReviewCenterRow['direction'] | undefined, page: CursorPageRequest = {}): Promise<ReviewCenterPage> {
  await ensureBackendSession('buyer', false)
  const params = new URLSearchParams()
  if (direction) params.set('direction', direction)
  if (page.limit) params.set('limit', String(page.limit))
  if (page.cursor) params.set('cursor', page.cursor)
  const query = params.toString()
  const response = await backendRequest<BackendReviewCenterResponse>(`/api/v1/me/reviews${query ? `?${query}` : ''}`)
  return {
    items: response.items.map(mapReviewCenterRow),
    presetTags: Array.isArray(response.presetTags) ? response.presetTags : [],
    nextCursor: normalizeNextCursor(response.nextCursor),
  }
}

export async function backendReviewCenterRows(): Promise<ReviewCenterData> {
  let presetTags: ReviewTag[] = []
  const items = await collectCursorPages(async (page) => {
    const response = await backendReviewCenterPage(undefined, page)
    presetTags = response.presetTags
    return response
  })
  return { items, presetTags }
}

export async function backendSubmitReview(payload: SubmitReviewPayload): Promise<ReviewCenterRow> {
  await ensureBackendSession('buyer', false)
  const response = await backendMutation<BackendReviewCenterRow>(
    `/api/v1/me/transactions/${encodeURIComponent(payload.transactionType)}/${encodeURIComponent(payload.transactionId)}/review`,
    {
      rating: payload.rating,
      tags: payload.tags,
      note: payload.note,
    },
    {
      method: payload.operation === 'edit' ? 'PUT' : 'POST',
      idempotencyPrefix: `review-${payload.operation}-${payload.transactionType}`,
    },
  )
  return mapReviewCenterRow(response)
}

export async function backendPublicUserReviews(username: string): Promise<PublicReviewRecord[]> {
  const response = await backendRequest<ListResponse<BackendPublicReview>>(`/api/v1/users/${encodeURIComponent(username)}/reviews`)
  return response.items.map(mapPublicReview)
}
