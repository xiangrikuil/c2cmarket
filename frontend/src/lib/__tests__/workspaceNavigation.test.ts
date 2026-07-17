import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const appShellSource = readFileSync(new URL('../../components/layout/AppShell.vue', import.meta.url), 'utf8')
const routerSource = readFileSync(new URL('../../router.ts', import.meta.url), 'utf8')
const myCenterSource = readFileSync(new URL('../../pages/MyCenterPage.vue', import.meta.url), 'utf8')
const myApiServicesSource = readFileSync(new URL('../../pages/MyApiServicesPage.vue', import.meta.url), 'utf8')
const myApiServiceDetailSource = readFileSync(new URL('../../pages/MyApiServiceDetailPage.vue', import.meta.url), 'utf8')
const publicApiServiceDetailSource = readFileSync(new URL('../../pages/ApiServiceDetailPage.vue', import.meta.url), 'utf8')
const apiSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const apiMarketBackendSource = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')

describe('个人与经营中心导航', () => {
  it('按个人活动与经营活动提供明确入口', () => {
    expect(appShellSource).toContain("title: '我的交易'")
    expect(appShellSource).toContain("{ label: '我的上车', to: '/my/rides'")
    expect(appShellSource).toContain("{ label: '我的 API 订单', to: '/my/api-orders'")
    expect(appShellSource).toContain("{ label: '收藏', to: '/my/favorites'")
    expect(appShellSource).toContain("{ label: '通知', to: '/my/notifications'")

    expect(appShellSource).toContain("title: '经营中心'")
    expect(appShellSource).toContain("{ label: '我的车源', to: '/my/carpools'")
    expect(appShellSource).toContain("{ label: '上车申请', to: '/merchant/carpool-applications'")
    expect(appShellSource).toContain("{ label: '我的 API 服务', to: '/my/api-services'")
    expect(appShellSource).toContain("{ label: 'API 订单', to: '/merchant/api-orders'")
    expect(appShellSource).toContain("title: '账户'")
    expect(appShellSource).toContain("{ label: '账户设置', to: '/my/profile'")
    expect(appShellSource).not.toContain("{ label: '账户与资料'")
    expect(appShellSource).not.toContain("{ label: '联系与收款'")
    expect(appShellSource).not.toContain("{ label: '安全设置'")
    expect(appShellSource).toContain("{ label: '进入管理台', to: '/admin'")
    expect(appShellSource).toContain('if (hasMerchantWorkspace.value) groups.push(merchantGroup)')
  })

  it('将账户设置合并为共享页签并保留深链接', () => {
    expect(myCenterSource).toContain('const accountSettingLinks = [')
    expect(myCenterSource).toContain("{ label: '个人资料', to: '/my/profile'")
    expect(myCenterSource).toContain("{ label: '联系与收款', to: '/my/contacts'")
    expect(myCenterSource).toContain("{ label: '安全与认证', to: '/my/account'")
    expect(myCenterSource).toContain('class="my-center-settings-tabs"')
    expect(myCenterSource).toContain(':aria-current="isSectionActive(item.to)')
    expect(routerSource).toContain("path: '/my', name: 'my', component: MyCenterPage")
    expect(routerSource).toContain("path: '/my/profile', name: 'my-profile', component: MyCenterPage")
    expect(routerSource).toContain("path: '/my/contacts', name: 'my-contacts', component: MyCenterPage")
    expect(routerSource).toContain("path: '/my/account', name: 'my-account', component: MyCenterPage")
  })

  it('不再暴露职责模糊的旧菜单名称', () => {
    for (const label of ['我的中心', '我的需求', '商户中心', '订单管理', '个人工作台', '商户工作台']) {
      expect(appShellSource).not.toContain(`label: '${label}'`)
    }
  })

  it('为 API 服务提供独立管理页并精简个人概览', () => {
    expect(routerSource).toContain("path: '/my/api-services'")
    expect(routerSource).toContain("path: '/my/api-services/:id'")
    expect(routerSource).toContain("import('@/pages/MyApiServicesPage.vue')")
    expect(routerSource).toContain("import('@/pages/MyApiServiceDetailPage.vue')")
    expect(myApiServicesSource).toContain('title="我的 API 服务"')
    expect(myApiServicesSource).toContain('useMyApiServices()')
    expect(myApiServicesSource).toContain('usePublishApiServiceMutation()')
    expect(myApiServicesSource).toContain('usePauseApiServiceMutation()')
    expect(myApiServicesSource).toContain('useResumeApiServiceMutation()')
    expect(myCenterSource).not.toContain('<SoftTable')
    expect(myCenterSource).toContain('hasMerchantObjects')
    expect(myCenterSource).toContain('发布 API 服务')
  })

  it('将卖家管理详情与买家公开购买页分离', () => {
    expect(myApiServicesSource).toContain('`/my/api-services/${item.id}`')
    expect(myApiServicesSource).toContain('?preview=owner')
    expect(myApiServiceDetailSource).toContain('useMyApiService(id)')
    expect(myApiServiceDetailSource).toContain('以买家视角预览')
    expect(myApiServiceDetailSource).toContain('查看 API 订单')
    expect(publicApiServiceDetailSource).toContain('useMyApiServices()')
    expect(publicApiServiceDetailSource).toContain("name: 'my-api-service-detail'")
    expect(publicApiServiceDetailSource).toContain('商户不能为自己的服务创建订单')
    expect(apiSource).toContain('getMyApiServiceById')
    expect(apiMarketBackendSource).toContain('/api/v1/owner/api-services/${encodeURIComponent(id)}')
  })

})
