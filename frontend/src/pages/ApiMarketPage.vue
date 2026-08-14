<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, onServerPrefetch, ref, watch, type ComponentPublicInstance } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CalendarClock, Code2, Filter, PackageOpen, PackagePlus, Search, Zap } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import InfiniteScrollSentinel from '@/components/market/InfiniteScrollSentinel.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import ApiFreeServiceCard from '@/components/api-market/ApiFreeServiceCard.vue'
import ApiPackageCard from '@/components/api-market/ApiPackageCard.vue'
import ApiQuotaOfferCard from '@/components/api-market/ApiQuotaOfferCard.vue'
import ApiServiceHealthPanel from '@/components/api-market/ApiServiceHealthPanel.vue'
import ApiMarketActiveFilters from '@/components/api-market/ApiMarketActiveFilters.vue'
import type { ApiFreeServiceCardData } from '@/components/api-market/apiFreeServiceCard'
import { usePromotionImpression, type PromotionAnalyticsProperties } from '@/composables/usePromotionImpression'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  getApiMerchantDisplayName,
  type ApiService,
  type ApiServiceFilters,
  type ApiQuotaDistributionSystem,
  type ApiQuotaOfferFilters,
  type ApiQuotaSaleMode,
  type ApiQuotaSystemSaleSlot,
  type PublicApiQuotaOffer,
} from '@/lib/api'
import {
  apiMarketViewFromQuery,
  apiQuotaOfferErrorMessage,
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
import { useApiPromotions, useApiQuotaSaleSlots, useCreateApiQuotaOrderMutation, useInfiniteApiQuotaOffers, useInfiniteApiServices, useModelCatalog, useMyProfileQuery } from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'
import { prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'
import { CAPABILITY, hasCapability } from '@/lib/capabilities'

type AvailabilityFilter = 'all' | 'available'
type SaleModeFilter = ApiQuotaSaleMode | 'all'
type ActiveFilterItem = { key: string, label: string }
type LimitedSort = 'updated_desc' | 'unit_price_asc' | 'allowance_desc' | 'delivery_asc'
type PackageSort = 'recommended' | 'package_price_asc'
type FreeSort = 'updated_desc' | 'price_asc' | 'minimum_purchase_asc'

const route = useRoute()
const router = useRouter()
const { data: myProfile } = useMyProfileQuery(import.meta.client)
const activeView = ref<ApiMarketView>(apiMarketViewFromQuery(route.query.view))
const canPublishQuota = computed(() => hasCapability(myProfile.value, CAPABILITY.apiQuotaPublish))
const canPublishApiService = computed(() => hasCapability(myProfile.value, CAPABILITY.apiServicePublish))
const canPublishCurrentView = computed(() => activeView.value === 'limited' ? canPublishQuota.value : canPublishApiService.value)
const marketSearch = ref('')
const distributionSystem = ref<ApiQuotaDistributionSystem | 'all'>('all')
const availability = ref<AvailabilityFilter>('available')
const limitedMultiplierMax = ref('')
const limitedSaleMode = ref<SaleModeFilter>('all')
const limitedSort = ref<LimitedSort>('updated_desc')
const packageModel = ref('')
const packageDuration = ref('')
const packagePriceMax = ref('')
const packageMultiplierMax = ref('')
const packageSort = ref<PackageSort>('recommended')
const freePriceMax = ref('')
const freeMinimumPurchaseMax = ref('')
const freeSort = ref<FreeSort>('updated_desc')
const packageDefaultsDismissed = ref(false)
const mobileFiltersOpen = ref(false)
const packageReady = computed(() => Boolean(packageModel.value && packageDuration.value))
const now = ref(Date.now())
const serverClockOffset = ref(0)
const selectedSlotKey = ref('')
const pendingOfferId = ref('')
let refreshedBoundary = ''
let timer: ReturnType<typeof setInterval> | undefined
let stopPackageDefaultWatch: (() => void) | undefined
let pendingMarketRouteWrites = 0

function useDebouncedFilter(source: typeof marketSearch, delay = 400) {
  const value = ref(source.value)
  let timeout: ReturnType<typeof setTimeout> | undefined
  watch(source, next => {
    if (next === value.value) return
    if (timeout) clearTimeout(timeout)
    timeout = setTimeout(() => {
      value.value = next
    }, delay)
  })
  onBeforeUnmount(() => {
    if (timeout) clearTimeout(timeout)
  })
  return {
    value,
    sync(next: string) {
      if (timeout) clearTimeout(timeout)
      value.value = next
    },
  }
}

const { value: debouncedSearch, sync: syncDebouncedSearch } = useDebouncedFilter(marketSearch)
const { value: debouncedLimitedMultiplierMax, sync: syncDebouncedLimitedMultiplierMax } = useDebouncedFilter(limitedMultiplierMax)
const { value: debouncedPackagePriceMax, sync: syncDebouncedPackagePriceMax } = useDebouncedFilter(packagePriceMax)
const { value: debouncedPackageMultiplierMax, sync: syncDebouncedPackageMultiplierMax } = useDebouncedFilter(packageMultiplierMax)
const { value: debouncedFreePriceMax, sync: syncDebouncedFreePriceMax } = useDebouncedFilter(freePriceMax)
const { value: debouncedFreeMinimumPurchaseMax, sync: syncDebouncedFreeMinimumPurchaseMax } = useDebouncedFilter(freeMinimumPurchaseMax)

function queryText(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function numericText(value: unknown) {
  const text = queryText(value)
  if (text === '') return ''
  const parsed = Number(text)
  return Number.isFinite(parsed) && parsed >= 0 ? text : ''
}

function numericFilter(value: unknown) {
  const text = typeof value === 'number' ? String(value) : numericText(value)
  const parsed = Number(text)
  return text !== '' && Number.isFinite(parsed) && parsed >= 0 ? parsed : undefined
}

function applyRouteFilters() {
  const view = apiMarketViewFromQuery(route.query.view)
  const previousView = activeView.value
  activeView.value = view
  marketSearch.value = queryText(route.query.q).slice(0, 100)
  packageModel.value = queryText(route.query.model)
  distributionSystem.value = ['sub2api', 'new_api_proxy', 'other'].includes(queryText(route.query.distribution))
    ? queryText(route.query.distribution) as ApiQuotaDistributionSystem
    : 'all'
  if (view === 'limited') {
    availability.value = queryText(route.query.availability) === 'all' ? 'all' : 'available'
    limitedMultiplierMax.value = numericText(route.query.multiplierMax)
    limitedSaleMode.value = ['continuous', 'scheduled'].includes(queryText(route.query.saleMode))
      ? queryText(route.query.saleMode) as ApiQuotaSaleMode
      : 'all'
    limitedSort.value = ['unit_price_asc', 'allowance_desc', 'delivery_asc'].includes(queryText(route.query.sort))
      ? queryText(route.query.sort) as LimitedSort
      : 'updated_desc'
  } else if (view === 'packages') {
    if (previousView !== 'packages') packageDefaultsDismissed.value = false
    packageDuration.value = ['1', '3', '7', '30'].includes(queryText(route.query.duration)) ? queryText(route.query.duration) : ''
    packagePriceMax.value = numericText(route.query.priceMax)
    packageMultiplierMax.value = numericText(route.query.multiplierMax)
    packageSort.value = queryText(route.query.sort) === 'package_price_asc' ? 'package_price_asc' : 'recommended'
  } else {
    freePriceMax.value = numericText(route.query.priceMax)
    freeMinimumPurchaseMax.value = numericText(route.query.minimumMax)
    freeSort.value = ['price_asc', 'minimum_purchase_asc'].includes(queryText(route.query.sort))
      ? queryText(route.query.sort) as FreeSort
      : 'updated_desc'
  }
  syncDebouncedSearch(marketSearch.value)
  syncDebouncedLimitedMultiplierMax(limitedMultiplierMax.value)
  syncDebouncedPackagePriceMax(packagePriceMax.value)
  syncDebouncedPackageMultiplierMax(packageMultiplierMax.value)
  syncDebouncedFreePriceMax(freePriceMax.value)
  syncDebouncedFreeMinimumPurchaseMax(freeMinimumPurchaseMax.value)
}

watch(() => route.query, () => {
  if (pendingMarketRouteWrites === 0) applyRouteFilters()
}, { deep: true, immediate: true })

const marketQuery = computed<Record<string, string>>(() => {
  const query: Record<string, string> = { view: activeView.value }
  if (debouncedSearch.value.trim()) query.q = debouncedSearch.value.trim()
  if (packageModel.value) query.model = packageModel.value
  if (distributionSystem.value !== 'all' && activeView.value !== 'packages') query.distribution = distributionSystem.value
  if (activeView.value === 'limited') {
    if (availability.value === 'all') query.availability = 'all'
    if (debouncedLimitedMultiplierMax.value) query.multiplierMax = debouncedLimitedMultiplierMax.value
    if (limitedSaleMode.value !== 'all') query.saleMode = limitedSaleMode.value
    if (limitedSort.value !== 'updated_desc') query.sort = limitedSort.value
  } else if (activeView.value === 'packages') {
    if (packageDuration.value) query.duration = packageDuration.value
    if (debouncedPackagePriceMax.value) query.priceMax = debouncedPackagePriceMax.value
    if (debouncedPackageMultiplierMax.value) query.multiplierMax = debouncedPackageMultiplierMax.value
    if (packageSort.value !== 'recommended') query.sort = packageSort.value
  } else {
    if (debouncedFreePriceMax.value) query.priceMax = debouncedFreePriceMax.value
    if (debouncedFreeMinimumPurchaseMax.value) query.minimumMax = debouncedFreeMinimumPurchaseMax.value
    if (freeSort.value !== 'updated_desc') query.sort = freeSort.value
  }
  return query
})

function routeQueryMatches(query: Record<string, string>) {
  const currentKeys = Object.keys(route.query)
  const nextKeys = Object.keys(query)
  return currentKeys.length === nextKeys.length && nextKeys.every(key => queryText(route.query[key]) === query[key])
}

watch(marketQuery, async query => {
  if (routeQueryMatches(query)) return
  pendingMarketRouteWrites += 1
  try {
    await router.replace({ query })
  } finally {
    pendingMarketRouteWrites -= 1
  }
}, { deep: true, immediate: true })

const quotaFilters = computed<ApiQuotaOfferFilters>(() => ({
  distributionSystem: distributionSystem.value,
  modelCatalogId: packageModel.value || undefined,
  maxMultiplier: numericFilter(debouncedLimitedMultiplierMax.value),
  onlyOrderable: availability.value === 'available',
  saleMode: limitedSaleMode.value,
  search: debouncedSearch.value.trim() || undefined,
  excludeSystemSlots: true,
  sort: limitedSort.value,
}))
const limitedViewEnabled = computed(() => activeView.value === 'limited')
const serviceViewEnabled = computed(() => activeView.value === 'packages' || activeView.value === 'free')
const serviceFilters = computed<ApiServiceFilters>(() => ({
  online: true,
  billingMode: activeView.value === 'packages' ? 'fixed_package' : 'metered_credit',
  search: debouncedSearch.value.trim() || undefined,
  modelCatalogId: activeView.value === 'free' ? packageModel.value || undefined : undefined,
  distributionSystem: activeView.value === 'free' ? distributionSystem.value : undefined,
  maxCnyPerUsd: activeView.value === 'free' ? numericFilter(debouncedFreePriceMax.value) : undefined,
  minimumPurchaseCnyMax: activeView.value === 'free' ? numericFilter(debouncedFreeMinimumPurchaseMax.value) : undefined,
  packageModelCatalogId: activeView.value === 'packages' && packageReady.value ? packageModel.value : undefined,
  packageDurationDays: activeView.value === 'packages' && packageReady.value ? Number(packageDuration.value) : undefined,
  packagePriceCnyMax: activeView.value === 'packages' ? numericFilter(debouncedPackagePriceMax.value) : undefined,
  packageMultiplierMax: activeView.value === 'packages' ? numericFilter(debouncedPackageMultiplierMax.value) : undefined,
  sort: activeView.value === 'packages' ? packageSort.value : freeSort.value,
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

const modelNameByID = computed(() => new Map(packageModelOptions.value.map(item => [item.id, item.name])))
const modelFilterValue = computed({
  get: () => packageModel.value || 'all_models',
  set: (value: string) => {
    packageModel.value = value === 'all_models' ? '' : value
    if (value === 'all_models') packageDefaultsDismissed.value = true
  },
})

function packageRowsFor(services: ApiService[]) {
  const rows = rankApiPackages(services, packageModel.value, Number(packageDuration.value))
    .filter(row => numericFilter(debouncedPackagePriceMax.value) === undefined || row.package.priceCny <= numericFilter(debouncedPackagePriceMax.value)!)
    .filter(row => numericFilter(debouncedPackageMultiplierMax.value) === undefined || row.selectedModel.merchantMultiplier <= numericFilter(debouncedPackageMultiplierMax.value)!)
  if (packageSort.value === 'package_price_asc') {
    rows.sort((left, right) => left.package.priceCny - right.package.priceCny || left.package.id.localeCompare(right.package.id))
  }
  return rows
}

function matchesFreeServiceFilters(service: ApiService) {
  const keyword = debouncedSearch.value.trim().toLowerCase()
  if (keyword && ![service.title, service.merchantDisplayName, ...service.models].some(value => value.toLowerCase().includes(keyword))) return false
  if (packageModel.value && !service.modelPriceRows.some(model => model.modelId === packageModel.value)) return false
  const priceMax = numericFilter(debouncedFreePriceMax.value)
  if (priceMax !== undefined && Number(service.cnyPerUsdAllowance) > priceMax) return false
  const minimumMax = numericFilter(debouncedFreeMinimumPurchaseMax.value)
  if (minimumMax !== undefined && service.minimumPurchaseCny > minimumMax) return false
  if (distributionSystem.value === 'sub2api' && service.delivery !== 'Sub2API') return false
  if ((distributionSystem.value === 'new_api_proxy' || distributionSystem.value === 'other') && service.delivery === 'Sub2API') return false
  return true
}

const packageRows = computed(() => packageRowsFor(packageServices.value))
const fixedPackagePromotions = computed(() => promotionsForBillingMode(
  promotionQuery.data.value ?? [],
  true,
  promotion => packageRowsFor([promotion.service]).length > 0,
))
const freeServicePromotions = computed(() => promotionsForBillingMode(
  promotionQuery.data.value ?? [],
  false,
  promotion => matchesFreeServiceFilters(promotion.service),
))
const packageDisplayRows = computed(() => {
  const naturalRows = packageRows.value.map((row, index) => ({ row, rank: index + 1, promotion: undefined as ApiServicePromotion | undefined, promotionPosition: undefined as PromotionAnalyticsProperties['display_position'] | undefined }))
  return placePromotions(
    naturalRows,
    fixedPackagePromotions.value,
    (rows, item) => {
      const promotedRow = packageRowsFor([item.service])[0]
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

const viewMeta = computed(() => ({
  limited: {
    title: '限量额度包',
    description: '固定额度、固定份数，到期时间明确；可直接购买，也可按固定场次抢购。',
    publishLabel: '发布限量额度包',
    publishTo: '/api-market/quota/new',
  },
  packages: {
    title: '短期流量包',
    description: '按精确模型购买固定套餐，商户提交交付后开始计算 1、3、7 或 30 天有效期。',
    publishLabel: '发布短期流量包',
    publishTo: '/api-market/new?mode=package',
  },
  free: {
    title: '自选额度',
    description: '自选人民币购买金额，按服务单价换算可得美元额度。',
    publishLabel: '发布自选额度',
    publishTo: '/api-market/new?mode=free',
  },
})[activeView.value])

function selectedModelLabel() {
  return modelNameByID.value.get(packageModel.value) ?? packageModel.value
}

const limitedActiveFilters = computed<ActiveFilterItem[]>(() => [
  ...(marketSearch.value.trim() ? [{ key: 'search', label: `关键词：${marketSearch.value.trim()}` }] : []),
  ...(packageModel.value ? [{ key: 'model', label: `模型：${selectedModelLabel()}` }] : []),
  ...(distributionSystem.value !== 'all' ? [{ key: 'distribution', label: `接入：${distributionSystem.value === 'sub2api' ? 'Sub2API' : distributionSystem.value === 'new_api_proxy' ? 'NewAPI' : '其他'}` }] : []),
  ...(availability.value === 'all' ? [{ key: 'availability', label: '包含暂不可购买' }] : []),
  ...(limitedMultiplierMax.value ? [{ key: 'multiplierMax', label: `倍率 ≤ ${limitedMultiplierMax.value}x` }] : []),
  ...(limitedSaleMode.value !== 'all' ? [{ key: 'saleMode', label: limitedSaleMode.value === 'continuous' ? '连续销售' : '按场次销售' }] : []),
])

const packageActiveFilters = computed<ActiveFilterItem[]>(() => [
  ...(marketSearch.value.trim() ? [{ key: 'search', label: `关键词：${marketSearch.value.trim()}` }] : []),
  ...(packageModel.value ? [{ key: 'model', label: `模型：${selectedModelLabel()}` }] : []),
  ...(packageDuration.value ? [{ key: 'duration', label: `有效期：${packageDuration.value} 天` }] : []),
  ...(packagePriceMax.value ? [{ key: 'priceMax', label: `套餐价 ≤ ¥${packagePriceMax.value}` }] : []),
  ...(packageMultiplierMax.value ? [{ key: 'multiplierMax', label: `倍率 ≤ ${packageMultiplierMax.value}x` }] : []),
])

const freeActiveFilters = computed<ActiveFilterItem[]>(() => [
  ...(marketSearch.value.trim() ? [{ key: 'search', label: `关键词：${marketSearch.value.trim()}` }] : []),
  ...(packageModel.value ? [{ key: 'model', label: `模型：${selectedModelLabel()}` }] : []),
  ...(freePriceMax.value ? [{ key: 'priceMax', label: `单价 ≤ ¥${freePriceMax.value} / $1` }] : []),
  ...(freeMinimumPurchaseMax.value ? [{ key: 'minimumMax', label: `起购 ≤ ¥${freeMinimumPurchaseMax.value}` }] : []),
  ...(distributionSystem.value !== 'all' ? [{ key: 'distribution', label: `接入：${distributionSystem.value === 'sub2api' ? 'Sub2API' : distributionSystem.value === 'new_api_proxy' ? 'NewAPI' : '其他'}` }] : []),
])

const currentActiveFilters = computed(() => activeView.value === 'limited'
  ? limitedActiveFilters.value
  : activeView.value === 'packages'
    ? packageActiveFilters.value
    : freeActiveFilters.value)

function removeActiveFilter(key: string) {
  if (key === 'search') marketSearch.value = ''
  if (key === 'model') {
    packageModel.value = ''
    packageDefaultsDismissed.value = true
  }
  if (key === 'distribution') distributionSystem.value = 'all'
  if (key === 'availability') availability.value = 'available'
  if (key === 'saleMode') limitedSaleMode.value = 'all'
  if (key === 'duration') {
    packageDuration.value = ''
    packageDefaultsDismissed.value = true
  }
  if (key === 'priceMax') {
    if (activeView.value === 'packages') packagePriceMax.value = ''
    else freePriceMax.value = ''
  }
  if (key === 'minimumMax') freeMinimumPurchaseMax.value = ''
  if (key === 'multiplierMax') {
    if (activeView.value === 'limited') limitedMultiplierMax.value = ''
    else packageMultiplierMax.value = ''
  }
}

function clearActiveFilters() {
  for (const item of currentActiveFilters.value) removeActiveFilter(item.key)
}

function openMobileFilters() {
  if (import.meta.client && window.matchMedia('(max-width: 1023px)').matches) {
    mobileFiltersOpen.value = true
  }
}

watch(activeView, () => {
  mobileFiltersOpen.value = false
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
    if (limitedViewEnabled.value && selectedSlotKey.value) await rushQuery.suspense()
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
    if (view !== 'packages' || packageDefaultsDismissed.value || packageModel.value || packageDuration.value) return
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
        <div class="flex items-center gap-2 text-sm font-medium text-primary"><Code2 class="h-4 w-4" />API 市场 / {{ viewMeta.title }}</div>
        <h1 class="mt-2 text-2xl font-semibold tracking-normal md:text-3xl">{{ viewMeta.title }}</h1>
        <p class="mt-2 max-w-3xl text-sm leading-6 text-muted-foreground">{{ viewMeta.description }} 额度来自卖家实际控制的站外中转系统，平台不代理 API 流量，也不验证上游余额。</p>
      </div>
      <RouterLink v-if="canPublishCurrentView" :to="viewMeta.publishTo" class="w-full sm:w-auto">
        <Button class="h-11 w-full gap-2 sm:h-9"><PackagePlus class="h-4 w-4" /><span class="hidden sm:inline">{{ viewMeta.publishLabel }}</span><span class="sm:hidden">发布</span></Button>
      </RouterLink>
    </header>

    <Tabs :model-value="activeView" @update:model-value="setView">
      <TabsList class="api-market-view-tabs grid h-11 w-full grid-cols-3 lg:hidden">
        <TabsTrigger class="min-h-11 px-2" value="limited">限量额度包</TabsTrigger>
        <TabsTrigger class="min-h-11 px-2" value="packages">短期流量包</TabsTrigger>
        <TabsTrigger class="min-h-11 px-2" value="free">自选额度</TabsTrigger>
      </TabsList>

      <TabsContent value="limited" class="mt-4 space-y-4">
        <section class="overflow-hidden border-y border-border bg-card">
          <div class="flex flex-col gap-3 border-b border-border px-4 py-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <div class="flex items-center gap-2 text-sm font-semibold text-primary"><Zap class="h-4 w-4" />今日限时抢</div>
              <h2 class="mt-1 text-xl font-semibold">{{ isTomorrowPreview && selectedSlot ? `明日 ${slotTime(selectedSlot)} 场预告` : selectedSlot ? `${formatSlotDate(selectedSlot)} 固定场次` : '北京时间固定场次' }}</h2>
              <p class="mt-1 text-xs text-muted-foreground">每天 20:00 开抢，场次持续 30 分钟。</p>
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
                <p class="mt-1 text-sm text-muted-foreground">可以切换其他场次，或发布自己的限量额度包。</p>
                <div class="mt-3 flex flex-wrap justify-center gap-2">
                  <Button class="h-11 sm:h-9" size="sm" variant="outline" @click="activeView = 'free'">查看自选额度</Button>
                  <RouterLink v-if="canPublishQuota" to="/api-market/quota/new"><Button class="h-11 sm:h-9" size="sm">发布额度包</Button></RouterLink>
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
          <div><h2 class="font-semibold">其他限量额度包</h2><p class="text-xs text-muted-foreground">连续销售和卖家自定义轮次。</p></div>
        </div>
        <div class="api-market-filter-toolbar space-y-3 rounded-md border border-border bg-muted/25 p-3">
          <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] lg:grid-cols-4 lg:items-center xl:grid-cols-8">
          <label class="relative block lg:col-span-2">
            <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input v-model="marketSearch" aria-label="搜索限量额度包" class="h-11 pl-9 lg:h-9" placeholder="搜索额度包、服务或卖家" />
          </label>
          <Button type="button" variant="outline" class="h-11 gap-2 lg:hidden" @click="openMobileFilters">
            <Filter class="h-4 w-4" />筛选 {{ currentActiveFilters.length || '' }}
          </Button>
          <Select v-model="limitedSort">
            <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="排序" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="updated_desc">最新发布</SelectItem>
              <SelectItem value="unit_price_asc">单价最低</SelectItem>
              <SelectItem value="allowance_desc">额度最多</SelectItem>
              <SelectItem value="delivery_asc">交付最快</SelectItem>
            </SelectContent>
          </Select>
          <div class="hidden lg:block">
            <Select v-model="modelFilterValue">
              <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="支持模型" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all_models">全部模型</SelectItem>
                <SelectItem v-for="model in packageModelOptions" :key="model.id" :value="model.id">{{ model.name }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="hidden lg:block">
            <Select v-model="distributionSystem">
              <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="接入系统" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部接入系统</SelectItem>
                <SelectItem value="sub2api">Sub2API</SelectItem>
                <SelectItem value="new_api_proxy">NewAPI</SelectItem>
                <SelectItem value="other">其他接入</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="hidden lg:block">
            <Select v-model="availability">
              <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="销售状态" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="available">当前可购买</SelectItem>
                <SelectItem value="all">全部状态</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="hidden lg:block">
            <Input v-model="limitedMultiplierMax" class="h-11 lg:h-9" type="number" min="0.01" step="0.01" aria-label="最高模型倍率" placeholder="最高倍率，如 1.2" />
          </div>
          <div class="hidden lg:block">
            <Select v-model="limitedSaleMode">
              <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="销售方式" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部销售方式</SelectItem>
                <SelectItem value="continuous">连续销售</SelectItem>
                <SelectItem value="scheduled">按场次销售</SelectItem>
              </SelectContent>
            </Select>
          </div>
          </div>
          <ApiMarketActiveFilters :items="limitedActiveFilters" @remove="removeActiveFilter" @clear="clearActiveFilters" />
        </div>

        <ErrorState v-if="quotaQuery.error.value && !quotaHasLoadedPages" description="额度包列表暂时无法加载。" @retry="quotaQuery.refetch()" />
        <SkeletonBlock v-else-if="quotaQuery.isLoading.value" :lines="8" />
        <EmptyState v-else-if="quotaRows.length === 0 && !quotaQuery.hasNextPage.value" class="min-h-32 p-5" title="暂无匹配的额度包" description="可以调整筛选、查看自选额度，卖家也可以发布自己的限量额度包。">
          <template #action>
            <div class="flex flex-wrap justify-center gap-2">
              <RouterLink v-if="canPublishQuota" to="/api-market/quota/new"><Button class="h-11 gap-2 sm:h-9"><PackagePlus class="h-4 w-4" />发布限量额度包</Button></RouterLink>
              <Button class="h-11 sm:h-9" variant="outline" @click="activeView = 'free'">查看自选额度</Button>
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
          <AlertTitle>短期流量包</AlertTitle>
          <AlertDescription>固定价格购买商户声明的面板额度，套餐有效期从商户提交交付时开始计算。先按精确模型和有效期筛选，再比较综合推荐结果；平台测量只代表当前探测模型与平台节点。</AlertDescription>
        </Alert>
        <div class="api-market-filter-toolbar space-y-3 rounded-md border border-border bg-muted/25 p-3">
          <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] lg:grid-cols-4 lg:items-center xl:grid-cols-7">
            <label class="relative block lg:col-span-2">
              <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input v-model="marketSearch" aria-label="搜索短期流量包" class="h-11 pl-9 lg:h-9" placeholder="搜索套餐、服务或卖家" />
            </label>
            <Button type="button" variant="outline" class="h-11 gap-2 lg:hidden" @click="openMobileFilters">
              <Filter class="h-4 w-4" />筛选 {{ currentActiveFilters.length || '' }}
            </Button>
            <Select v-model="packageSort">
              <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="排序" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="recommended">综合推荐</SelectItem>
                <SelectItem value="package_price_asc">套餐价最低</SelectItem>
              </SelectContent>
            </Select>
            <div class="hidden lg:block">
              <Select v-model="packageModel">
                <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="精确模型" /></SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="model in packageModelOptions" :key="model.id" :value="model.id">{{ model.name }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="hidden lg:block">
              <Select v-model="packageDuration">
                <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="有效期" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">1 天</SelectItem>
                  <SelectItem value="3">3 天</SelectItem>
                  <SelectItem value="7">7 天</SelectItem>
                  <SelectItem value="30">30 天</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="hidden lg:block">
              <Input v-model="packagePriceMax" class="h-11 lg:h-9" type="number" min="0" step="0.01" aria-label="最高套餐价格" placeholder="最高套餐价" />
            </div>
            <div class="hidden lg:block">
              <Input v-model="packageMultiplierMax" class="h-11 lg:h-9" type="number" min="0.01" step="0.01" aria-label="最高套餐倍率" placeholder="最高倍率" />
            </div>
          </div>
          <ApiMarketActiveFilters :items="packageActiveFilters" @remove="removeActiveFilter" @clear="clearActiveFilters" />
        </div>
        <ErrorState v-if="freeServicesQuery.error.value && !servicesHaveLoadedPages" description="短期流量包暂时无法加载。" @retry="freeServicesQuery.refetch()" />
        <SkeletonBlock v-else-if="freeServicesQuery.isLoading.value" :lines="6" />
        <template v-else>
          <EmptyState v-if="!packageReady" title="先选择精确模型和有效期" description="选择完成后才会展示可购买套餐和综合推荐顺序。" />
          <EmptyState v-else-if="packageDisplayRows.length === 0" title="暂无匹配的短期流量包" description="当前条件下没有可购买库存。" />
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
          <AlertTitle>自选额度</AlertTitle>
          <AlertDescription>按人民币金额购买卖家声明的美元额度，Sub2API 维持 1.00x 倍率。订单金额和预计额度在服务详情确认；平台测量只代表当前探测模型与平台节点。</AlertDescription>
        </Alert>
        <div class="api-market-filter-toolbar space-y-3 rounded-md border border-border bg-muted/25 p-3">
          <div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] lg:grid-cols-4 lg:items-center xl:grid-cols-7">
            <label class="relative block lg:col-span-2">
              <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
              <Input v-model="marketSearch" aria-label="搜索自选额度" class="h-11 pl-9 lg:h-9" placeholder="搜索服务、模型或卖家" />
            </label>
            <Button type="button" variant="outline" class="h-11 gap-2 lg:hidden" @click="openMobileFilters">
              <Filter class="h-4 w-4" />筛选 {{ currentActiveFilters.length || '' }}
            </Button>
            <Select v-model="freeSort">
              <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="排序" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="updated_desc">最近更新</SelectItem>
                <SelectItem value="price_asc">单价最低</SelectItem>
                <SelectItem value="minimum_purchase_asc">起购最低</SelectItem>
              </SelectContent>
            </Select>
            <div class="hidden lg:block">
              <Select v-model="modelFilterValue">
                <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="精确模型" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all_models">全部模型</SelectItem>
                  <SelectItem v-for="model in packageModelOptions" :key="model.id" :value="model.id">{{ model.name }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="hidden lg:block">
              <Input v-model="freePriceMax" class="h-11 lg:h-9" type="number" min="0" step="0.01" aria-label="最高额度单价" placeholder="最高单价 / $1" />
            </div>
            <div class="hidden lg:block">
              <Input v-model="freeMinimumPurchaseMax" class="h-11 lg:h-9" type="number" min="0" step="1" aria-label="最高最低起购金额" placeholder="最高起购金额" />
            </div>
            <div class="hidden lg:block">
              <Select v-model="distributionSystem">
                <SelectTrigger class="data-[size=default]:h-11 lg:data-[size=default]:h-9"><SelectValue placeholder="接入系统" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">全部接入系统</SelectItem>
                  <SelectItem value="sub2api">Sub2API</SelectItem>
                  <SelectItem value="new_api_proxy">NewAPI</SelectItem>
                  <SelectItem value="other">其他接入</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <ApiMarketActiveFilters :items="freeActiveFilters" @remove="removeActiveFilter" @clear="clearActiveFilters" />
        </div>
        <ErrorState v-if="freeServicesQuery.error.value && !servicesHaveLoadedPages" description="自选额度服务暂时无法加载。" @retry="freeServicesQuery.refetch()" />
        <SkeletonBlock v-else-if="freeServicesQuery.isLoading.value" :lines="8" />
        <EmptyState v-else-if="freeServiceDisplayRows.length === 0" title="暂无自选额度服务" description="当前条件下没有可公开下单的 API 服务。" />
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

    <Dialog v-model:open="mobileFiltersOpen">
      <DialogContent class="bottom-0 left-0 top-auto max-h-[82dvh] max-w-full translate-x-0 translate-y-0 gap-0 overflow-hidden rounded-b-none rounded-t-2xl p-0 lg:hidden">
        <div class="mx-auto mt-3 h-1 w-10 rounded-full bg-muted" />
        <div class="overflow-y-auto px-4 pb-[calc(1rem+env(safe-area-inset-bottom))] pt-3">
          <DialogHeader class="pr-8 text-left">
            <DialogTitle>{{ viewMeta.title }}筛选</DialogTitle>
            <DialogDescription class="sr-only">{{ viewMeta.title }}的详细筛选条件</DialogDescription>
          </DialogHeader>

          <div v-if="activeView === 'limited'" class="mt-4 grid gap-3">
            <Select v-model="modelFilterValue">
              <SelectTrigger class="data-[size=default]:h-11"><SelectValue placeholder="支持模型" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all_models">全部模型</SelectItem>
                <SelectItem v-for="model in packageModelOptions" :key="model.id" :value="model.id">{{ model.name }}</SelectItem>
              </SelectContent>
            </Select>
            <Select v-model="distributionSystem">
              <SelectTrigger class="data-[size=default]:h-11"><SelectValue placeholder="接入系统" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部接入系统</SelectItem>
                <SelectItem value="sub2api">Sub2API</SelectItem>
                <SelectItem value="new_api_proxy">NewAPI</SelectItem>
                <SelectItem value="other">其他接入</SelectItem>
              </SelectContent>
            </Select>
            <Select v-model="availability">
              <SelectTrigger class="data-[size=default]:h-11"><SelectValue placeholder="销售状态" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="available">当前可购买</SelectItem>
                <SelectItem value="all">全部状态</SelectItem>
              </SelectContent>
            </Select>
            <Input v-model="limitedMultiplierMax" class="h-11" type="number" min="0.01" step="0.01" aria-label="最高模型倍率" placeholder="最高倍率，如 1.2" />
            <Select v-model="limitedSaleMode">
              <SelectTrigger class="data-[size=default]:h-11"><SelectValue placeholder="销售方式" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部销售方式</SelectItem>
                <SelectItem value="continuous">连续销售</SelectItem>
                <SelectItem value="scheduled">按场次销售</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div v-else-if="activeView === 'packages'" class="mt-4 grid gap-3">
            <Select v-model="packageModel">
              <SelectTrigger class="data-[size=default]:h-11"><SelectValue placeholder="精确模型" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="model in packageModelOptions" :key="model.id" :value="model.id">{{ model.name }}</SelectItem>
              </SelectContent>
            </Select>
            <Select v-model="packageDuration">
              <SelectTrigger class="data-[size=default]:h-11"><SelectValue placeholder="有效期" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="1">1 天</SelectItem>
                <SelectItem value="3">3 天</SelectItem>
                <SelectItem value="7">7 天</SelectItem>
                <SelectItem value="30">30 天</SelectItem>
              </SelectContent>
            </Select>
            <Input v-model="packagePriceMax" class="h-11" type="number" min="0" step="0.01" aria-label="最高套餐价格" placeholder="最高套餐价" />
            <Input v-model="packageMultiplierMax" class="h-11" type="number" min="0.01" step="0.01" aria-label="最高套餐倍率" placeholder="最高倍率" />
          </div>

          <div v-else class="mt-4 grid gap-3">
            <Select v-model="modelFilterValue">
              <SelectTrigger class="data-[size=default]:h-11"><SelectValue placeholder="精确模型" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all_models">全部模型</SelectItem>
                <SelectItem v-for="model in packageModelOptions" :key="model.id" :value="model.id">{{ model.name }}</SelectItem>
              </SelectContent>
            </Select>
            <Input v-model="freePriceMax" class="h-11" type="number" min="0" step="0.01" aria-label="最高额度单价" placeholder="最高单价 / $1" />
            <Input v-model="freeMinimumPurchaseMax" class="h-11" type="number" min="0" step="1" aria-label="最高最低起购金额" placeholder="最高起购金额" />
            <Select v-model="distributionSystem">
              <SelectTrigger class="data-[size=default]:h-11"><SelectValue placeholder="接入系统" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部接入系统</SelectItem>
                <SelectItem value="sub2api">Sub2API</SelectItem>
                <SelectItem value="new_api_proxy">NewAPI</SelectItem>
                <SelectItem value="other">其他接入</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <ApiMarketActiveFilters class="mt-4" :items="currentActiveFilters" @remove="removeActiveFilter" @clear="clearActiveFilters" />
          <DialogFooter class="mt-5">
            <Button type="button" class="h-11 w-full" @click="mobileFiltersOpen = false">完成</Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
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
