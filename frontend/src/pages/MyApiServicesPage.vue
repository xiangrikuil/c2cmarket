<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { PackagePlus } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import CompactStats from '@/components/market/CompactStats.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { usePagination } from '@/composables/usePagination'
import {
  getApiMerchantDisplayName,
  getApiMerchantVisibilityLabel,
  getApiServicePublicDetailUrl,
  type ApiService,
} from '@/lib/api'
import { getApiServiceProductIconSrc } from '@/lib/productCategoryIcon'
import {
  useMyApiServices,
  usePauseApiServiceMutation,
  usePublishApiServiceMutation,
  useResumeApiServiceMutation,
} from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'

const { data: apiServices, isLoading } = useMyApiServices()
const { data: catalogCategories } = useProductCategories()
const route = useRoute()
const publishMutation = usePublishApiServiceMutation()
const pauseMutation = usePauseApiServiceMutation()
const resumeMutation = useResumeApiServiceMutation()
const rows = computed(() => apiServices.value ?? [])
const quotaPublishIntent = computed(() => route.query.intent === 'quota')
const publishServiceRoute = computed(() => quotaPublishIntent.value ? '/api-market/quota/new' : '/api-market/new')
const pagination = usePagination(rows)
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))

const stats = computed(() => [
  { label: '全部服务', value: rows.value.length },
  { label: '在线', value: rows.value.filter(item => item.online).length },
  { label: '审核中', value: rows.value.filter(item => item.state === 'reviewing').length },
  { label: '已暂停', value: rows.value.filter(item => item.state === 'paused').length },
])

function statusLabel(item: ApiService) {
  if (item.online) return '在线'
  if (item.state === 'reviewing') return '审核中'
  if (item.state === 'paused') return '暂停'
  return '离线'
}

function statusVariant(item: ApiService) {
  if (item.online) return 'default'
  if (item.state === 'reviewing' || item.state === 'paused') return 'secondary'
  return 'outline'
}

function serviceIconSrc(item: ApiService) {
  return getApiServiceProductIconSrc(item, categoryIconByCode.value)
}

function publishService(id: string) {
  publishMutation.mutate(id, {
    onSuccess: () => toast.success('API 服务已上线。'),
    onError: error => toast.error(error instanceof Error ? error.message : '上线失败。'),
  })
}

function pauseService(id: string) {
  pauseMutation.mutate(id, {
    onSuccess: () => toast.success('API 服务已暂停。'),
    onError: error => toast.error(error instanceof Error ? error.message : '暂停失败。'),
  })
}

function resumeService(id: string) {
  resumeMutation.mutate(id, {
    onSuccess: () => toast.success('API 服务已恢复上线。'),
    onError: error => toast.error(error instanceof Error ? error.message : '恢复失败。'),
  })
}
</script>

<template>
  <div class="space-y-4">
    <PageTitle
      :title="quotaPublishIntent ? '选择 API 服务' : '我的 API 服务'"
      :description="quotaPublishIntent ? '限时额度包需要关联一个 API 服务。选择服务后会直接进入额度包发布区。' : '管理自己发布的 API 服务、公开状态、展示身份和限时额度包。'"
      :action-text="quotaPublishIntent ? '发布 API 服务并继续' : '发布 API 服务'"
      :action-to="publishServiceRoute"
    />

    <CompactStats :items="stats" :loading="isLoading" />

    <SkeletonTable v-if="isLoading" :rows="5" :columns="5" />
    <EmptyState v-else-if="rows.length === 0" title="先发布一个 API 服务" description="限时额度包需要关联 API 服务，以复用接入方式、收款规则和卖家资料。"><template #action><RouterLink :to="publishServiceRoute"><Button>{{ quotaPublishIntent ? '发布 API 服务并继续' : '发布 API 服务' }}</Button></RouterLink></template></EmptyState>

    <SoftTable v-else :columns="['服务', '对外商家名', '可售额度', '状态', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td>
          <div class="flex min-w-0 items-center gap-2.5">
            <span class="grid h-9 w-9 shrink-0 place-items-center rounded-md border border-border bg-white">
              <img v-if="serviceIconSrc(item)" :src="serviceIconSrc(item) ?? undefined" alt="" class="h-5 w-5 object-contain" />
              <PackagePlus v-else class="h-4 w-4 text-muted-foreground" />
            </span>
            <div class="min-w-0">
              <div class="truncate font-medium">{{ item.title }}</div>
              <div class="text-xs text-muted-foreground"><ShortId :value="item.id" prefix="API-SVC" /> · {{ item.delivery }}</div>
            </div>
          </div>
        </td>
        <td>
          <div>{{ getApiMerchantDisplayName(item) }}</div>
          <div class="text-xs text-muted-foreground">{{ getApiMerchantVisibilityLabel(item) }}</div>
        </td>
        <td class="font-semibold">可售 ${{ item.balance }}</td>
        <td><Badge :variant="statusVariant(item)">{{ statusLabel(item) }}</Badge><div class="mt-1 text-xs text-muted-foreground"><LocalTime :value="item.lastOnlineConfirmedAt" /></div></td>
        <td>
          <div class="flex flex-wrap gap-2">
            <Button v-if="item.state === 'offline'" size="sm" @click="publishService(item.id)">上线</Button>
            <Button v-if="item.online" size="sm" variant="outline" @click="pauseService(item.id)">暂停</Button>
            <Button v-if="item.state === 'paused'" size="sm" variant="outline" @click="resumeService(item.id)">恢复</Button>
            <RouterLink :to="quotaPublishIntent ? `/api-market/quota/new?serviceId=${item.id}` : `/my/api-services/${item.id}#quota-offers`">
              <Button size="sm" class="gap-2"><PackagePlus class="h-4 w-4" />{{ quotaPublishIntent ? '选择并发布额度包' : '高级管理' }}</Button>
            </RouterLink>
            <RouterLink v-if="getApiServicePublicDetailUrl(item)" :to="`${getApiServicePublicDetailUrl(item)}?preview=owner`">
              <Button size="sm" variant="outline">公开预览</Button>
            </RouterLink>
          </div>
        </td>
      </tr>
      <template #footer>
        <TablePagination
          v-model:page="pagination.page.value"
          :page-count="pagination.pageCount.value"
          :total="pagination.total.value"
          :start-item="pagination.startItem.value"
          :end-item="pagination.endItem.value"
        />
      </template>
    </SoftTable>
  </div>
</template>
