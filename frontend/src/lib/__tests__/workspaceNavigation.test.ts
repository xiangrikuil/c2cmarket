import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { routes } from '../../router'

const appShellSource = readFileSync(new URL('../../components/layout/AppShell.vue', import.meta.url), 'utf8')
const routerSource = readFileSync(new URL('../../router.ts', import.meta.url), 'utf8')
const myCenterSource = readFileSync(new URL('../../pages/MyCenterPage.vue', import.meta.url), 'utf8')
const myApiServicesSource = readFileSync(new URL('../../pages/MyApiServicesPage.vue', import.meta.url), 'utf8')
const myApiServiceDetailSource = readFileSync(new URL('../../pages/MyApiServiceDetailPage.vue', import.meta.url), 'utf8')
const apiServiceOwnerHeaderSource = readFileSync(new URL('../../components/api-service-owner/ApiServiceOwnerHeader.vue', import.meta.url), 'utf8')
const publicApiServiceDetailSource = readFileSync(new URL('../../pages/ApiServiceDetailPage.vue', import.meta.url), 'utf8')
const apiSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const apiMarketBackendSource = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')
const myApiOrdersSource = readFileSync(new URL('../../pages/MyApiOrdersPage.vue', import.meta.url), 'utf8')
const merchantApiOrdersSource = readFileSync(new URL('../../pages/MerchantApiOrdersPage.vue', import.meta.url), 'utf8')
const apiOrderDetailSource = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')

describe('个人与经营中心导航', () => {
  it('按个人活动与经营活动提供明确入口', () => {
    expect(appShellSource).toContain("title: '我的交易'")
    expect(appShellSource).toContain("label: '我的上车', to: '/my/rides'")
    expect(appShellSource).toContain("label: 'API 购买订单', to: '/my/api-orders'")
    expect(appShellSource).toContain("label: '收藏', to: '/my/favorites'")
    expect(appShellSource).toContain("label: '消息中心', to: '/my/notifications'")

    expect(appShellSource).toContain("title: '经营中心'")
    expect(appShellSource).toContain("label: '我的车源', to: '/my/carpools'")
    expect(appShellSource).toContain("label: '上车申请', to: '/merchant/carpool-applications'")
    expect(appShellSource).toContain("label: '我的 API 服务', to: '/my/api-services'")
    expect(appShellSource).toContain("label: 'API 销售订单', to: '/merchant/api-orders'")
    expect(appShellSource).toContain("title: '账户'")
    expect(appShellSource).toContain("label: '个人中心', to: '/my'")
    expect(appShellSource).toContain("label: '账户设置', to: '/my/profile'")
    expect(appShellSource).toContain("label: '信誉与权益', to: '/my/reputation'")
    expect(appShellSource).toContain("label: '支持中心', to: '/my/reports'")
    expect(appShellSource).not.toContain("label: '推广权益'")
    expect(appShellSource).not.toContain('promotionRewardConfig')
    expect(appShellSource).not.toContain("{ label: '安全设置', to: '/my/account'")
    expect(appShellSource).not.toContain('/my/demands')
    expect(appShellSource).toContain("label: '进入管理台', to: '/admin'")
    expect(appShellSource).toContain('const groups: NavigationGroup[] = [browseGroup]')
    expect(appShellSource).toContain('if (publishGroup.items.length > 0) groups.push(publishGroup)')
    expect(appShellSource).toContain('if (canViewMerchantWorkspace.value) groups.push(merchantGroup)')
    expect(appShellSource).toContain('if (canViewAdminNav.value) groups.push(adminEntryGroup)')
    expect(appShellSource).toContain("...(canManageApiProbe.value ? [{ key: 'api-probe-connections', label: '探针连接'")
  })

  it('在导航、页面和详情返回入口明确区分 API 买卖角色', () => {
    expect(myApiOrdersSource).toContain('title="API 购买订单"')
    expect(myApiOrdersSource).toContain('查看自己作为买家创建的订单')
    expect(merchantApiOrdersSource).toContain('title="API 销售订单"')
    expect(merchantApiOrdersSource).toContain('管理自己作为商家收到的订单')
    expect(apiOrderDetailSource).toContain("isMerchantView.value ? '返回 API 销售订单' : '返回 API 购买订单'")
    expect(appShellSource).not.toContain("label: '我的 API 订单'")
  })

  it('匿名访问只展示公共导航并保留明确的登录发布入口', () => {
    expect(appShellSource).toContain('const isAuthenticated = computed(() => Boolean(myProfile.value))')
    expect(appShellSource).toContain('const authResolved = computed(() => import.meta.client && !profilePending.value)')
    expect(appShellSource).toContain('const showLoginAction = computed(() => authResolved.value && !isAuthenticated.value)')
    expect(appShellSource).toContain('useNotifications(isAuthenticated)')
    expect(appShellSource).toContain('if (!isAuthenticated.value) return [browseGroup]')
    expect(appShellSource).toContain('const groups: NavigationGroup[] = [browseGroup]')
    expect(appShellSource).not.toContain("label: '平台公告'")
    expect(appShellSource).toContain('v-if="showLoginAction"')
    expect(appShellSource).toContain('v-else-if="isAuthenticated"')
    expect(appShellSource).toContain('登录后发布')
    expect(appShellSource).toContain('const currentLoginTo = computed(() => loginRoute(route.fullPath))')
    expect(appShellSource).toContain('v-if="showLoginAction" class="grid gap-2 border-t border-border p-4"')
    expect(appShellSource).toContain(':to="currentLoginTo" @click="closeMenu"')
    expect(appShellSource).toContain("const anonymousCarpoolPublishTo = loginRoute('/carpools/new')")
    expect(appShellSource).toContain("const anonymousApiPublishTo = loginRoute('/api-market/new')")
    expect(appShellSource).toContain(':to="anonymousCarpoolPublishTo"')
    expect(appShellSource).toContain(':to="anonymousApiPublishTo"')
  })

  it('将账户设置合并为共享页签并保留深链接', () => {
    const workspaceKey = (path: string) => routes.find(route => route.path === path)?.meta?.workspaceNavKey

    expect(workspaceKey('/my')).toBe('personal-center')
    for (const path of ['/my/profile', '/my/contacts', '/my/account', '/my/privacy']) {
      expect(workspaceKey(path)).toBe('account-settings')
    }
    expect(workspaceKey('/my/favorites')).toBeUndefined()
    expect(appShellSource).toContain("workspaceNavKey: 'account-settings' as const")
    expect(appShellSource).not.toContain('accountSettingsPaths')
    expect(myCenterSource).toContain('<AccountSettingsShell')
    expect(myCenterSource).not.toContain('my-center-settings-nav')
  })

  it('将消息、信誉权益和支持详情映射到稳定工作区入口', () => {
    const workspaceKey = (path: string) => routes.find(route => route.path === path)?.meta?.workspaceNavKey

    expect(workspaceKey('/my/notifications')).toBe('message-center')
    for (const path of ['/my/reputation', '/my/promotion-benefits']) {
      expect(workspaceKey(path)).toBe('reputation-rights')
    }
    for (const path of ['/my/reports', '/my/reports/:kind/:id', '/my/feedback', '/my/feedback/:id']) {
      expect(workspaceKey(path)).toBe('support-center')
    }
    expect(appShellSource).not.toContain('to="/my/feedback" class="flex items-center justify-between gap-3"')
  })

  it('在 API 市场深链保持市场入口和二级目录可见', () => {
    expect(appShellSource).toContain("item.to === '/api-market' && matchesRoute(item)")
    expect(appShellSource).not.toContain("item.to === '/api-market' && route.path === '/api-market'")
  })

  it('独立保存资料与隐私，并在草稿变脏时拒绝查询回填覆盖', () => {
    expect(myCenterSource).toContain('const profileSettingsDirty = computed')
    expect(myCenterSource).toContain('const privacySettingsDirty = computed')
    expect(myCenterSource).toContain("if (activeSection.value === 'contacts') return hasContactDraftChanges.value")
    expect(myCenterSource).toContain('useUnsavedChangesGuard(currentSettingsDirty')
    expect(myCenterSource).toContain('privacy: profile.value.privacy')
    expect(myCenterSource).toMatch(/function savePrivacy\(\)[\s\S]*?displayName: profile\.value\.displayName[\s\S]*?privacy: privacyForm/)
    expect(myCenterSource).toContain('if (!profileSettingsDirty.value) syncProfileDraft(currentProfile)')
    expect(myCenterSource).toContain('if (!privacySettingsDirty.value) syncPrivacyDraft(currentProfile)')
    expect(myCenterSource).toMatch(/onSuccess: updatedProfile => \{[\s\S]*?syncProfileDraft\(updatedProfile\)/)
    expect(myCenterSource).toMatch(/function savePrivacy\(\)[\s\S]*?onSuccess: updatedProfile => \{[\s\S]*?syncPrivacyDraft\(updatedProfile\)/)
  })

  it('只允许 linux.do 账号配置备用密码', () => {
    expect(myCenterSource).toContain('const canConfigureBackupPassword = computed(() => Boolean(profile.value?.linuxDoBinding.bound))')
    expect(myCenterSource).toContain('if (!canConfigureBackupPassword.value) {')
    expect(myCenterSource).toContain("accountSetupActiveStep === 'password' && canConfigureBackupPassword")
    expect(myCenterSource).toContain('canConfigureBackupPassword && !profile.passwordConfigured')
    expect(myCenterSource).toContain('不适用（仅 linux.do 账号）')
    expect(myCenterSource).toContain("if (!profile.value || activeSection.value !== 'account' || accountRecoveryComplete.value) return ''")
    expect(myCenterSource).not.toContain('startOAuthLogin(route.fullPath)')
    expect(myCenterSource).not.toContain('改用 linux.do 登录')
  })

  it('不再暴露职责模糊的旧菜单名称', () => {
    for (const label of ['账户与资料', '我的中心', '我的需求', '商户中心', '订单管理', '个人工作台', '商户工作台']) {
      expect(appShellSource).not.toContain(`label: '${label}'`)
    }
  })

  it('为 API 服务提供独立管理页并精简个人概览', () => {
    expect(routerSource).toContain("path: '/my/api-services'")
    expect(routerSource).toContain("path: '/my/api-services/:id'")
    expect(routerSource).toContain("path: '/my/promotion-benefits'")
    expect(routerSource).toContain("import('@/pages/MyApiServicesPage.vue')")
    expect(routerSource).toContain("import('@/pages/MyApiServiceDetailPage.vue')")
    expect(myApiServicesSource).toContain(":title=\"quotaPublishIntent ? '选择 API 服务' : '我的 API 服务'\"")
    expect(myApiServicesSource).toContain('usePagedMyApiServices(salesView, pageRequest)')
    expect(myApiServicesSource).toContain('usePublishApiServiceMutation()')
    expect(myApiServicesSource).toContain('usePauseApiServiceMutation()')
    expect(myApiServicesSource).toContain('useResumeApiServiceMutation()')
    expect(myCenterSource).not.toContain('<SoftTable')
    expect(myCenterSource).not.toContain('hasMerchantObjects')
    expect(myCenterSource).toContain('<PersonalCenterDashboard')
    expect(myCenterSource).toContain('发布 API 服务')
  })

  it('将卖家管理详情与买家公开购买页分离', () => {
    expect(myApiServicesSource).toContain('`/my/api-services/${item.id}#quota-offers`')
    expect(myApiServicesSource).toContain('?preview=owner')
    expect(myApiServiceDetailSource).toContain('useMyApiService(id)')
    expect(myApiServiceDetailSource).toContain('<ApiServiceOwnerHeader')
    expect(apiServiceOwnerHeaderSource).toContain('买家视角预览')
    expect(apiServiceOwnerHeaderSource).toContain('查看 API 销售订单')
    expect(publicApiServiceDetailSource).toContain("useMyApiServices('all', canPublishApiService)")
    expect(publicApiServiceDetailSource).toContain("name: 'my-api-service-detail'")
    expect(publicApiServiceDetailSource).toContain('商户不能为自己的服务创建订单')
    expect(apiSource).toContain('getMyApiServiceById')
    expect(apiMarketBackendSource).toContain('/api/v1/owner/api-services/${encodeURIComponent(id)}')
  })

})
