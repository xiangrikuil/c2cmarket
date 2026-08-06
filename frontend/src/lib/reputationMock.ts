import type {
  AdminReputationAudit,
  MyReputationResponse,
  RecalculationResult,
  ReputationMetrics,
  ReputationRole,
  ReputationRuleSet,
  ReputationScope,
  ReputationScopeResponse,
  ReputationSnapshot,
} from '@/types/reputation'

const mockRuleVersion = 'reputation-v2'
const mockNow = '2026-07-24T12:00:00Z'

export const mockReputationRules: ReputationRuleSet = {
  version: mockRuleVersion,
  minimumNormalCompletions: 3,
  minimumReliableCompletions: 10,
  minimumHighTrustCompletions: 30,
  minimumRecentHighTrustCompletions: 3,
  minimumHighTrustReviews: 10,
  minimumReliableCompletionRate: 0.95,
  maximumReliableFaultCancelRate: 0.05,
  minimumHighTrustWeightedRating: 4.6,
  reliableContinuityDays: 90,
  bayesianPriorWeight: 5,
  platformPriorMinimumReviews: 20,
  neutralPlatformAverage: 4,
  cautionRecentFaultCount: 3,
  cautionFaultCancelRate: 0.2,
}

function metrics(role: ReputationRole, scope: ReputationScope): ReputationMetrics {
  const completedCount = role === 'seller'
    ? scope === 'overall' ? 14 : scope === 'carpool' ? 10 : 4
    : scope === 'overall' ? 6 : 3
  const verifiedReviewCount = role === 'seller' ? 8 : 3
  return {
    completedCount,
    completedCountLast90Days: Math.min(completedCount, 5),
    roleResponsibilityCancellationCount: 0,
    unknownResponsibilityCancellationCount: 0,
    roleControllableTerminalCount: completedCount,
    roleCompletionRate: 1,
    roleFaultCancelRate: 0,
    verifiedReviewCount,
    rawAverageRating: 4.8,
    weightedRating: 4.5,
    ratingDistribution: { 1: 0, 2: 0, 3: 0, 4: 2, 5: Math.max(1, verifiedReviewCount - 2) },
    recentReviewCount90Days: Math.min(verifiedReviewCount, 4),
    commonPositiveTags: [{ tag: '沟通清楚', count: 4 }, { tag: '按约完成', count: 3 }],
    commonNegativeTags: [],
    confirmedFaultDisputeCount365Days: 0,
    confirmedMajorFaultDisputeCount365Days: 0,
    unresolvedDisputeCount: 0,
    activeRestrictionCount: 0,
    sourceAuthorVerification: role === 'seller'
      ? {
          state: 'verified',
          counts: { total: 1, notSubmitted: 0, pending: 0, verified: 1, mismatch: 0, expired: 0 },
        }
      : {
          state: 'not_applicable',
          counts: { total: 0, notSubmitted: 0, pending: 0, verified: 0, mismatch: 0, expired: 0 },
        },
  }
}

export function mockReputationSnapshot(userId: string, role: ReputationRole, scope: ReputationScope): ReputationSnapshot {
  const value = metrics(role, scope)
  const tier = value.completedCount >= 10 ? 'reliable' : value.completedCount >= 3 ? 'normal' : 'insufficient'
  return {
    userId,
    role,
    scope,
    tier,
    state: 'active',
    confidence: value.completedCount >= 10 ? 'high' : value.completedCount >= 3 ? 'medium' : 'low',
    ruleVersion: mockRuleVersion,
    metrics: value,
    warnings: [],
    badges: tier === 'reliable' ? ['reliable'] : [],
    progress: [
      {
        code: 'completed_count',
        label: '可验证完成',
        status: value.completedCount >= 10 ? 'met' : 'not_met',
        currentValue: value.completedCount,
        requiredValue: value.completedCount >= 10 ? 30 : 10,
        remainingValue: value.completedCount >= 10 ? 16 : 10 - value.completedCount,
        actionLabel: null,
        actionUrl: null,
      },
      {
        code: 'completion_rate',
        label: '完成率',
        status: 'met',
        currentValue: value.roleCompletionRate,
        requiredValue: 0.95,
        remainingValue: 0,
        actionLabel: null,
        actionUrl: null,
      },
      {
        code: 'fault_cancel_rate',
        label: '责任取消率',
        status: 'met',
        currentValue: value.roleFaultCancelRate,
        requiredValue: 0.05,
        remainingValue: 0,
        actionLabel: null,
        actionUrl: null,
      },
      {
        code: 'verified_reviews',
        label: '已验证评价',
        status: 'unavailable',
        currentValue: value.verifiedReviewCount,
        requiredValue: 10,
        remainingValue: null,
        actionLabel: null,
        actionUrl: null,
      },
      {
        code: 'weighted_rating',
        label: '修正评分',
        status: 'unavailable',
        currentValue: value.weightedRating,
        requiredValue: 4.6,
        remainingValue: null,
        actionLabel: null,
        actionUrl: null,
      },
    ],
    tierEnteredAt: mockNow,
    reliableSince: tier === 'reliable' ? '2026-06-01T00:00:00Z' : null,
    stateEnteredAt: mockNow,
    calculatedAt: mockNow,
    sourceDataUpdatedAt: mockNow,
    nextRecalculationAt: null,
  }
}

function allSnapshots(userId: string) {
  return (['buyer', 'seller'] as const).flatMap(role =>
    (['overall', 'carpool', 'api'] as const).map(scope => mockReputationSnapshot(userId, role, scope)),
  )
}

export async function mockMyReputation(): Promise<MyReputationResponse> {
  return { ruleVersion: mockRuleVersion, items: allSnapshots('mock-current-user') }
}

export async function mockPublicUserReputation(username: string, scope: ReputationScope): Promise<ReputationScopeResponse> {
  const userId = `mock-${username || 'user'}`
  return {
    userId,
    scope,
    reputations: [
      mockReputationSnapshot(userId, 'buyer', scope),
      mockReputationSnapshot(userId, 'seller', scope),
    ],
  }
}

export async function mockAdminUserReputation(userId: string): Promise<AdminReputationAudit> {
  return {
    userId,
    ruleVersion: mockRuleVersion,
    items: allSnapshots(userId),
    history: [],
    restrictions: [],
    outcomes: [],
    appeals: [],
    sourceAuthorVerifications: [],
  }
}

export async function mockRecalculateReputation(userCount = 1): Promise<RecalculationResult> {
  return {
    requestedUsers: userCount,
    rebuiltStates: userCount * 6,
    completedAt: new Date().toISOString(),
  }
}
