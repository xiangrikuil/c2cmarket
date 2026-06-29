export type AnnouncementCategory =
  | 'platform'
  | 'rules'
  | 'maintenance'
  | 'feature'
  | 'risk'
  | 'operation'

export type AnnouncementLevel =
  | 'normal'
  | 'important'

export type AnnouncementStatus =
  | 'draft'
  | 'scheduled'
  | 'published'
  | 'offline'
  | 'expired'
  | 'archived'

export type AnnouncementChannel =
  | 'message_center'
  | 'home_banner'

export type AnnouncementAudience = {
  type: 'all'
}

export type Announcement = {
  id: string
  slug: string
  title: string
  summary: string
  contentMarkdown: string
  category: AnnouncementCategory
  level: AnnouncementLevel
  status: AnnouncementStatus
  channels: AnnouncementChannel[]
  audience: AnnouncementAudience
  isPinned: boolean
  isDismissible: boolean
  ctaLabel?: string
  ctaUrl?: string
  publishAt: string
  expireAt?: string
  version: number
  createdBy: string
  updatedBy: string
  createdAt: string
  updatedAt: string
  receipt?: AnnouncementReceipt
}

export type AnnouncementReceipt = {
  announcementId: string
  announcementVersion: number
  firstSeenAt?: string
  readAt?: string
  dismissedAt?: string
}

export type AnnouncementReceiptMap = Record<string, AnnouncementReceipt>

export type AnnouncementFormInput = {
  title: string
  summary: string
  contentMarkdown: string
  category: AnnouncementCategory
  level: AnnouncementLevel
  channels: AnnouncementChannel[]
  isPinned: boolean
  isDismissible: boolean
  ctaLabel?: string
  ctaUrl?: string
  publishAt: string
  expireAt?: string
}

export type AnnouncementAuditAction =
  | 'announcement_created'
  | 'announcement_updated'
  | 'announcement_published'
  | 'announcement_offlined'
  | 'announcement_duplicated'

export type AnnouncementAuditLog = {
  id: string
  action: AnnouncementAuditAction
  announcementId: string
  announcementTitle: string
  operatorId: string
  operatorName: string
  reason?: string
  createdAt: string
}

export type AnnouncementValidationResult = {
  valid: boolean
  errors: Record<string, string>
}
