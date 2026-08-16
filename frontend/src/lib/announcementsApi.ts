import { announcementAuditLogSeeds, announcementSeeds } from '@/data/announcements.mock'
import {
  assertValidAnnouncementFormInput,
  getAnnouncementDisplayStatus,
  isAnnouncementActive,
  isAnnouncementDismissed,
  isAnnouncementUnread,
  isAnnouncementUserVisible,
  sanitizeAnnouncementUrl,
  sortAnnouncementsForHome,
} from '@/lib/announcementUtils'
import {
  readAnnouncementReceipts,
  upsertAnnouncementReceipt,
} from '@/lib/announcementStorage'
import {
  backendMutation,
  backendRequest,
  ensureBackendSession,
  requireBackendSession,
  shouldUseRealBackend,
} from '@/lib/backendClient'
import { collectCursorPages, normalizeNextCursor, paginateCursorItems, type CursorPage, type CursorPageRequest } from '@/lib/cursorPagination'
import { getMockIdentity } from '@/lib/mockAuth'
import type {
  Announcement,
  AnnouncementAudience,
  AnnouncementAuditAction,
  AnnouncementAuditLog,
  AnnouncementChannel,
  AnnouncementFormInput,
  AnnouncementStatus,
  PublicAnnouncement,
} from '@/types/announcement'

const announcementStorageKey = 'marketplace.announcement.admin-drafts'
const announcementAuditStorageKey = 'marketplace.announcement.audit-logs'
const currentAdminId = 'admin-demo'
const currentAdminName = '演示管理员'

let announcementStore = readSessionStore<Announcement[]>(announcementStorageKey, announcementSeeds)
  .map(normalizeStoredAnnouncement)
let announcementAuditLogStore = readSessionStore<AnnouncementAuditLog[]>(announcementAuditStorageKey, announcementAuditLogSeeds)

const wait = () => new Promise(resolve => setTimeout(resolve, 80))

type ListResponse<T> = { items: T[], nextCursor?: string | null }
type CountResponse = { count: number }

export type AdminAnnouncementPageFilters = {
  q?: string
  statusGroup?: 'working' | AnnouncementStatus | 'history' | 'all'
}

export async function getAnnouncements(): Promise<Announcement[]> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    const response = await backendRequest<ListResponse<Announcement>>('/api/v1/announcements')
    return response.items
  }
  await wait()
  return clone(announcementStore
    .filter(item => isAnnouncementUserVisible(item))
    .filter(matchesCurrentMockAudience)
    .sort(compareAnnouncementsByTimeDesc))
}

export async function getActiveAnnouncements(channel?: AnnouncementChannel): Promise<Announcement[]> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    const query = channel ? `?channel=${encodeURIComponent(channel)}` : ''
    const response = await backendRequest<ListResponse<Announcement>>(`/api/v1/announcements/active${query}`)
    return response.items
  }
  await wait()
  return clone(announcementStore
    .filter(item => isAnnouncementActive(item))
    .filter(matchesCurrentMockAudience)
    .filter(item => !channel || item.channels.includes(channel))
    .filter(item => channel !== 'home_banner' || !isAnnouncementDismissed(item, readAnnouncementReceipts()[item.id]))
    .sort(compareAnnouncementsByTimeDesc))
}

export async function getPublicActiveAnnouncements(channel: 'global_bar' | 'modal'): Promise<PublicAnnouncement[]> {
  if (shouldUseRealBackend()) {
    const response = await backendRequest<ListResponse<PublicAnnouncement>>(`/api/v1/announcements/public-active?channel=${encodeURIComponent(channel)}`, {}, { affectsSessionCache: false })
    return response.items
  }
  await wait()
  return clone(announcementStore
    .filter(item => item.audience.type === 'all')
    .filter(item => item.level === 'important' || item.level === 'critical')
    .filter(item => isAnnouncementActive(item) && item.channels.includes(channel))
    .sort(compareAnnouncementsByTimeDesc)
    .map(toPublicAnnouncement))
}

export async function getActiveHomeAnnouncement(): Promise<Announcement | null> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    return backendRequest<Announcement | null>('/api/v1/announcements/home')
  }
  await wait()
  const receipts = readAnnouncementReceipts()
  const candidates = announcementStore
    .filter(item => item.channels.includes('home_banner'))
    .filter(item => isAnnouncementActive(item))
    .filter(matchesCurrentMockAudience)
    .filter(item => !isAnnouncementDismissed(item, receipts[item.id]))

  return clone(sortAnnouncementsForHome(candidates, receipts)[0] ?? null)
}

export async function getAnnouncementBySlug(slug: string): Promise<Announcement | null> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    return backendRequest<Announcement>(`/api/v1/announcements/${encodeURIComponent(slug)}`)
  }
  await wait()
  const announcement = announcementStore.find(item => item.slug === slug)
  if (!announcement || !isAnnouncementUserVisible(announcement) || !matchesCurrentMockAudience(announcement)) return null
  return clone(announcement)
}

export async function markAnnouncementRead(announcementId: string): Promise<void> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    await backendMutation(`/api/v1/me/announcements/${encodeURIComponent(announcementId)}/read`, {})
    return
  }
  await wait()
  const announcement = findUserVisibleAnnouncement(announcementId)
  if (!upsertAnnouncementReceipt(announcement, { readAt: nowIso() })) throw new Error('公告已读状态保存失败。')
}

export async function dismissAnnouncement(announcementId: string): Promise<void> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    await backendMutation(`/api/v1/me/announcements/${encodeURIComponent(announcementId)}/dismiss`, {})
    return
  }
  await wait()
  const announcement = findUserVisibleAnnouncement(announcementId)
  if (!upsertAnnouncementReceipt(announcement, { firstSeenAt: nowIso(), dismissedAt: nowIso() })) throw new Error('公告关闭状态保存失败。')
}

export async function markAnnouncementSeen(announcementId: string): Promise<void> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    await backendMutation(`/api/v1/me/announcements/${encodeURIComponent(announcementId)}/seen`, {})
    return
  }
  await wait()
  const announcement = findUserVisibleAnnouncement(announcementId)
  if (!upsertAnnouncementReceipt(announcement, { firstSeenAt: nowIso() })) throw new Error('公告展示状态保存失败。')
}

export async function acknowledgeAnnouncement(announcementId: string): Promise<void> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    await backendMutation(`/api/v1/me/announcements/${encodeURIComponent(announcementId)}/acknowledge`, {})
    return
  }
  await wait()
  const announcement = findUserVisibleAnnouncement(announcementId)
  if (announcement.level !== 'critical' || !announcement.requiresAck) throw new Error('该公告不需要确认知悉。')
  const acknowledgedAt = nowIso()
  if (!upsertAnnouncementReceipt(announcement, { firstSeenAt: acknowledgedAt, acknowledgedAt })) throw new Error('公告确认状态保存失败。')
}

export async function getAnnouncementUnreadCount(): Promise<number> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    const response = await backendRequest<CountResponse>('/api/v1/me/announcements/unread-count')
    return response.count
  }
  await wait()
  const receipts = readAnnouncementReceipts()
  return announcementStore
    .filter(item => isAnnouncementUserVisible(item))
    .filter(matchesCurrentMockAudience)
    .filter(item => isAnnouncementUnread(item, receipts[item.id]))
    .length
}

export async function getImportantAnnouncementUnreadCount(): Promise<number> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    const response = await backendRequest<CountResponse>('/api/v1/me/announcements/important-unread-count')
    return response.count
  }
  await wait()
  const receipts = readAnnouncementReceipts()
  return announcementStore
    .filter(item => isAnnouncementUserVisible(item))
    .filter(matchesCurrentMockAudience)
    .filter(item => item.level === 'important' || item.level === 'critical')
    .filter(item => isAnnouncementUnread(item, receipts[item.id]))
    .length
}

function filterAdminAnnouncementItems(items: Announcement[], filters: AdminAnnouncementPageFilters) {
  const query = filters.q?.trim().toLocaleLowerCase()
  return items.filter((item) => {
    const status = getAnnouncementDisplayStatus(item)
    const matchesStatus = !filters.statusGroup || filters.statusGroup === 'all'
      || filters.statusGroup === status
      || (filters.statusGroup === 'working' && ['draft', 'scheduled', 'published'].includes(status))
      || (filters.statusGroup === 'history' && ['offline', 'expired'].includes(status))
    if (!matchesStatus) return false
    if (!query) return true
    return [item.id, item.title, item.summary, item.category, item.level, status, item.channels.join(' '), item.isPinned ? '置顶' : '']
      .join(' ')
      .toLocaleLowerCase()
      .includes(query)
  })
}

export async function getAdminAnnouncementsPage(filters: AdminAnnouncementPageFilters = {}, page: CursorPageRequest = {}): Promise<CursorPage<Announcement>> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const params = new URLSearchParams()
    if (filters.q?.trim()) params.set('q', filters.q.trim())
    if (filters.statusGroup) params.set('statusGroup', filters.statusGroup)
    if (page.limit) params.set('limit', String(page.limit))
    if (page.cursor) params.set('cursor', page.cursor)
    const response = await backendRequest<ListResponse<Announcement>>(`/api/v1/admin/announcements?${params.toString()}`)
    return { items: response.items, nextCursor: normalizeNextCursor(response.nextCursor) }
  }
  await wait()
  return paginateCursorItems(clone(filterAdminAnnouncementItems(announcementStore, filters).sort(compareAnnouncementsByTimeDesc)), page)
}

export async function getAdminAnnouncements(): Promise<Announcement[]> {
  return collectCursorPages(page => getAdminAnnouncementsPage({}, page))
}

export async function getAnnouncementById(id: string): Promise<Announcement | null> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendRequest<Announcement>(`/api/v1/admin/announcements/${encodeURIComponent(id)}`)
  }
  await wait()
  return clone(announcementStore.find(item => item.id === id) ?? null)
}

export async function createAnnouncement(input: AnnouncementFormInput): Promise<Announcement> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<Announcement>('/api/v1/admin/announcements', input, {
      idempotencyPrefix: 'announcement-create',
    })
  }
  await wait()
  const normalized = normalizeAnnouncementInput(input)
  const actionTime = nowIso()
  const announcement: Announcement = {
    id: `ann-${Date.now()}`,
    slug: createSlug(normalized.title),
    ...normalized,
    status: 'draft',
    version: 1,
    createdBy: currentAdminId,
    updatedBy: currentAdminId,
    contentUpdatedAt: actionTime,
    createdAt: actionTime,
    updatedAt: actionTime,
  }
  announcementStore = [announcement, ...announcementStore]
  persistAnnouncementStores()
  appendAuditLog('announcement_created', announcement, '创建公告草稿')
  return clone(announcement)
}

export async function updateAnnouncement(id: string, input: AnnouncementFormInput): Promise<Announcement> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<Announcement>(`/api/v1/admin/announcements/${encodeURIComponent(id)}`, input, {
      method: 'PATCH',
    })
  }
  await wait()
  const announcement = findAnnouncement(id)
  const beforeStatus = getAnnouncementDisplayStatus(announcement)
  const normalized = normalizeAnnouncementInput(input)
  const actionTime = nowIso()
  const contentChanged = hasUserVisibleContentChanged(announcement, normalized)
  const deliveryChanged = hasDeliveryRevisionChanged(announcement, normalized)
  const next: Announcement = {
    ...announcement,
    ...normalized,
    slug: announcement.slug,
    version: announcement.version + (deliveryChanged ? 1 : 0),
    updatedBy: currentAdminId,
    contentUpdatedAt: contentChanged ? actionTime : announcement.contentUpdatedAt,
    updatedAt: actionTime,
  }
  announcementStore = announcementStore.map(item => item.id === id ? next : item)
  persistAnnouncementStores()
  if (beforeStatus === 'published') appendAuditLog('announcement_updated', next, '编辑已发布公告')
  return clone(next)
}

export async function publishAnnouncement(id: string): Promise<Announcement> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<Announcement>(`/api/v1/admin/announcements/${encodeURIComponent(id)}/publish`, {})
  }
  await wait()
  const announcement = findAnnouncement(id)
  const now = new Date()
  const actionTime = now.toISOString()
  const isScheduled = new Date(announcement.publishAt).getTime() > now.getTime()
  const effectivePublishTime = isScheduled ? new Date(announcement.publishAt).getTime() : now.getTime()
  if (announcement.expireAt && new Date(announcement.expireAt).getTime() <= effectivePublishTime) {
    throw new Error(isScheduled
      ? '结束时间必须晚于计划发布时间，请调整后再发布。'
      : '结束时间已过，请调整结束时间后再发布。')
  }
  const status = isScheduled ? 'scheduled' : 'published'
  const next: Announcement = {
    ...announcement,
    status,
    publishAt: isScheduled ? announcement.publishAt : actionTime,
    version: announcement.version + 1,
    updatedBy: currentAdminId,
    updatedAt: actionTime,
  }
  announcementStore = announcementStore.map(item => item.id === id ? next : item)
  persistAnnouncementStores()
  appendAuditLog('announcement_published', next, status === 'scheduled' ? '设置未来发布时间' : '立即发布公告', actionTime)
  return clone(next)
}

export async function offlineAnnouncement(id: string, reason: string): Promise<Announcement> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<Announcement>(`/api/v1/admin/announcements/${encodeURIComponent(id)}/offline`, { reason })
  }
  await wait()
  const trimmedReason = reason.trim()
  if (!trimmedReason) throw new Error('下线公告必须填写原因。')
  const announcement = findAnnouncement(id)
  const displayStatus = getAnnouncementDisplayStatus(announcement)
  if (displayStatus !== 'published' && displayStatus !== 'scheduled') throw new Error('只有发布中或待发布公告可以下线。')

  const next: Announcement = {
    ...announcement,
    status: 'offline',
    updatedBy: currentAdminId,
    updatedAt: nowIso(),
  }
  announcementStore = announcementStore.map(item => item.id === id ? next : item)
  persistAnnouncementStores()
  appendAuditLog('announcement_offlined', next, trimmedReason)
  return clone(next)
}

export async function duplicateAnnouncement(id: string): Promise<Announcement> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<Announcement>(`/api/v1/admin/announcements/${encodeURIComponent(id)}/duplicate`, {}, {
      idempotencyPrefix: 'announcement-duplicate',
    })
  }
  await wait()
  const announcement = findAnnouncement(id)
  const actionTime = nowIso()
  const duplicated: Announcement = {
    ...announcement,
    id: `ann-${Date.now()}`,
    slug: createSlug(`${announcement.title} 副本`),
    title: `${announcement.title} 副本`,
    status: 'draft',
    version: 1,
    createdBy: currentAdminId,
    updatedBy: currentAdminId,
    contentUpdatedAt: actionTime,
    createdAt: actionTime,
    updatedAt: actionTime,
  }
  announcementStore = [duplicated, ...announcementStore]
  persistAnnouncementStores()
  appendAuditLog('announcement_duplicated', duplicated, `复制自 ${announcement.title}`)
  return clone(duplicated)
}

export async function getAnnouncementAuditLogs(): Promise<AnnouncementAuditLog[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<AnnouncementAuditLog>>('/api/v1/admin/announcement-audit-logs')
    return response.items
  }
  await wait()
  return clone(announcementAuditLogStore)
}

function normalizeAnnouncementInput(input: AnnouncementFormInput): AnnouncementFormInput {
  const normalized: AnnouncementFormInput = {
    ...input,
    title: input.title.trim(),
    summary: input.summary.trim(),
    contentMarkdown: input.contentMarkdown.trim(),
    channels: normalizeChannels(input.channels),
    audience: normalizeAudience(input.audience),
    ctaLabel: input.ctaLabel?.trim() || undefined,
    ctaUrl: sanitizeAnnouncementUrl(input.ctaUrl),
    expireAt: input.expireAt?.trim() || undefined,
  }
  assertValidAnnouncementFormInput(normalized)
  return normalized
}

function hasDeliveryRevisionChanged(announcement: Announcement, input: AnnouncementFormInput) {
  return hasUserVisibleContentChanged(announcement, input)
    || JSON.stringify(announcement.channels) !== JSON.stringify(input.channels)
    || JSON.stringify(announcement.audience) !== JSON.stringify(input.audience)
    || announcement.isDismissible !== input.isDismissible
    || announcement.requiresAck !== input.requiresAck
}

function hasUserVisibleContentChanged(announcement: Announcement, input: AnnouncementFormInput) {
  return announcement.title !== input.title
    || announcement.summary !== input.summary
    || announcement.contentMarkdown !== input.contentMarkdown
    || announcement.category !== input.category
    || announcement.level !== input.level
    || announcement.ctaLabel !== input.ctaLabel
    || announcement.ctaUrl !== input.ctaUrl
}

function normalizeStoredAnnouncement(announcement: Announcement): Announcement {
  return {
    ...announcement,
    contentUpdatedAt: announcement.contentUpdatedAt || announcement.publishAt,
    requiresAck: announcement.requiresAck ?? false,
    audience: announcement.audience ?? { type: 'all' },
  }
}

function appendAuditLog(action: AnnouncementAuditAction, announcement: Announcement, reason?: string, createdAt = nowIso()) {
  announcementAuditLogStore = [
    {
      id: `ann-audit-${Date.now()}-${announcementAuditLogStore.length + 1}`,
      action,
      announcementId: announcement.id,
      announcementTitle: announcement.title,
      operatorId: currentAdminId,
      operatorName: currentAdminName,
      reason,
      createdAt,
    },
    ...announcementAuditLogStore,
  ]
  persistAnnouncementStores()
}

function findAnnouncement(id: string) {
  const announcement = announcementStore.find(item => item.id === id)
  if (!announcement) throw new Error(`未找到公告：${id}`)
  return announcement
}

function findUserVisibleAnnouncement(id: string) {
  const announcement = findAnnouncement(id)
  if (!isAnnouncementUserVisible(announcement) || !matchesCurrentMockAudience(announcement)) throw new Error('公告当前不可见。')
  return announcement
}

function matchesCurrentMockAudience(announcement: Announcement) {
  const identity = getMockIdentity()
  if (!identity) return false
  switch (announcement.audience.type) {
    case 'all':
      return true
    case 'specific_users':
      return announcement.audience.userIds.includes(identity.id)
    case 'roles': {
      const roles = announcement.audience.roles
      const buyer = Boolean(identity.studentClaim || identity.linuxDoBinding.bound)
      const merchant = identity.linuxDoBinding.bound
      const admin = identity.persona === 'admin'
      return (buyer && roles.includes('buyer'))
        || (merchant && roles.includes('merchant'))
        || (admin && roles.includes('admin'))
    }
  }
}

function normalizeChannels(channels: AnnouncementChannel[]): AnnouncementChannel[] {
  const selected = new Set<AnnouncementChannel>(['message_center', ...channels])
  return (['message_center', 'home_banner', 'global_bar', 'modal'] as const).filter(channel => selected.has(channel))
}

function normalizeAudience(audience: AnnouncementAudience): AnnouncementAudience {
  if (audience.type === 'roles') return { type: 'roles', roles: [...new Set(audience.roles)].sort() }
  if (audience.type === 'specific_users') return { type: 'specific_users', userIds: [...new Set(audience.userIds)].sort() }
  return { type: 'all' }
}

function toPublicAnnouncement(item: Announcement): PublicAnnouncement {
  return {
    id: item.id,
    slug: item.slug,
    title: item.title,
    summary: item.summary,
    contentMarkdown: item.contentMarkdown,
    category: item.category,
    level: item.level === 'critical' ? 'critical' : 'important',
    channels: [...item.channels],
    audience: { type: 'all' },
    isPinned: item.isPinned,
    isDismissible: item.isDismissible,
    requiresAck: item.requiresAck,
    ctaLabel: item.ctaLabel,
    ctaUrl: item.ctaUrl,
    publishAt: item.publishAt,
    expireAt: item.expireAt,
    contentUpdatedAt: item.contentUpdatedAt,
    version: item.version,
  }
}

function readSessionStore<T>(key: string, seed: T): T {
  if (typeof window === 'undefined') return clone(seed)
  try {
    const stored = window.sessionStorage.getItem(key)
    if (!stored) return clone(seed)
    return JSON.parse(stored) as T
  } catch {
    return clone(seed)
  }
}

function persistAnnouncementStores() {
  if (typeof window === 'undefined') return
  try {
    window.sessionStorage.setItem(announcementStorageKey, JSON.stringify(announcementStore))
    window.sessionStorage.setItem(announcementAuditStorageKey, JSON.stringify(announcementAuditLogStore))
  } catch {
    return
  }
}

function clone<T>(value: T): T {
  return structuredClone(value)
}

function nowIso() {
  return new Date().toISOString()
}

function compareAnnouncementsByTimeDesc(a: Announcement, b: Announcement) {
  return new Date(b.publishAt).getTime() - new Date(a.publishAt).getTime()
}

function createSlug(title: string) {
  const ascii = title
    .toLowerCase()
    .replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return `${ascii || 'announcement'}-${Date.now()}`
}
