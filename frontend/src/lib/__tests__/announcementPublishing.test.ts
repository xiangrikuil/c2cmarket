import { afterEach, describe, expect, it, vi } from 'vitest'
import { getAnnouncementDisplayStatus } from '@/lib/announcementUtils'
import type { Announcement, AnnouncementFormInput } from '@/types/announcement'

const announcementStorageKey = 'marketplace.announcement.admin-drafts'

function createStorage(initial: Record<string, string> = {}) {
  const store = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

function draft(overrides: Partial<Announcement> = {}): Announcement {
  return {
    id: 'announcement-publish-test',
    slug: 'announcement-publish-test',
    title: '公告发布时间测试',
    summary: '用于验证公告立即发布、定时发布与过期结束时间的处理规则。',
    contentMarkdown: '## 公告发布时间测试\n\n这是一条用于自动化验证的公告正文。',
    category: 'platform',
    level: 'normal',
    status: 'draft',
    channels: ['message_center'],
    audience: { type: 'all' },
    isPinned: false,
    isDismissible: true,
    requiresAck: false,
    publishAt: '2026-07-30T04:00:00.000Z',
    contentUpdatedAt: '2026-07-30T04:00:00.000Z',
    version: 1,
    createdBy: 'admin-demo',
    updatedBy: 'admin-demo',
    createdAt: '2026-07-30T04:00:00.000Z',
    updatedAt: '2026-07-30T04:00:00.000Z',
    ...overrides,
  }
}

function formFromAnnouncement(item: Announcement, overrides: Partial<AnnouncementFormInput> = {}): AnnouncementFormInput {
  return {
    title: item.title,
    summary: item.summary,
    contentMarkdown: item.contentMarkdown,
    category: item.category,
    level: item.level,
    channels: [...item.channels],
    audience: structuredClone(item.audience),
    isPinned: item.isPinned,
    isDismissible: item.isDismissible,
    requiresAck: item.requiresAck,
    ctaLabel: item.ctaLabel,
    ctaUrl: item.ctaUrl,
    publishAt: item.publishAt,
    expireAt: item.expireAt,
    ...overrides,
  }
}

async function loadAnnouncementsApi(item: Announcement) {
  vi.resetModules()
  vi.stubGlobal('window', {
    sessionStorage: createStorage({
      [announcementStorageKey]: JSON.stringify([item]),
    }),
    localStorage: createStorage(),
  })
  const api = await import('@/lib/announcementsApi')
  await vi.dynamicImportSettled()
  return api
}

async function advanceMockRequest<T>(request: Promise<T>) {
  await vi.advanceTimersByTimeAsync(100)
  return request
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('公告发布语义', () => {
  it('立即发布旧草稿时使用动作时间并进入发布中', async () => {
    const now = new Date('2026-08-09T04:00:00.000Z')
    vi.useFakeTimers()
    vi.setSystemTime(now)
    const api = await loadAnnouncementsApi(draft())

    const published = await advanceMockRequest(api.publishAnnouncement('announcement-publish-test'))
    expect(published.status).toBe('published')
    expect(published.publishAt).toBe(published.updatedAt)
    expect(new Date(published.publishAt).getTime()).toBeGreaterThanOrEqual(now.getTime())
    expect(new Date(published.publishAt).getTime()).toBeLessThanOrEqual(now.getTime() + 100)

    const rows = await advanceMockRequest(api.getAdminAnnouncements())
    expect(rows[0]?.id).toBe(published.id)
    expect(rows[0]?.status).toBe('published')
  })

  it('保留未来定时发布时间', async () => {
    const now = new Date('2026-08-09T04:00:00.000Z')
    const plannedAt = '2026-08-10T04:00:00.000Z'
    vi.useFakeTimers()
    vi.setSystemTime(now)
    const api = await loadAnnouncementsApi(draft({ publishAt: plannedAt }))

    const scheduled = await advanceMockRequest(api.publishAnnouncement('announcement-publish-test'))
    expect(scheduled.status).toBe('scheduled')
    expect(scheduled.publishAt).toBe(plannedAt)
    expect(new Date(scheduled.updatedAt).getTime()).toBeGreaterThanOrEqual(now.getTime())
    expect(new Date(scheduled.updatedAt).getTime()).toBeLessThanOrEqual(now.getTime() + 100)
  })

  it('结束时间已过时拒绝立即发布且不修改草稿', async () => {
    const now = new Date('2026-08-09T04:00:00.000Z')
    vi.useFakeTimers()
    vi.setSystemTime(now)
    const api = await loadAnnouncementsApi(draft({ expireAt: '2026-08-08T04:00:00.000Z' }))

    const request = api.publishAnnouncement('announcement-publish-test')
    const rejection = expect(request).rejects.toThrow('结束时间已过，请调整结束时间后再发布。')
    await vi.advanceTimersByTimeAsync(100)
    await rejection

    const rows = await advanceMockRequest(api.getAdminAnnouncements())
    expect(rows[0]?.status).toBe('draft')
    expect(rows[0]?.publishAt).toBe('2026-07-30T04:00:00.000Z')
  })

  it('结束时间不晚于未来计划发布时间时拒绝发布且不修改草稿', async () => {
    const now = new Date('2026-08-09T04:00:00.000Z')
    const publishAt = '2026-08-11T04:00:00.000Z'
    vi.useFakeTimers()
    vi.setSystemTime(now)
    const api = await loadAnnouncementsApi(draft({
      publishAt,
      expireAt: '2026-08-10T04:00:00.000Z',
    }))

    const request = api.publishAnnouncement('announcement-publish-test')
    const rejection = expect(request).rejects.toThrow('结束时间必须晚于计划发布时间，请调整后再发布。')
    await vi.advanceTimersByTimeAsync(100)
    await rejection

    const rows = await advanceMockRequest(api.getAdminAnnouncements())
    expect(rows[0]?.status).toBe('draft')
    expect(rows[0]?.publishAt).toBe(publishAt)
    expect(rows[0]?.version).toBe(1)
  })

  it('无需写操作即可按时间推导待发布、发布中和已结束', () => {
    const publishAt = new Date('2026-08-10T04:00:00.000Z')
    const expireAt = new Date('2026-08-10T06:00:00.000Z')
    const announcement = draft({
      status: 'scheduled',
      publishAt: publishAt.toISOString(),
      expireAt: expireAt.toISOString(),
    })

    expect(getAnnouncementDisplayStatus(announcement, new Date(publishAt.getTime() - 1))).toBe('scheduled')
    expect(getAnnouncementDisplayStatus(announcement, publishAt)).toBe('published')
    expect(getAnnouncementDisplayStatus(announcement, new Date(expireAt.getTime() - 1))).toBe('published')
    expect(getAnnouncementDisplayStatus(announcement, expireAt)).toBe('expired')
  })

  it('只有规范化后的用户可见内容变化才更新 contentUpdatedAt', async () => {
    const now = new Date('2026-08-09T04:00:00.000Z')
    vi.useFakeTimers()
    vi.setSystemTime(now)
    const api = await loadAnnouncementsApi(draft())
    const original = draft()

    const managementOnly = await advanceMockRequest(api.updateAnnouncement(original.id, formFromAnnouncement(original, {
      title: `  ${original.title}  `,
      summary: `  ${original.summary}  `,
      contentMarkdown: `\n${original.contentMarkdown}\n`,
      channels: ['message_center', 'home_banner'],
      isPinned: true,
      isDismissible: false,
    })))
    expect(managementOnly.contentUpdatedAt).toBe(original.contentUpdatedAt)
    expect(managementOnly.updatedAt).not.toBe(original.updatedAt)

    const contentEdited = await advanceMockRequest(api.updateAnnouncement(original.id, formFromAnnouncement(managementOnly, {
      title: `${managementOnly.title}（更新）`,
    })))
    expect(new Date(contentEdited.contentUpdatedAt).getTime()).toBeGreaterThan(new Date(original.contentUpdatedAt).getTime())

    const published = await advanceMockRequest(api.publishAnnouncement(original.id))
    expect(published.contentUpdatedAt).toBe(contentEdited.contentUpdatedAt)

    const offline = await advanceMockRequest(api.offlineAnnouncement(original.id, '公告生命周期测试完成'))
    expect(offline.contentUpdatedAt).toBe(contentEdited.contentUpdatedAt)

    const duplicated = await advanceMockRequest(api.duplicateAnnouncement(original.id))
    expect(new Date(duplicated.contentUpdatedAt).getTime()).toBeGreaterThan(new Date(contentEdited.contentUpdatedAt).getTime())
    expect(duplicated.contentUpdatedAt).toBe(duplicated.createdAt)
  })

  it('历史 Mock 数据缺少 contentUpdatedAt 时从 publishAt 保守补齐', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-09T04:00:00.000Z'))
    const legacy = { ...draft() } as Partial<Announcement>
    delete legacy.contentUpdatedAt
    const api = await loadAnnouncementsApi(legacy as Announcement)

    const rows = await advanceMockRequest(api.getAdminAnnouncements())
    expect(rows[0]?.contentUpdatedAt).toBe(rows[0]?.publishAt)
  })
})
