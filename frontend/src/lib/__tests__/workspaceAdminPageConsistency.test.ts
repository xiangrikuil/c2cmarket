import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const myCenter = source('../../pages/MyCenterPage.vue')
const myCarpools = source('../../pages/MyCarpoolsPage.vue')
const myDemands = source('../../pages/MyDemandsPage.vue')
const myApiServices = source('../../pages/MyApiServicesPage.vue')
const myApiServiceDetail = source('../../pages/MyApiServiceDetailPage.vue')
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

describe('个人、经营与管理工作区一致性', () => {
  it('个人中心使用真实查询构建待办而不是入口矩阵或硬编码指标', () => {
    expect(myCenter).toContain('useMyCarpoolApplications')
    expect(myCenter).toContain('useMyApiOrders')
    expect(myCenter).toContain('需要你处理')
    expect(myCenter).toContain('my-center-identity-stats')
    expect(myCenter).toContain('my-center-quick-grid')
    expect(myCenter).not.toContain('value="5" hint="1 个待完成"')
    expect(myCenter).not.toContain('value="2" hint="1 个待处理"')
    expect(myCenter).not.toContain('<StatCard')
  })

  it('联系与收款按真实能力分组，并使用统一表单组件', () => {
    expect(myCenter).toContain('contact-payment-main-grid')
    expect(myCenter).toContain("{ label: '联系与收款', to: '/my/contacts'")
    expect(myCenter).toContain('当前真实支持微信和验证邮箱')
    expect(myCenter).toContain('API 收款方式')
    expect(myCenter).toContain('<Checkbox v-model="option.enabled"')
    expect(myCenter).toContain("wechatBound ? '已填写' : '未填写'")
    expect(myCenter).toContain("emailBound ? '已验证' : '未验证'")
    expect(myCenter).not.toContain('支持撤销')
  })

  it('我的对象列表统一短编号、本地时间和异步空状态', () => {
    for (const page of [myCarpools, myDemands, myApiServices]) {
      expect(page).toContain('<ShortId')
      expect(page).toContain('<LocalTime')
      expect(page).toContain('<EmptyState')
      expect(page).toContain('<SkeletonTable')
    }
    expect(myApiServiceDetail).toContain('已有订单继续使用创建时冻结')
    expect(myApiServiceDetail).toContain('<CompactStats')
  })

  it('经营队列默认突出待处理、临近超时和下一动作', () => {
    expect(merchantCarpools).toContain("const activeStatus = ref('待处理')")
    expect(merchantCarpools).toContain("label: '临近超时'")
    expect(merchantCarpools).toContain('getCarpoolApplicationNextAction')
    expect(merchantOrders).toContain("sort: 'default_merchant'")
    expect(merchantOrders).toContain('待确认收款')
    expect(merchantOrders).toContain('待交付')
    expect(merchantOrders).toContain('订单联系方式')
    expect(merchantOrders).not.toContain('通过联系方式站外沟通')
  })

  it('收藏、评价、通知和反馈表达当前可用性与责任人', () => {
    expect(favorites).toContain("['全部', '拼车', 'API 服务']")
    expect(favorites).not.toContain('官网套餐')
    expect(favorites).toContain('当前不可用')
    expect(reviews).toContain("['待评价', '我发出的', '我收到的', '全部']")
    expect(reviews).toContain('关联交易')
    expect(notifications).toContain("type NotificationTab = 'todo' | 'transactions' | 'system'")
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
    expect(productPlans).toContain('已有交易记录不受影响')
    expect(productPlans).toContain('已有申请继续使用快照')
    expect(apiModels).toContain('已有订单仍使用快照')
  })
})
