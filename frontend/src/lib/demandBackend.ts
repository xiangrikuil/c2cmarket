import type { AdminRow, DemandRecord, SubmitDemandPayload } from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'

type ListResponse<T> = { items: T[] }

type BackendDemand = {
  id: string
  publisherUserId?: string
  publisherUsername: string
  publisherName: string
  title: string
  maxPriceCny: string
  regionCode: string
  ownerPreference: 'personal' | 'only_personal' | 'any'
  sourceUrl: string
  note: string
  status: 'pending_review' | 'active' | 'changes_requested' | 'rejected' | 'closed' | 'taken_down'
  reviewReason?: string
  reviewedByAdminId?: string
  reviewedAt?: string
  closedAt?: string
  createdAt: string
  updatedAt: string
  version: number
}

function numberFromDecimal(value: string, fallback = 0) {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function formatTime(value: string | undefined) {
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

function regionLabel(code: string) {
  const normalized = code.trim().toLowerCase()
  const labels: Record<string, string> = {
    any: '不限',
    us: '美国区',
    hk: '香港区',
    ph: '菲律宾区',
    tr: '土耳其区',
    jp: '日本区',
    sg: '新加坡区',
  }
  return labels[normalized] ?? code
}

function regionCode(value: string) {
  if (value.includes('美国')) return 'us'
  if (value.includes('香港')) return 'hk'
  if (value.includes('菲律宾')) return 'ph'
  if (value.includes('土耳其')) return 'tr'
  if (value.includes('日本')) return 'jp'
  if (value.includes('新加坡')) return 'sg'
  if (value.includes('不限')) return 'any'
  return value.trim().toLowerCase().replace(/[^a-z0-9_-]+/g, '_') || 'any'
}

function ownerPreferenceToBackend(value: SubmitDemandPayload['ownerPreference']): BackendDemand['ownerPreference'] {
  if (value === 'only-personal') return 'only_personal'
  if (value === 'personal') return 'personal'
  return 'any'
}

function ownerPreferenceToFrontend(value: BackendDemand['ownerPreference']): DemandRecord['ownerPreference'] {
  if (value === 'only_personal') return 'only-personal'
  if (value === 'personal') return 'personal'
  return 'any'
}

function statusLabel(value: BackendDemand['status']): DemandRecord['status'] {
  if (value === 'active') return '匹配中'
  if (value === 'closed' || value === 'taken_down') return '已关闭'
  if (value === 'pending_review' || value === 'changes_requested') return '待审核'
  if (value === 'rejected') return '已关闭'
  return '待审核'
}

function adminStatusLabel(value: BackendDemand['status']) {
  if (value === 'active') return '已通过'
  if (value === 'pending_review') return '待处理'
  if (value === 'changes_requested') return '待复核'
  if (value === 'rejected') return '已拒绝'
  if (value === 'taken_down') return '已下架'
  if (value === 'closed') return '已关闭'
  return value
}

export function mapBackendDemand(item: BackendDemand): DemandRecord {
  const preference = ownerPreferenceToFrontend(item.ownerPreference)
  return {
    id: item.id,
    title: item.title,
    maxPrice: numberFromDecimal(item.maxPriceCny),
    require: `${regionLabel(item.regionCode)} · ${preference === 'only-personal' ? '只看个人车主' : preference === 'personal' ? '个人车主优先' : '不限车主'} · ${item.note || '等待车主匹配'}`,
    poster: item.publisherName || item.publisherUsername || '发布者',
    trustLevel: 3,
    linuxdoPost: '已绑定求车帖',
    status: statusLabel(item.status),
    region: regionLabel(item.regionCode),
    ownerPreference: preference,
    sourceUrl: item.sourceUrl,
    note: item.note,
    createdAt: formatTime(item.createdAt),
    updatedAt: formatTime(item.updatedAt),
    backendKind: 'demand',
    backendVersion: item.version,
  }
}

export async function backendDemands() {
  const response = await backendRequest<ListResponse<BackendDemand>>('/api/v1/demands')
  return response.items.map(mapBackendDemand)
}

export async function backendDemandById(id: string) {
  try {
    return mapBackendDemand(await backendRequest<BackendDemand>(`/api/v1/demands/${encodeURIComponent(id)}`))
  } catch {
    await ensureBackendSession('buyer', false)
    return mapBackendDemand(await backendRequest<BackendDemand>(`/api/v1/me/demands/${encodeURIComponent(id)}`))
  }
}

export async function backendSubmitDemand(payload: SubmitDemandPayload) {
  await ensureBackendSession('buyer', false)
  const created = await backendMutation<BackendDemand>('/api/v1/demands', {
    title: payload.title,
    maxPriceCny: String(payload.maxPrice),
    regionCode: regionCode(payload.region),
    ownerPreference: ownerPreferenceToBackend(payload.ownerPreference),
    sourceUrl: payload.sourceUrl,
    note: payload.note,
  }, {
    idempotencyPrefix: 'demand-create',
  })
  return mapBackendDemand(created)
}

async function backendMyDemand(id: string) {
  await ensureBackendSession('buyer', false)
  return backendRequest<BackendDemand>(`/api/v1/me/demands/${encodeURIComponent(id)}`)
}

async function backendOwnerDemandAction(id: string, action: 'close' | 'reopen') {
  const detail = await backendMyDemand(id)
  const updated = await backendMutation<BackendDemand>(`/api/v1/me/demands/${encodeURIComponent(id)}/${action}`, {}, {
    idempotencyPrefix: `demand-${action}`,
    ifMatch: detail.version,
  })
  return mapBackendDemand(updated)
}

export async function backendCloseDemand(id: string) {
  const detail = await backendMyDemand(id)
  return backendOwnerDemandAction(id, detail.status === 'closed' ? 'reopen' : 'close')
}

export async function backendReopenDemand(id: string) {
  return backendOwnerDemandAction(id, 'reopen')
}

function adminDetailItems(item: BackendDemand): AdminRow['detailItems'] {
  return [
    { label: '后端状态', value: item.status },
    { label: '版本', value: String(item.version) },
    { label: '预算', value: `¥${numberFromDecimal(item.maxPriceCny)}/月` },
    { label: '地区', value: regionLabel(item.regionCode) },
    { label: '车主偏好', value: ownerPreferenceToFrontend(item.ownerPreference) },
    { label: '来源', value: item.sourceUrl },
    { label: '复核原因', value: item.reviewReason || '暂无' },
    { label: '更新时间', value: formatTime(item.updatedAt) },
  ]
}

function adminRow(item: BackendDemand): AdminRow {
  const record = mapBackendDemand(item)
  return {
    id: item.id,
    primary: item.title,
    secondary: `最高 ¥${record.maxPrice}/月 · ${record.require}`,
    owner: `${item.publisherName || item.publisherUsername} · 真实后端`,
    status: adminStatusLabel(item.status),
    risk: item.status === 'pending_review' ? '等待管理员审核' : item.reviewReason || '原帖已绑定',
    targetType: 'demand',
    backendKind: 'demand',
    backendVersion: item.version,
    detailItems: adminDetailItems(item),
    targetTo: `/demands/${item.id}`,
  }
}

export async function backendAdminDemandRows() {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<BackendDemand>>('/api/v1/admin/demands')
  return response.items.map(adminRow)
}

async function adminDemand(id: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<BackendDemand>(`/api/v1/admin/demands/${encodeURIComponent(id)}`)
}

async function runAdminDemandAction(row: AdminRow, action: 'approve' | 'request-changes' | 'reject' | 'take-down' | 'restore', reason: string) {
  const detail = await adminDemand(row.id)
  const updated = await backendMutation<BackendDemand>(`/api/v1/admin/demands/${encodeURIComponent(row.id)}/${action}`, {
    reason: reason || '管理台审核操作',
  }, {
    idempotencyPrefix: `demand-admin-${action}`,
    ifMatch: detail.version,
  })
  return adminRow(updated)
}

export async function backendUpdateAdminDemandStatus(row: AdminRow, status: string, reason: string) {
  if (row.targetType !== 'demand') return row
  if (status === '已通过') return runAdminDemandAction(row, 'approve', reason)
  if (status === '已恢复') return runAdminDemandAction(row, 'restore', reason)
  if (status === '已下架' || status === '已关闭') return runAdminDemandAction(row, 'take-down', reason)
  if (status === '已拒绝') return runAdminDemandAction(row, 'reject', reason)
  return runAdminDemandAction(row, 'request-changes', reason)
}

export async function backendRunAdminDemandAction(row: AdminRow, action: 'approve' | 'request_changes' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban', reason: string) {
  if (row.targetType !== 'demand') return row
  if (action === 'approve') return runAdminDemandAction(row, 'approve', reason)
  if (action === 'restore') return runAdminDemandAction(row, 'restore', reason)
  if (action === 'take_down' || action === 'restrict' || action === 'suspend' || action === 'ban') return runAdminDemandAction(row, 'take-down', reason)
  if (action === 'request_changes' || action === 'warn') return runAdminDemandAction(row, 'request-changes', reason)
  return runAdminDemandAction(row, 'reject', reason)
}
