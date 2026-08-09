import type { AdminRow } from '@/lib/api'
import type {
  Appeal,
  DisputeCase,
  SelfDispute,
  DisputeSettlementProposalRequest,
  SelfModerationSupplementMutation,
  SelfReport,
} from '@/api/generated/openapi'
import type { CreateContactReportRequest, PublicDisputeRecord } from '@/data/mock'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'

type ListResponse<T> = {
  items: T[]
  nextCursor?: string | null
}

type BackendReportTargetType = 'contact_snapshot' | 'public_user' | 'carpool_application' | 'carpool_membership' | 'api_purchase_intent' | 'api_order'
type BackendAppealTargetType = BackendReportTargetType | 'account_governance'
type BackendReportReasonCode = 'unreachable' | 'contact_invalid' | 'impersonation' | 'description_mismatch' | 'seat_rule_dispute' | 'api_quota_dispute' | 'order_delivery_dispute' | 'other'
type BackendPublicResultCode = DisputeCase['publicResultCode']

export type AdminInfoSupplement = {
  id: string
  infoRequestId: string
  submittedByUserId: string
  submittedByUsername: string
  submittedByName: string
  body: string
  createdAt: string
}

type BackendReport = {
  id: string
  reporterUserId?: string
  reporterUsername: string
  reporterName: string
  targetType: BackendReportTargetType
  targetId: string
  canonicalTargetType: BackendReportTargetType
  canonicalTargetId: string
  targetLabel: string
  targetSnapshotJson?: string
  reportedUsername: string
  reasonCode: BackendReportReasonCode
  title: string
  description?: string
  status: 'submitted' | 'triaged' | 'needs_info' | 'rejected' | 'dispute_opened' | 'closed'
  adminReason?: string
  handledByAdminId?: string
  handledAt?: string | null
  disputeId?: string
  createdAt: string
  updatedAt: string
  version: number
  supplements?: AdminInfoSupplement[]
}

type BackendDispute = DisputeCase & { supplements?: AdminInfoSupplement[] }

export type AdminReportDetail = BackendReport
export type AdminDisputeDetail = BackendDispute

export type ResolveAdminDisputeInput = {
  disputeId: string
  expectedVersion: number
  reason: string
  publicSummary: string
  publicResultCode: BackendPublicResultCode
  publicResult: string
}

type BackendAppeal = {
  id: string
  appellantUserId?: string
  appellantUsername: string
  appellantName: string
  reportId?: string
  disputeId?: string
  targetType: BackendAppealTargetType
  targetId: string
  title: string
  statement?: string
  status: 'submitted' | 'approved' | 'rejected'
  adminReason?: string
  handledByAdminId?: string
  handledAt?: string | null
  createdAt: string
  updatedAt: string
  version: number
}

type BackendPublicDispute = {
  id: string
  username: string
  type: string
  result: string
  handledAt: string
  unresolved: boolean
}

type BackendAdminMutation = {
  report?: BackendReport
  dispute?: BackendDispute
  appeal?: BackendAppeal
}

export type CreatePublicUserReportRequest = {
  username: string
  reasonCode: BackendReportReasonCode
  title: string
  description: string
}

export type CreateManualInterventionReportRequest = {
  targetType: BackendReportTargetType
  targetId: string
  targetLabel?: string
  reportedUsername?: string
  reasonCode: BackendReportReasonCode
  title: string
  description: string
}

export type MyReport = SelfReport
export type MyDispute = SelfDispute
export type CreateDisputeSettlementProposalRequest = DisputeSettlementProposalRequest
export type MyAppeal = Appeal
export type SubmitInfoSupplementRequest = {
  entityType: 'report' | 'dispute'
  entityId: string
  openInfoRequestId: string
  body: string
}
export type CreateAppealRequest = {
  title: string
  statement: string
} & (
  | { reportId: string, disputeId?: never }
  | { reportId?: never, disputeId: string }
)

function formatTime(value: string | undefined | null) {
  if (!value) return ''
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('sv-SE', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function targetTypeLabel(value: BackendReportTargetType) {
  const labels: Record<BackendReportTargetType, string> = {
    contact_snapshot: '联系快照',
    public_user: '公开主页',
    carpool_application: '拼车申请',
    carpool_membership: '拼车成员关系',
    api_purchase_intent: 'API 订单',
    api_order: 'API 订单',
  }
  return labels[value]
}

function appealTargetTypeLabel(value: BackendAppealTargetType) {
  if (value === 'account_governance') return '账号治理'
  return targetTypeLabel(value)
}

function reasonLabel(value: BackendReportReasonCode) {
  const labels: Record<BackendReportReasonCode, string> = {
    unreachable: '无法联系',
    contact_invalid: '联系方式无效',
    impersonation: '疑似冒充',
    description_mismatch: '服务描述不一致',
    seat_rule_dispute: '规则/席位争议',
    api_quota_dispute: 'API 接入或额度说明争议',
    order_delivery_dispute: '订单确认或交付说明争议',
    other: '其他问题',
  }
  return labels[value]
}

function reportStatusLabel(value: BackendReport['status']) {
  const labels: Record<BackendReport['status'], string> = {
    submitted: '待处理',
    triaged: '已分诊',
    needs_info: '需要补充信息',
    rejected: '已拒绝',
    dispute_opened: '处理中',
    closed: '已关闭',
  }
  return labels[value]
}

function disputeStatusLabel(value: BackendDispute['status']) {
  const labels: Record<BackendDispute['status'], string> = {
	negotiating: '协商中',
    open: '处理中',
    waiting_info: '需要补充信息',
    resolved: '已处理',
    closed: '已关闭',
  }
  return labels[value]
}

function publicResultCodeLabel(value: BackendPublicResultCode) {
  const labels: Record<BackendPublicResultCode, string> = {
    no_action: '未记录处置',
    contact_invalid: '联系方式无效',
    impersonation_confirmed: '确认冒充',
    description_mismatch: '描述不一致',
    rule_or_seat_issue: '规则/席位争议',
    api_delivery_issue: 'API 接入/额度争议',
    other_resolved: '其他已处理',
  }
  return labels[value]
}

function appealStatusLabel(value: BackendAppeal['status']) {
  const labels: Record<BackendAppeal['status'], string> = {
    submitted: '申诉复核中',
    approved: '已通过',
    rejected: '已拒绝',
  }
  return labels[value]
}

function mapContactTargetType(value: CreateContactReportRequest['orderType']): BackendReportTargetType {
  return 'contact_snapshot'
}

function reportTargetTo(row: BackendReport) {
  if (row.targetType === 'public_user' && row.reportedUsername) return `/u/${row.reportedUsername}`
  if (row.canonicalTargetType === 'carpool_application') return `/my/rides/${row.canonicalTargetId}`
  if (row.canonicalTargetType === 'carpool_membership') return null
  if (row.canonicalTargetType === 'api_purchase_intent') return `/my/api-orders/${row.canonicalTargetId}`
  if (row.canonicalTargetType === 'api_order') return `/my/api-orders/${row.canonicalTargetId}`
  if (row.targetType === 'carpool_application') return `/my/rides/${row.targetId}`
  if (row.targetType === 'carpool_membership') return null
  if (row.targetType === 'api_purchase_intent') return `/my/api-orders/${row.targetId}`
  if (row.targetType === 'api_order') return `/my/api-orders/${row.targetId}`
  return null
}

function moderationParticipants(candidates: Array<{ userId?: string, username?: string, name?: string, role: string }>) {
  const seen = new Set<string>()
  return candidates.flatMap(candidate => {
    const userId = candidate.userId?.trim() ?? ''
    if (!userId || seen.has(userId)) return []
    seen.add(userId)
    const identity = candidate.name?.trim() || candidate.username?.trim() || userId
    const username = candidate.username?.trim()
    return [{ userId, label: `${candidate.role} · ${identity}${username && username !== identity ? ` (@${username})` : ''}` }]
  })
}

function mapAdminSupplements(items?: AdminInfoSupplement[]) {
  return items?.map(item => ({
    id: item.id,
    submittedByUserId: item.submittedByUserId,
    submittedByUsername: item.submittedByUsername,
    submittedByName: item.submittedByName,
    body: item.body,
    createdAt: item.createdAt,
  })) ?? []
}

function mapReportRow(item: BackendReport): AdminRow {
  return {
    id: item.id,
    primary: item.title,
    secondary: `${targetTypeLabel(item.targetType)} · ${reasonLabel(item.reasonCode)} · ${item.targetLabel}`,
    owner: `${item.reporterName || item.reporterUsername}${item.reportedUsername ? ` / @${item.reportedUsername}` : ''}`,
    status: reportStatusLabel(item.status),
    risk: item.adminReason || item.description || `提交于 ${formatTime(item.createdAt)}`,
    targetType: 'report',
    backendKind: 'report',
    backendVersion: item.version,
    detailItems: [
      { label: '后端状态', value: item.status },
      { label: '目标类型', value: targetTypeLabel(item.targetType) },
      { label: '归一目标', value: `${targetTypeLabel(item.canonicalTargetType)} · ${item.canonicalTargetId}` },
      { label: '原因', value: reasonLabel(item.reasonCode) },
      { label: '关联目标', value: item.targetLabel || item.targetId },
      { label: '更新时间', value: formatTime(item.updatedAt) },
    ],
    targetTo: reportTargetTo(item),
    moderationParticipants: moderationParticipants([
      { userId: item.reporterUserId, username: item.reporterUsername, name: item.reporterName, role: '举报人' },
    ]),
    moderationSupplements: mapAdminSupplements(item.supplements),
  }
}

export function mapAdminDisputeRow(item: AdminDisputeDetail): AdminRow {
  return {
    id: item.id,
    primary: item.publicSummary || item.targetLabel,
    secondary: `${targetTypeLabel(item.targetType)} · ${item.publicResult || '等待处理结果'}`,
    owner: `${item.primaryDisplayName || item.primaryUsername}${item.counterpartyUsername ? ` / @${item.counterpartyUsername}` : ''}`,
    status: disputeStatusLabel(item.status),
    risk: item.adminReason || `公开摘要：${item.publicSummary || '未填写'}`,
    targetType: 'dispute',
    backendKind: 'dispute',
    backendVersion: item.version,
    detailItems: [
      { label: '后端状态', value: item.status },
      { label: '公开摘要', value: item.publicSummary || '未填写' },
      { label: '公开结果代码', value: publicResultCodeLabel(item.publicResultCode || 'no_action') },
      { label: '公开结果', value: item.publicResult || '未填写' },
      { label: '关联举报', value: item.reportId || '无' },
      { label: '更新时间', value: formatTime(item.updatedAt) },
    ],
    targetTo: item.primaryUsername ? `/u/${item.primaryUsername}` : null,
    moderationParticipants: moderationParticipants([
      { userId: item.primaryUserId, username: item.primaryUsername, name: item.primaryDisplayName, role: '主要参与方' },
      { userId: item.counterpartyUserId, username: item.counterpartyUsername, name: item.counterpartyName, role: '另一参与方' },
      { userId: item.subjectUserId, username: item.subjectUsername, name: item.subjectName, role: '责任主体' },
    ]),
    moderationSupplements: mapAdminSupplements(item.supplements),
  }
}

function mapAppealRow(item: BackendAppeal): AdminRow {
  return {
    id: item.id,
    primary: item.title,
    secondary: `${appealTargetTypeLabel(item.targetType)} · ${item.statement || '用户申诉说明已提交'}`,
    owner: item.appellantName || item.appellantUsername,
    status: appealStatusLabel(item.status),
    risk: item.adminReason || `提交于 ${formatTime(item.createdAt)}`,
    targetType: 'appeal',
    backendKind: 'appeal',
    backendVersion: item.version,
    detailItems: [
      { label: '后端状态', value: item.status },
      { label: '关联举报', value: item.reportId || '无' },
      { label: '关联纠纷', value: item.disputeId || '无' },
      { label: '更新时间', value: formatTime(item.updatedAt) },
    ],
    targetTo: item.appellantUsername ? `/u/${item.appellantUsername}` : null,
  }
}

function mapPublicDispute(item: BackendPublicDispute): PublicDisputeRecord {
  return {
    id: item.id,
    username: item.username,
    type: item.type,
    result: item.result,
    handledAt: item.handledAt,
    unresolved: item.unresolved,
  }
}

function mutationRow(result: BackendAdminMutation, fallback: AdminRow): AdminRow {
  if (result.dispute) return mapAdminDisputeRow(result.dispute)
  if (result.report) return mapReportRow(result.report)
  if (result.appeal) return mapAppealRow(result.appeal)
  return fallback
}

export async function backendCreateReport(payload: CreateContactReportRequest) {
  await ensureBackendSession('buyer', false)
  return backendMutation<BackendReport>('/api/v1/reports', {
    targetType: mapContactTargetType(payload.orderType),
    targetId: payload.orderId,
    targetLabel: `联系方式快照 · ${payload.orderType} · ${payload.contactType}`,
    reasonCode: payload.reasonCode,
    title: `举报 / 申请人工介入：${reasonLabel(payload.reasonCode)}`,
    description: payload.note || `联系快照存在问题：${reasonLabel(payload.reasonCode)}。平台仅记录脱敏说明和处理状态，不追回付款、不托管、不担保、不验真 API Key。`,
  }, {
    idempotencyPrefix: 'report-create',
  })
}

export async function backendCreateManualInterventionReport(payload: CreateManualInterventionReportRequest) {
  await ensureBackendSession('buyer', false)
  return backendMutation<BackendReport>('/api/v1/reports', {
    targetType: payload.targetType,
    targetId: payload.targetId,
    targetLabel: payload.targetLabel ?? '',
    reportedUsername: payload.reportedUsername ?? '',
    reasonCode: payload.reasonCode,
    title: payload.title,
    description: payload.description,
  }, {
    idempotencyPrefix: 'manual-intervention-report',
  })
}

export async function backendCreatePublicUserReport(payload: CreatePublicUserReportRequest) {
  await ensureBackendSession('buyer', false)
  return backendMutation<BackendReport>('/api/v1/reports', {
    targetType: 'public_user',
    targetId: payload.username,
    targetLabel: `公开主页 @${payload.username}`,
    reportedUsername: payload.username,
    reasonCode: payload.reasonCode,
    title: payload.title,
    description: `${payload.description}\n\n平台仅记录脱敏说明和处理状态，不追回付款、不托管、不担保、不裁决站外支付、不验真 API Key。`,
  }, {
    idempotencyPrefix: 'public-user-report',
  })
}

async function backendAllPages<T>(path: string) {
  const items: T[] = []
  const seenCursors = new Set<string>()
  let cursor = ''
  do {
    const params = new URLSearchParams({ limit: '100' })
    if (cursor) params.set('cursor', cursor)
    const response = await backendRequest<ListResponse<T>>(`${path}?${params.toString()}`)
    items.push(...response.items)
    cursor = response.nextCursor?.trim() ?? ''
    if (cursor && seenCursors.has(cursor)) {
      throw new Error('Backend repeated a pagination cursor.')
    }
    if (cursor) seenCursors.add(cursor)
  } while (cursor)
  return items
}

export async function backendMyReports() {
  await ensureBackendSession('buyer', false)
  return backendAllPages<MyReport>('/api/v1/me/reports')
}

export async function backendMyDisputes() {
  await ensureBackendSession('buyer', false)
  return backendAllPages<MyDispute>('/api/v1/me/disputes')
}

export async function backendMyDispute(id: string) {
  await ensureBackendSession('buyer', false)
  return backendRequest<MyDispute>(`/api/v1/me/disputes/${encodeURIComponent(id)}`)
}

export async function backendAppendDisputeMessage(disputeId: string, body: string) {
  await ensureBackendSession('buyer', false)
  return backendMutation<MyDispute>(`/api/v1/me/disputes/${encodeURIComponent(disputeId)}/messages`, { body }, {
    idempotencyPrefix: 'dispute-message',
  })
}

export async function backendCreateDisputeSettlementProposal(disputeId: string, input: CreateDisputeSettlementProposalRequest) {
  await ensureBackendSession('buyer', false)
  return backendMutation<MyDispute>(`/api/v1/me/disputes/${encodeURIComponent(disputeId)}/settlement-proposals`, input, {
    idempotencyPrefix: 'dispute-settlement-proposal',
  })
}

export async function backendConfirmDisputeSettlementProposal(disputeId: string, proposalId: string) {
  await ensureBackendSession('buyer', false)
  return backendMutation<MyDispute>(`/api/v1/me/disputes/${encodeURIComponent(disputeId)}/settlement-proposals/${encodeURIComponent(proposalId)}/confirm`, {}, {
    idempotencyPrefix: 'dispute-settlement-confirm',
  })
}

export async function backendRejectDisputeSettlementProposal(disputeId: string, proposalId: string, reason: string) {
  await ensureBackendSession('buyer', false)
  return backendMutation<MyDispute>(`/api/v1/me/disputes/${encodeURIComponent(disputeId)}/settlement-proposals/${encodeURIComponent(proposalId)}/reject`, { reason }, {
    idempotencyPrefix: 'dispute-settlement-reject',
  })
}

export async function backendEscalateDispute(disputeId: string, reason: string) {
  await ensureBackendSession('buyer', false)
  return backendMutation<MyDispute>(`/api/v1/me/disputes/${encodeURIComponent(disputeId)}/escalate`, { reason }, {
    idempotencyPrefix: 'dispute-escalate',
  })
}

export async function backendMyAppeals() {
  await ensureBackendSession('buyer', false)
  return backendAllPages<MyAppeal>('/api/v1/me/appeals')
}

export async function backendSubmitInfoSupplement(input: SubmitInfoSupplementRequest) {
  await ensureBackendSession('buyer', false)
  const result = await backendMutation<SelfModerationSupplementMutation>(
    `/api/v1/me/${input.entityType}s/${encodeURIComponent(input.entityId)}/supplements`,
    { openInfoRequestId: input.openInfoRequestId, body: input.body },
    { idempotencyPrefix: `${input.entityType}-supplement` },
  )
  const item = input.entityType === 'report' ? result.report : result.dispute
  if (!item) throw new Error('补充材料响应缺少最新案件数据。')
  return item
}

export async function backendCreateAppeal(payload: CreateAppealRequest) {
  await ensureBackendSession('buyer', false)
  const source = payload.reportId
    ? { reportId: payload.reportId }
    : { disputeId: payload.disputeId }
  return backendMutation<MyAppeal>('/api/v1/me/appeals', {
    ...source,
    title: payload.title,
    statement: payload.statement,
  }, {
    idempotencyPrefix: 'appeal-create',
  })
}

export async function backendAdminReportRows() {
  await ensureBackendSession('admin', true)
  const [reports, disputes, appeals] = await Promise.all([
    backendRequest<ListResponse<BackendReport>>('/api/v1/admin/reports'),
    backendRequest<ListResponse<BackendDispute>>('/api/v1/admin/disputes'),
    backendRequest<ListResponse<BackendAppeal>>('/api/v1/admin/appeals'),
  ])
  const disputeReportIds = new Set(disputes.items.map(item => item.reportId).filter(Boolean))
  return [
    ...reports.items.filter(item => item.status !== 'dispute_opened' && !disputeReportIds.has(item.id)).map(mapReportRow),
    ...disputes.items.map(mapAdminDisputeRow),
    ...appeals.items.map(mapAppealRow),
  ]
}

export async function backendAdminAppealRows() {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<BackendAppeal>>('/api/v1/admin/appeals')
  return response.items.map(mapAppealRow)
}

export async function backendAdminReportDetail(id: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<BackendReport>(`/api/v1/admin/reports/${encodeURIComponent(id)}`)
}

export async function backendAdminDisputeDetail(id: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<BackendDispute>(`/api/v1/admin/disputes/${encodeURIComponent(id)}`)
}

export async function backendAdminModerationDetailRow(row: AdminRow) {
  if (row.targetType === 'report') return mapReportRow(await backendAdminReportDetail(row.id))
  if (row.targetType === 'dispute') return mapAdminDisputeRow(await backendAdminDisputeDetail(row.id))
  return row
}

export async function backendResolveAdminDispute(input: ResolveAdminDisputeInput) {
  await ensureBackendSession('admin', true)
  const result = await backendMutation<BackendAdminMutation>(
    `/api/v1/admin/disputes/${encodeURIComponent(input.disputeId)}/resolve`,
    {
      reason: input.reason,
      publicSummary: input.publicSummary,
      publicResultCode: input.publicResultCode,
      publicResult: input.publicResult,
    },
    {
      idempotencyPrefix: 'dispute-admin-resolve',
      ifMatch: input.expectedVersion,
    },
  )
  if (!result.dispute) throw new Error('纠纷裁决响应缺少最新案件数据。')
  return result.dispute
}

async function adminAppeal(id: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<BackendAppeal>(`/api/v1/admin/appeals/${encodeURIComponent(id)}`)
}

export async function backendRunReportAdminAction(row: AdminRow, action: 'approve' | 'request_changes' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban', reason: string, requestedFromUserId = '') {
  if (row.backendKind === 'report' || row.targetType === 'report') {
    const detail = await backendAdminReportDetail(row.id)
    const pathAction = action === 'approve'
      ? 'triage'
      : action === 'request_changes'
        ? 'request-info'
        : action === 'restore'
        ? 'open-dispute'
        : action === 'suspend'
          ? 'close'
          : 'reject'
    if (pathAction === 'request-info' && (!requestedFromUserId || requestedFromUserId !== detail.reporterUserId)) {
      throw new Error('举报补充信息只能指定当前举报人。')
    }
    const result = await backendMutation<BackendAdminMutation>(`/api/v1/admin/reports/${encodeURIComponent(row.id)}/${pathAction}`, {
      reason: reason || '管理台举报处理',
      publicSummary: reason || detail.title || detail.targetLabel,
      publicResultCode: 'no_action',
      publicResult: pathAction === 'open-dispute' ? '已进入人工处理中' : '',
      ...(pathAction === 'request-info' ? { requestedFromUserId } : {}),
    }, {
      idempotencyPrefix: `report-admin-${pathAction}`,
      ifMatch: detail.version,
    })
    return mutationRow(result, row)
  }

  if (row.backendKind === 'dispute' || row.targetType === 'dispute') {
    if (action === 'approve' || action === 'restore') {
      throw new Error('请使用纠纷裁决面板填写公开结果和内部原因。')
    }
    const detail = await backendAdminDisputeDetail(row.id)
    const pathAction = action === 'request_changes' || action === 'warn'
        ? 'request-info'
        : 'close'
    const participantIds = new Set([detail.primaryUserId, detail.counterpartyUserId, detail.subjectUserId].filter((value): value is string => Boolean(value)))
    if (pathAction === 'request-info' && (!requestedFromUserId || !participantIds.has(requestedFromUserId))) {
      throw new Error('请选择当前纠纷中的有效参与者补充信息。')
    }
    const result = await backendMutation<BackendAdminMutation>(`/api/v1/admin/disputes/${encodeURIComponent(row.id)}/${pathAction}`, {
      reason: reason || '管理台纠纷处理',
      publicSummary: detail.publicSummary || reason || detail.targetLabel,
      publicResultCode: 'no_action',
      publicResult: pathAction === 'request-info' ? '等待补充信息' : '案件已关闭，未作责任认定',
      ...(pathAction === 'request-info' ? { requestedFromUserId } : {}),
    }, {
      idempotencyPrefix: `dispute-admin-${pathAction}`,
      ifMatch: detail.version,
    })
    return mutationRow(result, row)
  }

  if (row.backendKind === 'appeal' || row.targetType === 'appeal') {
    const detail = await adminAppeal(row.id)
    const pathAction = action === 'approve' || action === 'restore' ? 'approve' : 'reject'
    const result = await backendMutation<BackendAdminMutation>(`/api/v1/admin/appeals/${encodeURIComponent(row.id)}/${pathAction}`, {
      reason: reason || '管理台申诉处理',
    }, {
      idempotencyPrefix: `appeal-admin-${pathAction}`,
      ifMatch: detail.version,
    })
    return mutationRow(result, row)
  }

  return row
}

export async function backendUpdateReportAdminStatus(row: AdminRow, status: string, reason: string) {
  if (status === '已通过') return backendRunReportAdminAction(row, 'approve', reason)
  if (status === '待复核') return backendRunReportAdminAction(row, 'request_changes', reason)
  if (status === '已恢复') return backendRunReportAdminAction(row, 'restore', reason)
  return backendRunReportAdminAction(row, 'take_down', reason)
}

export async function backendPublicUserDisputes(username: string) {
  const response = await backendRequest<ListResponse<BackendPublicDispute>>(`/api/v1/users/${encodeURIComponent(username)}/disputes`)
  return response.items.map(mapPublicDispute)
}
