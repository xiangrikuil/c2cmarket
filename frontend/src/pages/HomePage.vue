<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import ArrowRight from 'lucide-vue-next/dist/esm/icons/arrow-right.js'
import ChevronRight from 'lucide-vue-next/dist/esm/icons/chevron-right.js'
import Code2 from 'lucide-vue-next/dist/esm/icons/code-xml.js'
import FileChartColumnIncreasing from 'lucide-vue-next/dist/esm/icons/file-chart-column-increasing.js'
import UsersRound from 'lucide-vue-next/dist/esm/icons/users-round.js'
import { Badge } from '@/components/ui/badge'
import HomeMarketSnapshot from '@/components/market/HomeMarketSnapshot.vue'
import { isApiServicePubliclyOrderable } from '@/lib/apiServicePresentation'
import { isCurrentTradable } from '@/lib/pricing'
import { useHomeMarket } from '@/queries/useHomeMarketQuery'
import { useProductCategories } from '@/queries/useProductCatalogQueries'
import { prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'

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

const carpoolPreview = computed(() => tradableCarpools.value.slice(0, 3))
const apiServicePreview = computed(() => orderableApiServices.value.slice(0, 3))
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))
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

      <section class="home-market-entries" aria-label="市场入口" :aria-busy="isLoading">
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
    </section>

    <HomeMarketSnapshot
      :carpools="carpoolPreview"
      :api-services="apiServicePreview"
      :carpool-count="tradableCarpools.length"
      :api-service-count="orderableApiServices.length"
      :has-market-data="hasMarketData"
      :is-loading="isLoading"
      :is-error="isError"
      :category-icon-by-code="categoryIconByCode"
      @retry="refetch()"
    />
  </div>
</template>
