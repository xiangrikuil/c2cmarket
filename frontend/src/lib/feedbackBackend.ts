import type {
  AdminRow,
  FeedbackAdminHandlePayload,
  FeedbackSupplementPayload,
  FeedbackTicket,
  SubmitFeedbackPayload,
} from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'

type ListResponse<T> = {
  items: T[]
  nextCursor?: string | null
}

type CountResponse = {
  count: number
}

type BackendFeedbackTicket = FeedbackTicket

async function ensureUserSession() {
  return ensureBackendSession('buyer', false)
}

async function ensureAdminSession() {
  return ensureBackendSession('admin', true)
}

export async function backendCreateFeedbackTicket(payload: SubmitFeedbackPayload): Promise<FeedbackTicket> {
  await ensureUserSession()
  return backendMutation<BackendFeedbackTicket>('/api/v1/me/feedback-tickets', payload, {
    idempotencyPrefix: 'feedback-create',
  })
}

export async function backendMyFeedbackTickets(): Promise<FeedbackTicket[]> {
  await ensureUserSession()
  const response = await backendRequest<ListResponse<BackendFeedbackTicket>>('/api/v1/me/feedback-tickets')
  return response.items
}

export async function backendMyFeedbackTicket(id: string): Promise<FeedbackTicket> {
  await ensureUserSession()
  return backendRequest<BackendFeedbackTicket>(`/api/v1/me/feedback-tickets/${encodeURIComponent(id)}`)
}

export async function backendFeedbackUnreadCount(): Promise<number> {
  await ensureUserSession()
  const response = await backendRequest<CountResponse>('/api/v1/me/feedback-tickets/unread-count')
  return response.count
}

export async function backendAddFeedbackSupplement(id: string, payload: FeedbackSupplementPayload): Promise<FeedbackTicket> {
  await ensureUserSession()
  return backendMutation<BackendFeedbackTicket>(`/api/v1/me/feedback-tickets/${encodeURIComponent(id)}/supplements`, payload, {
    idempotencyPrefix: 'feedback-supplement',
  })
}

export async function backendMarkFeedbackRead(id: string): Promise<FeedbackTicket> {
  await ensureUserSession()
  return backendMutation<BackendFeedbackTicket>(`/api/v1/me/feedback-tickets/${encodeURIComponent(id)}/read`, {})
}

export async function backendAdminFeedbackTickets(): Promise<FeedbackTicket[]> {
  await ensureAdminSession()
  const response = await backendRequest<ListResponse<BackendFeedbackTicket>>('/api/v1/admin/feedback-tickets')
  return response.items
}

export async function backendAdminFeedbackTicket(id: string): Promise<FeedbackTicket> {
  await ensureAdminSession()
  return backendRequest<BackendFeedbackTicket>(`/api/v1/admin/feedback-tickets/${encodeURIComponent(id)}`)
}

export async function backendHandleFeedbackTicket(id: string, payload: FeedbackAdminHandlePayload, version: number): Promise<FeedbackTicket> {
  await ensureAdminSession()
  return backendMutation<BackendFeedbackTicket>(`/api/v1/admin/feedback-tickets/${encodeURIComponent(id)}/handle`, payload, {
    idempotencyPrefix: 'feedback-handle',
    ifMatch: version,
  })
}

export async function backendAdminFeedbackRows(): Promise<AdminRow[]> {
  const tickets = await backendAdminFeedbackTickets()
  return tickets.map(item => ({
    id: item.id,
    primary: item.title,
    secondary: `${feedbackTypeLabel(item.type)} · ${item.contextPageLabel}${item.contextTargetLabel ? ` · ${item.contextTargetLabel}` : ''}`,
    owner: item.submitterName || item.submitterUsername || '用户',
    status: feedbackStatusLabel(item.status),
    risk: item.unread ? '用户未读处理结果' : feedbackImpactLabel(item.impact),
    targetType: 'feedback-ticket',
    backendKind: 'feedback-ticket',
    backendVersion: item.version,
    targetTo: `/admin/feedback/${item.id}`,
    detailItems: [
      { label: '影响程度', value: feedbackImpactLabel(item.impact) },
      { label: '当前页面', value: item.contextPageLabel },
      { label: '关联内容', value: item.contextTargetLabel || '未指定' },
      { label: '用户已读', value: item.unread ? '否' : '是' },
    ],
  }))
}

export function feedbackTypeLabel(value: FeedbackTicket['type']) {
  const labels: Record<FeedbackTicket['type'], string> = {
    function_issue: '功能问题',
    data_correction: '数据纠错',
    experience_suggestion: '体验建议',
    publish_contact_block: '发布/联系受阻',
  }
  return labels[value]
}

export function feedbackImpactLabel(value: FeedbackTicket['impact']) {
  const labels: Record<FeedbackTicket['impact'], string> = {
    general: '一般',
    blocks_operation: '影响操作',
    cannot_continue: '无法继续',
  }
  return labels[value]
}

export function feedbackStatusLabel(value: FeedbackTicket['status']) {
  const labels: Record<FeedbackTicket['status'], string> = {
    submitted: '待处理',
    recorded: '已记录',
    following_up: '跟进中',
    resolved: '已修复/已调整',
    declined: '暂不处理',
    needs_user_info: '需要补充信息',
    closed: '已关闭',
  }
  return labels[value]
}
