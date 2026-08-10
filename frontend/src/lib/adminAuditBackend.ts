import type { AdminAuditLog, AdminAuditLogList, ListAdminAuditLogsData } from '@/api/generated/openapi'
import type { AdminRow } from '@/lib/api'
import { backendRequest, ensureBackendSession } from '@/lib/backendClient'
import { normalizeNextCursor, type CursorPage, type CursorPageRequest } from '@/lib/cursorPagination'

export type AdminAuditLogFilters = Omit<NonNullable<ListAdminAuditLogsData['query']>, 'limit' | 'cursor'>

function adminAuditLogQuery(filters: AdminAuditLogFilters, page: CursorPageRequest) {
  const params = new URLSearchParams()
  if (filters.search?.trim()) params.set('search', filters.search.trim())
  if (filters.action?.trim()) params.set('action', filters.action.trim())
  if (filters.targetType?.trim()) params.set('targetType', filters.targetType.trim())
  if (filters.actorUserId?.trim()) params.set('actorUserId', filters.actorUserId.trim())
  if (filters.targetId?.trim()) params.set('targetId', filters.targetId.trim())
  if (page.limit) params.set('limit', String(page.limit))
  if (page.cursor) params.set('cursor', page.cursor)
  const query = params.toString()
  return query ? `?${query}` : ''
}

function statusSummary(item: AdminAuditLog) {
  if (item.beforeStatus && item.afterStatus) return `${item.beforeStatus} → ${item.afterStatus}`
  if (item.afterStatus) return `当前状态 ${item.afterStatus}`
  if (item.beforeStatus) return `原状态 ${item.beforeStatus}`
  return '无状态变更摘要'
}

function adminAuditLogRow(item: AdminAuditLog): AdminRow {
  return {
    id: item.id,
    primary: item.action,
    secondary: `${item.targetType} · ${item.targetId} · ${statusSummary(item)}`,
    owner: item.actorUsername || item.actorUserId,
    status: '已记录',
    risk: item.reason || item.createdAt,
    targetType: 'audit-log',
    backendKind: 'admin-audit-log',
    detailItems: [
      { label: '目标类型', value: item.targetType },
      { label: '目标 ID', value: item.targetId },
      { label: '管理员 ID', value: item.actorUserId },
      { label: '操作时间', value: item.createdAt },
      { label: '请求追踪', value: item.requestId },
    ],
  }
}

export async function backendAdminAuditLogRowsPage(filters: AdminAuditLogFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<AdminRow>> {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<AdminAuditLogList>(`/api/v1/admin/audit-logs${adminAuditLogQuery(filters, page)}`)
  return {
    items: response.items.map(adminAuditLogRow),
    nextCursor: normalizeNextCursor(response.nextCursor),
  }
}

export async function backendAdminAuditLogRows() {
  return (await backendAdminAuditLogRowsPage({}, { limit: 100 })).items
}
