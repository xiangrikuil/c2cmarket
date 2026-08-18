import { existsSync, readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const myCenter = source('../../pages/MyCenterPage.vue')
const router = source('../../router.ts')
const myCarpools = source('../../pages/MyCarpoolsPage.vue')
const myApiServices = source('../../pages/MyApiServicesPage.vue')
const myApiServiceDetail = source('../../pages/MyApiServiceDetailPage.vue')
const apiServiceOwnerMetrics = source('../../components/api-service-owner/ApiServiceOwnerMetrics.vue')
const apiServiceOwnerOverview = source('../../components/api-service-owner/ApiServiceOwnerOverview.vue')
const merchantCarpools = source('../../pages/MerchantCarpoolApplicationsPage.vue')
const merchantOrders = source('../../pages/MerchantApiOrdersPage.vue')
const favorites = source('../../pages/MyFavoritesPage.vue')
const reviews = source('../../pages/MyReviewsPage.vue')
const notifications = source('../../pages/MyNotificationsPage.vue')
const feedback = source('../../pages/MyFeedbackPage.vue')
const admin = source('../../pages/AdminPage.vue')
const adminSection = source('../../pages/AdminSectionPage.vue')
const announcementEditor = source('../../components/announcements/AnnouncementEditor.vue')
const productPlans = source('../../pages/AdminProductPlansPage.vue')
const apiModels = source('../../pages/AdminApiModelsPage.vue')
const api = source('../api.ts')
const contactMethodCard = optionalSource('../../components/contact-payment/ContactMethodCard.vue')
const paymentSettingsEditor = optionalSource('../../components/contact-payment/ApiPaymentSettingsEditor.vue')
const paymentMethodCard = optionalSource('../../components/contact-payment/PaymentMethodCard.vue')
const configurationProgressCard = optionalSource('../../components/contact-payment/ConfigurationProgressCard.vue')
const buyerPreviewDrawer = optionalSource('../../components/contact-payment/BuyerPreviewDrawer.vue')
const personalCenterDashboard = source('../../components/personal-center/PersonalCenterDashboard.vue')
const profileOverviewCard = source('../../components/personal-center/ProfileOverviewCard.vue')
const pendingActivityPanel = source('../../components/personal-center/PendingActivityPanel.vue')
const publishedContentSection = source('../../components/personal-center/PublishedContentSection.vue')
const accountCompletenessCard = source('../../components/personal-center/AccountCompletenessCard.vue')
const alertPrimitive = source('../../components/ui/alert/Alert.vue')
const alertPrimitiveVariants = source('../../components/ui/alert/index.ts')

function optionalSource(path: string) {
  const url = new URL(path, import.meta.url)
  return existsSync(url) ? readFileSync(url, 'utf8') : ''
}

describe('个人、经营与管理工作区一致性', () => {
  it('个人中心使用真实查询构建待办而不是入口矩阵或硬编码指标', () => {
    expect(myCenter).toContain('useMyCarpoolApplications')
    expect(myCenter).toContain('useMerchantCarpoolApplications')
    expect(myCenter).toContain('useMyApiOrders')
    expect(myCenter).toContain('useMerchantApiOrders')
    expect(myCenter).toContain('buildPendingTasks')
    expect(myCenter).toContain('<PersonalCenterDashboard')
    expect(myCenter).toContain('Promise.allSettled')
    expect(myCenter).not.toContain('my-center-identity-stats')
    expect(myCenter).not.toContain('my-center-quick-grid')
    expect(myCenter).not.toContain('value="5" hint="1 个待完成"')
    expect(myCenter).not.toContain('value="2" hint="1 个待处理"')
    expect(myCenter).not.toContain('<StatCard')
  })

  it('个人中心模块展示真实内容、局部失败与可执行完整度', () => {
    expect(personalCenterDashboard).toContain('<PendingActivityPanel')
    expect(personalCenterDashboard).toContain('<PublishedContentSection')
    expect(personalCenterDashboard).toContain('<AccountCompletenessCard')
    expect(personalCenterDashboard).not.toContain('求车')
    expect(personalCenterDashboard).toContain('帮助与规则')
    expect(profileOverviewCard).toContain('信任等级暂无数据')
    expect(pendingActivityPanel).toContain('部分待办暂时无法加载')
    expect(pendingActivityPanel).toContain('<ErrorState')
    expect(pendingActivityPanel).toContain('tasks.slice(0, 6)')
    expect(publishedContentSection).toContain('<Tabs')
    expect(publishedContentSection).toContain('<ErrorState')
    expect(publishedContentSection).toContain('filteredItems')
    expect(publishedContentSection).toContain(':aria-label="`管理 ${item.title}`"')
    expect(accountCompletenessCard).toContain('role="progressbar"')
    expect(accountCompletenessCard).toContain(':aria-valuenow="completeness.percentage"')
    expect(accountCompletenessCard).toContain('重新加载')
  })

  it('首单引导只接收完整查询结论并位于个人中心工作区之前', () => {
    expect(myCenter).toContain('shouldShowFirstTransactionGuide')
    expect(myCenter).toContain("useMyApiServices('all', canPublishApiService)")
    expect(myCenter).toContain('isSuccess: !canPublishApiService.value || apiServicesQuery.isSuccess.value')
    expect(myCenter).toContain(':show-first-transaction-guide="showFirstTransactionGuide"')

    const derivationStart = myCenter.indexOf('const showFirstTransactionGuide')
    const derivationEnd = myCenter.indexOf('const hasApiServices', derivationStart)
    const derivation = myCenter.slice(derivationStart, derivationEnd)
    expect(derivationStart).toBeGreaterThan(-1)
    expect(derivationEnd).toBeGreaterThan(derivationStart)
    expect(derivation).not.toContain('?? []')
    for (const queryName of [
      'ownedCarpools',
      'ownedApiServices',
      'buyerCarpoolApplications',
      'ownerCarpoolApplications',
      'buyerApiOrders',
      'merchantApiOrders',
    ]) {
      expect(derivation).toContain(`${queryName}:`)
    }

    const profileOverview = personalCenterDashboard.indexOf('<ProfileOverviewCard')
    const guideAlert = personalCenterDashboard.indexOf('<Alert v-if="showFirstTransactionGuide"')
    const workArea = personalCenterDashboard.indexOf('grid min-w-0 gap-4 min-[1100px]')
    const guideMarkup = personalCenterDashboard.slice(guideAlert, workArea)
    expect(profileOverview).toBeGreaterThan(-1)
    expect(guideAlert).toBeGreaterThan(profileOverview)
    expect(workArea).toBeGreaterThan(guideAlert)
    expect(guideMarkup).toContain('开始第一笔交易')
    expect(guideMarkup).toContain('to="/carpools"')
    expect(guideMarkup).toContain('to="/api-market"')
    expect(guideMarkup).not.toContain('/new')
    expect(guideMarkup).not.toContain('<Card')
    expect(personalCenterDashboard).toContain("from '@/components/ui/alert'")
    expect(alertPrimitive).toContain('role="alert"')
    expect(alertPrimitiveVariants).toContain('relative w-full')
    expect(router).toContain("path: '/carpools'")
    expect(router).toContain("path: '/api-market'")
  })

  it('联系与收款按真实能力分组，并使用统一表单组件', () => {
    expect(myCenter).toContain('contact-payment-main-grid')
    expect(myCenter).toContain(":contact-label=\"canPublishApiService ? '联系与收款' : '联系方式'\"")
    expect(myCenter).toContain('<ContactMethodCard')
    expect(myCenter).toContain('<ApiPaymentSettingsEditor')
    expect(myCenter).toContain('v-if="canPublishApiService"')
    expect(paymentSettingsEditor).toContain('<PaymentMethodCard')
    expect(myCenter).toContain('<ConfigurationProgressCard')
    expect(myCenter).toContain('<BuyerPreviewDrawer')
    expect(myCenter).toContain(':show-payment="canPublishApiService"')
    expect(myCenter).toContain('联系与收款')
    expect(myCenter).toContain('当前真实支持微信和验证邮箱')
    expect(paymentSettingsEditor).toContain('API 收款设置')
    expect(contactMethodCard).toContain('<Card')
    expect(paymentMethodCard).toContain('<RadioGroupItem')
    expect(paymentMethodCard).toContain('选择')
    expect(paymentMethodCard).not.toContain('<Checkbox')
    expect(configurationProgressCard).toContain('配置完成度')
    expect(buyerPreviewDrawer).toContain('不会出现在公开主页')
    expect(myCenter).toContain("wechatBound ? '已配置' : '未配置'")
    expect(myCenter).toContain("emailBound ? '已验证' : '未验证'")
    expect(myCenter).not.toContain('支持撤销')
  })

  it('我的对象列表统一短编号、本地时间和异步空状态', () => {
    for (const page of [myCarpools, myApiServices]) {
      expect(page).toContain('<ShortId')
      expect(page).toContain('<LocalTime')
      expect(page).toContain('<EmptyState')
      expect(page).toContain('<SkeletonTable')
    }
    expect(myApiServiceDetail).toContain('<ApiServiceOwnerMetrics')
    expect(myApiServiceDetail).toContain('<ApiServiceOwnerOverview')
    expect(apiServiceOwnerOverview).toContain('已有订单继续使用创建时冻结')
    expect(apiServiceOwnerMetrics).toContain('核心经营指标')
  })

  it('经营队列默认突出待处理、有效成员和下一动作', () => {
    expect(merchantCarpools).toContain("const activeStatus = ref('待处理')")
    expect(merchantCarpools).toContain("label: '有效成员'")
    expect(merchantCarpools).toContain('getCarpoolApplicationNextAction')
    expect(merchantOrders).toContain("sort: 'default_merchant'")
    expect(merchantOrders).toContain('待确认收款')
    expect(merchantOrders).toContain('待交付')
    expect(merchantOrders).toContain('订单联系方式')
    expect(merchantOrders).toContain('const baseFilteredRows = computed')
    expect(merchantOrders).toContain('const pageFilters = computed')
    expect(merchantOrders).toContain('useMerchantApiOrdersPage(pageFilters, pageRequest)')
    expect(merchantOrders).toContain("label: '已确认收款金额'")
    expect(merchantOrders).toContain("label: '已取消订单'")
    expect(merchantOrders).toContain('isApiOrderReceiptConfirmed(item.status)')
    expect(merchantOrders).toContain('class="[&_table]:min-w-[760px]"')
    expect(merchantOrders).not.toContain("label: '订单金额合计'")
    expect(merchantOrders).not.toContain('rows.value.reduce')
    expect(merchantOrders).not.toContain('通过联系方式站外沟通')
  })

  it('收藏、评价、通知和反馈表达当前可用性与责任人', () => {
    expect(favorites).toContain("['全部', '拼车', 'API 服务']")
    expect(favorites).not.toContain('官网套餐')
    expect(favorites).toContain('当前不可用')
    expect(reviews).toContain("['待评价', '我发出的', '我收到的', '全部']")
    expect(reviews).toContain('关联交易')
    expect(reviews).toContain("if (row.status === 'expired') return '已截止'")
    expect(reviews).toContain("return '已移除'")
    expect(reviews).toContain('评价公开前对对方不可见')
    expect(reviews).not.toContain('对方已提交评价')
    expect(reviews).toContain('[&_table]:min-w-[760px]')
    expect(notifications).toContain("type NotificationTab = 'todo' | 'transactions' | 'system' | 'announcements'")
    expect(notifications).toContain("type === 'API 意向' ? 'API 订单'")
    expect(feedback).toContain('下一责任人：你')
    expect(feedback).toContain('下一责任人：管理员')
  })

  it('管理首页由真实队列组成且不再平铺功能目录或直接执行危险动作', () => {
    expect(admin).toContain("useAdminSectionRows('reports')")
    expect(admin).toContain('管理待办队列')
    expect(admin).toContain('高风险举报/纠纷')
    expect(admin).toContain('最近审计动作')
    expect(admin).not.toContain('强制下线')
    expect(admin).not.toContain("['官网公开价', '/admin/official-prices']")
    expect(admin).not.toContain('useAdminOverview')
  })

  it('危险动作要求原因、二次确认并保留审计上下文', () => {
    expect(adminSection).toContain("if (!reason.value.trim())")
    expect(adminSection).toContain('if (!confirmedRiskAction.value)')
    expect(adminSection).toContain('审计日志会记录该说明')
    expect(adminSection).toContain('<SkeletonTable')
    expect(adminSection).toContain('<EmptyState')
    expect(api).toContain("{ label: '请求追踪', value: `trace-${item.id}` }")
  })

  it('公告和目录变更提供未保存或影响范围确认', () => {
    expect(announcementEditor).toContain('onBeforeRouteLeave')
    expect(announcementEditor).toContain('尚未保存')
    expect(productPlans).toContain('历史正式订单仍按快照继续处理')
    expect(apiModels).toContain('历史正式订单仍按快照继续处理')
  })
})
