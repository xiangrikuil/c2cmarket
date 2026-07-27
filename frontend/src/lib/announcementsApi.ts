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
import type {
  Announcement,
  AnnouncementAuditAction,
  AnnouncementAuditLog,
  AnnouncementChannel,
  AnnouncementFormInput,
} from '@/types/announcement'

const announcementStorageKey = 'marketplace.announcement.admin-drafts'
const announcementAuditStorageKey = 'marketplace.announcement.audit-logs'
const currentAdminId = 'admin-demo'
const currentAdminName = '演示管理员'

let announcementStore = readSessionStore<Announcement[]>(announcementStorageKey, announcementSeeds)
let announcementAuditLogStore = readSessionStore<AnnouncementAuditLog[]>(announcementAuditStorageKey, announcementAuditLogSeeds)

const wait = () => new Promise(resolve => setTimeout(resolve, 80))

type ListResponse<T> = { items: T[] }
type CountResponse = { count: number }

export async function getAnnouncements(): Promise<Announcement[]> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    const response = await backendRequest<ListResponse<Announcement>>('/api/v1/announcements')
    return response.items
  }
  await wait()
  return clone(announcementStore
    .filter(item => isAnnouncementUserVisible(item))
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
    .filter(item => !channel || item.channels.includes(channel))
    .filter(item => channel !== 'home_banner' || !isAnnouncementDismissed(item, readAnnouncementReceipts()[item.id]))
    .sort(compareAnnouncementsByTimeDesc))
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
  if (!announcement || !isAnnouncementUserVisible(announcement)) return null
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
  upsertAnnouncementReceipt(announcement, { readAt: nowIso() })
}

export async function dismissAnnouncement(announcementId: string): Promise<void> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    await backendMutation(`/api/v1/me/announcements/${encodeURIComponent(announcementId)}/dismiss`, {})
    return
  }
  await wait()
  const announcement = findUserVisibleAnnouncement(announcementId)
  upsertAnnouncementReceipt(announcement, { dismissedAt: nowIso() })
}

export async function markAnnouncementSeen(announcementId: string): Promise<void> {
  if (shouldUseRealBackend()) {
    await requireBackendSession()
    await backendMutation(`/api/v1/me/announcements/${encodeURIComponent(announcementId)}/seen`, {})
    return
  }
  await wait()
  const announcement = findUserVisibleAnnouncement(announcementId)
  upsertAnnouncementReceipt(announcement, { firstSeenAt: nowIso() })
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
    .filter(item => item.level === 'important')
    .filter(item => isAnnouncementUnread(item, receipts[item.id]))
    .length
}

export async function getAdminAnnouncements(): Promise<Announcement[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<Announcement>>('/api/v1/admin/announcements')
    return response.items
  }
  await wait()
  return clone(announcementStore.sort(compareAnnouncementsByTimeDesc))
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
  const announcement: Announcement = {
    id: `ann-${Date.now()}`,
    slug: createSlug(normalized.title),
    ...normalized,
    status: 'draft',
    audience: { type: 'all' },
    version: 1,
    createdBy: currentAdminId,
    updatedBy: currentAdminId,
    createdAt: nowIso(),
    updatedAt: nowIso(),
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
  const next: Announcement = {
    ...announcement,
    ...normalized,
    slug: announcement.slug,
    version: announcement.version + 1,
    updatedBy: currentAdminId,
    updatedAt: nowIso(),
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
  const status = new Date(announcement.publishAt).getTime() > Date.now() ? 'scheduled' : 'published'
  const next: Announcement = {
    ...announcement,
    status,
    version: announcement.version + 1,
    updatedBy: currentAdminId,
    updatedAt: nowIso(),
  }
  announcementStore = announcementStore.map(item => item.id === id ? next : item)
  persistAnnouncementStores()
  appendAuditLog('announcement_published', next, status === 'scheduled' ? '设置未来发布时间' : '立即发布公告')
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
    version: announcement.version + 1,
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
  const duplicated: Announcement = {
    ...announcement,
    id: `ann-${Date.now()}`,
    slug: createSlug(`${announcement.title} 副本`),
    title: `${announcement.title} 副本`,
    status: 'draft',
    version: 1,
    createdBy: currentAdminId,
    updatedBy: currentAdminId,
    createdAt: nowIso(),
    updatedAt: nowIso(),
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
    channels: Array.from(new Set(['message_center', ...input.channels])),
    ctaLabel: input.ctaLabel?.trim() || undefined,
    ctaUrl: sanitizeAnnouncementUrl(input.ctaUrl),
    expireAt: input.expireAt?.trim() || undefined,
  }
  assertValidAnnouncementFormInput(normalized)
  return normalized
}

function appendAuditLog(action: AnnouncementAuditAction, announcement: Announcement, reason?: string) {
  announcementAuditLogStore = [
    {
      id: `ann-audit-${Date.now()}-${announcementAuditLogStore.length + 1}`,
      action,
      announcementId: announcement.id,
      announcementTitle: announcement.title,
      operatorId: currentAdminId,
      operatorName: currentAdminName,
      reason,
      createdAt: nowIso(),
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
  if (!isAnnouncementUserVisible(announcement)) throw new Error('公告当前不可见。')
  return announcement
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
