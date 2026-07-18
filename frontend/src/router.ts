import type { RouteRecordRaw } from 'vue-router'

const HomePage = () => import('@/pages/HomePage.vue')
const OfficialPricesPage = () => import('@/pages/OfficialPricesPage.vue')
const OfficialPriceDetailPage = () => import('@/pages/OfficialPriceDetailPage.vue')
const OfficialPriceManagePage = () => import('@/pages/OfficialPriceManagePage.vue')
const CarpoolsPage = () => import('@/pages/CarpoolsPage.vue')
const CarpoolDetailPage = () => import('@/pages/CarpoolDetailPage.vue')
const CarpoolPublishPage = () => import('@/pages/CarpoolPublishPage.vue')
const DemandsPage = () => import('@/pages/DemandsPage.vue')
const DemandPublishPage = () => import('@/pages/DemandPublishPage.vue')
const DemandDetailPage = () => import('@/pages/DemandDetailPage.vue')
const ApiMarketPage = () => import('@/pages/ApiMarketPage.vue')
const ApiServiceDetailPage = () => import('@/pages/ApiServiceDetailPage.vue')
const ApiServicePublishPage = () => import('@/pages/ApiServicePublishPage.vue')
const SearchPage = () => import('@/pages/SearchPage.vue')
const MyCenterPage = () => import('@/pages/MyCenterPage.vue')
const MyCarpoolsPage = () => import('@/pages/MyCarpoolsPage.vue')
const MyDemandsPage = () => import('@/pages/MyDemandsPage.vue')
const MyRidesPage = () => import('@/pages/MyRidesPage.vue')
const CarpoolApplicationDetailPage = () => import('@/pages/CarpoolApplicationDetailPage.vue')
const MyApiOrdersPage = () => import('@/pages/MyApiOrdersPage.vue')
const MyApiServicesPage = () => import('@/pages/MyApiServicesPage.vue')
const MyApiServiceDetailPage = () => import('@/pages/MyApiServiceDetailPage.vue')
const ApiPurchaseOrderDetailPage = () => import('@/pages/ApiPurchaseOrderDetailPage.vue')
const LegacyApiIntentRedirectPage = () => import('@/pages/LegacyApiIntentRedirectPage.vue')
const MerchantApiOrdersPage = () => import('@/pages/MerchantApiOrdersPage.vue')
const MerchantCarpoolApplicationsPage = () => import('@/pages/MerchantCarpoolApplicationsPage.vue')
const MyFavoritesPage = () => import('@/pages/MyFavoritesPage.vue')
const MyReviewsPage = () => import('@/pages/MyReviewsPage.vue')
const MyNotificationsPage = () => import('@/pages/MyNotificationsPage.vue')
const MyFeedbackPage = () => import('@/pages/MyFeedbackPage.vue')
const AnnouncementDetailPage = () => import('@/pages/AnnouncementDetailPage.vue')
const LoginPage = () => import('@/pages/LoginPage.vue')
const PublicUserPage = () => import('@/pages/PublicUserPage.vue')
const AdminPage = () => import('@/pages/AdminPage.vue')
const AdminFeedbackPage = () => import('@/pages/AdminFeedbackPage.vue')
const AdminAnnouncementsPage = () => import('@/pages/AdminAnnouncementsPage.vue')
const AdminAnnouncementEditorPage = () => import('@/pages/AdminAnnouncementEditorPage.vue')
const AdminProductPlansPage = () => import('@/pages/AdminProductPlansPage.vue')
const AdminApiModelsPage = () => import('@/pages/AdminApiModelsPage.vue')
const AdminModelAuditPage = () => import('@/pages/AdminModelAuditPage.vue')
const AdminUsersPage = () => import('@/pages/AdminUsersPage.vue')
const AdminSectionPage = () => import('@/pages/AdminSectionPage.vue')
const NotFoundPage = () => import('@/pages/NotFoundPage.vue')

const adminChildren = [
  ['carpools', '车源异常处理', '处理暂停、下架、待复核和遗留审核车源；公开在售车源直接在普通列表巡查。'],
  ['demands', '求车管理', '查看求车需求、关闭状态和 linux.do 求车原帖绑定。'],
  ['api-services', 'API 服务审核', '审核模型价格、最低订单金额、交易说明和商户承诺规则。'],
  ['trade-intents', 'API 订单追踪', '查看 API 订单、参与方、金额快照、完成和取消状态。'],
  ['reports', '举报纠纷', '处理举报、纠纷和未解决记录。'],
  ['appeals', '申诉处理', '处理用户对限制、下架和封禁的申诉。'],
  ['logs', '审计日志', '查看系统与管理员操作记录。'],
] as const

export const routes: RouteRecordRaw[] = [
    { path: '/', name: 'home', component: HomePage },
    { path: '/search', name: 'search', component: SearchPage },
    { path: '/login', name: 'login', component: LoginPage, meta: { standalone: true } },
    { path: '/auth/mock', redirect: '/login' },
    { path: '/official-prices', name: 'official-prices', component: OfficialPricesPage },
    { path: '/official-prices/detail', redirect: '/official-prices/p1' },
    { path: '/official-prices/submit', redirect: '/official-prices' },
    { path: '/official-prices/manage', redirect: '/admin/official-prices' },
    { path: '/official-prices/:id', name: 'official-prices-detail', component: OfficialPriceDetailPage },
    { path: '/carpools', name: 'carpools', component: CarpoolsPage },
    { path: '/carpools/detail', redirect: '/carpools/c1' },
    { path: '/carpools/new', name: 'carpool-new', component: CarpoolPublishPage },
    { path: '/carpools/:id', name: 'carpool-detail', component: CarpoolDetailPage },
    { path: '/demands', name: 'demands', component: DemandsPage },
    { path: '/demands/new', name: 'demand-new', component: DemandPublishPage },
    { path: '/demands/:id', name: 'demand-detail', component: DemandDetailPage },
    { path: '/api-market', name: 'api-market', component: ApiMarketPage },
    { path: '/api-market/detail', redirect: '/api-market/a1' },
    { path: '/api-market/new', name: 'api-new', component: ApiServicePublishPage },
    { path: '/api-market/:id', name: 'api-detail', component: ApiServiceDetailPage },
    { path: '/my', name: 'my', component: MyCenterPage },
    { path: '/my/profile', name: 'my-profile', component: MyCenterPage },
    { path: '/my/contacts', name: 'my-contacts', component: MyCenterPage },
    { path: '/my/account', name: 'my-account', component: MyCenterPage },
    { path: '/my/privacy', name: 'my-privacy', component: MyCenterPage },
    { path: '/my/carpools', name: 'my-carpools', component: MyCarpoolsPage },
    { path: '/my/demands', name: 'my-demands', component: MyDemandsPage },
    { path: '/my/rides', name: 'my-rides', component: MyRidesPage },
    { path: '/my/rides/:id', name: 'my-ride-detail', component: CarpoolApplicationDetailPage },
    { path: '/my/api-orders', name: 'my-api-orders', component: MyApiOrdersPage },
    { path: '/my/api-orders/:id', name: 'my-api-order-detail', component: ApiPurchaseOrderDetailPage },
    { path: '/my/api-services', name: 'my-api-services', component: MyApiServicesPage },
    { path: '/my/api-services/:id', name: 'my-api-service-detail', component: MyApiServiceDetailPage },
    { path: '/api-intents/:id', name: 'legacy-api-intent-detail', component: LegacyApiIntentRedirectPage },
    { path: '/merchant/carpool-applications', name: 'merchant-carpool-applications', component: MerchantCarpoolApplicationsPage },
    { path: '/merchant/carpool-applications/:id', name: 'merchant-carpool-application-detail', component: CarpoolApplicationDetailPage },
    { path: '/merchant/api-orders', name: 'merchant-api-orders', component: MerchantApiOrdersPage },
    { path: '/merchant/api-orders/:id', name: 'merchant-api-order-detail', component: ApiPurchaseOrderDetailPage },
    { path: '/my/favorites', name: 'my-favorites', component: MyFavoritesPage },
    { path: '/my/reviews', name: 'my-reviews', component: MyReviewsPage },
    { path: '/my/notifications', name: 'my-notifications', component: MyNotificationsPage },
    { path: '/my/feedback', name: 'my-feedback', component: MyFeedbackPage },
    { path: '/my/feedback/:id', name: 'my-feedback-detail', component: MyFeedbackPage },
    { path: '/announcements/:slug', name: 'announcement-detail', component: AnnouncementDetailPage },
    { path: '/u/:username', name: 'public-user', component: PublicUserPage },
    { path: '/admin', name: 'admin', component: AdminPage },
    { path: '/admin/announcements', name: 'admin-announcements', component: AdminAnnouncementsPage },
    { path: '/admin/announcements/new', name: 'admin-announcement-new', component: AdminAnnouncementEditorPage },
    { path: '/admin/announcements/:id/edit', name: 'admin-announcement-edit', component: AdminAnnouncementEditorPage },
    { path: '/admin/product-plans', name: 'admin-product-plans', component: AdminProductPlansPage },
    { path: '/admin/api-models', name: 'admin-api-models', component: AdminApiModelsPage },
    { path: '/admin/model-audit', name: 'admin-model-audit', component: AdminModelAuditPage },
    { path: '/admin/feedback', name: 'admin-feedback', component: AdminFeedbackPage },
    { path: '/admin/feedback/:id', name: 'admin-feedback-detail', component: AdminFeedbackPage },
    { path: '/admin/official-prices', name: 'admin-official-prices', component: OfficialPriceManagePage },
    { path: '/admin/price-leads', redirect: '/admin/official-prices' },
    { path: '/admin/users', name: 'admin-users', component: AdminUsersPage },
    { path: '/admin/restrictions', redirect: '/admin/users' },
    { path: '/admin/api-merchants', redirect: '/admin/api-services' },
    { path: '/admin/audit-logs', redirect: '/admin/logs' },
    { path: '/admin/carpool-applications', redirect: '/admin/carpools' },
    { path: '/admin/certifications', redirect: '/admin/users' },
    ...adminChildren.map(([path, title, description]) => ({
      path: `/admin/${path}`,
      name: `admin-${path}`,
      component: AdminSectionPage,
      meta: { title, description, section: path },
    })),
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundPage },
]
