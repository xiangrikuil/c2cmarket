import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'vitest'

const pageSource = readFileSync(new URL('../../../pages/MyApiServiceDetailPage.vue', import.meta.url), 'utf8')
const headerSource = readFileSync(new URL('../ApiServiceOwnerHeader.vue', import.meta.url), 'utf8')
const metricsSource = readFileSync(new URL('../ApiServiceOwnerMetrics.vue', import.meta.url), 'utf8')
const overviewSource = readFileSync(new URL('../ApiServiceOwnerOverview.vue', import.meta.url), 'utf8')
const salesSource = readFileSync(new URL('../../api-quota/ApiQuotaOwnerManager.vue', import.meta.url), 'utf8')
const listSource = readFileSync(new URL('../../../pages/MyApiServicesPage.vue', import.meta.url), 'utf8')
const querySource = readFileSync(new URL('../../../queries/useMarketQueries.ts', import.meta.url), 'utf8')

describe('API 服务卖家管理页结构', () => {
  test('页面只编排服务头部、核心指标、服务概览和销售管理', () => {
    assert.match(pageSource, /<ApiServiceOwnerHeader/)
    assert.match(pageSource, /<ApiServiceOwnerMetrics/)
    assert.match(pageSource, /<ApiServiceOwnerOverview/)
    assert.match(pageSource, /<ApiQuotaOwnerManager/)
    assert.doesNotMatch(pageSource, /<CompactStats|服务配置|经营状态|公开状态/)
  })

  test('头部只显示单一状态并把真实低频操作放入更多菜单', () => {
    assert.match(headerSource, /getApiServiceOwnerStatus/)
    assert.match(headerSource, /买家视角预览/)
    assert.match(headerSource, /查看 API 销售订单/)
    assert.match(headerSource, /<DropdownMenu[\s\S]*?暂停接单[\s\S]*?恢复接单/)
    assert.doesNotMatch(headerSource, /编辑服务|归档服务|在线接单|买家可下单/)
    assert.match(pageSource, /window\.confirm\('确认暂停这项 API 服务的接单/)
  })

  test('三个经营指标合并最低起购规则并使用语义图标', () => {
    assert.match(metricsSource, /<WalletCards[\s\S]*?可售额度/)
    assert.match(metricsSource, /<ShoppingCart[\s\S]*?销售价格/)
    assert.match(metricsSource, /<ClipboardList[\s\S]*?今日订单/)
    assert.match(metricsSource, /销售价格[\s\S]*?最低 ¥/)
    assert.match(metricsSource, /今日订单[\s\S]*?已接单 \/ 每日上限/)
    assert.doesNotMatch(metricsSource, /最低订单金额/)
  })

  test('服务概览合并基础配置、履约字段和支付品牌图标', () => {
    for (const label of ['接入类型', '支持模型', '服务倍率', '额度有效期', '付款窗口', '首字响应', '建议并发', '收款方式', '未解决纠纷']) {
      assert.match(overviewSource, new RegExp(label))
    }
    assert.match(overviewSource, /<Popover[\s\S]*?已有订单继续使用创建时冻结的/)
    assert.match(overviewSource, /<ApiPaymentMethodIcon :method="method"/)
    assert.match(overviewSource, /service\.unresolvedDisputes \?\? '暂无数据'/)
  })

  test('销售管理使用四个标签页和真实创建入口', () => {
    assert.match(salesSource, /<Tabs v-model="salesTab"/)
    for (const value of ['batches', 'offers', 'rounds', 'credentials']) {
      assert.match(salesSource, new RegExp(`<TabsTrigger value="${value}"`))
      assert.match(salesSource, new RegExp(`<TabsContent value="${value}"`))
    }
    assert.match(salesSource, /新建额度批次/)
    assert.match(salesSource, /快速发布限时包/)
    assert.doesNotMatch(salesSource, /编辑服务|新建固定销售规格/)
    assert.match(salesSource, /window\.confirm\('确认暂停这个额度批次/)
    assert.match(salesSource, /window\.confirm\('确认归档这个额度批次/)
  })

  test('服务列表按销售生命周期筛选并区分桌面与移动布局', () => {
    assert.match(listSource, /getInitialApiServiceSalesView\(route\.query\.intent\)/)
    assert.match(listSource, /useMyApiServices\(salesView\)/)
    assert.match(listSource, /<Select v-model="salesView">/)
    assert.match(listSource, /class="hidden md:block"/)
    assert.match(listSource, /class="divide-y divide-border border-y border-border md:hidden"/)
    assert.match(listSource, /销售方式[\s\S]*?销售状态[\s\S]*?销售时间[\s\S]*?服务状态/)
    assert.match(listSource, /重新发布限时包/)
    assert.doesNotMatch(listSource, /<CompactStats/)
    assert.match(querySource, /queryKey: computed\(\(\) => \['my-api-services', valueOf\(salesView\)\]\)/)
  })
})
