import type {
  SourceAuthorResourceSummary,
  SourceAuthorVerificationStatus,
} from '@/data/mock'

const sourceAuthorLabels: Record<SourceAuthorVerificationStatus, string> = {
  not_submitted: '原帖作者未验证',
  pending: '原帖作者待核验',
  verified: '原帖作者已验证',
  mismatch: '原帖作者不一致',
  expired: '原帖作者验证已过期',
}

const sourceAuthorPriority: Record<SourceAuthorVerificationStatus, number> = {
  mismatch: 0,
  expired: 1,
  not_submitted: 2,
  pending: 3,
  verified: 4,
}

export function sourceAuthorVerificationStatus(
  summary: SourceAuthorResourceSummary | undefined,
): SourceAuthorVerificationStatus {
  return summary?.status ?? 'not_submitted'
}

export function sourceAuthorVerificationLabel(
  summary: SourceAuthorResourceSummary | undefined,
) {
  return sourceAuthorLabels[sourceAuthorVerificationStatus(summary)]
}

export function sourceAuthorVerificationRank(
  summary: SourceAuthorResourceSummary | undefined,
) {
  return sourceAuthorPriority[sourceAuthorVerificationStatus(summary)]
}

export function isSourceAuthorVerified(
  summary: SourceAuthorResourceSummary | undefined,
) {
  return sourceAuthorVerificationStatus(summary) === 'verified'
}
