import type { CreateAppealRequest, MyAppeal, MyDispute, MyReport } from '@/lib/reportBackend'

export type ModerationRecordKind = 'report' | 'dispute' | 'appeal'
export type ModerationRecordFilter = 'all' | ModerationRecordKind

type ModerationRecordBase = {
  id: string
  kind: ModerationRecordKind
  title: string
  targetLabel: string
  status: string
  statusLabel: string
  createdAt: string
  updatedAt: string
}

export type ModerationReportRecord = ModerationRecordBase & {
  kind: 'report'
  source: MyReport
}

export type ModerationDisputeRecord = ModerationRecordBase & {
  kind: 'dispute'
  source: MyDispute
}

export type ModerationAppealRecord = ModerationRecordBase & {
  kind: 'appeal'
  source: MyAppeal
}

export type ModerationRecord = ModerationReportRecord | ModerationDisputeRecord | ModerationAppealRecord

const reportStatusLabels: Record<MyReport['status'], string> = {
  submitted: '待处理',
  triaged: '已分诊',
  needs_info: '需要补充信息',
  rejected: '未受理',
  dispute_opened: '已转为纠纷',
  closed: '已关闭',
}

const disputeStatusLabels: Record<MyDispute['status'], string> = {
  open: '处理中',
  waiting_info: '需要补充信息',
  resolved: '已处理',
  closed: '已关闭',
}

const appealStatusLabels: Record<MyAppeal['status'], string> = {
  submitted: '复核中',
  approved: '已通过',
  rejected: '未通过',
}

const targetTypeLabels: Record<string, string> = {
  contact_snapshot: '联系快照',
  public_user: '公开主页',
  carpool_application: '拼车申请',
  carpool_membership: '拼车成员关系',
  api_purchase_intent: 'API 订单',
  api_order: 'API 订单',
}

const reasonLabels: Record<MyReport['reasonCode'], string> = {
  unreachable: '无法联系',
  contact_invalid: '联系方式无效',
  impersonation: '疑似冒充',
  description_mismatch: '服务描述不一致',
  seat_rule_dispute: '规则或席位争议',
  api_quota_dispute: 'API 接入或额度争议',
  order_delivery_dispute: '订单确认或交付争议',
  other: '其他问题',
}

export function moderationRecordKindLabel(kind: ModerationRecordKind) {
  if (kind === 'report') return '举报'
  if (kind === 'dispute') return '纠纷'
  return '申诉'
}

export function moderationTargetTypeLabel(value: string) {
  return targetTypeLabels[value] ?? '关联记录'
}

export function reportReasonLabel(value: MyReport['reasonCode']) {
  return reasonLabels[value]
}

function reportRecord(item: MyReport): ModerationReportRecord {
  return {
    id: item.id,
    kind: 'report',
    title: item.title,
    targetLabel: item.targetLabel || moderationTargetTypeLabel(item.targetType),
    status: item.status,
    statusLabel: reportStatusLabels[item.status],
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    source: item,
  }
}

function disputeRecord(item: MyDispute): ModerationDisputeRecord {
  return {
    id: item.id,
    kind: 'dispute',
    title: item.publicSummary || item.targetLabel || '关联纠纷',
    targetLabel: item.targetLabel || moderationTargetTypeLabel(item.targetType),
    status: item.status,
    statusLabel: disputeStatusLabels[item.status],
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    source: item,
  }
}

function appealRecord(item: MyAppeal): ModerationAppealRecord {
  return {
    id: item.id,
    kind: 'appeal',
    title: item.title,
    targetLabel: moderationTargetTypeLabel(item.targetType),
    status: item.status,
    statusLabel: appealStatusLabels[item.status],
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
    source: item,
  }
}

export function buildModerationRecords(reports: MyReport[], disputes: MyDispute[], appeals: MyAppeal[]) {
  return [
    ...reports.map(reportRecord),
    ...disputes.map(disputeRecord),
    ...appeals.map(appealRecord),
  ].sort((left, right) => Date.parse(right.updatedAt) - Date.parse(left.updatedAt))
}

export function filterModerationRecords(records: ModerationRecord[], filter: ModerationRecordFilter) {
  if (filter === 'all') return records
  return records.filter(item => item.kind === filter)
}

export function hasPendingAppeal(record: ModerationRecord, appeals: MyAppeal[]) {
  if (record.kind === 'appeal') return false
  return appeals.some(appeal => appeal.status === 'submitted'
    && (record.kind === 'report' ? appeal.reportId === record.id : appeal.disputeId === record.id))
}

export function canCreateAppeal(record: ModerationRecord, appeals: MyAppeal[]) {
  if (record.kind === 'appeal' || hasPendingAppeal(record, appeals)) return false
  if (record.kind === 'report') {
    return !record.source.disputeId && ['rejected', 'closed'].includes(record.source.status)
  }
  return record.source.canAppeal === true
}

export function createAppealPayload(record: ModerationRecord, title: string, statement: string): CreateAppealRequest | null {
  if (record.kind === 'appeal') return null
  if (record.kind === 'report') {
    return { reportId: record.id, title, statement }
  }
  return { disputeId: record.id, title, statement }
}
