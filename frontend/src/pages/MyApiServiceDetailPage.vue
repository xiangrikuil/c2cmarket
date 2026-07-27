<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { toast } from 'vue-sonner'
import ApiQuotaOwnerManager from '@/components/api-quota/ApiQuotaOwnerManager.vue'
import ApiServiceOwnerHeader from '@/components/api-service-owner/ApiServiceOwnerHeader.vue'
import ApiServiceOwnerMetrics from '@/components/api-service-owner/ApiServiceOwnerMetrics.vue'
import ApiServiceOwnerOverview from '@/components/api-service-owner/ApiServiceOwnerOverview.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { getApiServiceProductIconSrc } from '@/lib/productCategoryIcon'
import {
  useMyApiService,
  usePauseApiServiceMutation,
  usePublishApiServiceMutation,
  useResumeApiServiceMutation,
} from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const { data: service, isLoading, error, refetch } = useMyApiService(id)
const { data: catalogCategories } = useProductCategories()
const publishMutation = usePublishApiServiceMutation()
const pauseMutation = usePauseApiServiceMutation()
const resumeMutation = useResumeApiServiceMutation()
const actionPending = computed(() => publishMutation.isPending.value || pauseMutation.isPending.value || resumeMutation.isPending.value)
const errorMessage = computed(() => error.value instanceof Error ? error.value.message : '无法读取这条 API 服务，请确认当前账号是发布者。')
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))
const serviceIconSrc = computed(() => service.value ? getApiServiceProductIconSrc(service.value, categoryIconByCode.value) : null)

function publishService() {
  if (!service.value || actionPending.value) return
  publishMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已上线。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '上线失败。'),
  })
}

function pauseService() {
  if (!service.value || actionPending.value) return
  if (!window.confirm('确认暂停这项 API 服务的接单？暂停后买家将无法创建新订单，已有订单不受影响。')) return
  pauseMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已暂停。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '暂停失败。'),
  })
}

function resumeService() {
  if (!service.value || actionPending.value) return
  resumeMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已恢复上线。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '恢复失败。'),
  })
}
</script>

<template>
  <SkeletonBlock v-if="isLoading" :lines="8" />

  <ErrorState v-else-if="!service" title="无法打开服务管理页" :description="errorMessage" @retry="refetch()" />

  <main v-else class="mx-auto w-full max-w-[1440px] space-y-5">
    <ApiServiceOwnerHeader
      :service="service"
      :icon-src="serviceIconSrc"
      :action-pending="actionPending"
      @publish="publishService"
      @pause="pauseService"
      @resume="resumeService"
    />

    <ApiServiceOwnerMetrics :service="service" />

    <ApiServiceOwnerOverview :service="service" />

    <ApiQuotaOwnerManager :api-service-id="service.id" :distribution-system="service.delivery" />
  </main>
</template>
