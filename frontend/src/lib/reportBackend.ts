import type { AdminRow } from '@/lib/api'
import type { CreateContactReportRequest, PublicDisputeRecord } from '@/data/mock'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'

type ListResponse<T> = {
  items: T[]
  nextCursor?: string | null
}

type BackendReportTargetType = 'contact_snapshot' | 'public_user' | 'carpool_application' | 'carpool_membership' | 'api_purchase_intent' | 'api_order'
type BackendReportReasonCode = 'unreachable' | 'contact_invalid' | 'impersonation' | 'description_mismatch' | 'seat_rule_dispute' | 'api_quota_dispute' | 'order_delivery_dispute' | 'other'
type BackendPublicResultCode = 'no_action' | 'contact_invalid' | 'impersonation_confirmed' | 'description_mismatch' | 'rule_or_seat_issue' | 'api_delivery_issue' | 'other_resolved'

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
}

type BackendDispute = {
  id: string
  reportId?: string
  targetType: BackendReportTargetType
  targetId: string
  targetLabel: string
  primaryUserId?: string
  primaryUsername: string
  primaryDisplayName: string
  counterpartyUserId?: string
  counterpartyUsername: string
  counterpartyName: string
  status: 'open' | 'waiting_info' | 'resolved' | 'closed'
  publicSummary: string
  publicResultCode: BackendPublicResultCode
  publicResult: string
  adminReason?: string
  openedByAdminId?: string
  openedAt: string
  resolvedAt?: string | null
  closedAt?: string | null
  createdAt: string
  updatedAt: string
  version: number
}

type BackendAppeal = {
  id: string
  appellantUserId?: string
  appellantUsername: string
  appellantName: string
  reportId?: string
  disputeId?: string
  targetType: BackendReportTargetType
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

export type CreateAppealRequest = {
  reportId?: string
  disputeId?: string
  targetType?: BackendReportTargetType
  targetId?: string
  title: string
  statement: string
}

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
  }
}

function mapDisputeRow(item: BackendDispute): AdminRow {
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
  }
}

function mapAppealRow(item: BackendAppeal): AdminRow {
  return {
    id: item.id,
    primary: item.title,
    secondary: `${targetTypeLabel(item.targetType)} · ${item.statement || '用户申诉说明已提交'}`,
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
  if (result.dispute) return mapDisputeRow(result.dispute)
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

export async function backendCreateAppeal(payload: CreateAppealRequest) {
  await ensureBackendSession('buyer', false)
  return backendMutation<BackendAppeal>('/api/v1/me/appeals', {
    reportId: payload.reportId ?? '',
    disputeId: payload.disputeId ?? '',
    targetType: payload.targetType ?? '',
    targetId: payload.targetId ?? '',
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
  return [
    ...reports.items.map(mapReportRow),
    ...disputes.items.map(mapDisputeRow),
    ...appeals.items.map(mapAppealRow),
  ]
}

export async function backendAdminAppealRows() {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<BackendAppeal>>('/api/v1/admin/appeals')
  return response.items.map(mapAppealRow)
}

async function adminReport(id: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<BackendReport>(`/api/v1/admin/reports/${encodeURIComponent(id)}`)
}

async function adminDispute(id: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<BackendDispute>(`/api/v1/admin/disputes/${encodeURIComponent(id)}`)
}

async function adminAppeal(id: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<BackendAppeal>(`/api/v1/admin/appeals/${encodeURIComponent(id)}`)
}

export async function backendRunReportAdminAction(row: AdminRow, action: 'approve' | 'request_changes' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban', reason: string) {
  if (row.backendKind === 'report' || row.targetType === 'report') {
    const detail = await adminReport(row.id)
    const pathAction = action === 'approve'
      ? 'triage'
      : action === 'request_changes'
        ? 'request-info'
        : action === 'restore'
        ? 'open-dispute'
        : action === 'suspend'
          ? 'close'
          : 'reject'
    const result = await backendMutation<BackendAdminMutation>(`/api/v1/admin/reports/${encodeURIComponent(row.id)}/${pathAction}`, {
      reason: reason || '管理台举报处理',
      publicSummary: reason || detail.title || detail.targetLabel,
      publicResultCode: 'no_action',
      publicResult: pathAction === 'open-dispute' ? '已进入人工处理中' : '',
    }, {
      idempotencyPrefix: `report-admin-${pathAction}`,
      ifMatch: detail.version,
    })
    return mutationRow(result, row)
  }

  if (row.backendKind === 'dispute' || row.targetType === 'dispute') {
    const detail = await adminDispute(row.id)
    const pathAction = action === 'approve' || action === 'restore'
      ? 'resolve'
      : action === 'request_changes' || action === 'warn'
        ? 'request-info'
        : 'close'
    const result = await backendMutation<BackendAdminMutation>(`/api/v1/admin/disputes/${encodeURIComponent(row.id)}/${pathAction}`, {
      reason: reason || '管理台纠纷处理',
      publicSummary: detail.publicSummary || reason || detail.targetLabel,
      publicResultCode: pathAction === 'resolve' ? 'other_resolved' : 'no_action',
      publicResult: pathAction === 'request-info' ? '等待补充信息' : reason || detail.publicResult || '已处理',
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
