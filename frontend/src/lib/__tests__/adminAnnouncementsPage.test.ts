import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const page = readFileSync(new URL('../../pages/AdminAnnouncementsPage.vue', import.meta.url), 'utf8')
const detailPage = readFileSync(new URL('../../pages/AnnouncementDetailPage.vue', import.meta.url), 'utf8')
const statusTabs = readFileSync(new URL('../../components/market/StatusTabs.vue', import.meta.url), 'utf8')

describe('公告管理工作台', () => {
  it('默认聚焦工作中公告并把全部放到最后', () => {
    expect(page).toContain("const activeStatus = ref<StatusFilter>('工作中')")
    expect(page).toContain("['工作中', '草稿', '待发布', '发布中', '历史', '全部']")
    expect(page).toContain("if (activeStatus.value === '工作中') return 'working' as const")
    expect(page).toContain("if (activeStatus.value === '历史') return 'history' as const")
    expect(page).toContain('工作中: statusCounts.value.draft + statusCounts.value.scheduled + statusCounts.value.published')
    expect(page).toContain('历史: statusCounts.value.offline + statusCounts.value.expired')
    expect(page).not.toContain('CompactStats')
  })

  it('筛选计数不改变稳定的业务筛选值', () => {
    expect(statusTabs).toContain('counts?: Record<string, number>')
    expect(statusTabs).toContain('props.counts?.[item] !== undefined')
    expect(statusTabs).toContain("emit('update:modelValue', item)")
    expect(page).toContain(':counts="isLoading || error ? undefined : statusFilterCounts"')
  })

  it('首屏不选中公告并按需打开详情抽屉', () => {
    expect(page).toContain("const previewId = ref('')")
    expect(page).toContain('const previewOpen = ref(false)')
    expect(page).toContain('function openPreview(item: Announcement)')
    expect(page).toContain('<Dialog v-model:open="previewOpen">')
    expect(page).toContain(".filter(log => log.announcementId === previewId.value)")
    expect(page).not.toContain('近期公告审计')
    expect(page).not.toContain('previewAnnouncement.value ?? rows.value[0]')
  })

  it('桌面列表压缩字段和常驻操作', () => {
    expect(page).toContain(":columns=\"['公告', '展示范围', '发布时间', '状态', '操作']\"")
    expect(page).toContain(':aria-label="`${item.title}更多操作`"')
    expect(page).toContain('<DropdownMenuItem @select="editAnnouncement(item)">')
    expect(page).toContain('@select="publishAnnouncement(item)"')
    expect(page).toContain('@select="startOffline(item)"')
    expect(page).toContain('@select="duplicateAnnouncement(item)"')
    expect(page).not.toContain(":columns=\"['公告', '分类 / 级别', '展示位置', '面向用户', '时间', '状态', '操作']\"")
  })

  it('保留搜索、加载、错误恢复和空状态', () => {
    expect(page).toContain('v-model="keyword"')
    expect(page).toContain('q: keyword.value.trim() || undefined')
    expect(page).toContain('useAdminAnnouncementsPage(pageFilters, pageRequest)')
    expect(page).toContain('<SkeletonTable')
    expect(page).toContain('v-else-if="error"')
    expect(page).toContain('@click="refetch()"')
    expect(page).toContain('<EmptyState')
    expect(page).toContain('重置筛选')
  })

  it('下线使用独立对话框并保留原因和二次确认', () => {
    expect(page).toContain('<Dialog v-model:open="offlineOpen">')
    expect(page).toContain('if (!offlineReason.value.trim())')
    expect(page).toContain('if (!offlineConfirmed.value)')
    expect(page).toContain('<Checkbox v-model="offlineConfirmed"')
    expect(page).toContain('原因已写入审计记录')
    expect(page).not.toContain('<Card v-if="offlineTarget"')
  })

  it('用户详情只展示自然的发布和条件更新时间', () => {
    expect(detailPage).toContain('<span>发布于 {{ publishedAt }}</span>')
    expect(detailPage).toContain('<span v-if="wasUpdatedAfterPublish">更新于 {{ contentUpdatedAt }}</span>')
    expect(detailPage).toContain('contentUpdatedAt).getTime() > new Date(announcement.value.publishAt).getTime()')
    expect(detailPage).not.toContain('发布时间：')
    expect(detailPage).not.toContain('更新时间：')
    expect(detailPage).not.toContain('已读状态：已自动记录')
    expect(detailPage).not.toContain('最近更新')
  })
})
