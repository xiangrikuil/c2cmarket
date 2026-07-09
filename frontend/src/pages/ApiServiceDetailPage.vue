<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import ApiPurchasePanel from '@/components/api-service-detail/ApiPurchasePanel.vue'
import ApiServiceHeader from '@/components/api-service-detail/ApiServiceHeader.vue'
import ApiServiceSummary from '@/components/api-service-detail/ApiServiceSummary.vue'
import MerchantNote from '@/components/api-service-detail/MerchantNote.vue'
import ModelPriceTable from '@/components/api-service-detail/ModelPriceTable.vue'
import ServiceRules from '@/components/api-service-detail/ServiceRules.vue'
import { Button } from '@/components/ui/button'
import { BackendProblemError } from '@/lib/backendClient'
import {
  createApiOrderFromIntent,
  createApiPurchaseIntent,
  getApiServiceDefaultPaymentMethod,
  type ApiDeliveryMode,
  type ApiOrder,
  type ApiPurchaseIntent,
  type ApiService,
} from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import { useDetailVisibleAnalytics } from '@/composables/useDetailVisibleAnalytics'
import { useApiService, useFavoriteStatus, useToggleFavoriteMutation } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const id = computed(() => String(route.params.id ?? ''))
const { data: service, isLoading, error: serviceError } = useApiService(id)
const amount = ref(10)
const selectedDeliveryMode = ref<ApiDeliveryMode>('api_key_endpoint')
const { data: favoriteStatus } = useFavoriteStatus('api-service', id)
const toggleFavoriteMutation = useToggleFavoriteMutation()
const favorited = computed(() => Boolean(favoriteStatus.value))
const trackedServiceId = ref('')
const serviceVisible = computed(() => Boolean(service.value?.id))
const serviceMissing = computed(() => serviceError.value instanceof BackendProblemError && serviceError.value.status === 404)
const emptyTitle = computed(() => serviceMissing.value ? 'API 服务暂未公开' : '未找到 API 服务')
const emptyDescription = computed(() => serviceMissing.value
  ? '该服务尚未配置接单设置、已下架，或当前不在公开 API 集市展示。'
  : '该服务不存在、已下架，或当前不可接单。')

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

watch(service, value => {
  if (!value) return
  amount.value = value.minimumPurchaseCny
  selectedDeliveryMode.value = value.deliveryModes[0] ?? 'api_key_endpoint'
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
    const intent = await createApiPurchaseIntent({
      serviceId: service.value.id,
      purchaseAmountCny: amount.value,
      deliveryMode: selectedDeliveryMode.value,
      targetModel: service.value.models[0],
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
    if (service.value) {
      trackAnalytics('api_purchase_intent_create_success', {
        ...apiServiceAnalyticsProps(service.value),
        delivery_mode: selectedDeliveryMode.value,
        purchase_amount_cny: amount.value,
      })
    }
    toast.success('购买意向已提交，订单已创建，请查看收款资料后付款。')
    router.push(`/my/api-orders/${order.id}`)
  },
  onError(error) {
    toast.error(error instanceof Error ? error.message : '提交购买意向失败。')
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
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载 API 服务详情...</div>
  <div v-else-if="!service" class="rounded-xl border border-border bg-card p-8">
    <h1 class="text-xl font-semibold">{{ emptyTitle }}</h1>
    <p class="mt-2 text-sm text-muted-foreground">{{ emptyDescription }}</p>
    <RouterLink to="/api-market"><Button class="mt-5" variant="outline">返回 API 集市</Button></RouterLink>
  </div>
  <div v-else class="space-y-4">
    <ApiServiceHeader :service="service" :favorited="favorited" @toggle-favorite="toggleFavorite" />
    <ApiServiceSummary :service="service" />

    <div class="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,68fr)_minmax(320px,32fr)] lg:items-start">
      <div class="min-w-0 space-y-4">
        <ServiceRules :service="service" />
        <MerchantNote :service="service" />
        <ModelPriceTable :service="service" />
      </div>

      <ApiPurchasePanel
        v-model:amount="amount"
        v-model:selected-delivery-mode="selectedDeliveryMode"
        :service="service"
        :submitting="createOrderMutation.isPending.value"
        @confirm="createOrder"
      />
    </div>
  </div>
</template>
