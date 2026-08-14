<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import ArrowRight from 'lucide-vue-next/dist/esm/icons/arrow-right.js'
import CarFront from 'lucide-vue-next/dist/esm/icons/car-front.js'
import ChevronRight from 'lucide-vue-next/dist/esm/icons/chevron-right.js'
import Code2 from 'lucide-vue-next/dist/esm/icons/code-xml.js'
import FileChartColumnIncreasing from 'lucide-vue-next/dist/esm/icons/file-chart-column-increasing.js'
import RefreshCw from 'lucide-vue-next/dist/esm/icons/refresh-cw.js'
import TriangleAlert from 'lucide-vue-next/dist/esm/icons/triangle-alert.js'
import { toast } from 'vue-sonner'
import AnnouncementBanner from '@/components/announcements/AnnouncementBanner.vue'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import HomeMarketSnapshot from '@/components/market/HomeMarketSnapshot.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { isApiServicePubliclyOrderable } from '@/lib/apiServicePresentation'
import { isCurrentTradable } from '@/lib/pricing'
import { useActiveHomeAnnouncement, useDismissAnnouncement } from '@/queries/useAnnouncementQueries'
import { useMyProfileQuery } from '@/queries/useAppShellQueries'
import { useHomeMarket } from '@/queries/useHomeMarketQuery'
import { useProductCategories } from '@/queries/useProductCatalogQueries'
import { prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'

const homeMarketQuery = useHomeMarket()
const productCategoriesQuery = useProductCategories()
const { data: myProfile } = useMyProfileQuery(import.meta.client)
const homeAnnouncementEnabled = computed(() => Boolean(myProfile.value))
const homeAnnouncementQuery = useActiveHomeAnnouncement(homeAnnouncementEnabled)
const dismissHomeAnnouncementMutation = useDismissAnnouncement()
const { data, isLoading, isError, refetch } = homeMarketQuery
const { data: catalogCategories } = productCategoriesQuery
const {
  data: homeAnnouncement,
  isPending: homeAnnouncementPending,
  isError: homeAnnouncementFailed,
  isFetching: homeAnnouncementFetching,
  refetch: refetchHomeAnnouncement,
} = homeAnnouncementQuery
prefetchQueriesOnServer(homeMarketQuery, productCategoriesQuery)

const tradableCarpools = computed(() => (data.value?.carpools ?? [])
  .filter(item => isCurrentTradable(item) && item.applicationEligibility?.canApply !== false))
const orderableApiServices = computed(() => (data.value?.apiServices ?? [])
  .filter(isApiServicePubliclyOrderable))
const verifiedPriceRecords = computed(() => (data.value?.officialPrices ?? [])
  .filter(item => item.status === '已验证' && item.cny !== null))
const hasMarketData = computed(() => data.value !== undefined)
const homeMarketPreviewLimit = 5

const carpoolPreview = computed(() => tradableCarpools.value.slice(0, homeMarketPreviewLimit))
const apiServicePreview = computed(() => orderableApiServices.value.slice(0, homeMarketPreviewLimit))
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))

function dismissHomeAnnouncement(announcementId: string) {
  dismissHomeAnnouncementMutation.mutate(announcementId, {
    onError: error => toast.error(error instanceof Error ? error.message : '关闭首页公告失败。'),
  })
}
</script>

<template>
  <div class="home-market-page space-y-5">
    <SkeletonBlock
      v-if="homeAnnouncementEnabled && homeAnnouncementPending"
      class="min-h-12 rounded-lg p-3 [&>div]:mt-2"
      :lines="1"
    />

    <Alert v-else-if="homeAnnouncementEnabled && homeAnnouncementFailed" variant="destructive">
      <TriangleAlert />
      <AlertTitle>首页公告暂时无法加载</AlertTitle>
      <AlertDescription class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <span>其他首页内容不受影响，可以单独重试公告。</span>
        <Button size="sm" variant="outline" :disabled="homeAnnouncementFetching" @click="refetchHomeAnnouncement()">
          <RefreshCw class="h-4 w-4" :class="homeAnnouncementFetching ? 'animate-spin' : ''" />
          {{ homeAnnouncementFetching ? '正在重试' : '重新加载' }}
        </Button>
      </AlertDescription>
    </Alert>

    <AnnouncementBanner
      v-else-if="homeAnnouncementEnabled && homeAnnouncement"
      :announcement="homeAnnouncement"
      :dismissing="dismissHomeAnnouncementMutation.isPending.value"
      @dismiss="dismissHomeAnnouncement"
    />

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
          <span class="home-entry-icon" aria-hidden="true"><CarFront /></span>
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
