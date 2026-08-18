import type { RouteRecordRaw } from 'vue-router'
import { CAPABILITY, type Capability } from './lib/capabilities'
import { LIMITED_API_QUOTA_OFFERS_ENABLED } from './lib/featureFlags'

const HomePage = () => import('@/pages/HomePage.vue')
const OfficialPricesPage = () => import('@/pages/OfficialPricesPage.vue')
const OfficialPriceDetailPage = () => import('@/pages/OfficialPriceDetailPage.vue')
const OfficialPriceManagePage = () => import('@/pages/OfficialPriceManagePage.vue')
const CarpoolsPage = () => import('@/pages/CarpoolsPage.vue')
const CarpoolDetailPage = () => import('@/pages/CarpoolDetailPage.vue')
const CarpoolPublishPage = () => import('@/pages/CarpoolPublishPage.vue')
const ApiMarketPage = () => import('@/pages/ApiMarketPage.vue')
const ApiQuotaRushPublishPage = () => import('@/pages/ApiQuotaRushPublishPage.vue')
const ApiServiceDetailPage = () => import('@/pages/ApiServiceDetailPage.vue')
const ApiServicePublishPage = () => import('@/pages/ApiServicePublishPage.vue')
const SearchPage = () => import('@/pages/SearchPage.vue')
const MyCenterPage = () => import('@/pages/MyCenterPage.vue')
const MyCarpoolsPage = () => import('@/pages/MyCarpoolsPage.vue')
const MyRidesPage = () => import('@/pages/MyRidesPage.vue')
const CarpoolApplicationDetailPage = () => import('@/pages/CarpoolApplicationDetailPage.vue')
const MyApiOrdersPage = () => import('@/pages/MyApiOrdersPage.vue')
const MyApiServicesPage = () => import('@/pages/MyApiServicesPage.vue')
const MyApiServiceDetailPage = () => import('@/pages/MyApiServiceDetailPage.vue')
const MyApiProbeConnectionsPage = () => import('@/pages/MyApiProbeConnectionsPage.vue')
const ApiModelTesterPage = () => import('@/pages/ApiModelTesterPage.vue')
const ApiPurchaseOrderDetailPage = () => import('@/pages/ApiPurchaseOrderDetailPage.vue')
const ApiOrderDeliveryContentPage = () => import('@/pages/ApiOrderDeliveryContentPage.vue')
const MyApiOrderDisputePage = () => import('@/pages/MyApiOrderDisputePage.vue')
const LegacyApiIntentRedirectPage = () => import('@/pages/LegacyApiIntentRedirectPage.vue')
const MerchantApiOrdersPage = () => import('@/pages/MerchantApiOrdersPage.vue')
const MerchantCarpoolApplicationsPage = () => import('@/pages/MerchantCarpoolApplicationsPage.vue')
const MyFavoritesPage = () => import('@/pages/MyFavoritesPage.vue')
const MyReviewsPage = () => import('@/pages/MyReviewsPage.vue')
const MyReputationPage = () => import('@/pages/MyReputationPage.vue')
const MyPromotionBenefitsPage = () => import('@/pages/MyPromotionBenefitsPage.vue')
const MyNotificationsPage = () => import('@/pages/MyNotificationsPage.vue')
const MyFeedbackPage = () => import('@/pages/MyFeedbackPage.vue')
const MyReportsAppealsPage = () => import('@/pages/MyReportsAppealsPage.vue')
const AnnouncementDetailPage = () => import('@/pages/AnnouncementDetailPage.vue')
const LoginPage = () => import('@/pages/LoginPage.vue')
const PasswordResetPage = () => import('@/pages/PasswordResetPage.vue')
const AccountAppealPage = () => import('@/pages/AccountAppealPage.vue')
const RestrictedBusinessPage = () => import('@/pages/RestrictedBusinessPage.vue')
const PublicUserPage = () => import('@/pages/PublicUserPage.vue')
const AdminPage = () => import('@/pages/AdminPage.vue')
const AdminGrowthPage = () => import('@/pages/AdminGrowthPage.vue')
const AdminGrowthPromotionsPage = () => import('@/pages/AdminGrowthPromotionsPage.vue')
const AdminFeedbackPage = () => import('@/pages/AdminFeedbackPage.vue')
const AdminAnnouncementsPage = () => import('@/pages/AdminAnnouncementsPage.vue')
const AdminAnnouncementEditorPage = () => import('@/pages/AdminAnnouncementEditorPage.vue')
const AdminProductPlansPage = () => import('@/pages/AdminProductPlansPage.vue')
const AdminApiModelsPage = () => import('@/pages/AdminApiModelsPage.vue')
const AdminApiHealthCalibrationPage = () => import('@/pages/AdminApiHealthCalibrationPage.vue')
const AdminModelAuditPage = () => import('@/pages/AdminModelAuditPage.vue')
const AdminUsersPage = () => import('@/pages/AdminUsersPage.vue')
const AdminApiOrderDetailPage = () => import('@/pages/AdminApiOrderDetailPage.vue')
const AdminApiPromotionsPage = () => import('@/pages/AdminApiPromotionsPage.vue')
const AdminStudentRegistrationPage = () => import('@/pages/AdminStudentRegistrationPage.vue')
const AdminAuditLogsPage = () => import('@/pages/AdminAuditLogsPage.vue')
const AdminSectionPage = () => import('@/pages/AdminSectionPage.vue')
const NotFoundPage = () => import('@/pages/NotFoundPage.vue')
const ForbiddenPage = () => import('@/pages/ForbiddenPage.vue')

const userAuthMeta = { auth: 'user' } as const
const capabilityAuthMeta = (capability: Capability) => ({ auth: 'user', capability } as const)
const adminAuthMeta = { auth: 'admin', capability: CAPABILITY.adminAccess } as const
const apiQuotaRushPublishRoute: RouteRecordRaw = LIMITED_API_QUOTA_OFFERS_ENABLED
  ? {
      path: '/api-market/quota/new',
      name: 'api-quota-rush-new',
      component: ApiQuotaRushPublishPage,
      meta: capabilityAuthMeta(CAPABILITY.apiQuotaPublish),
    }
  : {
      path: '/api-market/quota/new',
      name: 'api-quota-rush-new',
      redirect: { path: '/api-market', query: { view: 'free' } },
      meta: capabilityAuthMeta(CAPABILITY.apiQuotaPublish),
    }

export type WorkspaceNavKey =
  | 'personal-center'
  | 'account-settings'
  | 'reputation-rights'
  | 'support-center'
  | 'message-center'

const workspaceUserAuthMeta = (workspaceNavKey: WorkspaceNavKey) => ({ ...userAuthMeta, workspaceNavKey } as const)

const adminChildren = [
  ['carpools', '车源管理', '集中巡查公开车源，并处理暂停、待复核和遗留审核记录。'],
  ['api-services', 'API 服务管理', '巡查公开服务，并处理遗留待审、下架和其他异常记录。'],
  ['trade-intents', 'API 订单监管', '查看 API 订单、参与方、金额快照、完成和取消状态。'],
  ['reports', '举报纠纷', '处理举报、纠纷和未解决记录。'],
  ['appeals', '申诉处理', '处理用户对限制、下架和封禁的申诉。'],
] as const

export const routes: RouteRecordRaw[] = [
    { path: '/', name: 'home', component: HomePage },
    { path: '/search', name: 'search', component: SearchPage },
    { path: '/login', name: 'login', component: LoginPage, meta: { standalone: true } },
    { path: '/password-reset', name: 'password-reset', component: PasswordResetPage, meta: { standalone: true } },
    { path: '/account-appeal', name: 'account-appeal', component: AccountAppealPage, meta: { standalone: true } },
    { path: '/restricted-business', name: 'restricted-business', component: RestrictedBusinessPage, meta: { standalone: true } },
    { path: '/forbidden', name: 'forbidden', component: ForbiddenPage },
    { path: '/auth/mock', redirect: '/login' },
    { path: '/official-prices', name: 'official-prices', component: OfficialPricesPage },
    { path: '/official-prices/detail', redirect: '/official-prices/p1' },
    { path: '/official-prices/submit', redirect: '/official-prices' },
    { path: '/official-prices/manage', redirect: '/admin/official-prices' },
    { path: '/official-prices/:id', name: 'official-prices-detail', component: OfficialPriceDetailPage },
    { path: '/carpools', name: 'carpools', component: CarpoolsPage },
    { path: '/carpools/detail', redirect: '/carpools/c1' },
    { path: '/carpools/new', name: 'carpool-new', component: CarpoolPublishPage, meta: capabilityAuthMeta(CAPABILITY.carpoolPublish) },
    { path: '/carpools/:id', name: 'carpool-detail', component: CarpoolDetailPage },
    { path: '/api-market', name: 'api-market', component: ApiMarketPage },
    apiQuotaRushPublishRoute,
    { path: '/api-market/detail', redirect: '/api-market/a1' },
    { path: '/api-market/new', name: 'api-new', component: ApiServicePublishPage, meta: capabilityAuthMeta(CAPABILITY.apiServicePublish) },
    { path: '/api-market/:id', name: 'api-detail', component: ApiServiceDetailPage },
    { path: '/my', name: 'my', component: MyCenterPage, meta: workspaceUserAuthMeta('personal-center') },
    { path: '/my/profile', name: 'my-profile', component: MyCenterPage, meta: workspaceUserAuthMeta('account-settings') },
    { path: '/my/contacts', name: 'my-contacts', component: MyCenterPage, meta: workspaceUserAuthMeta('account-settings') },
    { path: '/my/account', name: 'my-account', component: MyCenterPage, meta: workspaceUserAuthMeta('account-settings') },
    { path: '/my/privacy', name: 'my-privacy', component: MyCenterPage, meta: workspaceUserAuthMeta('account-settings') },
    { path: '/my/carpools', name: 'my-carpools', component: MyCarpoolsPage, meta: capabilityAuthMeta(CAPABILITY.carpoolPublish) },
    { path: '/my/carpools/:id/edit', name: 'my-carpool-edit', component: CarpoolPublishPage, meta: capabilityAuthMeta(CAPABILITY.carpoolPublish) },
    { path: '/my/rides', name: 'my-rides', component: MyRidesPage, meta: userAuthMeta },
    { path: '/my/rides/:id', name: 'my-ride-detail', component: CarpoolApplicationDetailPage, meta: userAuthMeta },
    { path: '/my/api-orders', name: 'my-api-orders', component: MyApiOrdersPage, meta: userAuthMeta },
    { path: '/my/api-orders/:id', name: 'my-api-order-detail', component: ApiPurchaseOrderDetailPage, meta: userAuthMeta },
    { path: '/my/api-orders/:id/delivery', name: 'my-api-order-delivery', component: ApiOrderDeliveryContentPage, meta: userAuthMeta },
    { path: '/my/disputes/:id', name: 'my-api-order-dispute', component: MyApiOrderDisputePage, meta: userAuthMeta },
    { path: '/my/api-services', name: 'my-api-services', component: MyApiServicesPage, meta: capabilityAuthMeta(CAPABILITY.apiServicePublish) },
    { path: '/my/api-services/:id', name: 'my-api-service-detail', component: MyApiServiceDetailPage, meta: capabilityAuthMeta(CAPABILITY.apiServicePublish) },
    { path: '/my/api-probe-connections', name: 'my-api-probe-connections', component: MyApiProbeConnectionsPage, meta: capabilityAuthMeta(CAPABILITY.apiProbeManage) },
    { path: '/tools/api-model-tester', name: 'api-model-tester', component: ApiModelTesterPage, meta: userAuthMeta },
    { path: '/api-intents/:id', name: 'legacy-api-intent-detail', component: LegacyApiIntentRedirectPage, meta: userAuthMeta },
    { path: '/merchant/carpool-applications', name: 'merchant-carpool-applications', component: MerchantCarpoolApplicationsPage, meta: capabilityAuthMeta(CAPABILITY.carpoolPublish) },
    { path: '/merchant/carpool-applications/:id', name: 'merchant-carpool-application-detail', component: CarpoolApplicationDetailPage, meta: capabilityAuthMeta(CAPABILITY.carpoolPublish) },
    { path: '/merchant/api-orders', name: 'merchant-api-orders', component: MerchantApiOrdersPage, meta: capabilityAuthMeta(CAPABILITY.apiServicePublish) },
    { path: '/merchant/api-orders/:id', name: 'merchant-api-order-detail', component: ApiPurchaseOrderDetailPage, meta: capabilityAuthMeta(CAPABILITY.apiServicePublish) },
    { path: '/my/favorites', name: 'my-favorites', component: MyFavoritesPage, meta: userAuthMeta },
    { path: '/my/reviews', name: 'my-reviews', component: MyReviewsPage, meta: userAuthMeta },
    { path: '/my/reputation', alias: '/me/reputation', name: 'my-reputation', component: MyReputationPage, meta: workspaceUserAuthMeta('reputation-rights') },
    { path: '/my/promotion-benefits', name: 'my-promotion-benefits', component: MyPromotionBenefitsPage, meta: workspaceUserAuthMeta('reputation-rights') },
    { path: '/my/notifications', name: 'my-notifications', component: MyNotificationsPage, meta: workspaceUserAuthMeta('message-center') },
    { path: '/my/reports', name: 'my-reports', component: MyReportsAppealsPage, meta: workspaceUserAuthMeta('support-center') },
    { path: '/my/reports/:kind/:id', name: 'my-report-detail', component: MyReportsAppealsPage, meta: workspaceUserAuthMeta('support-center') },
    { path: '/my/feedback', name: 'my-feedback', component: MyFeedbackPage, meta: workspaceUserAuthMeta('support-center') },
    { path: '/my/feedback/:id', name: 'my-feedback-detail', component: MyFeedbackPage, meta: workspaceUserAuthMeta('support-center') },
    { path: '/announcements/:slug', name: 'announcement-detail', component: AnnouncementDetailPage },
    { path: '/u/:username', name: 'public-user', component: PublicUserPage },
    { path: '/admin', name: 'admin', component: AdminPage, meta: adminAuthMeta },
    { path: '/admin/growth', name: 'admin-growth', component: AdminGrowthPage, meta: adminAuthMeta },
    { path: '/admin/growth-promotions', name: 'admin-growth-promotions', component: AdminGrowthPromotionsPage, meta: adminAuthMeta },
    { path: '/admin/announcements', name: 'admin-announcements', component: AdminAnnouncementsPage, meta: adminAuthMeta },
    { path: '/admin/announcements/new', name: 'admin-announcement-new', component: AdminAnnouncementEditorPage, meta: adminAuthMeta },
    { path: '/admin/announcements/:id/edit', name: 'admin-announcement-edit', component: AdminAnnouncementEditorPage, meta: adminAuthMeta },
    { path: '/admin/product-plans', name: 'admin-product-plans', component: AdminProductPlansPage, meta: adminAuthMeta },
    { path: '/admin/api-models', name: 'admin-api-models', component: AdminApiModelsPage, meta: adminAuthMeta },
    { path: '/admin/api-health', name: 'admin-api-health', component: AdminApiHealthCalibrationPage, meta: adminAuthMeta },
    { path: '/admin/model-audit', name: 'admin-model-audit', component: AdminModelAuditPage, meta: adminAuthMeta },
    { path: '/admin/feedback', name: 'admin-feedback', component: AdminFeedbackPage, meta: adminAuthMeta },
    { path: '/admin/feedback/:id', name: 'admin-feedback-detail', component: AdminFeedbackPage, meta: adminAuthMeta },
    { path: '/admin/official-prices', name: 'admin-official-prices', component: OfficialPriceManagePage, meta: adminAuthMeta },
    { path: '/admin/price-leads', redirect: '/admin/official-prices', meta: adminAuthMeta },
    { path: '/admin/users', name: 'admin-users', component: AdminUsersPage, meta: adminAuthMeta },
    { path: '/admin/api-orders/:id', name: 'admin-api-order-detail', component: AdminApiOrderDetailPage, meta: adminAuthMeta },
    { path: '/admin/api-promotions', name: 'admin-api-promotions', component: AdminApiPromotionsPage, meta: adminAuthMeta },
    { path: '/admin/student-registration', name: 'admin-student-registration', component: AdminStudentRegistrationPage, meta: adminAuthMeta },
    { path: '/admin/logs', name: 'admin-logs', component: AdminAuditLogsPage, meta: adminAuthMeta },
    { path: '/admin/restrictions', redirect: '/admin/users', meta: adminAuthMeta },
    { path: '/admin/api-merchants', redirect: '/admin/api-services', meta: adminAuthMeta },
    { path: '/admin/audit-logs', redirect: '/admin/logs', meta: adminAuthMeta },
    { path: '/admin/carpool-applications', redirect: '/admin/carpools', meta: adminAuthMeta },
    { path: '/admin/certifications', redirect: '/admin/users', meta: adminAuthMeta },
    ...adminChildren.map(([path, title, description]) => ({
      path: `/admin/${path}`,
      name: `admin-${path}`,
      component: AdminSectionPage,
      meta: { ...adminAuthMeta, title, description, section: path },
    })),
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundPage },
]
