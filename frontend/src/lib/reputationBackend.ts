import { backendMutation, backendRequest } from '@/lib/backendClient'
import type {
  AdminReputationAudit,
  CreateDisputeOutcomeInput,
  CreateReputationRestrictionInput,
  DisputeReputationOutcome,
  MyReputationResponse,
  RecalculationResult,
  ReputationConfidence,
  ReputationRole,
  ReputationRuleSet,
  ReputationScope,
  ReputationScopeResponse,
  ReputationSnapshot,
  ReputationState,
  ReputationSummary,
  ReputationTier,
  SourceAuthorAggregate,
  SourceAuthorVerificationAudit,
  UpdateSourceAuthorVerificationInput,
  UserReputationRestriction,
} from '@/types/reputation'

type ReputationRulesResponse = {
  rules: ReputationRuleSet
}

type ReputationGovernanceMutation = {
  outcome?: DisputeReputationOutcome
  restriction?: UserReputationRestriction
}

const reputationRoles = ['buyer', 'seller'] as const
const reputationScopes = ['overall', 'carpool', 'api'] as const
const reputationTiers = ['insufficient', 'normal', 'reliable', 'high_trust'] as const
const reputationStates = ['active', 'caution', 'restricted'] as const
const reputationConfidences = ['low', 'medium', 'high'] as const

function requiredEnum<T extends string>(value: string, options: readonly T[], field: string): T {
  if (options.includes(value as T)) return value as T
  throw new Error(`Unsupported ${field}: ${value}`)
}

function mapRole(value: string): ReputationRole {
  return requiredEnum(value, reputationRoles, 'reputation role')
}

function mapScope(value: string): ReputationScope {
  return requiredEnum(value, reputationScopes, 'reputation scope')
}

function mapTier(value: string): ReputationTier {
  return requiredEnum(value, reputationTiers, 'reputation tier')
}

function mapState(value: string): ReputationState {
  return requiredEnum(value, reputationStates, 'reputation state')
}

function mapConfidence(value: string): ReputationConfidence {
  return requiredEnum(value, reputationConfidences, 'reputation confidence')
}

function mapSourceAuthorAggregate(value: SourceAuthorAggregate): SourceAuthorAggregate {
  return {
    state: value.state,
    counts: {
      total: value.counts.total,
      notSubmitted: value.counts.notSubmitted,
      pending: value.counts.pending,
      verified: value.counts.verified,
      mismatch: value.counts.mismatch,
      expired: value.counts.expired,
    },
  }
}

export function mapBackendReputationSnapshot(value: ReputationSnapshot): ReputationSnapshot {
  return {
    ...value,
    role: mapRole(value.role),
    scope: mapScope(value.scope),
    tier: mapTier(value.tier),
    state: mapState(value.state),
    confidence: mapConfidence(value.confidence),
    metrics: {
      ...value.metrics,
      ratingDistribution: { ...value.metrics.ratingDistribution },
      commonPositiveTags: value.metrics.commonPositiveTags.map(item => ({ ...item })),
      commonNegativeTags: value.metrics.commonNegativeTags.map(item => ({ ...item })),
      sourceAuthorVerification: mapSourceAuthorAggregate(value.metrics.sourceAuthorVerification),
    },
    warnings: [...value.warnings],
    badges: [...value.badges],
    progress: value.progress.map(item => ({ ...item })),
  }
}

export function mapBackendReputationSummary(value?: ReputationSummary | null): ReputationSummary | null {
  if (!value) return null
  return {
    ...value,
    role: mapRole(value.role),
    scope: mapScope(value.scope),
    tier: mapTier(value.tier),
    state: mapState(value.state),
    confidence: mapConfidence(value.confidence),
    sourceAuthorVerification: mapSourceAuthorAggregate(value.sourceAuthorVerification),
    warnings: [...value.warnings],
    badges: [...value.badges],
  }
}

export async function backendReputationRules() {
  const response = await backendRequest<ReputationRulesResponse>('/api/v1/reputation/rules')
  return response.rules
}

export async function backendPublicUserReputation(username: string, scope: ReputationScope) {
  const response = await backendRequest<ReputationScopeResponse>(
    `/api/v1/users/${encodeURIComponent(username)}/reputation?scope=${encodeURIComponent(scope)}`,
  )
  return {
    ...response,
    scope: mapScope(response.scope),
    reputations: response.reputations.map(mapBackendReputationSnapshot),
  }
}

export async function backendMyReputation() {
  const response = await backendRequest<MyReputationResponse>('/api/v1/me/reputation')
  return {
    ruleVersion: response.ruleVersion,
    items: response.items.map(mapBackendReputationSnapshot),
  }
}

export async function backendAdminUserReputation(userId: string, historyLimit = 50) {
  const response = await backendRequest<AdminReputationAudit>(
    `/api/v1/admin/users/${encodeURIComponent(userId)}/reputation?limit=${historyLimit}`,
  )
  return {
    ...response,
    items: response.items.map(mapBackendReputationSnapshot),
    restrictions: [...response.restrictions],
    outcomes: [...response.outcomes],
    appeals: [...response.appeals],
    history: [...response.history],
    sourceAuthorVerifications: [...response.sourceAuthorVerifications],
  }
}

export async function backendRecalculateUserReputation(userId: string) {
  return backendMutation<RecalculationResult>(
    `/api/v1/admin/users/${encodeURIComponent(userId)}/reputation/recalculate`,
    {},
  )
}

export async function backendRecalculateAllReputation() {
  return backendMutation<RecalculationResult>('/api/v1/admin/reputation/recalculate', {})
}

export async function backendCreateDisputeReputationOutcome(input: CreateDisputeOutcomeInput) {
  const response = await backendMutation<ReputationGovernanceMutation>(
    `/api/v1/admin/disputes/${encodeURIComponent(input.disputeCaseId)}/reputation-outcome`,
    {
      subjectUserId: input.subjectUserId,
      responsibility: input.responsibility,
      severity: input.severity,
      roleScope: input.roleScope,
      reasonCode: input.reasonCode,
      publicReason: input.publicReason,
      internalReason: input.internalReason,
    },
    {
      idempotencyPrefix: 'reputation-outcome',
      ifMatch: input.expectedVersion,
    },
  )
  if (!response.outcome) throw new Error('Reputation outcome response is missing the created outcome.')
  return response.outcome
}

export async function backendCreateReputationRestriction(input: CreateReputationRestrictionInput) {
  const response = await backendMutation<ReputationGovernanceMutation>(
    `/api/v1/admin/users/${encodeURIComponent(input.userId)}/reputation-restrictions`,
    {
      restrictionType: input.restrictionType,
      roleScope: input.roleScope,
      actionCode: input.actionCode,
      reasonCode: input.reasonCode,
      publicReason: input.publicReason,
      internalReason: input.internalReason,
      startsAt: input.startsAt,
      endsAt: input.endsAt,
      sourceDisputeOutcomeId: input.sourceDisputeOutcomeId ?? '',
    },
    {
      idempotencyPrefix: 'reputation-restriction',
      ifMatch: input.expectedUserVersion,
    },
  )
  if (!response.restriction) throw new Error('Reputation restriction response is missing the created restriction.')
  return response.restriction
}

export async function backendRevokeReputationRestriction(restrictionId: string, version: number, reason: string) {
  const response = await backendMutation<ReputationGovernanceMutation>(
    `/api/v1/admin/reputation-restrictions/${encodeURIComponent(restrictionId)}/revoke`,
    { reason },
    {
      idempotencyPrefix: 'reputation-restriction-revoke',
      ifMatch: version,
    },
  )
  if (!response.restriction) throw new Error('Reputation restriction response is missing the revoked restriction.')
  return response.restriction
}

export async function backendSourceAuthorVerification(
  resourceType: 'carpool' | 'api_service',
  resourceId: string,
) {
  return backendRequest<SourceAuthorVerificationAudit>(
    `/api/v1/admin/source-author-verifications/${resourceType}/${encodeURIComponent(resourceId)}`,
  )
}

export async function backendUpdateSourceAuthorVerification(input: UpdateSourceAuthorVerificationInput) {
  return backendMutation<SourceAuthorVerificationAudit>(
    `/api/v1/admin/source-author-verifications/${input.resourceType}/${encodeURIComponent(input.resourceId)}`,
    {
      status: input.status,
      actualExternalUserId: input.actualExternalUserId,
      verificationMethod: input.verificationMethod,
      expiresAt: input.expiresAt,
      failureReason: input.failureReason,
    },
    {
      method: 'PUT',
      ifMatch: input.version,
    },
  )
}
