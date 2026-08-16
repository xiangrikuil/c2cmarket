import createDOMPurify, { type DOMPurify as DOMPurifyInstance } from 'dompurify'
import { marked } from 'marked'
import type {
  Announcement,
  AnnouncementAuditAction,
  AnnouncementCategory,
  AnnouncementChannel,
  AnnouncementFormInput,
  AnnouncementLevel,
  AnnouncementDelivery,
  AnnouncementReceipt,
  AnnouncementReceiptMap,
  AnnouncementStatus,
  AnnouncementValidationResult,
} from '@/types/announcement'

export const announcementCategoryLabels: Record<AnnouncementCategory, string> = {
  platform: '平台公告',
  rules: '规则更新',
  maintenance: '系统维护',
  feature: '功能更新',
  risk: '风险提示',
  operation: '运营公告',
}

export const announcementLevelLabels: Record<AnnouncementLevel, string> = {
  normal: '普通',
  important: '重要',
  critical: '紧急',
}

export const announcementStatusLabels: Record<AnnouncementStatus, string> = {
  draft: '草稿',
  scheduled: '待发布',
  published: '发布中',
  offline: '已下线',
  expired: '已结束',
  archived: '已归档',
}

export const announcementChannelLabels: Record<AnnouncementChannel, string> = {
  message_center: '公告中心',
  home_banner: '首页公告条',
  global_bar: '全站通知条',
  modal: '紧急弹窗',
}

export const announcementAuditActionLabels: Record<AnnouncementAuditAction, string> = {
  announcement_created: '创建草稿',
  announcement_updated: '编辑发布中公告',
  announcement_published: '发布公告',
  announcement_offlined: '下线公告',
  announcement_duplicated: '复制公告',
}

const allowedHttpsHosts = ['linux.do', 'www.linux.do', 'openai.com', 'help.openai.com']

export function createDefaultAnnouncementFormInput(): AnnouncementFormInput {
  return {
    title: '',
    summary: '',
    contentMarkdown: '',
    category: 'platform',
    level: 'normal',
    channels: ['message_center'],
    isPinned: false,
    isDismissible: true,
    requiresAck: false,
    audience: { type: 'all' },
    publishAt: new Date().toISOString(),
    expireAt: undefined,
    ctaLabel: undefined,
    ctaUrl: undefined,
  }
}

export function announcementToFormInput(announcement: Announcement): AnnouncementFormInput {
  return {
    title: announcement.title,
    summary: announcement.summary,
    contentMarkdown: announcement.contentMarkdown,
    category: announcement.category,
    level: announcement.level,
    channels: [...announcement.channels],
    isPinned: announcement.isPinned,
    isDismissible: announcement.isDismissible,
    requiresAck: announcement.requiresAck,
    audience: structuredClone(announcement.audience),
    publishAt: announcement.publishAt,
    expireAt: announcement.expireAt,
    ctaLabel: announcement.ctaLabel,
    ctaUrl: announcement.ctaUrl,
  }
}

export function formatAnnouncementDateTime(value?: string) {
  if (!value) return '未设置'
  const timestamp = new Date(value).getTime()
  if (!Number.isFinite(timestamp)) return '时间无效'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(value))
}

export function toDateTimeLocalValue(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ''
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

export function fromDateTimeLocalValue(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return ''
  const date = new Date(trimmed)
  return Number.isFinite(date.getTime()) ? date.toISOString() : trimmed
}

export function isAnnouncementActive(announcement: Announcement, now = new Date()) {
  return getAnnouncementDisplayStatus(announcement, now) === 'published'
}

export function getAnnouncementDisplayStatus(announcement: Announcement, now = new Date()): AnnouncementStatus {
  if (announcement.status === 'draft' || announcement.status === 'offline' || announcement.status === 'archived') {
    return announcement.status
  }

  const publishAt = new Date(announcement.publishAt).getTime()
  const expireAt = announcement.expireAt ? new Date(announcement.expireAt).getTime() : Number.POSITIVE_INFINITY
  const nowTime = now.getTime()

  if (Number.isFinite(expireAt) && nowTime >= expireAt) return 'expired'
  if (!Number.isFinite(publishAt) || publishAt > nowTime) return 'scheduled'
  return 'published'
}

export function isAnnouncementUserVisible(announcement: Announcement, now = new Date()) {
  const status = getAnnouncementDisplayStatus(announcement, now)
  return announcement.channels.includes('message_center') && (status === 'published' || status === 'expired')
}

export function sortAnnouncementsForHome(announcements: Announcement[], receipts: AnnouncementReceiptMap, now = new Date()) {
  return [...announcements].sort((a, b) => {
    const aReceipt = announcementReceipt(a, receipts[a.id])
    const bReceipt = announcementReceipt(b, receipts[b.id])
    const aImportantUnread = a.level === 'important' && isAnnouncementUnread(a, aReceipt)
    const bImportantUnread = b.level === 'important' && isAnnouncementUnread(b, bReceipt)
    if (aImportantUnread !== bImportantUnread) return aImportantUnread ? -1 : 1

    const aPinnedUnread = a.isPinned && isAnnouncementUnread(a, aReceipt)
    const bPinnedUnread = b.isPinned && isAnnouncementUnread(b, bReceipt)
    if (aPinnedUnread !== bPinnedUnread) return aPinnedUnread ? -1 : 1

    if (a.isPinned !== b.isPinned) return a.isPinned ? -1 : 1
    return new Date(b.publishAt).getTime() - new Date(a.publishAt).getTime()
  }).filter(item => isAnnouncementActive(item, now))
}

export function isAnnouncementUnread(announcement: Announcement, receipt?: AnnouncementReceipt) {
  const effectiveReceipt = announcementReceipt(announcement, receipt)
  return !effectiveReceipt || effectiveReceipt.announcementVersion !== announcement.version || !effectiveReceipt.readAt
}

export function isAnnouncementDismissed(announcement: AnnouncementDelivery, receipt?: AnnouncementReceipt) {
  const effectiveReceipt = announcementReceipt(announcement, receipt)
  return Boolean(effectiveReceipt && effectiveReceipt.announcementVersion === announcement.version && effectiveReceipt.dismissedAt)
}

export function isAnnouncementAcknowledged(announcement: AnnouncementDelivery, receipt?: AnnouncementReceipt) {
  const effectiveReceipt = announcementReceipt(announcement, receipt)
  return Boolean(effectiveReceipt && effectiveReceipt.announcementVersion === announcement.version && effectiveReceipt.acknowledgedAt)
}

export function sortAnnouncementsForDelivery<T extends AnnouncementDelivery>(announcements: T[]): T[] {
  const rank: Record<AnnouncementLevel, number> = { normal: 1, important: 2, critical: 3 }
  return [...announcements].sort((a, b) => {
    if (rank[a.level] !== rank[b.level]) return rank[b.level] - rank[a.level]
    const aPinned = 'isPinned' in a && a.isPinned
    const bPinned = 'isPinned' in b && b.isPinned
    if (aPinned !== bPinned) return aPinned ? -1 : 1
    const timeDifference = new Date(b.publishAt).getTime() - new Date(a.publishAt).getTime()
    return timeDifference || a.id.localeCompare(b.id)
  })
}

export function selectGlobalAnnouncementDeliveries<T extends AnnouncementDelivery>(
  announcements: T[],
  receipts: AnnouncementReceiptMap = {},
) {
  const sorted = sortAnnouncementsForDelivery(announcements)
  const critical = sorted
    .filter(item => item.level === 'critical' && item.requiresAck && item.channels.includes('modal'))
    .find(item => !isAnnouncementAcknowledged(item, receipts[item.id]))
  const bar = sorted
    .filter(item => item.channels.includes('global_bar'))
    .filter(item => item.id !== critical?.id)
    .find(item => !isAnnouncementDismissed(item, receipts[item.id]))
  return { critical, bar }
}

function announcementReceipt(announcement: AnnouncementDelivery, fallback?: AnnouncementReceipt) {
  return ('receipt' in announcement ? announcement.receipt : undefined) ?? fallback
}

export function sanitizeAnnouncementUrl(url?: string) {
  const trimmed = url?.trim()
  if (!trimmed) return undefined
  if (/[\u0000-\u001F\u007F\s]/.test(trimmed)) return undefined
  const lower = trimmed.toLowerCase()
  if (lower.startsWith('javascript:') || lower.startsWith('data:') || lower.startsWith('file:')) return undefined

  if (trimmed.startsWith('/') && !trimmed.startsWith('//')) return trimmed

  try {
    const parsed = new URL(trimmed)
    const hostname = parsed.hostname.toLowerCase()
    const currentHostname = typeof window !== 'undefined' ? window.location.hostname.toLowerCase() : ''
    if (parsed.protocol !== 'https:') return undefined
    if (hostname === currentHostname || allowedHttpsHosts.includes(hostname)) return parsed.toString()
    return undefined
  } catch {
    return undefined
  }
}

export function renderAnnouncementMarkdown(markdown: string) {
  const rawHtml = marked.parse(stripInlineHtml(markdown), { async: false, gfm: true, breaks: true }) as string
  const sanitized = getDOMPurify().sanitize(rawHtml, {
    USE_PROFILES: { html: true },
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input', 'style'],
    FORBID_ATTR: ['style', 'onerror', 'onload', 'onclick', 'onmouseover', 'onfocus'],
    ALLOW_UNKNOWN_PROTOCOLS: false,
  })

  return addSafeLinkAttributes(sanitized)
}

export function validateAnnouncementFormInput(input: AnnouncementFormInput): AnnouncementValidationResult {
  const errors: Record<string, string> = {}
  const title = input.title.trim()
  const summary = input.summary.trim()
  const content = input.contentMarkdown.trim()
  const ctaLabel = input.ctaLabel?.trim()
  const ctaUrl = input.ctaUrl?.trim()

  if (title.length < 2 || title.length > 80) errors.title = '标题需为 2 至 80 个字符。'
  if (summary.length < 10 || summary.length > 160) errors.summary = '摘要需为 10 至 160 个字符。'
  if (content.length < 10) errors.contentMarkdown = '正文不少于 10 个字符。'
  if (!input.channels.includes('message_center')) errors.channels = '展示渠道必须包含公告中心。'
  if (input.channels.length === 0) errors.channels = '至少选择公告中心。'
  if (input.level === 'normal' && (input.channels.includes('global_bar') || input.channels.includes('modal') || input.requiresAck)) {
    errors.level = '普通公告不能使用全站通知条、紧急弹窗或确认知悉。'
  }
  if (input.level === 'important' && (input.channels.includes('modal') || input.requiresAck)) {
    errors.channels = '重要公告不能使用紧急弹窗或确认知悉。'
  }
  if (input.level === 'critical' && (!input.channels.includes('global_bar') || !input.channels.includes('modal') || !input.requiresAck || input.isDismissible)) {
    errors.level = '紧急公告必须使用全站通知条和弹窗、要求确认知悉且不可关闭。'
  }
  if (input.audience.type === 'roles' && input.audience.roles.length === 0) errors['audience.roles'] = '至少选择一个用户角色。'
  if (input.audience.type === 'specific_users' && input.audience.userIds.length === 0) errors['audience.userIds'] = '至少选择一个指定用户。'
  if (!input.publishAt || Number.isNaN(new Date(input.publishAt).getTime())) errors.publishAt = '发布时间不能为空。'
  if (input.expireAt && Number.isNaN(new Date(input.expireAt).getTime())) errors.expireAt = '结束时间格式无效。'
  if (input.publishAt && input.expireAt && new Date(input.expireAt).getTime() <= new Date(input.publishAt).getTime()) {
    errors.expireAt = '结束时间必须晚于发布时间。'
  }
  if (ctaUrl && !ctaLabel) errors.ctaLabel = '填写跳转链接后，也需要填写按钮文案。'
  if (ctaLabel && !ctaUrl) errors.ctaUrl = '填写按钮文案后，也需要填写跳转链接。'
  if (ctaUrl && !sanitizeAnnouncementUrl(ctaUrl)) errors.ctaUrl = '跳转地址只允许站内相对路径或白名单 HTTPS 地址。'

  return {
    valid: Object.keys(errors).length === 0,
    errors,
  }
}

export function assertValidAnnouncementFormInput(input: AnnouncementFormInput) {
  const result = validateAnnouncementFormInput(input)
  if (!result.valid) {
    const firstError = Object.values(result.errors)[0] ?? '公告表单校验失败。'
    throw new Error(firstError)
  }
}

function stripInlineHtml(markdown: string) {
  return markdown.replace(/<[^>]+>/g, '')
}

function getDOMPurify(): DOMPurifyInstance {
  if (typeof createDOMPurify.sanitize === 'function') return createDOMPurify
  return createDOMPurify(window)
}

function addSafeLinkAttributes(html: string) {
  if (typeof document === 'undefined') return html

  const template = document.createElement('template')
  template.innerHTML = html
  template.content.querySelectorAll('a[href]').forEach(link => {
    const href = link.getAttribute('href') ?? ''
    const safeHref = sanitizeAnnouncementUrl(href) ?? (href.startsWith('#') ? href : undefined)
    if (!safeHref) {
      link.removeAttribute('href')
      return
    }
    link.setAttribute('href', safeHref)
    if (safeHref.startsWith('https://')) {
      link.setAttribute('target', '_blank')
      link.setAttribute('rel', 'noopener noreferrer')
    }
  })
  return template.innerHTML
}
