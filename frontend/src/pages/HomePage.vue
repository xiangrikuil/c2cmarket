<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { ArrowRight, Car, ChevronRight, Code2, FileChartColumnIncreasing, ShieldCheck, UsersRound } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { formatCnyPerUsdQuota, formatModelSummary } from '@/components/api-service-detail/utils'
import { getApiMerchantDisplayName, isApiServicePubliclyOrderable } from '@/lib/api'
import { getCurrentPayablePrice, getRemainingSeats, isCurrentTradable } from '@/lib/pricing'
import { getApiServiceProductIconSrc, getProductIconSrc } from '@/lib/productCategoryIcon'
import { useHomeMarket } from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'
import { prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'

const homeMarketTabs = ['carpools', 'api-services'] as const
type HomeMarketTab = typeof homeMarketTabs[number]

const homeMarketTabStorageKey = 'c2cmarket.home-market-tab.v1'

function isHomeMarketTab(value: string | null): value is HomeMarketTab {
  return homeMarketTabs.some(tab => tab === value)
}

function getInitialHomeMarketTab(): HomeMarketTab {
  if (typeof window === 'undefined') return 'carpools'
  const stored = window.localStorage.getItem(homeMarketTabStorageKey)
  return isHomeMarketTab(stored) ? stored : 'carpools'
}

const router = useRouter()
const activeMarketTab = ref<HomeMarketTab>(getInitialHomeMarketTab())
const homeMarketQuery = useHomeMarket()
const productCategoriesQuery = useProductCategories()
const { data, isLoading, isError, refetch } = homeMarketQuery
const { data: catalogCategories } = productCategoriesQuery
prefetchQueriesOnServer(homeMarketQuery, productCategoriesQuery)

const tradableCarpools = computed(() => (data.value?.carpools ?? [])
  .filter(item => isCurrentTradable(item) && item.applicationEligibility?.canApply !== false))
const orderableApiServices = computed(() => (data.value?.apiServices ?? [])
  .filter(isApiServicePubliclyOrderable))
const verifiedPriceRecords = computed(() => (data.value?.officialPrices ?? [])
  .filter(item => item.status === '已验证' && item.cny !== null))
const hasMarketData = computed(() => data.value !== undefined)

const carpoolPreview = computed(() => tradableCarpools.value.slice(0, 4))
const apiServicePreview = computed(() => orderableApiServices.value.slice(0, 4))
const stats = computed(() => [
  { label: '可申请车源', value: tradableCarpools.value.length, hint: '当前公开可申请', icon: Car, tone: 'carpool' },
  { label: '可购买 API', value: orderableApiServices.value.length, hint: '当前可创建订单', icon: Code2, tone: 'api' },
  { label: '官网价格记录', value: verifiedPriceRecords.value.length, hint: '已验证参考价', icon: FileChartColumnIncreasing, tone: 'price' },
])

const marketTabMeta: Record<HomeMarketTab, { to: string, emptyTitle: string, emptyDescription: string }> = {
  carpools: {
    to: '/carpools',
    emptyTitle: '暂无可申请车源',
    emptyDescription: '当前没有符合申请条件的公开车源，可以稍后再查看。',
  },
  'api-services': {
    to: '/api-market',
    emptyTitle: '暂无可购买 API 服务',
    emptyDescription: '当前没有符合公开下单条件的 API 服务。',
  },
}

const activeMarketMeta = computed(() => marketTabMeta[activeMarketTab.value])
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))

function setActiveMarketTab(value: string | number) {
  const normalized = String(value)
  if (!isHomeMarketTab(normalized)) return
  activeMarketTab.value = normalized
  if (typeof window !== 'undefined') window.localStorage.setItem(homeMarketTabStorageKey, normalized)
}

function openMarketRecord(event: MouseEvent | KeyboardEvent, to: string) {
  if (event instanceof MouseEvent && (event.target as HTMLElement).closest('a,button')) return
  router.push(to)
}

function productIconSrc(product: string) {
  return getProductIconSrc(product, categoryIconByCode.value)
}

function apiServiceIconSrc(service: NonNullable<typeof data.value>['apiServices'][number]) {
  return getApiServiceProductIconSrc(service, categoryIconByCode.value)
}
</script>

<template>
  <div class="home-market-page space-y-5">
    <section class="home-market-overview" aria-labelledby="home-market-title">
      <div class="home-market-overview-copy">
        <Badge class="home-market-kicker" variant="secondary">市场概览</Badge>
        <h1 id="home-market-title">发现车源、API 服务与官网价格</h1>
        <p>平台仅提供信息撮合与交易记录，不代收、不托管资金。</p>
        <RouterLink to="/my/notifications?tab=announcements" class="home-market-rules-link">
          查看平台公告<ArrowRight aria-hidden="true" />
        </RouterLink>
      </div>

      <div class="home-market-stats" :aria-busy="isLoading">
        <div v-for="item in stats" :key="item.label" class="home-market-stat">
          <span :class="['home-market-stat-icon', `is-${item.tone}`]" aria-hidden="true">
            <component :is="item.icon" />
          </span>
          <dl>
            <dt>{{ item.label }}</dt>
            <dd v-if="hasMarketData">{{ item.value }}</dd>
            <dd v-else-if="isLoading"><span class="home-market-count-skeleton" /></dd>
            <dd v-else aria-label="数据暂不可用">—</dd>
            <small>{{ item.hint }}</small>
          </dl>
        </div>
      </div>
    </section>

    <section class="home-market-entries" aria-label="市场入口">
      <RouterLink to="/carpools" class="home-entry-card is-carpool">
        <span class="home-entry-icon" aria-hidden="true"><UsersRound /></span>
        <span class="home-entry-copy"><strong>订阅拼车</strong><small>比较月费、访问安排与剩余名额</small></span>
        <span class="home-entry-meta"><span v-if="hasMarketData">{{ tradableCarpools.length }} 个车源可申请</span><span v-else-if="isLoading" class="home-market-count-skeleton" /><span v-else>数据暂不可用</span><ChevronRight aria-hidden="true" /></span>
      </RouterLink>
      <RouterLink to="/api-market" class="home-entry-card is-api">
        <span class="home-entry-icon" aria-hidden="true"><Code2 /></span>
        <span class="home-entry-copy"><strong>API 服务</strong><small>比较额度售价、最低订单与接入说明</small></span>
        <span class="home-entry-meta"><span v-if="hasMarketData">{{ orderableApiServices.length }} 个服务可购买</span><span v-else-if="isLoading" class="home-market-count-skeleton" /><span v-else>数据暂不可用</span><ChevronRight aria-hidden="true" /></span>
      </RouterLink>
      <RouterLink to="/official-prices" class="home-entry-card is-price">
        <span class="home-entry-icon" aria-hidden="true"><FileChartColumnIncreasing /></span>
        <span class="home-entry-copy"><strong>官网价格</strong><small>查看已维护的官网公开价与更新时间</small></span>
        <span class="home-entry-meta"><span v-if="hasMarketData">{{ verifiedPriceRecords.length }} 条已验证记录</span><span v-else-if="isLoading" class="home-market-count-skeleton" /><span v-else>数据暂不可用</span><ChevronRight aria-hidden="true" /></span>
      </RouterLink>
    </section>

    <Tabs :model-value="activeMarketTab" @update:model-value="setActiveMarketTab">
      <Card class="home-latest-market overflow-hidden p-0">
        <div class="home-latest-header">
          <div>
            <h2>最新可交易内容</h2>
            <p>仅展示当前可申请或可购买的公开记录</p>
          </div>
          <RouterLink :to="activeMarketMeta.to" class="home-latest-all">查看全部<ChevronRight aria-hidden="true" /></RouterLink>
        </div>

        <div class="home-latest-tabs-wrap">
          <TabsList class="home-latest-tabs">
            <TabsTrigger value="carpools">可申请车源</TabsTrigger>
            <TabsTrigger value="api-services">可购买 API 服务</TabsTrigger>
          </TabsList>
        </div>

        <div v-if="isLoading" class="home-latest-state"><SkeletonTable :rows="4" :columns="5" /></div>
        <div v-else-if="isError" class="home-latest-state"><ErrorState title="首页市场内容加载失败" @retry="refetch()" /></div>
        <template v-else>
          <TabsContent value="carpools" class="mt-0">
            <EmptyState v-if="carpoolPreview.length === 0" :title="marketTabMeta.carpools.emptyTitle" :description="marketTabMeta.carpools.emptyDescription">
              <template #action><RouterLink to="/carpools" class="home-empty-link">浏览全部车源</RouterLink></template>
            </EmptyState>
            <template v-else>
              <div class="home-latest-table-wrap">
                <Table class="home-latest-table">
                  <TableHeader><TableRow><TableHead>车源</TableHead><TableHead>地区 / 接入</TableHead><TableHead>车主</TableHead><TableHead>月费</TableHead><TableHead>状态</TableHead></TableRow></TableHeader>
                  <TableBody>
                    <TableRow v-for="item in carpoolPreview" :key="item.id" class="home-latest-row" tabindex="0" @click="openMarketRecord($event, `/carpools/${item.id}`)" @keydown.enter="openMarketRecord($event, `/carpools/${item.id}`)">
                      <TableCell><div class="home-record-primary"><span class="home-record-icon"><img v-if="productIconSrc(item.product)" :src="productIconSrc(item.product)!" alt="" /></span><strong>{{ item.product }}</strong></div></TableCell>
                      <TableCell><strong>{{ item.region }}</strong><small>{{ item.openingMethod }}</small></TableCell>
                      <TableCell><strong>{{ item.owner }}</strong><small>{{ item.ownerType }} · {{ item.trustLevel === null ? '信任等级暂无数据' : `信任等级 ${item.trustLevel}` }}</small></TableCell>
                      <TableCell><strong>¥{{ getCurrentPayablePrice(item) }}/月</strong><small>{{ item.pricingMode === 'fixed' ? '固定月费' : '人数变化时价格可调整' }}</small></TableCell>
                      <TableCell><div class="home-record-status"><Badge class="home-status-available" variant="secondary">可申请 {{ getRemainingSeats(item) }} 位</Badge><ChevronRight aria-hidden="true" /></div></TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
              <div class="home-latest-mobile-list">
                <RouterLink v-for="item in carpoolPreview" :key="item.id" :to="`/carpools/${item.id}`" class="home-mobile-record">
                  <span class="home-record-icon"><img v-if="productIconSrc(item.product)" :src="productIconSrc(item.product)!" alt="" /></span>
                  <span class="home-mobile-record-copy"><strong>{{ item.product }}</strong><small>{{ item.region }} · {{ item.owner }} · ¥{{ getCurrentPayablePrice(item) }}/月</small></span>
                  <span class="home-mobile-record-status">可申请 {{ getRemainingSeats(item) }} 位</span><ChevronRight aria-hidden="true" />
                </RouterLink>
              </div>
            </template>
          </TabsContent>

          <TabsContent value="api-services" class="mt-0">
            <EmptyState v-if="apiServicePreview.length === 0" :title="marketTabMeta['api-services'].emptyTitle" :description="marketTabMeta['api-services'].emptyDescription">
              <template #action><RouterLink to="/api-market" class="home-empty-link">前往 API 市场</RouterLink></template>
            </EmptyState>
            <template v-else>
              <div class="home-latest-table-wrap">
                <Table class="home-latest-table">
                  <TableHeader><TableRow><TableHead>服务名称</TableHead><TableHead>模型 / 平台</TableHead><TableHead>卖家</TableHead><TableHead>价格</TableHead><TableHead>状态</TableHead></TableRow></TableHeader>
                  <TableBody>
                    <TableRow v-for="item in apiServicePreview" :key="item.id" class="home-latest-row" tabindex="0" @click="openMarketRecord($event, `/api-market/${item.id}`)" @keydown.enter="openMarketRecord($event, `/api-market/${item.id}`)">
                      <TableCell><div class="home-record-primary"><span class="home-record-icon is-api"><img v-if="apiServiceIconSrc(item)" :src="apiServiceIconSrc(item)!" :alt="`${formatModelSummary(item.models)} 品牌图标`" /><Code2 v-else aria-hidden="true" /></span><strong>{{ item.title }}</strong></div></TableCell>
                      <TableCell><strong>{{ formatModelSummary(item.models) }}</strong><small>{{ item.delivery }} · {{ item.billingMode }}</small></TableCell>
                      <TableCell><strong>{{ getApiMerchantDisplayName(item) }}</strong><small>{{ item.merchantType }} · {{ item.trustLevel === null ? '信任等级暂无数据' : `信任等级 ${item.trustLevel}` }}</small></TableCell>
                      <TableCell><strong>{{ formatCnyPerUsdQuota(item) }}</strong><small>¥{{ item.minimumPurchaseCny }} 起</small></TableCell>
                      <TableCell><div class="home-record-status"><Badge class="home-status-available" variant="secondary">可创建订单</Badge><ChevronRight aria-hidden="true" /></div></TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
              <div class="home-latest-mobile-list">
                <RouterLink v-for="item in apiServicePreview" :key="item.id" :to="`/api-market/${item.id}`" class="home-mobile-record">
                  <span class="home-record-icon is-api"><img v-if="apiServiceIconSrc(item)" :src="apiServiceIconSrc(item)!" :alt="`${formatModelSummary(item.models)} 品牌图标`" /><Code2 v-else aria-hidden="true" /></span>
                  <span class="home-mobile-record-copy"><strong>{{ item.title }}</strong><small>{{ getApiMerchantDisplayName(item) }} · {{ formatCnyPerUsdQuota(item) }}</small></span>
                  <span class="home-mobile-record-status">可购买</span><ChevronRight aria-hidden="true" />
                </RouterLink>
              </div>
            </template>
          </TabsContent>

        </template>

        <div class="home-market-boundary"><ShieldCheck aria-hidden="true" /><span><strong>平台提示：</strong>请先核对卖家资料、计费方式与可用窗口，再进行线下交易。</span></div>
      </Card>
    </Tabs>
  </div>
</template>
