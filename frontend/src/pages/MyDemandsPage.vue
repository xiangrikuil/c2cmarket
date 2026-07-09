<script setup lang="ts">
import { computed, ref } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import type { DemandRecord } from '@/features/demand/types'
import { useCloseDemandMutation, useMyDemands } from '@/queries/useMarketQueries'
import { toast } from 'vue-sonner'

const { data, isLoading, error } = useMyDemands()
const closeMutation = useCloseDemandMutation()
const activeTab = ref('全部')
const keyword = ref('')
const sortMode = ref<'updated' | 'created' | 'budget'>('updated')

const tabItems = ['全部', '匹配中', '已关闭', '需处理']

const stats = computed(() => {
  const rows = data.value ?? []
  return [
    { label: '全部需求', value: rows.length },
    { label: '匹配中', value: rows.filter(item => item.status === '匹配中').length },
    { label: '已关闭', value: rows.filter(item => item.status === '已关闭').length },
    { label: '需处理', value: rows.filter(item => item.status === '需处理').length },
  ]
})

function timestamp(value: string) {
  const parsed = new Date(value).getTime()
  return Number.isFinite(parsed) ? parsed : 0
}

function searchableText(item: DemandRecord) {
  return [
    item.id,
    item.title,
    item.region,
    item.require,
    item.note,
    item.sourceUrl,
    item.status,
  ].join(' ').toLowerCase()
}

const rows = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  return [...(data.value ?? [])]
    .filter(item => (activeTab.value === '全部' || item.status === activeTab.value) && (!q || searchableText(item).includes(q)))
    .sort((a, b) => {
      if (sortMode.value === 'budget') return b.maxPrice - a.maxPrice
      if (sortMode.value === 'created') return timestamp(b.createdAt) - timestamp(a.createdAt)
      return timestamp(b.updatedAt) - timestamp(a.updatedAt)
    })
})

const pagination = usePagination(rows)

function statusVariant(item: DemandRecord) {
  if (item.status === '匹配中') return 'default'
  if (item.status === '需处理') return 'secondary'
  return 'outline'
}

function ownerCanToggle(item: DemandRecord) {
  if (!item.backendStatus) return item.status === '匹配中' || item.status === '已关闭'
  return item.backendStatus === 'active'
    || item.backendStatus === 'closed'
    || item.backendStatus === 'pending_review'
    || item.backendStatus === 'changes_requested'
}

function nextActionLabel(item: DemandRecord) {
  return item.backendStatus === 'closed' || (!item.backendStatus && item.status === '已关闭') ? '重新打开' : '关闭需求'
}

function statusHint(item: DemandRecord) {
  if (item.backendStatus === 'taken_down') return '已由管理台下架，暂不能自行重开。'
  if (item.backendStatus === 'rejected') return '已被管理台拒绝，暂不能自行重开。'
  if (item.status === '匹配中') return '正在需求大厅公开展示。'
  if (item.status === '已关闭') return '已从公开需求大厅移除。'
  return '需要按页面提示补充或等待治理处理。'
}

function toggleDemand(item: DemandRecord) {
  if (!ownerCanToggle(item)) return
  closeMutation.mutate(item.id, {
    onSuccess(data) {
      toast.success(data.status === '已关闭' ? '求车需求已关闭。' : '求车需求已重新打开。')
    },
    onError(error) {
      toast.error(error instanceof Error ? error.message : '操作失败')
    },
  })
}
</script>

<template>
  <div class="space-y-4">
    <PageTitle title="我的需求" description="查看自己发布的求车需求、公开状态和 linux.do 原帖绑定；关闭后可从这里重新打开。" action-text="继续找车源" action-to="/demands" />

    <div class="grid gap-3 md:grid-cols-4">
      <div v-for="item in stats" :key="item.label" class="rounded-lg border border-border bg-card p-4">
        <div class="text-xs text-muted-foreground">{{ item.label }}</div>
        <div class="mt-1 text-2xl font-semibold">{{ item.value }}</div>
      </div>
    </div>

    <StatusTabs v-model="activeTab" :items="tabItems" />

    <div class="grid gap-2 md:grid-cols-[1fr_180px]">
      <Input v-model="keyword" placeholder="搜索产品、地区、原帖或需求编号" />
      <select v-model="sortMode" class="h-9 rounded-md border border-input bg-background px-3 text-sm">
        <option value="updated">最近更新</option>
        <option value="created">发布时间</option>
        <option value="budget">预算从高到低</option>
      </select>
    </div>

    <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">正在加载我的需求...</div>
    <div v-else-if="error" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-destructive">我的需求加载失败，请稍后重试。</div>
    <div v-else-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无求车需求。</div>

    <SoftTable v-else :columns="['需求', '预算', '偏好', '状态', '更新时间', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td>
          <RouterLink :to="`/demands/${item.id}`" class="font-medium hover:underline">{{ item.title }}</RouterLink>
          <div class="text-xs text-muted-foreground">{{ item.region }} · {{ item.linuxdoPost }}</div>
        </td>
        <td>
          <div class="font-semibold">¥{{ item.maxPrice }}</div>
          <div class="text-xs text-muted-foreground">最高月费</div>
        </td>
        <td class="text-xs text-muted-foreground">{{ item.require }}</td>
        <td>
          <Badge :variant="statusVariant(item)">{{ item.status }}</Badge>
          <div class="mt-1 text-xs text-muted-foreground">{{ statusHint(item) }}</div>
        </td>
        <td class="text-muted-foreground">{{ item.updatedAt }}</td>
        <td>
          <div class="flex flex-wrap gap-2">
            <RouterLink :to="`/demands/${item.id}`"><Button size="sm" variant="outline">查看</Button></RouterLink>
            <Button size="sm" variant="outline" :disabled="!ownerCanToggle(item) || closeMutation.isPending.value" @click="toggleDemand(item)">
              {{ nextActionLabel(item) }}
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
  </div>
</template>
