import { readFileSync } from 'node:fs'
import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  selectGlobalAnnouncementDeliveries,
  sortAnnouncementsForDelivery,
  validateAnnouncementFormInput,
} from '@/lib/announcementUtils'
import {
  readAnnouncementReceipts,
  upsertAnnouncementReceipt,
} from '@/lib/announcementStorage'
import type { Announcement, AnnouncementFormInput, AnnouncementReceiptMap } from '@/types/announcement'

const announcementStorageKey = 'marketplace.announcement.admin-drafts'
const mockAuthStorageKey = 'c2cmarket.mock-auth.v1'

function createStorage(initial: Record<string, string> = {}) {
  const store = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

function announcement(overrides: Partial<Announcement> = {}): Announcement {
  return {
    id: 'announcement-critical-a',
    slug: 'announcement-critical-a',
    title: '紧急服务通知',
    summary: '用于验证全站公告强触达、排序和确认回执语义。',
    contentMarkdown: '## 紧急服务通知\n\n请确认当前服务状态。',
    category: 'risk',
    level: 'critical',
    status: 'published',
    channels: ['message_center', 'global_bar', 'modal'],
    audience: { type: 'all' },
    isPinned: false,
    isDismissible: false,
    requiresAck: true,
    publishAt: '2026-08-16T04:00:00.000Z',
    contentUpdatedAt: '2026-08-16T04:00:00.000Z',
    version: 2,
    createdBy: 'admin-user',
    updatedBy: 'admin-user',
    createdAt: '2026-08-16T03:00:00.000Z',
    updatedAt: '2026-08-16T04:00:00.000Z',
    ...overrides,
  }
}

function criticalForm(overrides: Partial<AnnouncementFormInput> = {}): AnnouncementFormInput {
  const item = announcement()
  return {
    title: item.title,
    summary: item.summary,
    contentMarkdown: item.contentMarkdown,
    category: item.category,
    level: item.level,
    channels: [...item.channels],
    audience: { type: 'all' },
    isPinned: item.isPinned,
    isDismissible: item.isDismissible,
    requiresAck: item.requiresAck,
    publishAt: item.publishAt,
    ...overrides,
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.resetModules()
})

describe('公告强触达契约', () => {
  it('严格执行普通、重要和紧急公告的渠道矩阵', () => {
    expect(validateAnnouncementFormInput(criticalForm())).toEqual({ valid: true, errors: {} })
    expect(validateAnnouncementFormInput(criticalForm({ channels: ['message_center', 'global_bar'] })).errors.level).toContain('紧急公告必须')
    expect(validateAnnouncementFormInput(criticalForm({ isDismissible: true })).errors.level).toContain('不可关闭')
    expect(validateAnnouncementFormInput(criticalForm({ level: 'important', requiresAck: false, channels: ['message_center', 'modal'] })).errors.channels).toContain('重要公告不能')
    expect(validateAnnouncementFormInput(criticalForm({ level: 'normal', requiresAck: false, isDismissible: true, channels: ['message_center', 'global_bar'] })).errors.level).toContain('普通公告不能')
  })

  it('按严重级别、置顶、发布时间和 ID 稳定排序，并串行推进紧急弹窗', () => {
    const first = announcement({ id: 'a', publishAt: '2026-08-16T05:00:00.000Z' })
    const second = announcement({ id: 'b', publishAt: '2026-08-16T04:00:00.000Z' })
    const important = announcement({
      id: 'c',
      level: 'important',
      channels: ['message_center', 'global_bar'],
      requiresAck: false,
      isDismissible: true,
      isPinned: true,
    })

    expect(sortAnnouncementsForDelivery([important, second, first]).map(item => item.id)).toEqual(['a', 'b', 'c'])
    expect(selectGlobalAnnouncementDeliveries([important, second, first]).critical?.id).toBe('a')

    const receipts: AnnouncementReceiptMap = {
      a: { announcementId: 'a', announcementVersion: first.version, acknowledgedAt: '2026-08-16T06:00:00.000Z' },
    }
    expect(selectGlobalAnnouncementDeliveries([important, second, first], receipts).critical?.id).toBe('b')
  })

  it('本地已读不会伪造展示回执，确认知悉按当前版本失效', () => {
    const localStorage = createStorage()
    vi.stubGlobal('window', { localStorage })
    const item = announcement()

    expect(upsertAnnouncementReceipt(item, { readAt: '2026-08-16T05:00:00.000Z' })?.firstSeenAt).toBeUndefined()
    expect(upsertAnnouncementReceipt(item, {
      firstSeenAt: '2026-08-16T05:01:00.000Z',
      acknowledgedAt: '2026-08-16T05:01:00.000Z',
    })?.acknowledgedAt).toBeTruthy()

    const revised = announcement({ version: item.version + 1 })
    expect(upsertAnnouncementReceipt(revised, { readAt: '2026-08-16T05:02:00.000Z' })?.acknowledgedAt).toBeUndefined()
    expect(readAnnouncementReceipts()[item.id]?.announcementVersion).toBe(revised.version)
  })

  it('Mock 用户读取会执行角色受众过滤，公开响应不泄露管理字段', async () => {
    const merchant = announcement({ id: 'merchant', audience: { type: 'roles', roles: ['merchant'] } })
    const admin = announcement({ id: 'admin', audience: { type: 'roles', roles: ['admin'] } })
    const all = announcement({ id: 'all', audience: { type: 'all' } })
    const sessionStorage = createStorage({
      [announcementStorageKey]: JSON.stringify([merchant, admin, all]),
      [mockAuthStorageKey]: JSON.stringify({ persona: 'linuxdo' }),
    })
    vi.stubGlobal('window', { sessionStorage, localStorage: createStorage() })
    const api = await import('@/lib/announcementsApi')

    expect((await api.getAnnouncements()).map(item => item.id).sort()).toEqual(['all', 'merchant'])
    const publicItems = await api.getPublicActiveAnnouncements('modal')
    expect(publicItems.map(item => item.id)).toEqual(['all'])
    expect(publicItems[0]).not.toHaveProperty('createdBy')
    expect(publicItems[0]).not.toHaveProperty('receipt')
    expect(publicItems[0]?.audience).toEqual({ type: 'all' })
    expect(publicItems[0]?.isPinned).toBe(all.isPinned)
  })

  it('根级交付层保留失败重试，匿名详情复用窄公开查询', () => {
    const layer = readFileSync(new URL('../../components/announcements/GlobalAnnouncementLayer.vue', import.meta.url), 'utf8')
    const detail = readFileSync(new URL('../../pages/AnnouncementDetailPage.vue', import.meta.url), 'utf8')
    expect(layer).toContain('seenInFlight')
    expect(layer).toContain('seenMutation.mutateAsync')
    expect(layer).toContain('@retry="retryDelivery"')
    expect(detail).toContain("usePublicActiveAnnouncements('global_bar'")
    expect(detail).toContain("usePublicActiveAnnouncements('modal'")
    expect(detail).toContain("if (!item || !authenticated.value")
  })
})
