import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const home = source('../../pages/HomePage.vue')
const banner = source('../../components/announcements/AnnouncementBanner.vue')
const queries = source('../../queries/useAnnouncementQueries.ts')

describe('首页公告条', () => {
  it('仅在确认登录后读取当前用户的首页公告', () => {
    expect(home).toContain('useMyProfileQuery(import.meta.client)')
    expect(home).toContain('const homeAnnouncementEnabled = computed(() => Boolean(myProfile.value))')
    expect(home).toContain('useActiveHomeAnnouncement(homeAnnouncementEnabled)')
    expect(queries).toContain('useActiveHomeAnnouncement(enabled: Ref<boolean> | boolean = true)')
    expect(queries).toContain('enabled: computed(() => valueOf(enabled))')
  })

  it('不把用户相关公告加入公开首页的服务端缓存预取', () => {
    expect(home).toContain('prefetchQueriesOnServer(homeMarketQuery, productCategoriesQuery)')
    expect(home).not.toContain('prefetchQueriesOnServer(homeMarketQuery, productCategoriesQuery, homeAnnouncementQuery)')
  })

  it('覆盖加载、失败重试、展示和关闭状态', () => {
    expect(home).toContain('homeAnnouncementEnabled && homeAnnouncementPending')
    expect(home).toContain('homeAnnouncementEnabled && homeAnnouncementFailed')
    expect(home).toContain('<AnnouncementBanner')
    expect(home).toContain('v-else-if="homeAnnouncementEnabled && homeAnnouncement"')
    expect(home).toContain(':dismissing="dismissHomeAnnouncementMutation.isPending.value"')
    expect(home).toContain('@dismiss="dismissHomeAnnouncement"')
    expect(home).toContain('@click="refetchHomeAnnouncement()"')
    expect(home).toContain("toast.error(error instanceof Error ? error.message : '关闭首页公告失败。')")
  })

  it('只在允许关闭时显示关闭按钮，并保持唯一详情入口语义有效', () => {
    expect(banner).toContain('const canDismiss = computed(() => props.announcement.isDismissible)')
    expect(banner).toContain('v-if="canDismiss"')
    expect(banner).toContain('aria-label="关闭首页公告"')
    expect(banner).toContain("dismiss: [announcementId: string]")
    expect(banner).toContain('@click="emit(\'dismiss\', announcement.id)"')
    expect(banner.match(/<RouterLink :to="detailTo">/g)).toHaveLength(1)
    expect(banner).toContain('查看详情')
    expect(banner).toContain('<ArrowRight class="h-4 w-4" aria-hidden="true" />')
    expect(banner).not.toContain("from 'lucide-vue-next'")
  })

  it('普通公告保持克制，只有重要公告显示级别强调', () => {
    expect(banner).toContain('v-if="announcement.level === \'important\'"')
    expect(banner).toContain('<Badge v-if="announcement.level === \'important\'">重要</Badge>')
    expect(banner).not.toContain('announcementLevelLabels')
    expect(banner).not.toContain('>普通</Badge>')
  })

  it('使用稳定的响应式信息与操作布局', () => {
    expect(banner).toContain('grid min-h-12 grid-cols-[1.25rem_minmax(0,1fr)]')
    expect(banner).toContain('sm:grid-cols-[1.25rem_minmax(0,1fr)_auto]')
    expect(banner).toContain('flex min-w-0 flex-col gap-1 sm:flex-row')
    expect(banner).toContain('col-start-2 flex shrink-0 items-center')
    expect(home).toContain('class="min-h-12 rounded-lg p-3 [&>div]:mt-2"')
  })
})
