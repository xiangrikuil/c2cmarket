<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import ApiPurchasePanel from '@/components/api-service-detail/ApiPurchasePanel.vue'
import ApiServiceDetailsTabs from '@/components/api-service-detail/ApiServiceDetailsTabs.vue'
import ApiServiceHeader from '@/components/api-service-detail/ApiServiceHeader.vue'
import ApiServiceSummary from '@/components/api-service-detail/ApiServiceSummary.vue'
import { Button } from '@/components/ui/button'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Card } from '@/components/ui/card'
import { BackendProblemError } from '@/lib/backendClient'
import {
  createApiOrderFromIntent,
  createApiPurchaseIntent,
  getApiServiceDefaultPaymentMethod,
  type ApiDeliveryMode,
  type ApiOrder,
  type ApiPurchaseIntent,
  type ApiService,
  type ApiServicePackage,
} from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import { useDetailVisibleAnalytics } from '@/composables/useDetailVisibleAnalytics'
import { useApiService, useFavoriteStatus, useMyApiServices, useToggleFavoriteMutation } from '@/queries/useMarketQueries'
import { markMissingQueryAsNotFoundOnServer, prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'
import { useEntitySeo } from '@/composables/useEntitySeo'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const id = computed(() => String(route.params.id ?? ''))
const apiServiceQuery = useApiService(id)
const { data: service, isLoading, error: serviceError, refetch: refetchService } = apiServiceQuery
prefetchQueriesOnServer(apiServiceQuery)
markMissingQueryAsNotFoundOnServer(apiServiceQuery, () => Boolean(service.value))
const { data: ownedServices, isLoading: ownershipLoading } = useMyApiServices(import.meta.client)
const amount = ref(10)
const selectedPackageId = ref('')
const selectedDeliveryMode = ref<ApiDeliveryMode>('api_key_endpoint')
const { data: favoriteStatus } = useFavoriteStatus('api-service', id, import.meta.client)
const toggleFavoriteMutation = useToggleFavoriteMutation()
const favorited = computed(() => Boolean(favoriteStatus.value))
const trackedServiceId = ref('')
const serviceVisible = computed(() => Boolean(service.value?.id))
const serviceMissing = computed(() => serviceError.value instanceof BackendProblemError && serviceError.value.status === 404)
const emptyTitle = computed(() => serviceMissing.value ? 'API 服务暂未公开' : '未找到 API 服务')
const emptyDescription = computed(() => serviceMissing.value
  ? '该服务尚未配置接单设置、已下架，或当前不在公开 API 集市展示。'
  : '该服务不存在、已下架，或当前不可接单。')
const ownerPreview = computed(() => route.query.preview === 'owner')
const isOwnedService = computed(() => Boolean(ownedServices.value?.some(item => item.id === id.value)))
const availablePackages = computed(() => (service.value?.packages ?? []).filter(item => item.enabled && item.stockAvailable > 0))
const selectedPackage = computed<ApiServicePackage | null>(() => availablePackages.value.find(item => item.id === selectedPackageId.value) ?? null)

useEntitySeo({
  indexable: computed(() => Boolean(service.value)),
  title: computed(() => service.value ? `${service.value.title}｜API 服务｜C2CMarket` : 'API 服务详情｜C2CMarket'),
  description: computed(() => service.value ? `${service.value.title}，支持 ${service.value.models.join('、')}，最低 ¥${service.value.minimumPurchaseCny} 起，查看交付方式与商户说明。` : '查看公开 API 服务详情。'),
  schema: computed(() => service.value ? {
    '@type': 'Service',
    name: service.value.title,
    serviceType: 'API Service',
    offers: {
      '@type': 'Offer',
      priceCurrency: 'CNY',
      price: service.value.minimumPurchaseCny,
      availability: service.value.publiclyOrderable ? 'https://schema.org/InStock' : 'https://schema.org/OutOfStock',
    },
  } : null),
})

useDetailVisibleAnalytics({
  enabled: serviceVisible,
  entityType: 'api_service',
  sourceRoute: analyticsSourceRoute,
})

function apiServiceAnalyticsProps(value: ApiService) {
  return {
    source_route: analyticsSourceRoute(),
    models_text: value.models.join(' '),
    billing_mode: value.billingMode,
    delivery_mode: value.deliveryModes[0],
    minimum_purchase_cny: value.minimumPurchaseCny,
  }
}

watch([service, () => route.query.package], ([value, packageQuery]) => {
  if (!value) return
  selectedDeliveryMode.value = value.deliveryModes[0] ?? 'api_key_endpoint'
  if (value.billingMode === 'fixed_package') {
    const requestedId = typeof packageQuery === 'string' ? packageQuery : ''
    const requested = availablePackages.value.find(item => item.id === requestedId)
    const nextPackage = requested ?? availablePackages.value[0]
    selectedPackageId.value = nextPackage?.id ?? ''
    amount.value = nextPackage?.priceCny ?? value.minimumPurchaseCny
    return
  }
  selectedPackageId.value = ''
  amount.value = value.minimumPurchaseCny
}, { immediate: true })

watch(selectedPackageId, value => {
  if (service.value?.billingMode !== 'fixed_package' || !value) return
  const item = availablePackages.value.find(row => row.id === value)
  if (!item) return
  amount.value = item.priceCny
  if (route.query.package !== value) router.replace({ query: { ...route.query, package: value } })
})

watch([isOwnedService, ownerPreview], ([owned, preview]) => {
  if (!owned || preview) return
  router.replace({ name: 'my-api-service-detail', params: { id: id.value } })
}, { immediate: true })

watch(service, value => {
  if (!value || trackedServiceId.value === value.id) return
  trackedServiceId.value = value.id
  trackAnalytics('api_service_detail_view', apiServiceAnalyticsProps(value))
}, { immediate: true })

const createOrderMutation = useMutation({
  mutationFn: async () => {
    if (!service.value) throw new Error('API 服务不存在。')
    const paymentMethod = getApiServiceDefaultPaymentMethod(service.value)
    if (!paymentMethod) throw new Error('商户尚未配置可用的微信或支付宝收款方式。')
    if (service.value.billingMode === 'fixed_package' && !selectedPackage.value) throw new Error('请选择有库存的限时流量包。')
    const intent = await createApiPurchaseIntent({
      serviceId: service.value.id,
      purchaseAmountCny: amount.value,
      deliveryMode: selectedDeliveryMode.value,
      targetModel: selectedPackage.value?.models[0]?.modelName ?? service.value.models[0],
      selectedPackageId: selectedPackage.value?.id,
    })
    const order = await createApiOrderFromIntent(intent.id, paymentMethod)
    return { intent, order }
  },
  onSuccess(result) {
    const { intent, order } = result
    queryClient.setQueriesData<ApiPurchaseIntent[]>({ queryKey: ['my-api-purchase-intents'] }, old => old ? [intent, ...old.filter(item => item.id !== intent.id)] : old)
    queryClient.setQueriesData<ApiPurchaseIntent[]>({ queryKey: ['merchant-api-purchase-intents'] }, old => old && old.some(item => item.merchantId === intent.merchantId) ? [intent, ...old.filter(item => item.id !== intent.id)] : old)
    queryClient.setQueriesData<ApiPurchaseIntent[]>({ queryKey: ['api-purchase-intents'] }, old => old ? [intent, ...old.filter(item => item.id !== intent.id)] : old)
    queryClient.setQueriesData<ApiOrder[]>({ queryKey: ['my-api-orders'] }, old => old ? [order, ...old.filter(item => item.id !== order.id)] : old)
    queryClient.setQueryData(['api-orders', 'buyer', order.id], order)
    queryClient.invalidateQueries({ queryKey: ['my-api-purchase-intents'] })
    queryClient.invalidateQueries({ queryKey: ['merchant-api-purchase-intents'] })
    queryClient.invalidateQueries({ queryKey: ['api-purchase-intents'] })
    queryClient.invalidateQueries({ queryKey: ['my-api-orders'] })
    queryClient.invalidateQueries({ queryKey: ['merchant-api-orders'] })
    queryClient.invalidateQueries({ queryKey: ['api-orders'] })
    queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    queryClient.invalidateQueries({ queryKey: ['api-order-notifications'] })
    queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
    if (service.value) {
      trackAnalytics('api_purchase_intent_create_success', {
        ...apiServiceAnalyticsProps(service.value),
        delivery_mode: selectedDeliveryMode.value,
        purchase_amount_cny: amount.value,
      })
    }
    toast.success('订单已创建，请查看商户收款方式并在倒计时内完成付款。')
    router.push(`/my/api-orders/${order.id}`)
  },
  onError(error) {
    toast.error(error instanceof Error ? error.message : '创建订单失败。')
  },
})

function toggleFavorite() {
  if (!service.value) return
  toggleFavoriteMutation.mutate({ targetType: 'api-service', targetId: service.value.id }, {
    onSuccess(data) {
      trackAnalytics('favorite_toggle', {
        source_route: analyticsSourceRoute(),
        entity_type: 'api_service',
        action: data.favorited ? 'add' : 'remove',
      })
      toast.success(data.favorited ? '已收藏该 API 服务。' : '已取消收藏。')
    },
    onError(error) {
      toast.error(error instanceof Error ? error.message : '操作失败')
    },
  })
}

function createOrder() {
  createOrderMutation.mutate()
}
</script>

<template>
  <SkeletonBlock v-if="isLoading" :lines="8" />
  <ErrorState v-else-if="serviceError" description="API 服务详情暂时无法加载。" @retry="refetchService()" />
  <EmptyState v-else-if="!service" :title="emptyTitle" :description="emptyDescription">
    <template #action><RouterLink to="/api-market"><Button variant="outline">返回 API 市场</Button></RouterLink></template>
  </EmptyState>
  <div v-else class="api-service-detail-page space-y-4">
    <ApiServiceHeader :service="service" />

    <div class="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,65fr)_minmax(340px,35fr)] lg:items-start">
      <ApiServiceSummary :service="service" />

      <Card v-if="ownershipLoading" class="p-5 text-sm text-muted-foreground">
        正在确认当前账号的服务视角…
      </Card>

      <Card v-else-if="isOwnedService" class="p-5">
        <h2 class="font-semibold">商户预览模式</h2>
        <p class="mt-2 text-sm text-muted-foreground">这是买家看到的公开服务内容。商户不能为自己的服务创建订单。</p>
        <RouterLink :to="`/my/api-services/${service.id}`">
          <Button class="mt-4 w-full">返回服务管理</Button>
        </RouterLink>
      </Card>

      <ApiPurchasePanel
        v-else
        v-model:amount="amount"
        v-model:selected-package-id="selectedPackageId"
        :service="service"
        :submitting="createOrderMutation.isPending.value"
        :favorited="favorited"
        @toggle-favorite="toggleFavorite"
        @confirm="createOrder"
      />
    </div>

    <ApiServiceDetailsTabs :service="service" />
  </div>
</template>
