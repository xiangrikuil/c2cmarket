<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, onServerPrefetch, ref, watch, type ComponentPublicInstance } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CalendarClock, Code2, PackageOpen, PackagePlus, Search, Zap } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import InfiniteScrollSentinel from '@/components/market/InfiniteScrollSentinel.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import ApiFreeServiceCard from '@/components/api-market/ApiFreeServiceCard.vue'
import ApiPackageCard from '@/components/api-market/ApiPackageCard.vue'
import ApiQuotaOfferCard from '@/components/api-market/ApiQuotaOfferCard.vue'
import ApiServiceHealthPanel from '@/components/api-market/ApiServiceHealthPanel.vue'
import type { ApiFreeServiceCardData } from '@/components/api-market/apiFreeServiceCard'
import { usePromotionImpression, type PromotionAnalyticsProperties } from '@/composables/usePromotionImpression'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  getApiMerchantDisplayName,
  type ApiService,
  type ApiServiceFilters,
  type ApiQuotaDistributionSystem,
  type ApiQuotaOfferFilters,
  type ApiQuotaSystemSaleSlot,
  type PublicApiQuotaOffer,
} from '@/lib/api'
import {
  apiMarketViewFromQuery,
  apiQuotaOfferErrorMessage,
  withApiMarketViewQuery,
  type ApiMarketView,
} from '@/lib/apiQuotaOfferUi'
import { getDefaultApiPackageFilter, rankApiPackages } from '@/lib/apiPackageRecommendation'
import { getApiMerchantBadges } from '@/lib/apiMerchantBadges'
import type { ApiServicePromotion } from '@/lib/apiMarketBackend'
import { flattenUniqueCursorPages } from '@/lib/cursorPagination'
import { placePromotions, promotionsForBillingMode } from '@/lib/apiPromotionPlacement'
import {
  getProductCategory,
  getProductCategoryLabel,
  type ConcreteProductCategoryKey,
} from '@/lib/productCategories'
import { getApiServiceProductCategory, getApiServiceProductIconSrc, getProductIconSrc } from '@/lib/productCategoryIcon'
import { useApiPromotions, useApiQuotaSaleSlots, useCreateApiQuotaOrderMutation, useInfiniteApiQuotaOffers, useInfiniteApiServices, useModelCatalog } from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'
import { prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'

type AvailabilityFilter = 'all' | 'available'

const route = useRoute()
const router = useRouter()
const activeView = ref<ApiMarketView>(apiMarketViewFromQuery(route.query.view))
const quotaSearch = ref('')
const distributionSystem = ref<ApiQuotaDistributionSystem | 'all'>('all')
const availability = ref<AvailabilityFilter>('all')
const oneMultiplier = ref(false)
const packageModel = ref('')
const packageDuration = ref('')
const packageReady = computed(() => Boolean(packageModel.value && packageDuration.value))
const now = ref(Date.now())
const serverClockOffset = ref(0)
const selectedSlotKey = ref('')
const pendingOfferId = ref('')
let refreshedBoundary = ''
let timer: ReturnType<typeof setInterval> | undefined
let stopPackageDefaultWatch: (() => void) | undefined

watch(
  () => route.query.view,
  value => {
    activeView.value = apiMarketViewFromQuery(value)
  },
  { immediate: true },
)

watch(activeView, value => {
  if (route.query.view === value) return
  router.replace({ query: withApiMarketViewQuery(route.query, value) })
})

const quotaFilters = computed<ApiQuotaOfferFilters>(() => ({
  distributionSystem: distributionSystem.value,
  oneMultiplier: oneMultiplier.value,
  onlyOrderable: availability.value === 'available',
  search: quotaSearch.value.trim() || undefined,
  excludeSystemSlots: true,
}))
const limitedViewEnabled = computed(() => activeView.value === 'limited')
const serviceViewEnabled = computed(() => activeView.value === 'packages' || activeView.value === 'free')
const serviceFilters = computed<ApiServiceFilters>(() => ({
  online: true,
  billingMode: activeView.value === 'packages' ? 'fixed_package' : 'metered_credit',
  packageModelCatalogId: activeView.value === 'packages' && packageReady.value ? packageModel.value : undefined,
  packageDurationDays: activeView.value === 'packages' && packageReady.value ? Number(packageDuration.value) : undefined,
}))
const quotaQuery = useInfiniteApiQuotaOffers(quotaFilters, limitedViewEnabled)
const slotQuery = useApiQuotaSaleSlots()
const rushFilters = computed<ApiQuotaOfferFilters>(() => ({ slotKey: selectedSlotKey.value }))
const rushQuery = useInfiniteApiQuotaOffers(rushFilters, computed(() => limitedViewEnabled.value && Boolean(selectedSlotKey.value)))
const freeServicesQuery = useInfiniteApiServices(serviceFilters, serviceViewEnabled, activeView)
const promotionQuery = useApiPromotions()
const productCategoriesQuery = useProductCategories()
const modelCatalogQuery = useModelCatalog()
const { data: catalogCategories } = productCategoriesQuery
const createOrderMutation = useCreateApiQuotaOrderMutation()
const { setPromotionElement, trackPromotionClick } = usePromotionImpression()
const visibleMarketQuery = activeView.value === 'limited' ? quotaQuery : freeServicesQuery
prefetchQueriesOnServer(visibleMarketQuery, productCategoriesQuery, modelCatalogQuery)
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))
const quotaHasLoadedPages = computed(() => Boolean(quotaQuery.data.value?.pages.length))
const rushHasLoadedPages = computed(() => Boolean(rushQuery.data.value?.pages.length))
const servicesHaveLoadedPages = computed(() => Boolean(freeServicesQuery.data.value?.pages.length))

const quotaRows = computed(() => {
  return flattenUniqueCursorPages(quotaQuery.data.value?.pages)
})
const loadedServices = computed(() => flattenUniqueCursorPages(freeServicesQuery.data.value?.pages))
const freeServices = computed(() => loadedServices.value.filter(service => service.billingMode !== 'fixed_package'))
const packageServices = computed(() => loadedServices.value.filter(service => service.billingMode === 'fixed_package'))
const packageModelOptions = computed(() => {
  return (modelCatalogQuery.data.value ?? [])
    .filter(item => item.active)
    .map(item => ({ id: item.id, name: item.name }))
    .sort((left, right) => left.name.localeCompare(right.name))
})

const packageRows = computed(() => rankApiPackages(packageServices.value, packageModel.value, Number(packageDuration.value)))
const fixedPackagePromotions = computed(() => promotionsForBillingMode(
  promotionQuery.data.value ?? [],
  true,
  promotion => rankApiPackages([promotion.service], packageModel.value, Number(packageDuration.value)).length > 0,
))
const freeServicePromotions = computed(() => promotionsForBillingMode(promotionQuery.data.value ?? [], false))
const packageDisplayRows = computed(() => {
  const naturalRows = packageRows.value.map((row, index) => ({ row, rank: index + 1, promotion: undefined as ApiServicePromotion | undefined, promotionPosition: undefined as PromotionAnalyticsProperties['display_position'] | undefined }))
  return placePromotions(
    naturalRows,
    fixedPackagePromotions.value,
    (rows, item) => {
      const promotedRow = rankApiPackages([item.service], packageModel.value, Number(packageDuration.value))[0]
      return rows.find(row => row.row.service.id === item.service.id)
        ?? (promotedRow ? { row: promotedRow, rank: 0, promotion: undefined, promotionPosition: undefined } : undefined)
    },
    item => item.row.service.id,
  )
})
const freeServiceDisplayRows = computed(() => {
  const naturalRows = freeServices.value.map(service => ({ service, promotion: undefined as ApiServicePromotion | undefined, promotionPosition: undefined as PromotionAnalyticsProperties['display_position'] | undefined }))
  return placePromotions(
    naturalRows,
    freeServicePromotions.value,
    (rows, promotion) => rows.find(item => item.service.id === promotion.service.id)
      ?? { service: promotion.service, promotion: undefined, promotionPosition: undefined },
    item => item.service.id,
  )
})
const firstSlotDate = computed(() => slotQuery.data.value?.items[0]?.key.slice(0, 10) ?? '')
const displayedSlotDate = computed(() => {
  const items = slotQuery.data.value?.items ?? []
  const today = items.filter(item => item.key.startsWith(firstSlotDate.value))
  if (today.some(item => item.state !== 'ended')) return firstSlotDate.value
  return items.find(item => item.key.slice(0, 10) !== firstSlotDate.value)?.key.slice(0, 10) ?? firstSlotDate.value
})
const displayedSlots = computed(() => (slotQuery.data.value?.items ?? []).filter(item => item.key.startsWith(displayedSlotDate.value)))
const selectedSlot = computed(() => displayedSlots.value.find(item => item.key === selectedSlotKey.value))
const rushRows = computed(() => flattenUniqueCursorPages(rushQuery.data.value?.pages))
const isTomorrowPreview = computed(() => Boolean(firstSlotDate.value && displayedSlotDate.value !== firstSlotDate.value))

watch(() => slotQuery.data.value, value => {
  if (!value) return
  const parsed = Date.parse(value.serverNow)
  if (Number.isFinite(parsed)) {
    serverClockOffset.value = parsed - Date.now()
    now.value = Date.now() + serverClockOffset.value
  }
}, { immediate: true })

function selectDisplayedSlot(slots: ApiQuotaSystemSaleSlot[]) {
  if (slots.some(item => item.key === selectedSlotKey.value)) return
  selectedSlotKey.value = slots.find(item => item.state === 'active')?.key
    ?? slots.find(item => item.state !== 'ended')?.key
    ?? slots[0]?.key
    ?? ''
}

watch(displayedSlots, selectDisplayedSlot, { immediate: true })

if (import.meta.server) {
  onServerPrefetch(async () => {
    await slotQuery.suspense()
    selectDisplayedSlot(displayedSlots.value)
    if (selectedSlotKey.value) await rushQuery.suspense()
  })
}

watch(selectedSlotKey, () => {
  refreshedBoundary = ''
})

function setView(value: string | number) {
  activeView.value = apiMarketViewFromQuery(value)
}

function formatSlotDate(slot: ApiQuotaSystemSaleSlot) {
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: 'numeric',
    day: 'numeric',
    weekday: 'short',
  }).format(new Date(slot.startsAt))
}

function slotTime(slot: ApiQuotaSystemSaleSlot) {
  return slot.key.slice(11, 16)
}

function slotStatusLabel(slot: ApiQuotaSystemSaleSlot) {
  if (slot.state === 'active') return '正在抢购'
  if (slot.state === 'ended') return '本场结束'
  if (slot.state === 'registration_closed') return '等待开抢'
  return '即将开抢'
}

function slotCountdown(slot: ApiQuotaSystemSaleSlot) {
  if (slot.state === 'ended') return '00:00:00'
  const target = slot.state === 'active' ? Date.parse(slot.endsAt) : Date.parse(slot.startsAt)
  const seconds = Math.max(Math.ceil((target - now.value) / 1000), 0)
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainingSeconds = seconds % 60
  return [hours, minutes, remainingSeconds].map(value => String(value).padStart(2, '0')).join(':')
}

function countdownPrefix(slot: ApiQuotaSystemSaleSlot) {
  if (slot.state === 'active') return '距结束'
  if (slot.state === 'ended') return '本场已结束'
  return '距开抢'
}

function quotaOfferIconSrc(item: PublicApiQuotaOffer) {
  return getProductIconSrc(`${item.serviceTitle} ${item.name}`, categoryIconByCode.value)
}

function quotaOfferCategory(item: PublicApiQuotaOffer): ConcreteProductCategoryKey {
  return getProductCategory(`${item.serviceTitle} ${item.name}`)
}

function freeServiceIconSrc(service: ApiService) {
  return getApiServiceProductIconSrc(service, categoryIconByCode.value)
}

function freeServiceCategory(service: ApiService): ConcreteProductCategoryKey {
  return getApiServiceProductCategory(service)
}

function freeServiceCard(service: ApiService): ApiFreeServiceCardData {
  const category = freeServiceCategory(service)
  return {
    title: service.title,
    delivery: service.delivery,
    models: service.models,
    category,
    categoryLabel: getProductCategoryLabel(category),
    iconSrc: freeServiceIconSrc(service),
    cnyPerUsdAllowance: service.cnyPerUsdAllowance || '1',
    minimumPurchaseCny: service.minimumPurchaseCny,
    availableUsdAllowance: service.availableUsdAllowance ?? String(service.balance),
    quotaUsagePolicy: service.quotaUsagePolicy,
    maximumPurchaseCny: service.maxBuy,
    multiplier: service.rate,
    declaredMaxConcurrency: service.declaredMaxConcurrency ?? '—',
    promptAuditEnabled: service.promptAuditEnabled ?? null,
    paymentWindowMinutes: service.expectedResponseMinutes,
    merchantName: getApiMerchantDisplayName(service),
    merchantType: service.merchantType,
    expiresAt: service.expiresAt,
    accountPoolLabel: service.accountPoolLabel ?? '',
    merchantRefundCommitment: Boolean(service.merchantRefundCommitment),
    merchantBadges: getApiMerchantBadges(service),
    sellerReputation: service.sellerReputation,
    actionHref: `/api-market/${service.id}`,
  }
}

function promotionAnalytics(item: ApiServicePromotion, position: PromotionAnalyticsProperties['display_position']): PromotionAnalyticsProperties {
  return {
    placement: item.placement,
    display_position: position,
    provider_category: getApiServiceProductCategory(item.service),
    billing_mode: item.service.billingMode,
    target_type: 'api_service',
    source_route: '/api-market',
  }
}

function setPromotedElement(element: Element | ComponentPublicInstance | null, item?: ApiServicePromotion, position?: PromotionAnalyticsProperties['display_position']) {
  if (!item || !position) return
  const domElement = typeof Element !== 'undefined' && element instanceof Element ? element : null
  setPromotionElement(domElement, item.promotionId, promotionAnalytics(item, position))
}

function trackPromotedCardClick(item?: ApiServicePromotion, position?: PromotionAnalyticsProperties['display_position']) {
  if (item && position) trackPromotionClick(promotionAnalytics(item, position))
}

async function purchaseOffer(offer: PublicApiQuotaOffer) {
  if (!offer.isOrderable || pendingOfferId.value) return
  pendingOfferId.value = offer.id
  try {
    const order = await createOrderMutation.mutateAsync({
      offerId: offer.id,
      saleRoundId: offer.saleMode === 'scheduled' ? offer.currentRound?.id : undefined,
    })
    toast.success('额度包订单已创建，请在付款截止前完成站外付款。')
    await router.push(`/my/api-orders/${order.id}`)
  } catch (error) {
    toast.error(apiQuotaOfferErrorMessage(error))
    await Promise.all([slotQuery.refetch(), rushQuery.refetch(), quotaQuery.refetch()])
  } finally {
    pendingOfferId.value = ''
  }
}

function refreshAtSlotBoundary() {
  const slot = selectedSlot.value
  if (!slot || slot.state === 'ended') return
  const target = slot.state === 'active' ? Date.parse(slot.endsAt) : Date.parse(slot.startsAt)
  const signature = `${slot.key}:${slot.state}`
  if (now.value < target || refreshedBoundary === signature) return
  refreshedBoundary = signature
  void Promise.all([slotQuery.refetch(), rushQuery.refetch()])
}

onMounted(() => {
  stopPackageDefaultWatch = watch([activeView, packageServices, packageModelOptions], ([view, services, modelOptions]) => {
    if (view !== 'packages' || packageModel.value || packageDuration.value) return
    const selection = getDefaultApiPackageFilter(services, new Set(modelOptions.map(model => model.id)))
    if (!selection) return
    packageModel.value = selection.modelCatalogId
    packageDuration.value = String(selection.durationDays)
  }, { immediate: true })
  timer = setInterval(() => {
    now.value = Date.now() + serverClockOffset.value
    refreshAtSlotBoundary()
  }, 1000)
})

onBeforeUnmount(() => {
  stopPackageDefaultWatch?.()
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="api-market-catalog space-y-5">
    <header class="flex flex-col gap-3 border-b border-border pb-4 lg:flex-row lg:items-end lg:justify-between">
      <div>
        <div class="flex items-center gap-2 text-sm font-medium text-primary"><Code2 class="h-4 w-4" />API 额度市场</div>
        <h1 class="mt-2 text-2xl font-semibold tracking-normal md:text-3xl">短期额度包与自由额度</h1>
        <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">额度来自卖家实际控制的站外中转系统。平台记录商品和订单，不提供额度、不代理 API 流量，也不验证上游余额。</p>
      </div>
      <RouterLink to="/api-market/quota/new" class="w-full sm:w-auto">
        <Button class="h-11 w-full gap-2 sm:h-9"><PackagePlus class="h-4 w-4" />发布限时额度包</Button>
      </RouterLink>
    </header>

    <Tabs :model-value="activeView" @update:model-value="setView">
      <TabsList class="api-market-view-tabs grid h-11 w-full max-w-xl grid-cols-3">
        <TabsTrigger class="min-h-11" value="limited">限时额度包</TabsTrigger>
        <TabsTrigger class="min-h-11" value="packages">限时流量包</TabsTrigger>
        <TabsTrigger class="min-h-11" value="free">自由额度</TabsTrigger>
      </TabsList>

      <TabsContent value="limited" class="mt-4 space-y-4">
        <section class="overflow-hidden border-y border-border bg-card">
          <div class="flex flex-col gap-3 border-b border-border px-4 py-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <div class="flex items-center gap-2 text-sm font-semibold text-primary"><Zap class="h-4 w-4" />今日限时抢</div>
              <h2 class="mt-1 text-xl font-semibold">{{ isTomorrowPreview && selectedSlot ? `明日 ${slotTime(selectedSlot)} 场预告` : selectedSlot ? `${formatSlotDate(selectedSlot)} 固定场次` : '北京时间固定场次' }}</h2>
              <p class="mt-1 text-xs text-muted-foreground">每天 09:00、13:00、20:00 开抢，每场持续 30 分钟。</p>
            </div>
            <div v-if="selectedSlot" class="min-w-[180px] sm:text-right">
              <div class="text-xs text-muted-foreground">{{ countdownPrefix(selectedSlot) }}</div>
              <div class="mt-1 font-mono text-3xl font-semibold tabular-nums tracking-normal">{{ slotCountdown(selectedSlot) }}</div>
              <Badge class="mt-2" :variant="selectedSlot.state === 'active' ? 'verified' : selectedSlot.state === 'ended' ? 'secondary' : 'trust'">{{ slotStatusLabel(selectedSlot) }}</Badge>
            </div>
          </div>

          <div class="p-4">
            <ErrorState v-if="slotQuery.error.value" description="固定场次暂时无法加载。" @retry="slotQuery.refetch()" />
            <SkeletonBlock v-else-if="slotQuery.isLoading.value" :lines="3" />
            <template v-else>
              <Tabs v-model="selectedSlotKey">
                <TabsList class="api-market-slot-tabs grid h-auto w-full grid-cols-3">
                  <TabsTrigger v-for="slot in displayedSlots" :key="slot.key" :value="slot.key" class="min-h-12 flex-col gap-0.5">
                    <span class="font-mono text-base tabular-nums">{{ slotTime(slot) }}</span>
                    <span class="text-[10px]">{{ slotStatusLabel(slot) }}</span>
                  </TabsTrigger>
                </TabsList>
              </Tabs>

              <ErrorState v-if="rushQuery.error.value && !rushHasLoadedPages" class="mt-4" description="本场额度包暂时无法加载。" @retry="rushQuery.refetch()" />
              <SkeletonBlock v-else-if="rushQuery.isLoading.value || !selectedSlotKey" class="mt-4" :lines="5" />
              <div v-else-if="rushRows.length === 0" class="mt-4 flex min-h-32 flex-col items-center justify-center border-t border-dashed border-border px-4 py-5 text-center">
                <CalendarClock class="h-7 w-7 text-muted-foreground" />
                <div class="mt-2 font-medium">本场暂无额度包</div>
                <p class="mt-1 text-sm text-muted-foreground">可以切换其他场次，或发布自己的限时额度包。</p>
                <div class="mt-3 flex flex-wrap justify-center gap-2">
                  <Button class="h-11 sm:h-9" size="sm" variant="outline" @click="activeView = 'free'">查看自由额度</Button>
                  <RouterLink to="/api-market/quota/new"><Button class="h-11 sm:h-9" size="sm">发布额度包</Button></RouterLink>
                </div>
              </div>
              <div v-else class="api-product-card-grid mt-4">
                <ApiQuotaOfferCard
                  v-for="item in rushRows"
                  :key="item.id"
                  :offer="item"
                  :category="quotaOfferCategory(item)"
                  :category-label="getProductCategoryLabel(quotaOfferCategory(item))"
                  :icon-src="quotaOfferIconSrc(item)"
                  :now="now"
                  variant="rush"
                  :pending-offer-id="pendingOfferId"
                  @purchase="purchaseOffer"
                >
                  <template #health><ApiServiceHealthPanel :summary="item.healthSummary" /></template>
                </ApiQuotaOfferCard>
              </div>
              <InfiniteScrollSentinel
                v-if="selectedSlotKey && !(rushQuery.error.value && !rushHasLoadedPages)"
                :has-more="Boolean(rushQuery.hasNextPage.value)"
                :loading="rushQuery.isFetchingNextPage.value"
                :error="rushQuery.isFetchNextPageError.value"
                @load="rushQuery.fetchNextPage()"
                @retry="rushQuery.fetchNextPage()"
              />
            </template>
          </div>
        </section>

        <div class="flex items-center justify-between gap-3 pt-1">
          <div><h2 class="font-semibold">其他限时额度包</h2><p class="text-xs text-muted-foreground">连续销售和卖家自定义轮次。</p></div>
        </div>
        <div class="api-market-filter-toolbar grid gap-3 rounded-lg border border-border bg-muted/25 p-3 lg:grid-cols-[minmax(240px,1fr)_180px_150px_auto] lg:items-center">
          <label class="relative block">
            <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="quotaSearch" class="h-11 pl-9 lg:h-9" placeholder="搜索额度包、服务或卖家" />
          </label>
          <Select v-model="distributionSystem">
            <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="接入系统" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部接入系统</SelectItem>
              <SelectItem value="sub2api">Sub2API</SelectItem>
              <SelectItem value="new_api_proxy">NewAPI</SelectItem>
              <SelectItem value="other">其他接入</SelectItem>
            </SelectContent>
          </Select>
          <Select v-model="availability">
            <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="销售状态" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部状态</SelectItem>
              <SelectItem value="available">当前可购买</SelectItem>
            </SelectContent>
          </Select>
          <label class="flex min-h-11 cursor-pointer items-center gap-2 rounded-md border border-border bg-background px-3 text-sm">
            <Checkbox v-model="oneMultiplier" />仅看 1.00x
          </label>
        </div>

        <ErrorState v-if="quotaQuery.error.value && !quotaHasLoadedPages" description="额度包列表暂时无法加载。" @retry="quotaQuery.refetch()" />
        <SkeletonBlock v-else-if="quotaQuery.isLoading.value" :lines="8" />
        <EmptyState v-else-if="quotaRows.length === 0 && !quotaQuery.hasNextPage.value" class="min-h-32 p-5" title="暂无匹配的额度包" description="可以调整筛选、查看自由额度，卖家也可以发布自己的限时额度包。">
          <template #action>
            <div class="flex flex-wrap justify-center gap-2">
              <RouterLink to="/api-market/quota/new"><Button class="h-11 gap-2 sm:h-9"><PackagePlus class="h-4 w-4" />发布限时额度包</Button></RouterLink>
              <Button class="h-11 sm:h-9" variant="outline" @click="activeView = 'free'">查看自由额度</Button>
            </div>
          </template>
        </EmptyState>

        <div v-else class="api-product-card-grid">
          <ApiQuotaOfferCard
            v-for="item in quotaRows"
            :key="item.id"
            :offer="item"
            :category="quotaOfferCategory(item)"
            :category-label="getProductCategoryLabel(quotaOfferCategory(item))"
            :icon-src="quotaOfferIconSrc(item)"
            :now="now"
            :pending-offer-id="pendingOfferId"
            @purchase="purchaseOffer"
          >
            <template #health><ApiServiceHealthPanel :summary="item.healthSummary" /></template>
          </ApiQuotaOfferCard>
        </div>
        <InfiniteScrollSentinel
          v-if="!(quotaQuery.error.value && !quotaHasLoadedPages)"
          :has-more="Boolean(quotaQuery.hasNextPage.value)"
          :loading="quotaQuery.isFetchingNextPage.value"
          :error="quotaQuery.isFetchNextPageError.value"
          @load="quotaQuery.fetchNextPage()"
          @retry="quotaQuery.fetchNextPage()"
        />
      </TabsContent>

      <TabsContent value="packages" class="mt-4 space-y-4">
        <Alert>
          <PackageOpen />
          <AlertTitle>限时流量包</AlertTitle>
          <AlertDescription>固定价格购买商户声明的面板额度，套餐有效期从商户提交交付时开始计算。先按精确模型和有效期筛选，再比较综合推荐结果；平台测量只代表当前探测模型与平台节点。</AlertDescription>
        </Alert>
        <ErrorState v-if="freeServicesQuery.error.value && !servicesHaveLoadedPages" description="限时流量包暂时无法加载。" @retry="freeServicesQuery.refetch()" />
        <SkeletonBlock v-else-if="freeServicesQuery.isLoading.value" :lines="6" />
        <template v-else>
          <div class="grid gap-3 border-y border-border py-3 md:grid-cols-2">
            <label class="grid gap-1.5 text-xs font-medium text-muted-foreground">
              精确模型
              <Select v-model="packageModel">
                <SelectTrigger><SelectValue placeholder="请选择模型" /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="model in packageModelOptions" :key="model.id" :value="model.id">{{ model.name }}</SelectItem>
                </SelectContent>
              </Select>
            </label>
            <label class="grid gap-1.5 text-xs font-medium text-muted-foreground">
              套餐有效期
              <Select v-model="packageDuration">
                <SelectTrigger><SelectValue placeholder="请选择有效期" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">1 天</SelectItem>
                  <SelectItem value="3">3 天</SelectItem>
                  <SelectItem value="7">7 天</SelectItem>
                  <SelectItem value="30">30 天</SelectItem>
                </SelectContent>
              </Select>
            </label>
          </div>
          <EmptyState v-if="!packageReady" title="先选择精确模型和有效期" description="选择完成后才会展示可购买套餐和综合推荐顺序。" />
          <EmptyState v-else-if="packageDisplayRows.length === 0" title="暂无匹配的限时流量包" description="当前模型和有效期下没有可购买库存。" />
          <div v-else class="api-product-card-grid">
            <div
              v-for="entry in packageDisplayRows"
              :key="entry.row.package.id"
              class="min-w-0"
              :ref="element => setPromotedElement(element, entry.promotion, entry.promotionPosition)"
            >
              <ApiPackageCard
                :row="entry.row"
                :rank="entry.rank"
                :product-icon-src="freeServiceIconSrc(entry.row.service)"
                :promoted="Boolean(entry.promotion)"
                @activate="trackPromotedCardClick(entry.promotion, entry.promotionPosition)"
              >
                <template #health><ApiServiceHealthPanel :summary="entry.row.service.healthSummary" /></template>
              </ApiPackageCard>
            </div>
          </div>
          <InfiniteScrollSentinel
            :has-more="Boolean(freeServicesQuery.hasNextPage.value)"
            :loading="freeServicesQuery.isFetchingNextPage.value"
            :error="freeServicesQuery.isFetchNextPageError.value"
            @load="freeServicesQuery.fetchNextPage()"
            @retry="freeServicesQuery.fetchNextPage()"
          />
        </template>
      </TabsContent>

      <TabsContent value="free" class="mt-4 space-y-4">
        <Alert>
          <Code2 />
          <AlertTitle>自由额度</AlertTitle>
          <AlertDescription>按人民币金额购买卖家声明的美元额度，Sub2API 维持 1.00x 倍率。订单金额和预计额度在服务详情确认；平台测量只代表当前探测模型与平台节点。</AlertDescription>
        </Alert>
        <ErrorState v-if="freeServicesQuery.error.value && !servicesHaveLoadedPages" description="自由额度服务暂时无法加载。" @retry="freeServicesQuery.refetch()" />
        <SkeletonBlock v-else-if="freeServicesQuery.isLoading.value" :lines="8" />
        <EmptyState v-else-if="freeServiceDisplayRows.length === 0" title="暂无自由额度服务" description="当前没有可公开下单的 API 服务。" />
        <div v-else class="api-product-card-grid">
          <div
            v-for="entry in freeServiceDisplayRows"
            :key="entry.service.id"
            class="min-w-0"
            :ref="element => setPromotedElement(element, entry.promotion, entry.promotionPosition)"
          >
            <ApiFreeServiceCard
              :card="freeServiceCard(entry.service)"
              :promoted="Boolean(entry.promotion)"
              @activate="trackPromotedCardClick(entry.promotion, entry.promotionPosition)"
            >
              <template #health><ApiServiceHealthPanel :summary="entry.service.healthSummary" /></template>
            </ApiFreeServiceCard>
          </div>
        </div>
        <InfiniteScrollSentinel
          v-if="!(freeServicesQuery.error.value && !servicesHaveLoadedPages)"
          :has-more="Boolean(freeServicesQuery.hasNextPage.value)"
          :loading="freeServicesQuery.isFetchingNextPage.value"
          :error="freeServicesQuery.isFetchNextPageError.value"
          @load="freeServicesQuery.fetchNextPage()"
          @retry="freeServicesQuery.fetchNextPage()"
        />
      </TabsContent>
    </Tabs>
  </div>
</template>

<style scoped>
.api-market-catalog {
  min-width: 0;
  background: #fff;
}

.api-market-view-tabs {
  border: 1px solid var(--border);
  background: color-mix(in oklab, var(--muted) 72%, var(--card));
}

.api-market-slot-tabs {
  border: 1px solid color-mix(in oklab, var(--primary) 16%, var(--border));
  background: var(--card);
}

.api-product-card-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  width: 100%;
  align-items: stretch;
  gap: 0.75rem;
}

@media (min-width: 760px) {
  .api-product-card-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (min-width: 1100px) {
  .api-product-card-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (min-width: 1360px) {
  .api-product-card-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}
</style>
