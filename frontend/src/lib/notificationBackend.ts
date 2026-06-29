import type { UnifiedNotification } from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession, getCurrentBackendSession } from '@/lib/backendClient'

type ListResponse<T> = {
  items: T[]
  nextCursor?: string | null
}

type BackendNotification = {
  id: string
  type: string
  title: string
  detail: string
  targetType: string
  targetId: string
  to: string
  unread: boolean
  readAt: string | null
  createdAt: string
  time?: string
}

type BackendNotificationReadAll = {
  count: number
  items: BackendNotification[]
}

type BackendNotificationUnreadCount = {
  count: number
}

function mapNotificationType(type: string): UnifiedNotification['type'] {
  if (type === '审核结果' || type === '上车申请' || type === 'API 意向' || type === '求车需求' || type === '问题反馈' || type === '管理操作' || type === '边界提醒') {
    return type
  }
  return '管理操作'
}

function mapNotification(item: BackendNotification): UnifiedNotification {
  return {
    id: item.id,
    type: mapNotificationType(item.type),
    title: item.title,
    detail: item.detail,
    time: item.time || item.createdAt,
    unread: item.unread,
    to: item.to || '/my/notifications',
  }
}

async function ensureNotificationSession() {
  try {
    return await getCurrentBackendSession()
  } catch {
    return ensureBackendSession('buyer', false)
  }
}

export async function backendNotifications(): Promise<UnifiedNotification[]> {
  await ensureNotificationSession()
  const response = await backendRequest<ListResponse<BackendNotification>>('/api/v1/me/notifications')
  return response.items.map(mapNotification)
}

export async function backendNotificationUnreadCount(): Promise<number> {
  await ensureNotificationSession()
  const response = await backendRequest<BackendNotificationUnreadCount>('/api/v1/me/notifications/unread-count')
  return response.count
}

export async function backendMarkNotificationRead(id: string): Promise<UnifiedNotification | null> {
  await ensureNotificationSession()
  const response = await backendMutation<BackendNotification>(`/api/v1/me/notifications/${encodeURIComponent(id)}/read`, {})
  return mapNotification(response)
}

export async function backendMarkAllNotificationsRead(): Promise<UnifiedNotification[]> {
  await ensureNotificationSession()
  const response = await backendMutation<BackendNotificationReadAll>('/api/v1/me/notifications/read-all', {})
  return response.items.map(mapNotification)
}
