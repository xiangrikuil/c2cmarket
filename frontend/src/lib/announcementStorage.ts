import type { Announcement, AnnouncementReceipt, AnnouncementReceiptMap } from '@/types/announcement'

export const announcementReceiptStorageKey = 'marketplace.announcement.receipts'

function nowIso() {
  return new Date().toISOString()
}

function canUseLocalStorage() {
  return typeof window !== 'undefined' && Boolean(window.localStorage)
}

export function readAnnouncementReceipts(): AnnouncementReceiptMap {
  if (!canUseLocalStorage()) return {}

  try {
    const stored = window.localStorage.getItem(announcementReceiptStorageKey)
    if (!stored) return {}
    const parsed = JSON.parse(stored) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    return Object.fromEntries(Object.entries(parsed).filter(([, value]) => isAnnouncementReceipt(value)))
  } catch {
    return {}
  }
}

export function writeAnnouncementReceipts(receipts: AnnouncementReceiptMap) {
  if (!canUseLocalStorage()) return

  try {
    window.localStorage.setItem(announcementReceiptStorageKey, JSON.stringify(receipts))
  } catch {
    return
  }
}

export function getAnnouncementReceipt(announcementId: string) {
  return readAnnouncementReceipts()[announcementId]
}

export function upsertAnnouncementReceipt(
  announcement: Announcement,
  patch: Partial<Pick<AnnouncementReceipt, 'firstSeenAt' | 'readAt' | 'dismissedAt'>>,
) {
  const receipts = readAnnouncementReceipts()
  const existing = receipts[announcement.id]
  const base: AnnouncementReceipt = existing?.announcementVersion === announcement.version
    ? existing
    : {
        announcementId: announcement.id,
        announcementVersion: announcement.version,
      }
  const next: AnnouncementReceipt = {
    ...base,
    firstSeenAt: patch.firstSeenAt ?? base.firstSeenAt ?? nowIso(),
    readAt: patch.readAt ?? base.readAt,
    dismissedAt: patch.dismissedAt ?? base.dismissedAt,
  }
  const nextReceipts = { ...receipts, [announcement.id]: next }
  writeAnnouncementReceipts(nextReceipts)
  return next
}

function isAnnouncementReceipt(value: unknown): value is AnnouncementReceipt {
  if (!value || typeof value !== 'object') return false
  const record = value as Record<string, unknown>
  return typeof record.announcementId === 'string'
    && typeof record.announcementVersion === 'number'
    && (record.firstSeenAt === undefined || typeof record.firstSeenAt === 'string')
    && (record.readAt === undefined || typeof record.readAt === 'string')
    && (record.dismissedAt === undefined || typeof record.dismissedAt === 'string')
}
