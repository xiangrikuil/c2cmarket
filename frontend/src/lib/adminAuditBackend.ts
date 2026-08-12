import type {
  AdminOperationAuditEntry,
  AdminOperationAuditEntryList,
  ListAdminAuditLogsData,
} from '@/api/generated/openapi'
import type { AdminRow } from '@/lib/api'
import { backendRequest, ensureBackendSession, shouldUseRealBackend } from '@/lib/backendClient'
import { normalizeNextCursor, paginateCursorItems, type CursorPage, type CursorPageRequest } from '@/lib/cursorPagination'

export type AdminAuditLogEntry = AdminOperationAuditEntry
export type AdminAuditLogFilters = Omit<NonNullable<ListAdminAuditLogsData['query']>, 'limit' | 'cursor'>

const mockAuditEntries: AdminAuditLogEntry[] = [
  {
    id: 'admin:00000000-0000-4000-8000-000000000092',
    sourceKind: 'admin',
    domain: 'institution',
    actorKind: 'admin',
    actorUserId: '00000000-0000-4000-8000-000000000001',
    actorUsername: 'admin',
    action: 'student_registration.updated',
    actionLabel: '调整学生注册开关',
    targetType: 'student_registration_setting',
    targetId: '00000000-0000-4000-8000-000000000091',
    targetLabel: '学生邮箱注册',
    outcome: 'status_changed',
    summary: '学生邮箱注册开关已更新。',
    detailPath: '/admin/student-registration',
    requestId: 'mock-request-audit-2',
    createdAt: '2026-08-12T03:20:00Z',
  },
  {
    id: 'admin:00000000-0000-4000-8000-000000000093',
    sourceKind: 'admin',
    domain: 'account',
    actorKind: 'admin',
    actorUserId: '00000000-0000-4000-8000-000000000001',
    actorUsername: 'admin',
    action: 'user.account_status_changed',
    actionLabel: '调整账号状态',
    targetType: 'user',
    targetId: '00000000-0000-4000-8000-000000000002',
    targetLabel: 'orbit',
    outcome: 'status_changed',
    summary: '管理员完成账号状态调整。',
    detailPath: null,
    requestId: 'mock-request-audit-1',
    createdAt: '2026-08-11T10:10:00Z',
  },
]

function normalizedFilter(value: string | undefined) {
  return value?.trim() || undefined
}

export function adminAuditLogQuery(filters: AdminAuditLogFilters, page: CursorPageRequest) {
  const params = new URLSearchParams()
  const fields: Array<keyof AdminAuditLogFilters> = [
    'sourceKind',
    'domain',
    'action',
    'actorKind',
    'actorUserId',
    'targetType',
    'targetId',
    'outcome',
    'from',
    'to',
    'search',
  ]
  for (const field of fields) {
    const value = normalizedFilter(filters[field])
    if (value) params.set(field, value)
  }
  if (page.limit) params.set('limit', String(page.limit))
  if (page.cursor) params.set('cursor', page.cursor)
  const query = params.toString()
  return query ? `?${query}` : ''
}

function matchesMockEntry(item: AdminAuditLogEntry, filters: AdminAuditLogFilters) {
  const exactFields: Array<keyof Pick<AdminAuditLogFilters,
    'sourceKind' | 'domain' | 'action' | 'actorKind' | 'actorUserId' | 'targetType' | 'targetId' | 'outcome'
  >> = ['sourceKind', 'domain', 'action', 'actorKind', 'actorUserId', 'targetType', 'targetId', 'outcome']
  for (const field of exactFields) {
    const expected = normalizedFilter(filters[field])
    if (expected && item[field] !== expected) return false
  }
  if (filters.from && item.createdAt < filters.from) return false
  if (filters.to && item.createdAt > filters.to) return false
  const search = normalizedFilter(filters.search)?.toLocaleLowerCase()
  if (!search) return true
  return [
    item.action,
    item.actionLabel,
    item.actorUsername,
    item.targetId,
    item.targetLabel,
    item.summary,
    item.requestId,
  ].some(value => value?.toLocaleLowerCase().includes(search))
}

export async function backendAdminAuditLogsPage(
  filters: AdminAuditLogFilters = {},
  page: CursorPageRequest = {},
): Promise<CursorPage<AdminAuditLogEntry>> {
  await ensureBackendSession('admin', true)
  if (!shouldUseRealBackend()) {
    return paginateCursorItems(mockAuditEntries.filter(item => matchesMockEntry(item, filters)), page)
  }
  const response = await backendRequest<AdminOperationAuditEntryList>(`/api/v1/admin/audit-logs${adminAuditLogQuery(filters, page)}`)
  return {
    items: response.items,
    nextCursor: normalizeNextCursor(response.nextCursor),
  }
}

function adminAuditLogRow(item: AdminAuditLogEntry): AdminRow {
  return {
    id: item.id,
    primary: item.actionLabel || item.action,
    secondary: `${item.domain} · ${item.summary}`,
    owner: item.actorUsername || item.actorUserId || item.actorKind,
    status: item.outcome,
    risk: item.createdAt,
    targetType: 'audit-log',
    backendKind: 'admin-audit-log',
    targetTo: item.detailPath || null,
    detailItems: [
      { label: '来源', value: item.sourceKind },
      { label: '领域', value: item.domain },
      { label: '动作', value: item.action },
      { label: '对象', value: item.targetLabel || `${item.targetType} · ${item.targetId}` },
      { label: '结果', value: item.outcome },
      { label: '操作时间', value: item.createdAt },
      { label: '请求追踪', value: item.requestId },
    ],
  }
}

export async function backendAdminAuditLogRowsPage(
  filters: AdminAuditLogFilters = {},
  page: CursorPageRequest = {},
): Promise<CursorPage<AdminRow>> {
  const response = await backendAdminAuditLogsPage(filters, page)
  return {
    items: response.items.map(adminAuditLogRow),
    nextCursor: response.nextCursor,
  }
}

export async function backendAdminAuditLogRows() {
  return (await backendAdminAuditLogRowsPage({}, { limit: 100 })).items
}
