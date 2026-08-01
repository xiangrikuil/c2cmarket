<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  Eye,
  ListFilter,
  PackagePlus,
  Pause,
  Play,
  Plus,
  Settings2,
  Upload,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import StatusBadge from '@/components/market/StatusBadge.vue'
import {
  apiServiceSalesViewOptions,
  getApiServiceOwnerStatus,
  getApiServiceSalesAvailabilitySummary,
  getApiServiceSalesChannelLabel,
  getApiServiceSalesStatus,
  getApiServiceSalesTimeSummary,
  getInitialApiServiceSalesView,
} from '@/components/api-service-owner/apiServiceOwnerPresentation'
import { usePagination } from '@/composables/usePagination'
import {
  getApiServicePublicDetailUrl,
  type ApiServiceSalesChannel,
  type ApiServiceSalesView,
  type OwnerApiService,
} from '@/lib/api'
import { getApiServiceProductIconSrc } from '@/lib/productCategoryIcon'
import {
  useMyApiServices,
  usePauseApiServiceMutation,
  usePublishApiServiceMutation,
  useResumeApiServiceMutation,
} from '@/queries/useMarketQueries'
import { useProductCategories } from '@/queries/useProductCatalogQueries'

const route = useRoute()
const quotaPublishIntent = computed(() => route.query.intent === 'quota')
// 筛选只按首次入口设置，后续查询参数变化不覆盖卖家手动选择。
const salesView = ref<ApiServiceSalesView>(getInitialApiServiceSalesView(route.query.intent))
const { data: apiServices, error, isLoading, refetch } = useMyApiServices(salesView)
const { data: catalogCategories } = useProductCategories()
const publishMutation = usePublishApiServiceMutation()
const pauseMutation = usePauseApiServiceMutation()
const resumeMutation = useResumeApiServiceMutation()
const rows = computed(() => apiServices.value ?? [])
const publishServiceRoute = computed(() => quotaPublishIntent.value ? '/api-market/quota/new' : '/api-market/new')
const pagination = usePagination(rows)
const categoryIconByCode = computed(() => new Map((catalogCategories.value ?? []).map(category => [category.code, category.iconDataUrl])))
const selectedSalesView = computed(() => (
  apiServiceSalesViewOptions.find(option => option.value === salesView.value)
  ?? apiServiceSalesViewOptions[0]
))
const emptyTitle = computed(() => quotaPublishIntent.value && salesView.value === 'all'
  ? '先发布一个 API 服务'
  : `没有${selectedSalesView.value.label}的服务`)
const emptyDescription = computed(() => {
  if (quotaPublishIntent.value && salesView.value === 'all') {
    return '限时额度包需要关联 API 服务，以复用接入方式、收款规则和卖家资料。'
  }
  if (salesView.value === 'active') {
    return '已过期、暂停、草稿和离线服务仍会保留，可切换筛选继续管理。'
  }
  return '可以切换其他销售状态，或发布新的 API 服务。'
})

function serviceIconSrc(item: OwnerApiService) {
  return getApiServiceProductIconSrc(item, categoryIconByCode.value)
}

function serviceStatus(item: OwnerApiService) {
  return getApiServiceOwnerStatus(item)
}

function channelStatus(channel: ApiServiceSalesChannel) {
  return getApiServiceSalesStatus(channel.state)
}

function hasExpiredLimitedQuota(item: OwnerApiService) {
  return item.salesSummary.channels.some(channel => (
    channel.kind === 'limited_quota' && channel.state === 'expired'
  ))
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
  <div class="min-w-0 space-y-4">
    <PageTitle
      :title="quotaPublishIntent ? '选择 API 服务' : '我的 API 服务'"
      :description="quotaPublishIntent ? '选择一个可复用的 API 服务，继续发布限时额度包。' : '默认查看仍在销售或即将开售的服务，历史状态可通过筛选继续管理。'"
    >
      <template #action>
        <Button as-child class="w-full md:w-auto">
          <RouterLink :to="publishServiceRoute">
            <Plus class="h-4 w-4" />
            {{ quotaPublishIntent ? '发布 API 服务并继续' : '发布 API 服务' }}
          </RouterLink>
        </Button>
      </template>
    </PageTitle>

    <section class="border-y border-border py-3" aria-labelledby="api-service-sales-filter-title">
      <div class="hidden items-center gap-3 md:flex">
        <div id="api-service-sales-filter-title" class="flex shrink-0 items-center gap-2 text-sm text-muted-foreground">
          <ListFilter class="h-4 w-4" />
          销售状态
        </div>
        <div class="flex flex-wrap gap-2" role="group" aria-label="销售状态筛选">
          <Button
            v-for="option in apiServiceSalesViewOptions"
            :key="option.value"
            size="sm"
            :variant="salesView === option.value ? 'default' : 'outline'"
            :aria-pressed="salesView === option.value"
            @click="salesView = option.value"
          >
            {{ option.label }}
          </Button>
        </div>
      </div>

      <div class="space-y-2 md:hidden">
        <label id="api-service-sales-filter-mobile-label" class="flex items-center gap-2 text-sm font-medium">
          <ListFilter class="h-4 w-4 text-muted-foreground" />
          销售状态
        </label>
        <Select v-model="salesView">
          <SelectTrigger class="w-full" aria-labelledby="api-service-sales-filter-mobile-label">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem
              v-for="option in apiServiceSalesViewOptions"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>
    </section>

    <div class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <h2 class="text-sm font-semibold">{{ selectedSalesView.label }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ selectedSalesView.description }}</p>
      </div>
      <p class="text-xs text-muted-foreground">限时包过期不会删除长期 API 服务或历史订单。</p>
    </div>

    <SkeletonTable v-if="isLoading" :rows="5" :columns="6" />
    <ErrorState
      v-else-if="error"
      title="API 服务暂时无法加载"
      description="请稍后重试，当前筛选不会改变服务状态。"
      @retry="refetch()"
    />
    <EmptyState v-else-if="rows.length === 0" :title="emptyTitle" :description="emptyDescription">
      <template #action>
        <Button as-child>
          <RouterLink :to="publishServiceRoute">
            <Plus class="h-4 w-4" />
            {{ quotaPublishIntent ? '发布 API 服务并继续' : '发布 API 服务' }}
          </RouterLink>
        </Button>
      </template>
    </EmptyState>

    <template v-else>
      <SoftTable class="hidden md:block" :columns="['服务', '销售方式', '销售状态', '销售时间', '服务状态', '操作']">
        <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
          <td class="align-top">
          <div class="flex min-w-0 items-center gap-2.5">
            <span class="grid h-9 w-9 shrink-0 place-items-center rounded-md border border-border bg-white">
              <img v-if="serviceIconSrc(item)" :src="serviceIconSrc(item) ?? undefined" alt="" class="h-5 w-5 object-contain" />
              <PackagePlus v-else class="h-4 w-4 text-muted-foreground" />
            </span>
            <div class="min-w-0">
              <div class="break-words font-medium">{{ item.title }}</div>
              <div class="text-xs text-muted-foreground"><ShortId :value="item.id" prefix="API-SVC" /> · {{ item.delivery }}</div>
            </div>
          </div>
        </td>
          <td class="align-top">
            <div v-if="item.salesSummary.channels.length" class="flex max-w-36 flex-wrap gap-1.5">
              <Badge
                v-for="channel in item.salesSummary.channels"
                :key="channel.kind"
                variant="secondary"
              >
                {{ getApiServiceSalesChannelLabel(channel.kind) }}
              </Badge>
            </div>
            <Badge v-else variant="outline">暂无销售方式</Badge>
          </td>
          <td class="align-top">
            <div v-if="item.salesSummary.channels.length" class="grid min-w-44 gap-2">
              <div
                v-for="channel in item.salesSummary.channels"
                :key="channel.kind"
                class="flex items-center gap-2"
              >
                <span class="w-16 shrink-0 text-xs text-muted-foreground">
                  {{ getApiServiceSalesChannelLabel(channel.kind) }}
                </span>
                <StatusBadge
                  :status="channel.state"
                  :label="channelStatus(channel).label"
                  :tone="channelStatus(channel).tone"
                />
                <span v-if="getApiServiceSalesAvailabilitySummary(channel)" class="text-xs text-muted-foreground">
                  {{ getApiServiceSalesAvailabilitySummary(channel) }}
                </span>
              </div>
            </div>
            <StatusBadge
              v-else
              :status="item.salesSummary.overallState"
              :label="getApiServiceSalesStatus(item.salesSummary.overallState).label"
              :tone="getApiServiceSalesStatus(item.salesSummary.overallState).tone"
            />
          </td>
          <td class="align-top">
            <div v-if="item.salesSummary.channels.length" class="grid min-w-44 gap-2 text-xs">
              <div v-for="channel in item.salesSummary.channels" :key="channel.kind">
                <div class="text-muted-foreground">{{ getApiServiceSalesChannelLabel(channel.kind) }}</div>
                <div class="mt-0.5">{{ getApiServiceSalesTimeSummary(channel) }}</div>
              </div>
            </div>
            <span v-else class="text-xs text-muted-foreground">未配置销售计划</span>
          </td>
          <td class="align-top">
            <StatusBadge
              :status="item.state"
              :label="serviceStatus(item).label"
              :tone="serviceStatus(item).tone"
            />
            <div class="mt-1 text-xs text-muted-foreground">
              最后确认 <LocalTime :value="item.lastOnlineConfirmedAt" />
            </div>
          </td>
          <td class="align-top">
            <div class="flex min-w-44 flex-wrap gap-1">
              <Button
                v-if="item.state === 'offline'"
                size="sm"
                :disabled="publishMutation.isPending.value"
                @click="publishService(item.id)"
              >
                <Upload class="h-4 w-4" />
                上线
              </Button>
              <Button
                v-if="item.online"
                size="sm"
                variant="outline"
                :disabled="pauseMutation.isPending.value"
                @click="pauseService(item.id)"
              >
                <Pause class="h-4 w-4" />
                暂停
              </Button>
              <Button
                v-if="item.state === 'paused'"
                size="sm"
                variant="outline"
                :disabled="resumeMutation.isPending.value"
                @click="resumeService(item.id)"
              >
                <Play class="h-4 w-4" />
                恢复
              </Button>
              <Button v-if="quotaPublishIntent" size="sm" as-child>
                <RouterLink :to="`/api-market/quota/new?serviceId=${item.id}`">
                  <PackagePlus class="h-4 w-4" />
                  选择并发布额度包
                </RouterLink>
              </Button>
              <Button v-else size="sm" as-child>
                <RouterLink :to="`/my/api-services/${item.id}#quota-offers`">
                  <Settings2 class="h-4 w-4" />
                  高级管理
                </RouterLink>
              </Button>
              <Button v-if="!quotaPublishIntent && hasExpiredLimitedQuota(item)" size="sm" variant="outline" as-child>
                <RouterLink :to="`/api-market/quota/new?serviceId=${item.id}`">
                  <PackagePlus class="h-4 w-4" />
                  重新发布限时包
                </RouterLink>
              </Button>
              <Button v-if="getApiServicePublicDetailUrl(item)" size="sm" variant="outline" as-child>
                <RouterLink :to="`${getApiServicePublicDetailUrl(item)}?preview=owner`">
                  <Eye class="h-4 w-4" />
                  公开预览
                </RouterLink>
              </Button>
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

      <div class="divide-y divide-border border-y border-border md:hidden">
        <article
          v-for="item in pagination.paginatedRows.value"
          :key="item.id"
          class="min-w-0 space-y-4 py-4"
        >
          <div class="flex min-w-0 items-start gap-3">
            <span class="grid h-10 w-10 shrink-0 place-items-center rounded-md border border-border bg-white">
              <img v-if="serviceIconSrc(item)" :src="serviceIconSrc(item) ?? undefined" alt="" class="h-5 w-5 object-contain" />
              <PackagePlus v-else class="h-4 w-4 text-muted-foreground" />
            </span>
            <div class="min-w-0 flex-1">
              <div class="break-words font-medium">{{ item.title }}</div>
              <div class="mt-1 text-xs text-muted-foreground">
                <ShortId :value="item.id" prefix="API-SVC" /> · {{ item.delivery }}
              </div>
            </div>
            <StatusBadge
              :status="item.state"
              :label="serviceStatus(item).label"
              :tone="serviceStatus(item).tone"
            />
          </div>

          <div v-if="item.salesSummary.channels.length" class="grid gap-3">
            <div
              v-for="channel in item.salesSummary.channels"
              :key="channel.kind"
              class="grid min-w-0 gap-1.5"
            >
              <div class="flex flex-wrap items-center gap-2">
                <Badge variant="secondary">{{ getApiServiceSalesChannelLabel(channel.kind) }}</Badge>
                <StatusBadge
                  :status="channel.state"
                  :label="channelStatus(channel).label"
                  :tone="channelStatus(channel).tone"
                />
                <span v-if="getApiServiceSalesAvailabilitySummary(channel)" class="text-xs text-muted-foreground">
                  {{ getApiServiceSalesAvailabilitySummary(channel) }}
                </span>
              </div>
              <div class="text-xs text-muted-foreground">{{ getApiServiceSalesTimeSummary(channel) }}</div>
            </div>
          </div>
          <div v-else class="flex flex-wrap items-center gap-2">
            <Badge variant="outline">暂无销售方式</Badge>
            <StatusBadge
              :status="item.salesSummary.overallState"
              :label="getApiServiceSalesStatus(item.salesSummary.overallState).label"
              :tone="getApiServiceSalesStatus(item.salesSummary.overallState).tone"
            />
          </div>

          <div class="text-xs text-muted-foreground">
            服务最后确认 <LocalTime :value="item.lastOnlineConfirmedAt" />
          </div>

          <div class="flex min-w-0 flex-wrap gap-2">
            <Button
              v-if="item.state === 'offline'"
              size="sm"
              :disabled="publishMutation.isPending.value"
              @click="publishService(item.id)"
            >
              <Upload class="h-4 w-4" />
              上线
            </Button>
            <Button
              v-if="item.online"
              size="sm"
              variant="outline"
              :disabled="pauseMutation.isPending.value"
              @click="pauseService(item.id)"
            >
              <Pause class="h-4 w-4" />
              暂停
            </Button>
            <Button
              v-if="item.state === 'paused'"
              size="sm"
              variant="outline"
              :disabled="resumeMutation.isPending.value"
              @click="resumeService(item.id)"
            >
              <Play class="h-4 w-4" />
              恢复
            </Button>
            <Button v-if="quotaPublishIntent" size="sm" as-child>
              <RouterLink :to="`/api-market/quota/new?serviceId=${item.id}`">
                <PackagePlus class="h-4 w-4" />
                选择并发布额度包
              </RouterLink>
            </Button>
            <Button v-else size="sm" as-child>
              <RouterLink :to="`/my/api-services/${item.id}#quota-offers`">
                <Settings2 class="h-4 w-4" />
                高级管理
              </RouterLink>
            </Button>
            <Button v-if="!quotaPublishIntent && hasExpiredLimitedQuota(item)" size="sm" variant="outline" as-child>
              <RouterLink :to="`/api-market/quota/new?serviceId=${item.id}`">
                <PackagePlus class="h-4 w-4" />
                重新发布限时包
              </RouterLink>
            </Button>
            <Button v-if="getApiServicePublicDetailUrl(item)" size="sm" variant="outline" as-child>
              <RouterLink :to="`${getApiServicePublicDetailUrl(item)}?preview=owner`">
                <Eye class="h-4 w-4" />
                公开预览
              </RouterLink>
            </Button>
          </div>
        </article>
        <TablePagination
          v-model:page="pagination.page.value"
          :page-count="pagination.pageCount.value"
          :total="pagination.total.value"
          :start-item="pagination.startItem.value"
          :end-item="pagination.endItem.value"
        />
      </div>
    </template>
  </div>
</template>
