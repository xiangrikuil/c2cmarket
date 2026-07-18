<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, CheckCircle2, CircleHelp, Code2, FileSearch, Search, ShieldCheck, UsersRound, Zap } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { formatCnyPerUsdQuota, formatModelSummary } from '@/components/api-service-detail/utils'
import { getPricingDisplay } from '@/lib/pricing'
import { getApiMerchantDisplayName } from '@/lib/api'
import { getApiServiceProductIconSrc, getProductIconSrc } from '@/lib/productCategoryIcon'
import { useHomeMarket } from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'
import { prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'

const homeMarketQuery = useHomeMarket()
const productCategoriesQuery = useProductCategories()
const { data, isLoading } = homeMarketQuery
const { data: catalogCategories } = productCategoriesQuery
prefetchQueriesOnServer(homeMarketQuery, productCategoriesQuery)

const availableCarpools = computed(() => (data.value?.carpools ?? [])
  .filter(item => item.status === '可上车')
  .slice(0, 4))
const availableApiServices = computed(() => (data.value?.apiServices ?? [])
  .filter(item => item.publiclyOrderable)
  .slice(0, 4))
const verifiedPrices = computed(() => (data.value?.officialPrices ?? [])
  .filter(item => item.status === '已验证' && item.cny !== null)
  .slice(0, 5))
const openDemandCount = computed(() => (data.value?.demands ?? []).filter(item => item.status === '匹配中').length)
const stats = computed(() => [
  { label: '可上车席位', value: availableCarpools.value.length, hint: '当前公开可申请', icon: UsersRound },
  { label: '可购买 API', value: availableApiServices.value.length, hint: '当前可创建订单', icon: Code2 },
  { label: '求车需求', value: openDemandCount.value, hint: '等待车主回应', icon: Search },
  { label: '已验证官网价', value: verifiedPrices.value.length, hint: '仅作价格参考', icon: ShieldCheck },
])

const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))

function productIconSrc(product: string) {
  return getProductIconSrc(product, categoryIconByCode.value)
}

function apiServiceIconSrc(service: NonNullable<typeof data.value>['apiServices'][number]) {
  return getApiServiceProductIconSrc(service, categoryIconByCode.value)
}
</script>

<template>
  <div class="home-reference">
    <div class="home-reference-layout">
      <main class="home-reference-main min-w-0 space-y-5">
        <section class="home-reference-hero overflow-hidden rounded-2xl border px-6 py-7 md:px-8 md:py-5">
      <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-center">
        <div class="home-hero-copy">
          <Badge class="home-hero-badge">AI 服务撮合与风险治理</Badge>
          <h1 class="mt-3 max-w-3xl text-3xl font-semibold tracking-tight md:text-[34px]">欢迎来到 C2CMarket</h1>
          <p class="mt-2 max-w-2xl text-sm leading-6">在这里浏览订阅拼车、API 市场、求车需求和官网价格；平台提供信息撮合与风险治理，不代收、不托管资金。</p>
        </div>
        <div class="home-hero-visual" aria-hidden="true">
          <div class="home-hero-grid-lines" />
          <div class="home-hero-chart"><span /><span /><span /><span /></div>
          <div class="home-hero-platform home-hero-platform-back" />
          <div class="home-hero-platform home-hero-platform-front" />
          <div class="home-hero-shield">
            <ShieldCheck class="h-20 w-20" :stroke-width="1.6" />
          </div>
          <span class="home-hero-cube home-hero-cube-one" />
          <span class="home-hero-cube home-hero-cube-two" />
          <span class="home-hero-cube home-hero-cube-three" />
        </div>
      </div>
      <div class="home-hero-stats">
        <div v-for="item in stats" :key="item.label"><span><component :is="item.icon" /></span><dl><dt>{{ item.label }}</dt><dd>{{ item.value }}</dd><small>{{ item.hint }}</small></dl></div>
      </div>
        </section>

        <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <RouterLink to="/carpools" class="home-module-card home-module-carpool group rounded-xl border bg-card p-4 transition hover:-translate-y-0.5 hover:shadow-md">
        <div class="home-module-copy"><span class="home-module-icon grid h-10 w-10 place-items-center rounded-xl"><UsersRound class="h-5 w-5" /></span><div><h2>订阅拼车</h2><p>浏览共享订阅席位，比较月费、访问安排与剩余名额。</p></div></div>
        <span class="home-module-action">去浏览<ArrowRight /></span>
      </RouterLink>
      <RouterLink to="/api-market" class="home-module-card home-module-api group rounded-xl border bg-card p-4 transition hover:-translate-y-0.5 hover:shadow-md">
        <div class="home-module-copy"><span class="home-module-icon grid h-10 w-10 place-items-center rounded-xl"><Code2 class="h-5 w-5" /></span><div><h2>API 服务</h2><p>发现公开 API 服务，比较额度售价、最低订单与接入说明。</p></div></div>
        <span class="home-module-action">去浏览<ArrowRight /></span>
      </RouterLink>
      <RouterLink to="/demands" class="home-module-card home-module-demand group rounded-xl border bg-card p-4 transition hover:-translate-y-0.5 hover:shadow-md">
        <div class="home-module-copy"><span class="home-module-icon grid h-10 w-10 place-items-center rounded-xl"><Search class="h-5 w-5" /></span><div><h2>求车需求</h2><p>发布或查看求车需求，按套餐、预算和地区匹配车源。</p></div></div>
        <span class="home-module-action">去查看<ArrowRight /></span>
      </RouterLink>
      <RouterLink to="/official-prices" class="home-module-card home-module-price group rounded-xl border bg-card p-4 transition hover:-translate-y-0.5 hover:shadow-md">
        <div class="home-module-copy"><span class="home-module-icon grid h-10 w-10 place-items-center rounded-xl"><ShieldCheck class="h-5 w-5" /></span><div><h2>官网价格</h2><p>查看公开官网价格与更新时间，作为市场选择参考。</p></div></div>
        <span class="home-module-action">去查看<ArrowRight /></span>
      </RouterLink>
        </section>

        <SkeletonBlock v-if="isLoading" :lines="8" />
        <template v-else>
          <section class="grid gap-5 xl:grid-cols-2">
        <Card class="p-0">
          <div class="flex items-center justify-between border-b border-border px-5 py-4"><div><h2 class="font-semibold">当前可申请车源</h2><p class="mt-1 text-xs text-muted-foreground">优先展示真实、公开且仍有名额的车源</p></div><RouterLink to="/carpools" class="text-sm text-primary">查看全部</RouterLink></div>
          <div v-if="availableCarpools.length" class="divide-y divide-border">
            <RouterLink v-for="item in availableCarpools" :key="item.id" :to="`/carpools/${item.id}`" class="flex items-center justify-between gap-4 px-5 py-4 transition hover:bg-accent/60">
              <div class="flex min-w-0 items-center gap-3"><span class="home-record-icon"><img v-if="productIconSrc(item.product)" :src="productIconSrc(item.product)!" alt="" /></span><div class="min-w-0"><div class="truncate font-medium">{{ item.product }}</div><div class="mt-1 text-xs text-muted-foreground">{{ item.owner }} · {{ item.region }} · {{ item.openingMethod }}</div></div></div>
              <div class="shrink-0 text-right"><div class="font-semibold">¥{{ getPricingDisplay(item).primaryPrice }}/月</div><div class="mt-1 text-xs text-primary">可申请 {{ Math.max(item.maxMembers - item.currentConfirmedMembers, 0) }} 位</div></div>
            </RouterLink>
          </div>
          <div v-else class="p-6 text-sm text-muted-foreground">暂时没有可申请车源，可前往求车大厅发布需求。</div>
        </Card>

        <Card class="p-0">
          <div class="flex items-center justify-between border-b border-border px-5 py-4"><div><h2 class="font-semibold">当前可购买 API 服务</h2><p class="mt-1 text-xs text-muted-foreground">仅展示可创建订单的公开服务</p></div><RouterLink to="/api-market" class="text-sm text-primary">查看全部</RouterLink></div>
          <div v-if="availableApiServices.length" class="divide-y divide-border">
            <RouterLink v-for="item in availableApiServices" :key="item.id" :to="`/api-market/${item.id}`" class="flex items-center justify-between gap-4 px-5 py-4 transition hover:bg-accent/60">
              <div class="flex min-w-0 items-center gap-3"><span class="home-record-icon home-record-icon-api"><img v-if="apiServiceIconSrc(item)" :src="apiServiceIconSrc(item)!" :alt="`${formatModelSummary(item.models)} 品牌图标`" /><Code2 v-else class="h-4 w-4" /></span><div class="min-w-0"><div class="truncate font-medium">{{ item.title }}</div><div class="mt-1 text-xs text-muted-foreground">{{ getApiMerchantDisplayName(item) }} · {{ formatModelSummary(item.models) }}</div></div></div>
              <div class="shrink-0 text-right"><div class="font-semibold">{{ formatCnyPerUsdQuota(item) }}</div><div class="mt-1 text-xs text-muted-foreground">¥{{ item.minimumPurchaseCny }} 起</div></div>
            </RouterLink>
          </div>
          <div v-else class="p-6 text-sm text-muted-foreground">暂时没有可购买 API 服务，请稍后再看。</div>
        </Card>
          </section>
        </template>
      </main>

      <aside class="home-reference-aside space-y-4">
        <Card class="home-quick-card p-4">
          <div class="flex items-center gap-2 font-semibold"><span class="home-aside-title-icon"><Zap class="h-4 w-4" /></span>快速开始</div>
          <div class="mt-4 grid gap-2">
            <RouterLink to="/carpools" class="home-quick-link"><span class="home-quick-icon home-quick-icon--carpool"><UsersRound class="h-4 w-4" /></span><span><strong>浏览订阅拼车</strong><small>比较月费与剩余名额</small></span><ArrowRight class="h-4 w-4" /></RouterLink>
            <RouterLink to="/api-market" class="home-quick-link"><span class="home-quick-icon home-quick-icon--api"><Code2 class="h-4 w-4" /></span><span><strong>浏览 API 服务</strong><small>比较额度售价与接入说明</small></span><ArrowRight class="h-4 w-4" /></RouterLink>
            <RouterLink to="/demands/new" class="home-quick-link"><span class="home-quick-icon home-quick-icon--demand"><Search class="h-4 w-4" /></span><span><strong>发布求车需求</strong><small>等待合适车主回应</small></span><ArrowRight class="h-4 w-4" /></RouterLink>
            <RouterLink to="/official-prices" class="home-quick-link"><span class="home-quick-icon home-quick-icon--price"><FileSearch class="h-4 w-4" /></span><span><strong>查看官网价格</strong><small>核对公开价格记录</small></span><ArrowRight class="h-4 w-4" /></RouterLink>
          </div>
        </Card>

        <Card class="home-boundary-card p-4">
          <div class="flex items-center gap-2 font-semibold"><span class="home-aside-title-icon"><ShieldCheck class="h-4 w-4" /></span>平台边界</div>
          <div class="mt-4 space-y-3 text-sm">
            <div class="flex gap-2"><CheckCircle2 class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /><span>公开车源与服务记录可追溯</span></div>
            <div class="flex gap-2"><CheckCircle2 class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /><span>申请、订单与纠纷状态站内留痕</span></div>
            <div class="flex gap-2"><CheckCircle2 class="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" /><span>平台不代收、不托管资金</span></div>
          </div>
        </Card>

        <Card class="p-4">
          <div class="flex items-center gap-2 font-semibold"><CircleHelp class="h-4 w-4 text-primary" />帮助入口</div>
          <p class="mt-2 text-sm leading-6 text-muted-foreground">不确定应该选择拼车还是 API 服务？先查看平台边界和具体详情。</p>
          <RouterLink to="/search"><Button class="mt-4 w-full" variant="outline">打开全局搜索</Button></RouterLink>
        </Card>
      </aside>
    </div>

  </div>
</template>
