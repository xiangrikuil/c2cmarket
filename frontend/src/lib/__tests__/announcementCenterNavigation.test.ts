import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const notificationsPage = source('../../pages/MyNotificationsPage.vue')
const appShell = source('../../components/layout/AppShell.vue')

describe('平台公告中心导航', () => {
  it('把平台公告作为独立页签且不与系统通知混排', () => {
    expect(notificationsPage).toContain("type NotificationTab = 'todo' | 'transactions' | 'system' | 'announcements'")
    expect(notificationsPage).toContain("if (route.query.tab === 'announcements') return 'announcements'")
    expect(notificationsPage).not.toContain("route.query.tab === 'system' || route.query.tab === 'announcements'")
    expect(notificationsPage).toContain('<TabsTrigger value="announcements">平台公告')
    expect(notificationsPage).toContain('<TabsTrigger value="todo">待办 {{ todoCount }}</TabsTrigger>')
    expect(notificationsPage).toContain('<TabsTrigger value="transactions">交易 {{ transactionCount }}</TabsTrigger>')
    expect(notificationsPage).toContain('<TabsTrigger value="system">系统 {{ systemCount }}</TabsTrigger>')
    expect(notificationsPage).toContain('v-else-if="activeTab === \'system\'"')
    expect(notificationsPage).toContain('<AnnouncementListItem v-for="item in announcementRows"')
    expect(notificationsPage).not.toContain("reviewCount + (announcementUnreadCount ?? 0)")
    expect(notificationsPage).not.toContain("'business'")
  })

  it('在公告页显示公告标题并隐藏通知批量操作', () => {
    expect(notificationsPage).toContain('title="消息中心"')
    expect(notificationsPage).toContain("activeTab === 'todo' || activeTab === 'transactions'")
  })

  it('用一个消息中心入口承载业务通知和重要公告提醒', () => {
    expect(appShell).toContain("key: 'message-center', label: '消息中心', to: '/my/notifications'")
    expect(appShell).toContain('const messageCenterCount = computed(() => unreadBusinessCount.value + importantAnnouncementUnreadCount.value)')
    expect(appShell).not.toContain("label: '平台公告'")
    expect(appShell).not.toContain('announcementCenterTo')
  })
})
