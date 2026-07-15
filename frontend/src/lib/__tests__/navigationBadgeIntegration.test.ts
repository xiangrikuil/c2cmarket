import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const appShellSource = readFileSync(new URL('../../components/layout/AppShell.vue', import.meta.url), 'utf8')
const adminShellSource = readFileSync(new URL('../../components/layout/AdminShell.vue', import.meta.url), 'utf8')
const marketQueriesSource = readFileSync(new URL('../../queries/useMarketQueries.ts', import.meta.url), 'utf8')
const realtimeSyncSource = readFileSync(new URL('../../composables/useRealtimeSync.ts', import.meta.url), 'utf8')
const apiServiceDetailSource = readFileSync(new URL('../../pages/ApiServiceDetailPage.vue', import.meta.url), 'utf8')
const merchantCarpoolSource = readFileSync(new URL('../../pages/MerchantCarpoolApplicationsPage.vue', import.meta.url), 'utf8')
const carpoolDetailSource = readFileSync(new URL('../../pages/CarpoolApplicationDetailPage.vue', import.meta.url), 'utf8')

describe('实时通知与导航徽标接入', () => {
  it('由统一摘要驱动导航且不再保留演示管理数字', () => {
    expect(appShellSource).toContain('useNavigationBadges')
    expect(appShellSource).toContain('useRealtimeSync')
    expect(appShellSource).toContain('navigationBadges.value?.admin?.total')
    expect(adminShellSource).toContain('badges.value?.admin?.officialPrices')
    expect(adminShellSource).toContain('badges.value?.admin?.apiServices')
    expect(adminShellSource).toContain("{ label: '用户目录', to: '/admin/users'")
    expect(appShellSource).not.toContain("count: 12")
    expect(appShellSource).not.toContain("count: 6")
  })

  it('铃铛只打开下拉并提供独立通知中心入口', () => {
    expect(appShellSource).not.toContain('@click="openNotifications"')
    expect(appShellSource).toContain('查看全部通知')
    expect(appShellSource).toContain('unreadBusinessCount')
    expect(appShellSource).toContain('importantAnnouncementUnreadCount')
  })

  it('致命断开后持续重连且畸形事件不会中断同步', () => {
    expect(realtimeSyncSource).toContain('autoReconnect: { retries: -1, delay: 3_000 }')
    expect(realtimeSyncSource).toContain('tryDecodeRealtimeEventEnvelope')
    expect(realtimeSyncSource).toContain("stream.status.value === 'CLOSED'")
  })

  it('本地订单与通知动作立即失效权威摘要', () => {
    expect(marketQueriesSource).toContain("queryKey: ['navigation-badges']")
    expect(marketQueriesSource).toContain("queryKey: ['notifications']")
    expect(apiServiceDetailSource).toContain("queryKey: ['navigation-badges']")
    expect(merchantCarpoolSource).toContain("queryKey: ['navigation-badges']")
    expect(carpoolDetailSource).toContain("queryKey: ['navigation-badges']")
  })
})
