import type { DisputeCase } from '@/api/generated/openapi'
import type { DisputeReputationOutcome, ReputationRole } from '@/types/reputation'

export type AdminDisputeTargetType = DisputeCase['targetType']
export type PublicResultCode = DisputeCase['publicResultCode']

export type DisputeResponsibility = DisputeReputationOutcome['responsibility']
export type DisputeSeverity = DisputeReputationOutcome['severity']
export type DisputeRoleScope = ReputationRole | 'all'

export type DisputeDetailLike = {
  id: string
  targetType: AdminDisputeTargetType
  targetId: string
  primaryUserId?: string
  primaryUsername: string
  primaryDisplayName: string
  counterpartyUserId?: string
  counterpartyUsername: string
  counterpartyName: string
  subjectUserId?: string
  subjectUsername?: string
  subjectName?: string
}

export type DisputeEvidenceParticipant = {
  role: string
  userId: string
  username: string
}

export type DisputeEvidenceSnapshot = {
  submittedTargetType: string
  submittedTargetId: string
  canonicalTargetType: string
  canonicalTargetId: string
  targetLabel: string
  reportedUsername: string
  reporterRole: string
  primaryRespondentUserId: string
  primaryRespondentUsername: string
  participants: DisputeEvidenceParticipant[]
  businessStatus: string
  hasOrder: boolean
  hasMembership: boolean
}

export type SnapshotParseResult =
  | { status: 'empty', snapshot: null }
  | { status: 'invalid', snapshot: null, message: string }
  | { status: 'valid', snapshot: DisputeEvidenceSnapshot }

export type DisputeParticipantOption = {
  userId: string
  username: string
  name: string
  roleScope: DisputeRoleScope
  roleLabel: string
}

export type ResolutionForm = {
  publicResultCode: PublicResultCode | ''
  publicSummary: string
  publicResult: string
  internalReason: string
  confirmed: boolean
}

export type OutcomeForm = {
  subjectUserId: string
  responsibility: DisputeResponsibility
  severity: DisputeSeverity
  roleScope: DisputeRoleScope
  reasonCode: string
  publicReason: string
  internalReason: string
  confirmed: boolean
}

export const publicResultOptions = [
  { value: 'no_action', label: '无需进一步处置' },
  { value: 'contact_invalid', label: '联系方式无效' },
  { value: 'impersonation_confirmed', label: '确认存在冒充' },
  { value: 'description_mismatch', label: '服务描述不一致' },
  { value: 'rule_or_seat_issue', label: '规则或席位问题' },
  { value: 'api_delivery_issue', label: 'API 接入或交付问题' },
  { value: 'other_resolved', label: '其他已处理事项' },
] as const satisfies ReadonlyArray<{ value: PublicResultCode, label: string }>

export const responsibilityOptions = [
  { value: 'responsible', label: '承担主要责任' },
  { value: 'shared', label: '双方共同责任' },
  { value: 'not_responsible', label: '不承担责任' },
  { value: 'undetermined', label: '证据不足，无法认定' },
] as const satisfies ReadonlyArray<{ value: DisputeResponsibility, label: string }>

export const severityOptions = [
  { value: 'none', label: '无' },
  { value: 'low', label: '低' },
  { value: 'medium', label: '中' },
  { value: 'high', label: '高' },
  { value: 'critical', label: '严重' },
] as const satisfies ReadonlyArray<{ value: DisputeSeverity, label: string }>

export const outcomeReasonOptions = [
  { value: 'confirmed_responsibility', label: '责任事实成立' },
  { value: 'shared_responsibility', label: '双方均有责任' },
  { value: 'not_responsible', label: '未发现主体责任' },
  { value: 'insufficient_evidence', label: '现有证据不足' },
  { value: 'other', label: '其他原因' },
] as const

const resultCodes = new Set<string>(publicResultOptions.map(item => item.value))
const responsibilities = new Set<string>(responsibilityOptions.map(item => item.value))
const severities = new Set<string>(severityOptions.map(item => item.value))
const roleScopes = new Set<string>(['buyer', 'seller', 'all'])
const transactionDisputeTargets = new Set<AdminDisputeTargetType>([
  'carpool_application',
  'carpool_membership',
  'api_purchase_intent',
  'api_order',
])
const reasonCodePattern = /^[a-z][a-z0-9_]{1,63}$/

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function stringField(record: Record<string, unknown>, field: string) {
  return typeof record[field] === 'string' ? record[field].trim() : ''
}

function booleanField(record: Record<string, unknown>, field: string) {
  return record[field] === true
}

function parseParticipants(value: unknown): DisputeEvidenceParticipant[] | null {
  if (value === undefined) return []
  if (!Array.isArray(value)) return null
  const participants: DisputeEvidenceParticipant[] = []
  for (const item of value) {
    if (!isRecord(item)) return null
    const userId = stringField(item, 'userId')
    if (!userId) return null
    participants.push({
      role: stringField(item, 'role'),
      userId,
      username: stringField(item, 'username'),
    })
  }
  return participants
}

export function parseDisputeEvidenceSnapshot(raw?: string | null): SnapshotParseResult {
  const source = raw?.trim()
  if (!source) return { status: 'empty', snapshot: null }
  let parsed: unknown
  try {
    parsed = JSON.parse(source)
  } catch {
    return { status: 'invalid', snapshot: null, message: '脱敏目标快照不是有效 JSON。' }
  }
  if (!isRecord(parsed)) {
    return { status: 'invalid', snapshot: null, message: '脱敏目标快照的数据结构不正确。' }
  }
  const participants = parseParticipants(parsed.participants)
  if (!participants) {
    return { status: 'invalid', snapshot: null, message: '脱敏目标快照中的参与方结构不正确。' }
  }
  if (parsed.containsContactValue === true) {
    return { status: 'invalid', snapshot: null, message: '脱敏目标快照包含不应展示的联系方式标记。' }
  }
  return {
    status: 'valid',
    snapshot: {
      submittedTargetType: stringField(parsed, 'submittedTargetType'),
      submittedTargetId: stringField(parsed, 'submittedTargetId'),
      canonicalTargetType: stringField(parsed, 'canonicalTargetType'),
      canonicalTargetId: stringField(parsed, 'canonicalTargetId'),
      targetLabel: stringField(parsed, 'targetLabel'),
      reportedUsername: stringField(parsed, 'reportedUsername'),
      reporterRole: stringField(parsed, 'reporterRole'),
      primaryRespondentUserId: stringField(parsed, 'primaryRespondentUserId'),
      primaryRespondentUsername: stringField(parsed, 'primaryRespondentUsername'),
      participants,
      businessStatus: stringField(parsed, 'businessStatus'),
      hasOrder: booleanField(parsed, 'hasOrder'),
      hasMembership: booleanField(parsed, 'hasMembership'),
    },
  }
}

export function roleScopeFromSnapshot(userId: string, snapshot?: DisputeEvidenceSnapshot | null): DisputeRoleScope {
  const role = snapshot?.participants.find(item => item.userId === userId)?.role.toLowerCase()
  if (role === 'buyer') return 'buyer'
  if (role === 'owner' || role === 'merchant' || role === 'seller') return 'seller'
  return 'all'
}

export function disputeRoleLabel(roleScope: DisputeRoleScope) {
  if (roleScope === 'buyer') return '买家'
  if (roleScope === 'seller') return '卖家'
  return '全部角色'
}

export function disputeParticipantOptions(
  dispute: DisputeDetailLike,
  snapshot?: DisputeEvidenceSnapshot | null,
): DisputeParticipantOption[] {
  const candidates = [
    {
      userId: dispute.primaryUserId ?? '',
      username: dispute.primaryUsername,
      name: dispute.primaryDisplayName,
    },
    {
      userId: dispute.counterpartyUserId ?? '',
      username: dispute.counterpartyUsername,
      name: dispute.counterpartyName,
    },
    {
      userId: dispute.subjectUserId ?? '',
      username: dispute.subjectUsername ?? '',
      name: dispute.subjectName ?? '',
    },
  ].filter(item => item.userId)

  const uniqueCandidates = candidates.filter(
    (item, index, items) => items.findIndex(candidate => candidate.userId === item.userId) === index,
  )

  if (dispute.targetType === 'public_user') {
    const target = uniqueCandidates.find(item => item.userId === dispute.targetId)
      ?? uniqueCandidates.find(item => item.userId === dispute.subjectUserId)
    return target ? [{ ...target, roleScope: 'all', roleLabel: disputeRoleLabel('all') }] : []
  }

  if (!transactionDisputeTargets.has(dispute.targetType)) return []

  const participantIds = snapshot ? new Set(snapshot.participants.map(item => item.userId)) : null
  return uniqueCandidates.filter(item => !participantIds || participantIds.has(item.userId)).map(item => {
    const roleScope = roleScopeFromSnapshot(item.userId, snapshot)
    return { ...item, roleScope, roleLabel: disputeRoleLabel(roleScope) }
  })
}

export function normalizeOutcomeSeverity(
  responsibility: DisputeResponsibility,
  severity: DisputeSeverity,
): DisputeSeverity {
  if (responsibility === 'not_responsible' || responsibility === 'undetermined') return 'none'
  return severity === 'none' ? 'low' : severity
}

export function defaultOutcomeReasonCode(responsibility: DisputeResponsibility) {
  if (responsibility === 'responsible') return 'confirmed_responsibility'
  if (responsibility === 'shared') return 'shared_responsibility'
  if (responsibility === 'not_responsible') return 'not_responsible'
  return 'insufficient_evidence'
}

function validateLength(value: string, min: number, max: number, label: string) {
  const length = value.trim().length
  if (length < min) return `${label}至少需要 ${min} 个字符。`
  if (length > max) return `${label}不能超过 ${max} 个字符。`
  return ''
}

export function validateResolutionForm(form: ResolutionForm) {
  const errors: Partial<Record<keyof ResolutionForm, string>> = {}
  if (!resultCodes.has(form.publicResultCode)) errors.publicResultCode = '请选择公开裁决分类。'
  errors.publicSummary = validateLength(form.publicSummary, 2, 120, '公开摘要')
  errors.publicResult = validateLength(form.publicResult, 2, 120, '公开结果')
  errors.internalReason = validateLength(form.internalReason, 2, 800, '内部裁决原因')
  if (!form.confirmed) errors.confirmed = '请确认已核对案件证据和公开文案。'
  return Object.fromEntries(Object.entries(errors).filter(([, message]) => message)) as Partial<Record<keyof ResolutionForm, string>>
}

export function validateOutcomeForm(form: OutcomeForm, allowedSubjectIds: string[] = []) {
  const errors: Partial<Record<keyof OutcomeForm, string>> = {}
  if (!form.subjectUserId || (allowedSubjectIds.length > 0 && !allowedSubjectIds.includes(form.subjectUserId))) {
    errors.subjectUserId = '请选择该纠纷中的实际责任主体。'
  }
  if (!responsibilities.has(form.responsibility)) errors.responsibility = '请选择责任认定。'
  if (!severities.has(form.severity)) errors.severity = '请选择严重度。'
  if ((form.responsibility === 'not_responsible' || form.responsibility === 'undetermined') && form.severity !== 'none') {
    errors.severity = '未认定责任时严重度必须为无。'
  }
  if ((form.responsibility === 'responsible' || form.responsibility === 'shared') && form.severity === 'none') {
    errors.severity = '认定责任时请选择具体严重度。'
  }
  if (!roleScopes.has(form.roleScope)) errors.roleScope = '角色范围不正确。'
  if (!reasonCodePattern.test(form.reasonCode.trim())) errors.reasonCode = '请选择有效的责任原因。'
  errors.publicReason = validateLength(form.publicReason, 1, 120, '公开责任说明')
  errors.internalReason = validateLength(form.internalReason, 1, 800, '内部责任说明')
  if (!form.confirmed) errors.confirmed = '请确认责任认定将写入信誉治理记录。'
  return Object.fromEntries(Object.entries(errors).filter(([, message]) => message)) as Partial<Record<keyof OutcomeForm, string>>
}

export function findDisputeOutcome(
  audits: Array<{ outcomes: DisputeReputationOutcome[] } | null | undefined>,
  disputeCaseId: string,
) {
  for (const audit of audits) {
    const outcome = audit?.outcomes.find(item => item.disputeCaseId === disputeCaseId)
    if (outcome) return outcome
  }
  return null
}

export function responsibilityLabel(value: DisputeResponsibility) {
  return responsibilityOptions.find(item => item.value === value)?.label ?? value
}

export function severityLabel(value: DisputeSeverity) {
  return severityOptions.find(item => item.value === value)?.label ?? value
}
