import { shouldUseRealBackend } from '@/lib/backendClient'
import {
  backendAdminUserReputation,
  backendCreateDisputeReputationOutcome,
  backendCreateReputationRestriction,
  backendMyReputation,
  backendPublicUserReputation,
  backendRecalculateAllReputation,
  backendRecalculateUserReputation,
  backendReputationRules,
  backendRevokeReputationRestriction,
  backendSourceAuthorVerification,
  backendUpdateSourceAuthorVerification,
} from '@/lib/reputationBackend'
import {
  mockAdminUserReputation,
  mockMyReputation,
  mockPublicUserReputation,
  mockRecalculateReputation,
  mockReputationRules,
} from '@/lib/reputationMock'
import type {
  CreateDisputeOutcomeInput,
  CreateReputationRestrictionInput,
  ReputationScope,
  UpdateSourceAuthorVerificationInput,
} from '@/types/reputation'

export async function getReputationRules() {
  return shouldUseRealBackend() ? backendReputationRules() : mockReputationRules
}

export async function getPublicUserReputation(username: string, scope: ReputationScope) {
  return shouldUseRealBackend()
    ? backendPublicUserReputation(username, scope)
    : mockPublicUserReputation(username, scope)
}

export async function getMyReputation() {
  return shouldUseRealBackend() ? backendMyReputation() : mockMyReputation()
}

export async function getAdminUserReputation(userId: string, historyLimit = 50) {
  return shouldUseRealBackend()
    ? backendAdminUserReputation(userId, historyLimit)
    : mockAdminUserReputation(userId)
}

export async function recalculateUserReputation(userId: string) {
  return shouldUseRealBackend()
    ? backendRecalculateUserReputation(userId)
    : mockRecalculateReputation(1)
}

export async function recalculateAllReputation() {
  return shouldUseRealBackend()
    ? backendRecalculateAllReputation()
    : mockRecalculateReputation(3)
}

export async function createReputationRestriction(input: CreateReputationRestrictionInput) {
  if (shouldUseRealBackend()) return backendCreateReputationRestriction(input)
  throw new Error('本地演示模式不写入信誉限制。')
}

export async function revokeReputationRestriction(restrictionId: string, version: number, reason: string) {
  if (shouldUseRealBackend()) return backendRevokeReputationRestriction(restrictionId, version, reason)
  throw new Error('本地演示模式不撤销信誉限制。')
}

export async function createDisputeReputationOutcome(input: CreateDisputeOutcomeInput) {
  if (shouldUseRealBackend()) return backendCreateDisputeReputationOutcome(input)
  throw new Error('本地演示模式不写入纠纷信誉裁定。')
}

export async function getSourceAuthorVerification(resourceType: 'carpool' | 'api_service', resourceId: string) {
  if (shouldUseRealBackend()) return backendSourceAuthorVerification(resourceType, resourceId)
  throw new Error('本地演示模式没有原帖作者核验审计。')
}

export async function updateSourceAuthorVerification(input: UpdateSourceAuthorVerificationInput) {
  if (shouldUseRealBackend()) return backendUpdateSourceAuthorVerification(input)
  throw new Error('本地演示模式不写入原帖作者核验。')
}
