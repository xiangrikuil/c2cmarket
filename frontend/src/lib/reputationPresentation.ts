import type {
  ReputationProgressItem,
  ReputationScope,
  ReputationSnapshot,
  ReputationSummary,
} from '@/types/reputation'

const badgeLabels: Record<string, string> = {
  high_trust: '高可信表现',
  reliable: '较可靠表现',
  source_verified: '原帖作者已验证',
  recent_completion: '近期有完成',
  stable_completion: '履约稳定',
}

export function reputationRoleLabel(role: ReputationSummary['role']) {
  return role === 'buyer' ? '买家信誉' : '卖家信誉'
}

export function reputationScopeLabel(scope: ReputationScope) {
  if (scope === 'carpool') return '订阅拼车'
  if (scope === 'api') return 'API 服务'
  return '综合'
}

export function reputationTierLabel(tier: ReputationSummary['tier']) {
  if (tier === 'high_trust') return '高可信'
  if (tier === 'reliable') return '较可靠'
  if (tier === 'normal') return '正常'
  return '证据不足'
}

export function reputationStateLabel(state: ReputationSummary['state']) {
  if (state === 'restricted') return '存在有效限制'
  if (state === 'caution') return '需要谨慎核对'
  return '状态正常'
}

export function reputationConfidenceLabel(confidence: ReputationSummary['confidence']) {
  if (confidence === 'high') return '证据较充分'
  if (confidence === 'medium') return '证据一般'
  return '证据较少'
}

export function reputationBadgeLabel(code: string) {
  return badgeLabels[code] ?? code.replaceAll('_', ' ')
}

export function publicReputationBadges(summary: ReputationSummary) {
  const codes = [...summary.badges]
  if (
    summary.role === 'seller'
    && summary.sourceAuthorVerification.state === 'verified'
    && !codes.includes('source_verified')
  ) {
    codes.unshift('source_verified')
  }
  return [...new Set(codes)].slice(0, 2)
}

export function snapshotToSummary(snapshot: ReputationSnapshot): ReputationSummary {
  return {
    role: snapshot.role,
    scope: snapshot.scope,
    tier: snapshot.tier,
    state: snapshot.state,
    confidence: snapshot.confidence,
    ruleVersion: snapshot.ruleVersion,
    completedCount: snapshot.metrics.completedCount,
    roleCompletionRate: snapshot.metrics.roleCompletionRate,
    roleFaultCancelRate: snapshot.metrics.roleFaultCancelRate,
    hasUnknownCancellation: snapshot.metrics.unknownResponsibilityCancellationCount > 0,
    unresolvedDisputes: snapshot.metrics.unresolvedDisputeCount,
    activeRestrictions: snapshot.metrics.activeRestrictionCount,
    verifiedReviewCount: snapshot.metrics.verifiedReviewCount,
    weightedRating: snapshot.metrics.weightedRating,
    sourceAuthorVerification: snapshot.metrics.sourceAuthorVerification,
    warnings: [...snapshot.warnings],
    badges: [...snapshot.badges],
    calculatedAt: snapshot.calculatedAt,
  }
}

export function isPassiveReputationEvidence(item: ReputationProgressItem) {
  return item.code === 'verified_reviews' || item.code === 'weighted_rating'
}

export function reputationProgressValue(item: ReputationProgressItem) {
  if (item.currentValue === null) return '暂无数据'
  if (item.code.includes('rate')) return `${Math.round(item.currentValue * 100)}%`
  if (item.code === 'weighted_rating') return item.currentValue.toFixed(2)
  if (item.code === 'reliable_continuity_days') return `${Math.floor(item.currentValue)} 天`
  return String(Math.round(item.currentValue))
}

export function reputationProgressTarget(item: ReputationProgressItem) {
  if (item.requiredValue === null || item.status === 'unavailable') return null
  if (item.code.includes('rate')) return `${Math.round(item.requiredValue * 100)}%`
  if (item.code === 'reliable_continuity_days') return `${Math.round(item.requiredValue)} 天`
  return String(Math.round(item.requiredValue))
}
