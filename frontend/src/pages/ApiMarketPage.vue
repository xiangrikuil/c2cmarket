<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch, type ComponentPublicInstance } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CalendarClock, Clock3, Code2, Gauge, PackageOpen, PackagePlus, Search, ShoppingCart, Zap } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import ApiFreeServiceCard from '@/components/api-market/ApiFreeServiceCard.vue'
import ApiPackageCard from '@/components/api-market/ApiPackageCard.vue'
import type { ApiFreeServiceCardData } from '@/components/api-market/apiFreeServiceCard'
import { usePromotionImpression, type PromotionAnalyticsProperties } from '@/composables/usePromotionImpression'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  getApiMerchantDisplayName,
  getApiQuotaDeliveryModeLabel,
  getApiQuotaDistributionLabel,
  getApiQuotaSaleModeLabel,
  getApiTTFTBandLabel,
  type ApiService,
  type ApiQuotaDistributionSystem,
  type ApiQuotaOfferFilters,
  type ApiQuotaSystemSaleSlot,
  type PublicApiQuotaOffer,
} from '@/lib/api'
import {
  apiMarketViewFromQuery,
  apiQuotaDurationLabel,
  apiQuotaOfferCountdown,
  apiQuotaOfferErrorMessage,
  withApiMarketViewQuery,
  type ApiMarketView,
} from '@/lib/apiQuotaOfferUi'
import { rankApiPackages } from '@/lib/apiPackageRecommendation'
import { getApiMerchantBadges } from '@/lib/apiMerchantBadges'
import type { ApiServicePromotion } from '@/lib/apiMarketBackend'
import { placePromotions, promotionsForBillingMode } from '@/lib/apiPromotionPlacement'
import { formatDecimal } from '@/lib/decimal'
import {
  getProductCategory,
  getProductCategoryLabel,
  type ConcreteProductCategoryKey,
} from '@/lib/productCategories'
import { getApiServiceProductCategory, getApiServiceProductIconSrc, getProductIconSrc } from '@/lib/productCategoryIcon'
import { useApiPromotions, useApiQuotaOffers, useApiQuotaSaleSlots, useApiServices, useCreateApiQuotaOrderMutation } from '@/queries/useMarketQueries'
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
const now = ref(Date.now())
const serverClockOffset = ref(0)
const selectedSlotKey = ref('')
const pendingOfferId = ref('')
let refreshedBoundary = ''
let timer: ReturnType<typeof setInterval> | undefined

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
}))
const quotaQuery = useApiQuotaOffers(quotaFilters)
const slotQuery = useApiQuotaSaleSlots()
const rushFilters = computed<ApiQuotaOfferFilters>(() => ({ slotKey: selectedSlotKey.value }))
const rushQuery = useApiQuotaOffers(rushFilters)
const freeServicesQuery = useApiServices({ online: true })
const promotionQuery = useApiPromotions()
const productCategoriesQuery = useProductCategories()
const { data: catalogCategories } = productCategoriesQuery
const createOrderMutation = useCreateApiQuotaOrderMutation()
const { setPromotionElement, trackPromotionClick } = usePromotionImpression()
prefetchQueriesOnServer(quotaQuery, slotQuery, freeServicesQuery, productCategoriesQuery)
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))

const quotaRows = computed(() => {
  const keyword = quotaSearch.value.trim().toLowerCase()
  const rows = (quotaQuery.data.value ?? []).filter(item => !item.currentRound?.systemSlotKey && !item.nextRound?.systemSlotKey)
  if (!keyword) return rows
  return rows.filter(item => [item.name, item.serviceTitle, item.sellerDisplayName, getApiQuotaDistributionLabel(item.distributionSystem)]
    .some(value => value.toLowerCase().includes(keyword)))
})
const freeServices = computed(() => (freeServicesQuery.data.value ?? []).filter(service => service.billingMode !== 'fixed_package'))
const packageServices = computed(() => (freeServicesQuery.data.value ?? []).filter(service => service.billingMode === 'fixed_package'))
const packageModelOptions = computed(() => {
  const options = new Map<string, string>()
  for (const service of packageServices.value) {
    for (const item of service.packages ?? []) {
      if (!item.enabled || item.stockAvailable <= 0) continue
      for (const model of item.models) options.set(model.modelCatalogId, model.modelName)
    }
  }
  return [...options.entries()]
    .map(([id, name]) => ({ id, name }))
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
const packageReady = computed(() => Boolean(packageModel.value && packageDuration.value))
const firstSlotDate = computed(() => slotQuery.data.value?.items[0]?.key.slice(0, 10) ?? '')
const displayedSlotDate = computed(() => {
  const items = slotQuery.data.value?.items ?? []
  const today = items.filter(item => item.key.startsWith(firstSlotDate.value))
  if (today.some(item => item.state !== 'ended')) return firstSlotDate.value
  return items.find(item => item.key.slice(0, 10) !== firstSlotDate.value)?.key.slice(0, 10) ?? firstSlotDate.value
})
const displayedSlots = computed(() => (slotQuery.data.value?.items ?? []).filter(item => item.key.startsWith(displayedSlotDate.value)))
const selectedSlot = computed(() => displayedSlots.value.find(item => item.key === selectedSlotKey.value))
const rushRows = computed(() => rushQuery.data.value ?? [])
const isTomorrowPreview = computed(() => Boolean(firstSlotDate.value && displayedSlotDate.value !== firstSlotDate.value))

watch(() => slotQuery.data.value, value => {
  if (!value) return
  const parsed = Date.parse(value.serverNow)
  if (Number.isFinite(parsed)) {
    serverClockOffset.value = parsed - Date.now()
    now.value = Date.now() + serverClockOffset.value
  }
}, { immediate: true })

watch(displayedSlots, slots => {
  if (slots.some(item => item.key === selectedSlotKey.value)) return
  selectedSlotKey.value = slots.find(item => item.state === 'active')?.key
    ?? slots.find(item => item.state !== 'ended')?.key
    ?? slots[0]?.key
    ?? ''
}, { immediate: true })

watch(selectedSlotKey, () => {
  refreshedBoundary = ''
})

function setView(value: string | number) {
  activeView.value = apiMarketViewFromQuery(value)
}

function formatAbsoluteTime(value: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '时间待确认'
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: 'Asia/Shanghai',
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(parsed)
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

function offerCountdown(item: PublicApiQuotaOffer) {
  return apiQuotaOfferCountdown(item, now.value)
}

function offerRemainingCopies(item: PublicApiQuotaOffer) {
  if (item.saleMode === 'scheduled') return `本轮剩余 ${item.availableCopies} 份`
  return `剩余 ${item.availableCopies} 份`
}

function pricePerUsd(item: PublicApiQuotaOffer) {
  return formatDecimal(item.cnyPerUsd, 3, 6)
}

function multiplierLabel(item: PublicApiQuotaOffer) {
  return `${Number(item.modelMultiplier).toFixed(2)}x`
}

function sellerTypeLabel(item: PublicApiQuotaOffer) {
  return item.sellerIdentityType === 'merchant' ? '商户' : '个人卖家'
}

function offerStatusVariant(item: PublicApiQuotaOffer) {
  if (item.isOrderable) return 'verified'
  if (item.orderabilityCode === 'not_started') return 'trust'
  if (item.orderabilityCode === 'sold_out') return 'secondary'
  return 'status'
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
    maximumPurchaseCny: service.maxBuy,
    multiplier: service.rate,
    ttftLabel: getApiTTFTBandLabel(service.declaredTtftBand),
    declaredMaxConcurrency: service.declaredMaxConcurrency ?? '—',
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
  timer = setInterval(() => {
    now.value = Date.now() + serverClockOffset.value
    refreshAtSlotBoundary()
  }, 1000)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="space-y-5">
    <header class="flex flex-col gap-3 border-b border-border pb-4 lg:flex-row lg:items-end lg:justify-between">
      <div>
        <div class="flex items-center gap-2 text-sm font-medium text-primary"><Code2 class="h-4 w-4" />API 额度市场</div>
        <h1 class="mt-2 text-2xl font-semibold tracking-normal md:text-3xl">短期额度包与自由额度</h1>
        <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">额度来自卖家实际控制的站外中转系统。平台记录商品和订单，不提供额度、不代理 API 流量，也不验证上游余额。</p>
      </div>
      <RouterLink to="/api-market/quota/new">
        <Button class="gap-2"><PackagePlus class="h-4 w-4" />发布限时额度包</Button>
      </RouterLink>
    </header>

    <Tabs :model-value="activeView" @update:model-value="setView">
      <TabsList class="grid h-10 w-full max-w-xl grid-cols-3">
        <TabsTrigger value="limited">限时额度包</TabsTrigger>
        <TabsTrigger value="packages">限时流量包</TabsTrigger>
        <TabsTrigger value="free">自由额度</TabsTrigger>
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
                <TabsList class="grid h-auto w-full grid-cols-3">
                  <TabsTrigger v-for="slot in displayedSlots" :key="slot.key" :value="slot.key" class="min-h-12 flex-col gap-0.5">
                    <span class="font-mono text-base tabular-nums">{{ slotTime(slot) }}</span>
                    <span class="text-[10px]">{{ slotStatusLabel(slot) }}</span>
                  </TabsTrigger>
                </TabsList>
              </Tabs>

              <ErrorState v-if="rushQuery.error.value" class="mt-4" description="本场额度包暂时无法加载。" @retry="rushQuery.refetch()" />
              <SkeletonBlock v-else-if="rushQuery.isLoading.value || !selectedSlotKey" class="mt-4" :lines="5" />
              <div v-else-if="rushRows.length === 0" class="mt-4 flex min-h-36 flex-col items-center justify-center border-t border-dashed border-border px-4 text-center">
                <CalendarClock class="h-7 w-7 text-muted-foreground" />
                <div class="mt-2 font-medium">本场暂无额度包</div>
                <p class="mt-1 text-sm text-muted-foreground">可以切换其他场次，或发布自己的限时额度包。</p>
              </div>
              <div v-else class="mt-4 grid items-stretch gap-3 xl:grid-cols-3">
                <Card
                  v-for="item in rushRows"
                  :key="item.id"
                  class="quota-offer-card gap-0 overflow-hidden py-0"
                  :data-category="quotaOfferCategory(item)"
                >
                  <img
                    v-if="quotaOfferIconSrc(item)"
                    :src="quotaOfferIconSrc(item) ?? undefined"
                    alt=""
                    aria-hidden="true"
                    class="quota-offer-watermark"
                  />
                  <div class="relative z-[1] flex h-full flex-col">
                    <div class="p-4 pb-3">
                      <div class="flex items-start gap-3">
                        <span class="quota-offer-icon-well">
                          <img v-if="quotaOfferIconSrc(item)" :src="quotaOfferIconSrc(item) ?? undefined" alt="" class="h-6 w-6 object-contain" />
                          <PackageOpen v-else class="h-5 w-5" />
                        </span>
                        <div class="min-w-0 flex-1">
                          <div class="flex flex-wrap items-center gap-1.5">
                            <Badge variant="outline" class="quota-offer-category">{{ getProductCategoryLabel(quotaOfferCategory(item)) }}</Badge>
                            <Badge :variant="offerStatusVariant(item)">{{ item.isOrderable ? '正在抢购' : item.orderabilityReason }}</Badge>
                          </div>
                          <h3 class="mt-2 break-words text-base font-semibold">{{ item.name }}</h3>
                          <p class="mt-1 break-words text-xs text-muted-foreground">{{ item.serviceTitle }} · {{ getApiQuotaDistributionLabel(item.distributionSystem) }}</p>
                        </div>
                      </div>
                      <div class="mt-4 flex flex-wrap items-end justify-between gap-2">
                        <div>
                          <div class="quota-offer-price text-3xl font-semibold">¥{{ formatDecimal(item.priceCny, 2, 2) }}</div>
                          <div class="mt-1 text-xs text-muted-foreground">一次购买 · ${{ formatDecimal(item.usdAllowance, 0, 6) }} 美元额度</div>
                        </div>
                        <div class="text-right text-xs text-muted-foreground">¥{{ pricePerUsd(item) }} / $1</div>
                      </div>
                    </div>

                    <dl class="quota-offer-metrics grid grid-cols-2 gap-px text-sm sm:grid-cols-4">
                      <div><dt>模型倍率</dt><dd>{{ multiplierLabel(item) }}</dd></div>
                      <div><dt>首字响应</dt><dd>{{ getApiTTFTBandLabel(item.declaredTtftBand) }}</dd></div>
                      <div><dt>最大并发</dt><dd>{{ item.declaredMaxConcurrency }}</dd></div>
                      <div><dt>预计交付</dt><dd>≤ {{ item.deliveryEtaMinutes }} 分钟</dd></div>
                    </dl>

                    <div class="flex-1 p-4">
                      <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
                        <div class="min-w-0"><dt class="text-xs text-muted-foreground">卖家</dt><dd class="mt-0.5 break-words font-medium">{{ item.sellerDisplayName }}</dd><dd class="text-xs text-muted-foreground">{{ sellerTypeLabel(item) }}</dd></div>
                        <div class="min-w-0"><dt class="text-xs text-muted-foreground">本轮库存</dt><dd class="mt-0.5 font-medium">{{ item.availableCopies }} 份</dd><dd class="text-xs text-muted-foreground">{{ item.sellerLinuxDoBound ? '已绑定 linux.do' : '未绑定 linux.do' }}</dd></div>
                        <div class="min-w-0"><dt class="text-xs text-muted-foreground">交付方式</dt><dd class="mt-0.5 break-words font-medium">{{ getApiQuotaDeliveryModeLabel(item.deliveryMode) }}</dd></div>
                        <div class="min-w-0"><dt class="text-xs text-muted-foreground">额度有效期</dt><dd class="mt-0.5 break-words font-medium">{{ formatAbsoluteTime(item.expiresAt) }}</dd><dd class="text-xs text-muted-foreground">约剩 {{ apiQuotaDurationLabel(item.expiresAt, now) }}</dd></div>
                      </dl>
                    </div>

                    <div class="border-t border-border p-4">
                      <Button class="h-10 w-full" :disabled="!item.isOrderable || Boolean(pendingOfferId)" @click="purchaseOffer(item)">
                        <ShoppingCart class="h-4 w-4" />{{ pendingOfferId === item.id ? '正在创建订单...' : item.isOrderable ? `立即抢购 ¥${formatDecimal(item.priceCny, 2, 2)}` : item.orderabilityReason }}
                      </Button>
                      <p class="mt-2 flex items-start gap-1.5 text-xs leading-5 text-muted-foreground"><Gauge class="mt-0.5 h-3.5 w-3.5 shrink-0" />{{ item.performanceDisclaimer }}</p>
                    </div>
                  </div>
                </Card>
              </div>
            </template>
          </div>
        </section>

        <div class="flex items-center justify-between gap-3 pt-1">
          <div><h2 class="font-semibold">其他限时额度包</h2><p class="text-xs text-muted-foreground">连续销售和卖家自定义轮次。</p></div>
        </div>
        <div class="grid gap-3 border-y border-border py-3 lg:grid-cols-[minmax(240px,1fr)_180px_150px_auto] lg:items-center">
          <label class="relative block">
            <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="quotaSearch" class="pl-9" placeholder="搜索额度包、服务或卖家" />
          </label>
          <Select v-model="distributionSystem">
            <SelectTrigger><SelectValue placeholder="接入系统" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部接入系统</SelectItem>
              <SelectItem value="sub2api">Sub2API</SelectItem>
              <SelectItem value="new_api_proxy">NewAPI</SelectItem>
              <SelectItem value="other">其他接入</SelectItem>
            </SelectContent>
          </Select>
          <Select v-model="availability">
            <SelectTrigger><SelectValue placeholder="销售状态" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部状态</SelectItem>
              <SelectItem value="available">当前可购买</SelectItem>
            </SelectContent>
          </Select>
          <label class="flex min-h-10 cursor-pointer items-center gap-2 rounded-md border border-border px-3 text-sm">
            <Checkbox v-model="oneMultiplier" />仅看 1.00x
          </label>
        </div>

        <ErrorState v-if="quotaQuery.error.value" description="额度包列表暂时无法加载。" @retry="quotaQuery.refetch()" />
        <SkeletonBlock v-else-if="quotaQuery.isLoading.value" :lines="8" />
        <EmptyState v-else-if="quotaRows.length === 0" title="暂无匹配的额度包" description="可以调整筛选、查看自由额度，卖家也可以发布自己的限时额度包。">
          <template #action>
            <div class="flex flex-wrap justify-center gap-2">
              <RouterLink to="/api-market/quota/new"><Button class="gap-2"><PackagePlus class="h-4 w-4" />发布限时额度包</Button></RouterLink>
              <Button variant="outline" @click="activeView = 'free'">查看自由额度</Button>
            </div>
          </template>
        </EmptyState>

        <div v-else class="grid items-stretch gap-4 xl:grid-cols-3">
          <Card
            v-for="item in quotaRows"
            :key="item.id"
            class="quota-offer-card gap-0 overflow-hidden py-0"
            :data-category="quotaOfferCategory(item)"
          >
            <img
              v-if="quotaOfferIconSrc(item)"
              :src="quotaOfferIconSrc(item) ?? undefined"
              alt=""
              aria-hidden="true"
              class="quota-offer-watermark"
            />
            <div class="relative z-[1] flex h-full flex-col">
              <div class="p-4 pb-3">
                <div class="flex items-start gap-3">
                  <span class="quota-offer-icon-well">
                    <img v-if="quotaOfferIconSrc(item)" :src="quotaOfferIconSrc(item) ?? undefined" alt="" class="h-6 w-6 object-contain" />
                    <PackageOpen v-else class="h-5 w-5" />
                  </span>
                  <div class="min-w-0 flex-1">
                    <div class="flex flex-wrap items-center gap-1.5">
                      <Badge variant="outline" class="quota-offer-category">{{ getProductCategoryLabel(quotaOfferCategory(item)) }}</Badge>
                      <Badge :variant="offerStatusVariant(item)">{{ item.isOrderable ? '可购买' : item.orderabilityReason }}</Badge>
                      <Badge variant="secondary">{{ getApiQuotaSaleModeLabel(item.saleMode) }}</Badge>
                    </div>
                    <h2 class="mt-2 break-words text-base font-semibold">{{ item.name }}</h2>
                    <p class="mt-1 break-words text-xs text-muted-foreground">{{ item.serviceTitle }} · {{ getApiQuotaDistributionLabel(item.distributionSystem) }}</p>
                  </div>
                </div>
                <div class="mt-4 flex flex-wrap items-end justify-between gap-2">
                  <div>
                    <div class="quota-offer-price text-3xl font-semibold">¥{{ formatDecimal(item.priceCny, 2, 2) }}</div>
                    <div class="mt-1 text-xs text-muted-foreground">一次购买 · ${{ formatDecimal(item.usdAllowance, 0, 6) }} 美元额度</div>
                  </div>
                  <div class="text-right text-xs text-muted-foreground">¥{{ pricePerUsd(item) }} / $1</div>
                </div>
              </div>

              <dl class="quota-offer-metrics grid grid-cols-2 gap-px text-sm sm:grid-cols-4">
                <div><dt>模型倍率</dt><dd>{{ multiplierLabel(item) }}</dd></div>
                <div><dt>首字响应</dt><dd>{{ getApiTTFTBandLabel(item.declaredTtftBand) }}</dd></div>
                <div><dt>最大并发</dt><dd>{{ item.declaredMaxConcurrency }}</dd></div>
                <div><dt>预计交付</dt><dd>≤ {{ item.deliveryEtaMinutes }} 分钟</dd></div>
              </dl>

              <div class="flex-1 p-4">
                <dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
                  <div class="min-w-0"><dt class="text-xs text-muted-foreground">卖家</dt><dd class="mt-0.5 break-words font-medium">{{ item.sellerDisplayName }}</dd><dd class="text-xs text-muted-foreground">{{ sellerTypeLabel(item) }} · {{ item.sellerLinuxDoBound ? '已绑定 linux.do' : '未绑定 linux.do' }}</dd></div>
                  <div class="min-w-0"><dt class="text-xs text-muted-foreground">剩余库存</dt><dd class="mt-0.5 font-medium">{{ offerRemainingCopies(item) }}</dd></div>
                  <div class="min-w-0"><dt class="text-xs text-muted-foreground">交付方式</dt><dd class="mt-0.5 break-words font-medium">{{ getApiQuotaDeliveryModeLabel(item.deliveryMode) }}</dd></div>
                  <div class="min-w-0"><dt class="text-xs text-muted-foreground">销售时间</dt><dd class="mt-0.5 break-words font-medium">{{ formatAbsoluteTime(item.saleCutoffAt) }}</dd><dd class="min-h-5 font-mono text-xs tabular-nums text-muted-foreground">{{ offerCountdown(item) }}</dd></div>
                </dl>
                <div class="mt-3 flex gap-2 border-t border-border pt-3 text-xs leading-5 text-muted-foreground">
                  <Clock3 class="mt-0.5 h-3.5 w-3.5 shrink-0" />
                  <span>额度有效至 <strong class="font-medium text-foreground">{{ formatAbsoluteTime(item.expiresAt) }}</strong> · 约剩 {{ apiQuotaDurationLabel(item.expiresAt, now) }}</span>
                </div>
              </div>

              <div class="border-t border-border p-4">
                <Button :disabled="!item.isOrderable || Boolean(pendingOfferId)" class="h-10 w-full" @click="purchaseOffer(item)">
                  <ShoppingCart class="h-4 w-4" />{{ pendingOfferId === item.id ? '正在创建订单...' : item.isOrderable ? `立即抢购 ¥${formatDecimal(item.priceCny, 2, 2)}` : item.orderabilityReason }}
                </Button>
                <p class="mt-2 flex items-start gap-1.5 text-xs leading-5 text-muted-foreground"><Gauge class="mt-0.5 h-3.5 w-3.5 shrink-0" />{{ item.performanceDisclaimer }}<span v-if="item.performanceConfirmedAt"> · {{ formatAbsoluteTime(item.performanceConfirmedAt) }} 确认</span></p>
              </div>
            </div>
          </Card>
        </div>
      </TabsContent>

      <TabsContent value="packages" class="mt-4 space-y-4">
        <Alert>
          <PackageOpen />
          <AlertTitle>限时流量包</AlertTitle>
          <AlertDescription>固定价格购买商户声明的面板额度，套餐有效期从商户提交交付时开始计算。先按精确模型和有效期筛选，再比较综合推荐结果；性能由商户自报，平台未测速。</AlertDescription>
        </Alert>
        <ErrorState v-if="freeServicesQuery.error.value" description="限时流量包暂时无法加载。" @retry="freeServicesQuery.refetch()" />
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
          <div v-else class="api-service-card-grid">
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
              />
            </div>
          </div>
        </template>
      </TabsContent>

      <TabsContent value="free" class="mt-4 space-y-4">
        <Alert>
          <Code2 />
          <AlertTitle>自由额度</AlertTitle>
          <AlertDescription>按人民币金额购买卖家声明的美元额度，Sub2API 维持 1.00x 倍率。订单金额和预计额度在服务详情确认；价格、额度与性能由商户声明，平台未测速。</AlertDescription>
        </Alert>
        <ErrorState v-if="freeServicesQuery.error.value" description="自由额度服务暂时无法加载。" @retry="freeServicesQuery.refetch()" />
        <SkeletonBlock v-else-if="freeServicesQuery.isLoading.value" :lines="8" />
        <EmptyState v-else-if="freeServiceDisplayRows.length === 0" title="暂无自由额度服务" description="当前没有可公开下单的 API 服务。" />
        <div v-else class="quota-free-grid">
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
            />
          </div>
        </div>
      </TabsContent>
    </Tabs>
  </div>
</template>

<style scoped>
.quota-offer-card {
  --quota-accent: #64748b;

  position: relative;
  isolation: isolate;
  border-radius: 0.5rem;
  border-color: color-mix(in oklab, var(--quota-accent) 28%, var(--border));
  background-color: color-mix(in oklab, var(--quota-accent) 4%, var(--card));
  box-shadow: inset 0 3px 0 color-mix(in oklab, var(--quota-accent) 72%, transparent);
  transition: border-color 150ms ease, box-shadow 150ms ease;
}

.quota-offer-card:hover {
  border-color: color-mix(in oklab, var(--quota-accent) 48%, var(--border));
  box-shadow:
    inset 0 3px 0 color-mix(in oklab, var(--quota-accent) 82%, transparent),
    0 8px 24px rgb(15 23 42 / 0.06);
}

.quota-free-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 330px), 1fr));
  width: 100%;
  max-width: 1640px;
  margin-inline: auto;
  align-items: start;
  gap: 1rem;
}

.quota-offer-card[data-category='gpt'] {
  --quota-accent: #7c3aed;
}

.quota-offer-card[data-category='claude'] {
  --quota-accent: #dc5f45;
}

.quota-offer-card[data-category='gemini'] {
  --quota-accent: #2563eb;
}

.quota-offer-card[data-category='cursor'] {
  --quota-accent: #0891b2;
}

.quota-offer-card[data-category='perplexity'] {
  --quota-accent: #059669;
}

.quota-offer-card[data-category='other'] {
  --quota-accent: #64748b;
}

.quota-offer-icon-well {
  display: grid;
  width: 2.5rem;
  height: 2.5rem;
  flex: none;
  place-items: center;
  border: 1px solid color-mix(in oklab, var(--quota-accent) 28%, var(--border));
  border-radius: 0.5rem;
  color: var(--quota-accent);
  background-color: color-mix(in oklab, var(--quota-accent) 10%, var(--card));
}

.quota-offer-category {
  border-color: color-mix(in oklab, var(--quota-accent) 36%, var(--border));
  color: var(--quota-accent);
  background-color: color-mix(in oklab, var(--quota-accent) 8%, var(--card));
}

.quota-offer-price {
  color: var(--quota-accent);
}

.quota-offer-watermark {
  position: absolute;
  top: 3.75rem;
  right: -1rem;
  z-index: 0;
  width: 7.5rem;
  height: 7.5rem;
  object-fit: contain;
  opacity: 0.055;
  pointer-events: none;
}

.quota-offer-metrics {
  border-block: 1px solid color-mix(in oklab, var(--quota-accent) 16%, var(--border));
  background-color: color-mix(in oklab, var(--quota-accent) 14%, var(--border));
}

.quota-offer-metrics > div {
  min-width: 0;
  padding: 0.75rem;
  background-color: color-mix(in oklab, var(--quota-accent) 3%, var(--card));
}

.quota-offer-metrics dt {
  font-size: 0.75rem;
  color: var(--muted-foreground);
}

.quota-offer-metrics dd {
  margin-top: 0.25rem;
  overflow-wrap: anywhere;
  font-weight: 600;
}

@media (max-width: 639px) {
  .quota-free-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
