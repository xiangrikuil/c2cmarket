import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const home = source('../../pages/HomePage.vue')
const search = source('../../pages/SearchPage.vue')
const login = source('../../pages/LoginPage.vue')
const officialDetail = source('../../pages/OfficialPriceDetailPage.vue')
const carpools = source('../../pages/CarpoolsPage.vue')
const carpoolDetail = source('../../pages/CarpoolDetailPage.vue')
const carpoolPublish = source('../../pages/CarpoolPublishPage.vue')
const demands = source('../../pages/DemandsPage.vue')
const demandDetail = source('../../pages/DemandDetailPage.vue')
const apiMarket = source('../../pages/ApiMarketPage.vue')
const apiPurchasePanel = source('../../components/api-service-detail/ApiPurchasePanel.vue')
const apiPublish = source('../../pages/ApiServicePublishPage.vue')
const rides = source('../../pages/MyRidesPage.vue')
const rideDetail = source('../../pages/CarpoolApplicationDetailPage.vue')
const orders = source('../../pages/MyApiOrdersPage.vue')
const orderDetail = source('../../pages/ApiPurchaseOrderDetailPage.vue')
const publicUser = source('../../pages/PublicUserPage.vue')
const router = source('../../router.ts')

describe('公开市场与交易页面一致性', () => {
  it('让首页成为市场入口而不是运营看板', () => {
    expect(home).toContain('欢迎来到 C2CMarket')
    expect(home).toContain('home-hero-stats')
    expect(home).toContain('home-module-action')
    expect(home).toContain('平台不代收、不托管资金')
    expect(home).toContain('浏览订阅拼车')
    expect(home).toContain('浏览 API 服务')
    expect(home).not.toContain('官网价格参考')
    expect(home).not.toContain('完整价格表')
    expect(home).not.toContain('HomeTrendChart')
    expect(home).not.toContain('社区行情总览')
  })

  it('为搜索、登录和官网价格提供明确状态与边界', () => {
    expect(search).toContain('<SkeletonTable')
    expect(search).toContain('热门拼车')
    expect(login).toContain('if (session.value) await router.replace(returnTo.value)')
    expect(officialDetail).toContain('此价格仅供参考，不等于市场交易价格')
    expect(officialDetail).toContain('税费口径')
  })

  it('让拼车和 API 列表支持高密度整行导航', () => {
    expect(carpools).toContain('carpool-reference-top')
    expect(carpools).toContain('carpool-catalog-panel')
    expect(carpools).toContain('@keydown.enter="openCarpool')
    expect(carpools).not.toContain('carpool-view-button')
    expect(apiMarket).toContain('@keydown.enter="openService')
    expect(apiMarket).not.toContain('api-market-intent-button')
  })

  it('让详情行动卡只有一个权威主操作并保留次级动作', () => {
    expect(carpoolDetail).toContain('lg:sticky lg:top-16')
    expect(carpoolDetail).toContain('申请上车')
    expect(carpoolDetail).toContain('分享')
    expect(apiPurchasePanel).toContain('创建订单并查看付款方式')
    expect(apiPurchasePanel).toContain('平台记录状态但不代收、不托管资金')
    expect(apiPurchasePanel).toContain('付款窗口')
  })

  it('拆分求车发布并复用现有车源回应流程', () => {
    expect(router).toContain("path: '/demands/new'")
    expect(demands).not.toContain('发布求车需求</h2>')
    expect(demandDetail).toContain('使用我的车源回应')
    expect(demandDetail).toContain("query: { respondTo: demand.id }")
  })

  it('保护发布页未保存内容并保持真实预览', () => {
    expect(carpoolPublish).toContain('useUnsavedChangesGuard')
    expect(carpoolPublish).toContain('CarpoolPublishPreview')
    expect(apiPublish).toContain('useUnsavedChangesGuard')
    expect(apiPublish).toContain('ApiServicePublishPreview')
  })

  it('统一申请和订单的短编号、本地时间、快照与下一动作', () => {
    expect(rides).toContain('<ShortId')
    expect(rides).toContain('<LocalTime')
    expect(rideDetail).toContain('ride-order-stepper')
    expect(rideDetail).toContain('ride-order-action-card')
    expect(rideDetail).toContain('申请快照')
    expect(orders).toContain('<ShortId')
    expect(orders).toContain('<LocalTime')
    expect(orderDetail).toContain('<Stepper')
    expect(orderDetail).toContain('当前可执行操作')
  })

  it('公开用户页只展示有来源的公开活动并统一全空状态', () => {
    expect(publicUser).toContain('hasPublicActivity')
    expect(publicUser).toContain('暂无公开业务记录')
    expect(publicUser).toContain('不展示任何联系方式')
  })
})
