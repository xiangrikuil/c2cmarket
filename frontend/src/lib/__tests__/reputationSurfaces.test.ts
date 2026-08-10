import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  isPassiveReputationEvidence,
  publicReputationBadges,
  snapshotToSummary,
} from '../reputationPresentation'
import type { ReputationSnapshot, ReputationSummary } from '@/types/reputation'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

function reputationSummary(overrides: Partial<ReputationSummary> = {}): ReputationSummary {
  return {
    role: 'seller',
    scope: 'overall',
    tier: 'high_trust',
    state: 'active',
    confidence: 'high',
    ruleVersion: 'reputation-v2',
    completedCount: 18,
    roleCompletionRate: 0.98,
    roleFaultCancelRate: 0.02,
    hasUnknownCancellation: false,
    unresolvedDisputes: 0,
    activeRestrictions: 0,
    verifiedReviewCount: 12,
    weightedRating: 4.82,
    sourceAuthorVerification: {
      state: 'verified',
      counts: { total: 1, notSubmitted: 0, pending: 0, verified: 1, mismatch: 0, expired: 0 },
    },
    warnings: [],
    badges: ['high_trust', 'stable_completion', 'recent_completion'],
    calculatedAt: '2026-07-24T00:00:00Z',
    ...overrides,
  }
}

function reputationSnapshot(): ReputationSnapshot {
  return {
    userId: '11111111-1111-4111-8111-111111111111',
    role: 'buyer',
    scope: 'api',
    tier: 'normal',
    state: 'active',
    confidence: 'low',
    ruleVersion: 'reputation-v2',
    metrics: {
      completedCount: 0,
      completedCountLast90Days: 0,
      roleResponsibilityCancellationCount: 0,
      unknownResponsibilityCancellationCount: 1,
      roleControllableTerminalCount: 0,
      roleCompletionRate: null,
      roleFaultCancelRate: null,
      verifiedReviewCount: 0,
      rawAverageRating: null,
      weightedRating: null,
      ratingDistribution: { 1: 0, 2: 0, 3: 0, 4: 0, 5: 0 },
      recentReviewCount90Days: 0,
      commonPositiveTags: [],
      commonNegativeTags: [],
      confirmedFaultDisputeCount365Days: 0,
      confirmedMajorFaultDisputeCount365Days: 0,
      unresolvedDisputeCount: 0,
      activeRestrictionCount: 0,
      sourceAuthorVerification: {
        state: 'not_applicable',
        counts: { total: 0, notSubmitted: 0, pending: 0, verified: 0, mismatch: 0, expired: 0 },
      },
    },
    warnings: [],
    badges: [],
    progress: [],
    tierEnteredAt: '2026-07-24T00:00:00Z',
    reliableSince: null,
    stateEnteredAt: '2026-07-24T00:00:00Z',
    calculatedAt: '2026-07-24T00:00:00Z',
    sourceDataUpdatedAt: null,
    nextRecalculationAt: null,
  }
}

describe('信誉摘要规则', () => {
  it('公共徽章最多显示两个并优先补充原帖作者验证', () => {
    const badges = publicReputationBadges(reputationSummary())

    assert.equal(badges.length, 2)
    assert.equal(badges[0], 'source_verified')
    assert.equal(new Set(badges).size, badges.length)
  })

  it('从快照生成摘要时保留真实零值和未知比率', () => {
    const summary = snapshotToSummary(reputationSnapshot())

    assert.equal(summary.completedCount, 0)
    assert.equal(summary.roleCompletionRate, null)
    assert.equal(summary.roleFaultCancelRate, null)
    assert.equal(summary.hasUnknownCancellation, true)
    assert.equal(summary.weightedRating, null)
  })

  it('评价数量和修正评分属于被动证据', () => {
    const item = {
      label: '已验证评价',
      status: 'unavailable' as const,
      currentValue: 0,
      requiredValue: null,
      remainingValue: null,
      actionLabel: null,
      actionUrl: null,
    }

    assert.equal(isPassiveReputationEvidence({ ...item, code: 'verified_reviews' }), true)
    assert.equal(isPassiveReputationEvidence({ ...item, code: 'weighted_rating' }), true)
    assert.equal(isPassiveReputationEvidence({ ...item, code: 'completed_count' }), false)
  })
})

describe('信誉页面接线', () => {
  const summaryCard = source('../../components/reputation/ReputationSummaryCard.vue')
  const inlineSummary = source('../../components/reputation/ReputationInlineSummary.vue')
  const progressList = source('../../components/reputation/ReputationProgressList.vue')
  const carpoolDetail = source('../../pages/CarpoolDetailPage.vue')
  const carpoolList = source('../../pages/CarpoolsPage.vue')
  const apiMarket = source('../../pages/ApiMarketPage.vue')
  const apiFreeServiceCard = source('../../components/api-market/ApiFreeServiceCard.vue')
  const purchasePanel = source('../../components/api-service-detail/ApiPurchasePanel.vue')
  const rideDetail = source('../../pages/CarpoolApplicationDetailPage.vue')
  const orderDetail = source('../../pages/ApiPurchaseOrderDetailPage.vue')
  const reviewCenter = source('../../pages/MyReviewsPage.vue')
  const adminUsers = source('../../pages/AdminUsersPage.vue')
  const adminAudit = source('../../components/reputation/AdminReputationAuditPanel.vue')
  const disputeDialog = source('../../components/admin/AdminDisputeResolutionDialog.vue')
  const myReputation = source('../../pages/MyReputationPage.vue')
  const privacy = source('../../pages/MyCenterPage.vue')
  const router = source('../../router.ts')
  const reputationBackend = source('../reputationBackend.ts')
  const reputationQueries = source('../../queries/useReputationQueries.ts')

  it('风险状态先于等级与徽章表达', () => {
    expect(summaryCard.indexOf('<Alert v-if="summary.state === \'restricted\'"')).toBeLessThan(summaryCard.indexOf('{{ reputationTierLabel(summary.tier) }}'))
    expect(summaryCard.indexOf('<Alert v-else-if="summary.state === \'caution\'"')).toBeLessThan(summaryCard.indexOf('<div v-if="badges.length"'))
    expect(summaryCard).toContain('summary.warnings[0]')
    expect(inlineSummary.indexOf("summary.state === 'restricted'")).toBeLessThan(inlineSummary.indexOf('reputationTierLabel(summary.tier)'))
    expect(inlineSummary).toContain('publicReputationBadges')
  })

  it('紧凑信誉摘要合并正常状态下的重复低证据文案', () => {
    expect(inlineSummary).toContain("props.summary?.state === 'active'")
    expect(inlineSummary).toContain("props.summary.tier === 'insufficient'")
    expect(inlineSummary).toContain("props.summary.confidence === 'low'")
    expect(inlineSummary).toContain('状态正常')
    expect(inlineSummary).toContain('交易样本较少')
    expect(inlineSummary).toContain('<template v-else>')
  })

  it('成长中心不诱导索评或展示还差多少五星', () => {
    expect(progressList).toContain('被动证据')
    expect(progressList).toContain('由已验证交易自然形成')
    expect(progressList).not.toContain('还差')
    expect(progressList).not.toContain('五星')
  })

  it('所有交易决策页使用权威信誉字段和正确视角', () => {
    expect(carpoolList).toContain(':summary="row.sellerReputation"')
    expect(apiMarket).toContain('sellerReputation: service.sellerReputation')
    expect(apiFreeServiceCard).toContain(':summary="card.sellerReputation"')
    expect(carpoolDetail).toContain(':summary="carpool.sellerReputation"')
    expect(purchasePanel).toContain(':summary="service.sellerReputation"')
    expect(rideDetail).toContain('ownerMode.value ? application.value.buyerReputation : application.value.snapshot.ownerReputation')
    expect(orderDetail).toContain('isMerchantView.value ? order.value.buyerReputation : order.value.sellerReputation')
    expect(orderDetail).toContain('counterpartyReputation.value?.roleCompletionRate')
    expect(orderDetail).toContain('isMerchantView.value ? order.value.buyer : order.value.seller')
    expect(orderDetail).toContain('订单创建时锁定的参与方')
    expect(orderDetail).toContain('apiOrderMerchantContactSnapshot(order.value)')
    expect(orderDetail).not.toContain('useApiService')
    expect(orderDetail).not.toContain('getApiMerchantProfileUrl')
    expect(orderDetail).toContain('已完成订单')
    expect(orderDetail).toContain('完成率')
    expect(orderDetail).not.toContain('<ReputationSummaryCard')
    expect(orderDetail).not.toContain('交易对手信誉')
    expect(carpoolDetail).not.toContain('信任等级${carpool.trustLevel}')
    expect(purchasePanel).not.toContain('信任等级 ${service.trustLevel}')
    expect(carpoolList).not.toContain(':trust="row.trustLevel"')
    expect(carpoolList).not.toContain('(b.trustLevel ?? -1)')
  })

  it('评价中心加载交易对手角色对应的公开信誉', () => {
    expect(reviewCenter).toContain('usePublicUserReputationQuery')
    expect(reviewCenter).toContain("selectedRow.value.direction === 'received'")
    expect(reviewCenter).toContain(':summary="counterpartyReputation"')
  })

  it('管理端提供六快照审计和真实治理操作', () => {
    expect(adminUsers).toContain('AdminReputationAuditPanel')
    expect(adminAudit).toContain('v-for="snapshot in audit.items"')
    expect(adminAudit).toContain('useRecalculateUserReputationMutation')
    expect(adminAudit).toContain('useRecalculateAllReputationMutation')
    expect(adminAudit).toContain('useCreateReputationRestrictionMutation')
    expect(adminAudit).toContain('useRevokeReputationRestrictionMutation')
    expect(adminAudit).toContain('useUpdateSourceAuthorVerificationMutation')
    expect(adminAudit).toContain('纠纷信誉裁定')
    expect(adminAudit).toContain('相关申诉')
  })

  it('真实治理请求使用后端注册的单数裁定路径', () => {
    expect(reputationBackend).toContain('/reputation-outcome`')
    expect(reputationBackend).not.toContain('/reputation-outcomes`')
  })

  it('API 订单逾期处罚使用专用建议、主体版本和跨界面失效', () => {
    expect(reputationBackend).toContain('/sanction-recommendation`')
    expect(reputationBackend).toContain('/sanction`')
    expect(reputationBackend).toContain("ifMatch: input.expectedUserVersion")
    expect(reputationQueries).toContain('useAPIOrderSanctionRecommendationQuery')
    expect(reputationQueries).toContain('useApplyAPIOrderSanctionMutation')
    expect(reputationQueries).toContain("reputationQueryKeys.my()")
    expect(reputationQueries).toContain("['admin-dispute-resolution', input.disputeCaseId]")
    expect(disputeDialog).toContain('近 180 天确认逾期')
    expect(disputeDialog).toContain('暂停新接单、发布和恢复')
    expect(disputeDialog).toContain('expectedUserVersion: recommendation.subjectUserVersion')
    expect(disputeDialog).toContain('sanctionConfirmed.value = false')
    expect(disputeDialog).toContain("['VERSION_CONFLICT', 'INVALID_STATE_TRANSITION']")
    const conflictHandler = disputeDialog.slice(
      disputeDialog.indexOf("if (error instanceof BackendProblemError && ['VERSION_CONFLICT', 'INVALID_STATE_TRANSITION'].includes(error.code))"),
      disputeDialog.indexOf("sanctionSubmitError.value = errorMessage(error, 'API 服务限制创建失败。')"),
    )
    expect(conflictHandler).toContain('sanctionConfirmed.value = false')
    expect(conflictHandler).toContain('await sanctionQuery.refetch()')
  })

  it('个人信誉页只使用公开限制投影并准确说明存量订单边界', () => {
    expect(myReputation).toContain('data.value?.activeRestrictions')
    expect(myReputation).toContain('当前暂停 API 服务新接单、发布和恢复')
    expect(myReputation).toContain('已成立订单仍可继续付款、交付、完成、售后和纠纷处理')
    expect(myReputation).not.toContain('internalReason')
    expect(myReputation).not.toContain('createdByAdminId')
    expect(myReputation).not.toContain('sourceDisputeOutcomeId')
    expect(myReputation).not.toContain('sourceDisputeRemedyId')
  })

  it('隐私设置说明公共最小信誉不可隐藏', () => {
    expect(privacy).toContain('公共最小信誉摘要')
    expect(privacy).toContain('不会把公共信誉摘要改成零')
  })

  it('个人信誉页兼容原始 /me/reputation 契约', () => {
    expect(router).toContain("path: '/my/reputation', alias: '/me/reputation'")
  })
})

it('管理员用户信誉审计使用服务端账号版本', () => {
  const adminUsers = source('../../pages/AdminUsersPage.vue')
  assert.equal(adminUsers.includes(':user-version="selectedDetail.user.version"'), true)
  assert.equal(adminUsers.includes('reputationVersion'), false)
})
