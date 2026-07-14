<script setup lang="ts">
import { computed } from 'vue'
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
import {
  useMyApiServices,
  usePauseApiServiceMutation,
  usePublishApiServiceMutation,
  useResumeApiServiceMutation,
} from '@/queries/useMarketQueries'

const { data: apiServices, isLoading } = useMyApiServices()
const publishMutation = usePublishApiServiceMutation()
const pauseMutation = usePauseApiServiceMutation()
const resumeMutation = useResumeApiServiceMutation()
const rows = computed(() => apiServices.value ?? [])
const pagination = usePagination(rows)

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
      title="我的 API 服务"
      description="管理自己发布的 API 服务、公开状态、展示身份和可售额度。"
      action-text="发布 API 服务"
      action-to="/api-market/new"
    />

    <CompactStats :items="stats" :loading="isLoading" />

    <SkeletonTable v-if="isLoading" :rows="5" :columns="5" />
    <EmptyState v-else-if="rows.length === 0" title="暂未发布 API 服务" description="发布后可在这里管理价格、额度、付款规则和接单状态。"><template #action><RouterLink to="/api-market/new"><Button>发布 API 服务</Button></RouterLink></template></EmptyState>

    <SoftTable v-else :columns="['服务', '对外商家名', '可售额度', '状态', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td>
          <div class="font-medium">{{ item.title }}</div>
          <div class="text-xs text-muted-foreground"><ShortId :value="item.id" prefix="API-SVC" /> · {{ item.delivery }}</div>
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
            <RouterLink :to="`/my/api-services/${item.id}`">
              <Button size="sm" variant="outline">管理</Button>
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
