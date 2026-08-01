import type {
  AdminUser,
  AdminUserDetail,
  AdminUserDirectorySummary,
  AdminUserLimit,
  AdminUserList,
  AdminUserPage,
  AdminUserPermissionRequest,
  AdminUserStatusRequest,
  ListAdminUsersData,
} from '@/api/generated/openapi'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'

export type AdminUserDirectoryQuery = Required<NonNullable<ListAdminUsersData['query']>>
export type AdminUserDirectoryStatus = AdminUserDirectoryQuery['status']
export type AdminUserDirectoryRole = AdminUserDirectoryQuery['role']
export type AdminUserDirectoryLinuxDo = AdminUserDirectoryQuery['linuxDo']
export type AdminUserDirectorySort = AdminUserDirectoryQuery['sort']
export type AdminUserStatus = AdminUser['accountStatus']
export type {
  AdminUser,
  AdminUserDetail,
  AdminUserDirectorySummary,
  AdminUserLimit,
  AdminUserList,
  AdminUserPage,
}

export const defaultAdminUserDirectoryQuery: AdminUserDirectoryQuery = {
  page: 1,
  limit: 20,
  search: '',
  status: 'all',
  role: 'all',
  linuxDo: 'all',
  sort: 'created_desc',
}

const directoryStatuses: AdminUserDirectoryStatus[] = ['all', 'active', 'suspended', 'banned', 'archived']
const directoryRoles: AdminUserDirectoryRole[] = ['all', 'admin', 'user']
const directoryLinuxDoValues: AdminUserDirectoryLinuxDo[] = ['all', 'bound', 'unbound']
const directorySorts: AdminUserDirectorySort[] = ['created_desc', 'created_asc', 'active_desc', 'username_asc', 'username_desc']
const directoryLimits: AdminUserLimit[] = [20, 50, 100]

function firstQueryValue(value: unknown) {
  return Array.isArray(value) ? value[0] : value
}

function queryString(value: unknown) {
  const first = firstQueryValue(value)
  return typeof first === 'string' ? first : ''
}

function positiveInteger(value: unknown, fallback: number) {
  const parsed = Number(queryString(value))
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback
}

function oneOf<T extends string>(value: unknown, values: readonly T[], fallback: T): T {
  const candidate = queryString(value) as T
  return values.includes(candidate) ? candidate : fallback
}

export function normalizeAdminUserDirectoryQuery(query: Record<string, unknown>): AdminUserDirectoryQuery {
  const parsedLimit = positiveInteger(query.limit, defaultAdminUserDirectoryQuery.limit) as AdminUserLimit
  return {
    page: positiveInteger(query.page, defaultAdminUserDirectoryQuery.page) as AdminUserPage,
    limit: directoryLimits.includes(parsedLimit) ? parsedLimit : defaultAdminUserDirectoryQuery.limit,
    search: queryString(query.search).trim().slice(0, 100),
    status: oneOf(query.status, directoryStatuses, defaultAdminUserDirectoryQuery.status),
    role: oneOf(query.role, directoryRoles, defaultAdminUserDirectoryQuery.role),
    linuxDo: oneOf(query.linuxDo, directoryLinuxDoValues, defaultAdminUserDirectoryQuery.linuxDo),
    sort: oneOf(query.sort, directorySorts, defaultAdminUserDirectoryQuery.sort),
  }
}

export function adminUserDirectoryRouteQuery(query: AdminUserDirectoryQuery) {
  const routeQuery: Record<string, string> = {}
  if (query.search) routeQuery.search = query.search
  if (query.status !== defaultAdminUserDirectoryQuery.status) routeQuery.status = query.status
  if (query.role !== defaultAdminUserDirectoryQuery.role) routeQuery.role = query.role
  if (query.linuxDo !== defaultAdminUserDirectoryQuery.linuxDo) routeQuery.linuxDo = query.linuxDo
  if (query.sort !== defaultAdminUserDirectoryQuery.sort) routeQuery.sort = query.sort
  if (query.page !== defaultAdminUserDirectoryQuery.page) routeQuery.page = String(query.page)
  if (query.limit !== defaultAdminUserDirectoryQuery.limit) routeQuery.limit = String(query.limit)
  return routeQuery
}

export function serializeAdminUserDirectoryQuery(query: AdminUserDirectoryQuery) {
  const params = new URLSearchParams({
    page: String(query.page),
    limit: String(query.limit),
    status: query.status,
    role: query.role,
    linuxDo: query.linuxDo,
    sort: query.sort,
  })
  if (query.search) params.set('search', query.search)
  return params.toString()
}

export async function backendAdminUserDirectory(query: AdminUserDirectoryQuery) {
  await ensureBackendSession('admin', true)
  return backendRequest<AdminUserList>(`/api/v1/admin/users?${serializeAdminUserDirectoryQuery(query)}`)
}

export async function backendAdminUserDetail(userId: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<AdminUserDetail>(`/api/v1/admin/users/${encodeURIComponent(userId)}`)
}

export async function backendUpdateAdminUserStatus(input: {
  userId: string
  version: number
  status: AdminUserStatusRequest['status']
  reason: string
}) {
  await ensureBackendSession('admin', true)
  return backendMutation<AdminUserDetail>(
    `/api/v1/admin/users/${encodeURIComponent(input.userId)}/status`,
    { status: input.status, reason: input.reason.trim() } satisfies AdminUserStatusRequest,
    { ifMatch: input.version, idempotencyPrefix: 'admin-user-status' },
  )
}

export async function backendUpdateAdminUserPermission(input: {
  userId: string
  version: number
  isAdmin: boolean
  reason: string
}) {
  await ensureBackendSession('admin', true)
  return backendMutation<AdminUserDetail>(
    `/api/v1/admin/users/${encodeURIComponent(input.userId)}/admin-permission`,
    { isAdmin: input.isAdmin, reason: input.reason.trim() } satisfies AdminUserPermissionRequest,
    { ifMatch: input.version, idempotencyPrefix: 'admin-user-permission' },
  )
}
