export type ReputationRole = 'buyer' | 'seller'
export type ReputationScope = 'overall' | 'carpool' | 'api'
export type ReputationTier = 'insufficient' | 'normal' | 'reliable' | 'high_trust'
export type ReputationState = 'active' | 'caution' | 'restricted'
export type ReputationConfidence = 'low' | 'medium' | 'high'
export type ReputationProgressStatus = 'met' | 'not_met' | 'blocked' | 'unavailable'

export type ReputationTagCount = {
  tag: string
  count: number
}

export type RatingDistribution = {
  1: number
  2: number
  3: number
  4: number
  5: number
}

export type SourceAuthorAggregateState =
  | 'not_applicable'
  | 'no_sources'
  | 'pending'
  | 'partial'
  | 'verified'
  | 'mismatch'

export type SourceAuthorStatusCounts = {
  total: number
  notSubmitted: number
  pending: number
  verified: number
  mismatch: number
  expired: number
}

export type SourceAuthorAggregate = {
  state: SourceAuthorAggregateState
  counts: SourceAuthorStatusCounts
}

export type ReputationMetrics = {
  completedCount: number
  completedCountLast90Days: number
  roleResponsibilityCancellationCount: number
  unknownResponsibilityCancellationCount: number
  roleControllableTerminalCount: number
  roleCompletionRate: number | null
  roleFaultCancelRate: number | null
  verifiedReviewCount: number
  rawAverageRating: number | null
  weightedRating: number | null
  ratingDistribution: RatingDistribution
  recentReviewCount90Days: number
  commonPositiveTags: ReputationTagCount[]
  commonNegativeTags: ReputationTagCount[]
  confirmedFaultDisputeCount365Days: number
  confirmedMajorFaultDisputeCount365Days: number
  unresolvedDisputeCount: number
  activeRestrictionCount: number
  sourceAuthorVerification: SourceAuthorAggregate
}

export type ReputationProgressItem = {
  code: string
  label: string
  status: ReputationProgressStatus
  currentValue: number | null
  requiredValue: number | null
  remainingValue: number | null
  actionLabel: string | null
  actionUrl: string | null
}

export type ReputationSnapshot = {
  userId: string
  role: ReputationRole
  scope: ReputationScope
  tier: ReputationTier
  state: ReputationState
  confidence: ReputationConfidence
  ruleVersion: string
  metrics: ReputationMetrics
  warnings: string[]
  badges: string[]
  progress: ReputationProgressItem[]
  tierEnteredAt: string
  reliableSince: string | null
  stateEnteredAt: string
  calculatedAt: string
  sourceDataUpdatedAt: string | null
  nextRecalculationAt: string | null
}

export type ReputationSummary = {
  role: ReputationRole
  scope: ReputationScope
  tier: ReputationTier
  state: ReputationState
  confidence: ReputationConfidence
  ruleVersion: string
  completedCount: number
  roleCompletionRate: number | null
  roleFaultCancelRate: number | null
  hasUnknownCancellation: boolean
  unresolvedDisputes: number
  activeRestrictions: number
  verifiedReviewCount: number
  weightedRating: number | null
  sourceAuthorVerification: SourceAuthorAggregate
  warnings: string[]
  badges: string[]
  calculatedAt: string
}

export type ReputationRuleSet = {
  version: string
  minimumNormalCompletions: number
  minimumReliableCompletions: number
  minimumHighTrustCompletions: number
  minimumRecentHighTrustCompletions: number
  minimumHighTrustReviews: number
  minimumReliableCompletionRate: number
  maximumReliableFaultCancelRate: number
  minimumHighTrustWeightedRating: number
  reliableContinuityDays: number
  bayesianPriorWeight: number
  platformPriorMinimumReviews: number
  neutralPlatformAverage: number
  cautionRecentFaultCount: number
  cautionFaultCancelRate: number
}

export type ReputationScopeResponse = {
  userId: string
  scope: ReputationScope
  reputations: ReputationSnapshot[]
}

export type MyReputationResponse = {
  ruleVersion: string
  items: ReputationSnapshot[]
}

export type ReputationHistory = {
  id: string
  userId: string
  role: ReputationRole
  scope: ReputationScope
  fromTier: ReputationTier | null
  toTier: ReputationTier
  fromState: ReputationState | null
  toState: ReputationState
  ruleVersion: string
  reasonSnapshot: unknown
  createdAt: string
}

export type UserReputationRestriction = {
  id: string
  userId: string
  restrictionType: string
  roleScope: ReputationRole | 'all'
  actionCode: string
  reasonCode: string
  publicReason: string
  internalReason: string
  startsAt: string
  endsAt: string | null
  sourceDisputeOutcomeId?: string
  createdByAdminId: string
  revokedAt: string | null
  revokedByAdminId?: string
  revocationReason?: string
  createdAt: string
  updatedAt: string
  version: number
  userVersion?: number
}

export type DisputeReputationOutcome = {
  id: string
  disputeCaseId: string
  subjectUserId: string
  responsibility: 'responsible' | 'shared' | 'not_responsible' | 'undetermined'
  severity: 'none' | 'low' | 'medium' | 'high' | 'critical'
  roleScope: ReputationRole | 'all'
  status: 'active' | 'reversed'
  reasonCode: string
  publicReason: string
  internalReason: string
  decidedByAdminId: string
  decidedAt: string
  reversedAt: string | null
  reversedByAdminId?: string
  reversalAppealId?: string
  reversalReason?: string
  createdAt: string
  updatedAt: string
  version: number
  disputeVersion: number
}

export type ReputationAppealAudit = {
  id: string
  appellantUserId: string
  reportId?: string
  disputeId?: string
  targetType: string
  targetId: string
  title: string
  statement: string
  status: 'submitted' | 'approved' | 'rejected'
  adminReason: string
  handledByAdminId?: string
  handledAt: string | null
  createdAt: string
  updatedAt: string
  version: number
}

export type SourceAuthorVerificationStatus =
  | 'not_submitted'
  | 'pending'
  | 'verified'
  | 'mismatch'
  | 'expired'

export type SourceAuthorVerification = {
  id?: string
  resourceType: 'carpool' | 'api_service'
  resourceId: string
  ownerUserId?: string
  sourceUrl?: string
  expectedExternalUserId?: string
  actualExternalUserId?: string
  status: SourceAuthorVerificationStatus
  verificationMethod?: string
  verifiedByAdminId?: string
  verifiedAt?: string
  expiresAt?: string
  failureReason?: string
  createdAt?: string
  updatedAt?: string
  version: number
}

export type SourceAuthorVerificationEvent = {
  id: string
  verificationId: string
  resourceType: SourceAuthorVerification['resourceType']
  resourceId: string
  action: string
  fromStatus?: SourceAuthorVerificationStatus
  toStatus: SourceAuthorVerificationStatus
  sourceUrl: string
  expectedExternalUserId: string
  actualExternalUserId?: string
  verificationMethod?: string
  verifiedByAdminId: string
  verifiedAt?: string
  expiresAt?: string
  failureReason?: string
  version: number
  createdAt: string
}

export type SourceAuthorVerificationAudit = {
  verification: SourceAuthorVerification
  events: SourceAuthorVerificationEvent[]
}

export type AdminReputationAudit = {
  userId: string
  ruleVersion: string
  items: ReputationSnapshot[]
  history: ReputationHistory[]
  restrictions: UserReputationRestriction[]
  outcomes: DisputeReputationOutcome[]
  appeals: ReputationAppealAudit[]
  sourceAuthorVerifications: SourceAuthorVerificationAudit[]
}

export type RecalculationResult = {
  requestedUsers: number
  rebuiltStates: number
  completedAt: string
}

export type CreateReputationRestrictionInput = {
  userId: string
  restrictionType: string
  roleScope: ReputationRole | 'all'
  actionCode: string
  reasonCode: string
  publicReason: string
  internalReason: string
  startsAt: string
  endsAt: string | null
  sourceDisputeOutcomeId?: string
  expectedUserVersion: number
}

export type CreateDisputeOutcomeInput = {
  disputeCaseId: string
  subjectUserId: string
  responsibility: DisputeReputationOutcome['responsibility']
  severity: DisputeReputationOutcome['severity']
  roleScope: ReputationRole | 'all'
  reasonCode: string
  publicReason: string
  internalReason: string
  expectedVersion: number
}

export type UpdateSourceAuthorVerificationInput = {
  resourceType: SourceAuthorVerification['resourceType']
  resourceId: string
  status: SourceAuthorVerificationStatus
  actualExternalUserId: string
  verificationMethod: string
  expiresAt: string | null
  failureReason: string
  version: number
}
