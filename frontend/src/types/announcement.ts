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
  | 'critical'

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
  | 'global_bar'
  | 'modal'

export type AnnouncementAudienceRole = 'buyer' | 'merchant' | 'admin'

export type AnnouncementAudience =
  | { type: 'all' }
  | { type: 'roles', roles: AnnouncementAudienceRole[] }
  | { type: 'specific_users', userIds: string[] }

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
  requiresAck: boolean
  ctaLabel?: string
  ctaUrl?: string
  publishAt: string
  expireAt?: string
  contentUpdatedAt: string
  version: number
  createdBy: string
  updatedBy: string
  createdAt: string
  updatedAt: string
  receipt?: AnnouncementReceipt
}

export type PublicAnnouncement = {
  id: string
  slug: string
  title: string
  summary: string
  contentMarkdown: string
  category: AnnouncementCategory
  level: 'important' | 'critical'
  channels: AnnouncementChannel[]
  audience: { type: 'all' }
  isPinned: boolean
  isDismissible: boolean
  requiresAck: boolean
  ctaLabel?: string
  ctaUrl?: string
  publishAt: string
  expireAt?: string
  contentUpdatedAt: string
  version: number
}

export type AnnouncementDelivery = Announcement | PublicAnnouncement

export type AnnouncementReceipt = {
  announcementId: string
  announcementVersion: number
  firstSeenAt?: string
  readAt?: string
  dismissedAt?: string
  acknowledgedAt?: string
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
  requiresAck: boolean
  audience: AnnouncementAudience
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
