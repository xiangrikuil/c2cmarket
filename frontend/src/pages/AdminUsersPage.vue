<script setup lang="ts">
import { computed, ref } from 'vue'
import { Eye, Search, ShieldCheck, UsersRound } from 'lucide-vue-next'
import ErrorState from '@/components/market/ErrorState.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import AdminReputationAuditPanel from '@/components/reputation/AdminReputationAuditPanel.vue'
import { usePagination } from '@/composables/usePagination'
import { useAdminSectionRows } from '@/queries/useMarketQueries'

const { data, isLoading, error, refetch } = useAdminSectionRows('users')
const keyword = ref('')
const activeStatus = ref('全部')
const selectedUserId = ref('')

const rows = computed(() => data.value ?? [])
const selectedRow = computed(() => rows.value.find(row => row.id === selectedUserId.value) ?? null)
const statusTabs = ['全部', '正常', '已暂停', '已封禁', '已归档']
const visibleRows = computed(() => {
  const normalizedKeyword = keyword.value.trim().toLowerCase()
  return rows.value.filter(row => {
    const matchesStatus = activeStatus.value === '全部' || row.status === activeStatus.value
    const matchesKeyword = !normalizedKeyword || [row.primary, row.secondary, row.owner]
      .join(' ')
      .toLowerCase()
      .includes(normalizedKeyword)
    return matchesStatus && matchesKeyword
  })
})
const pagination = usePagination(visibleRows)
const adminCount = computed(() => rows.value.filter(row => row.owner === '管理员账号').length)
const linuxDoBoundCount = computed(() => rows.value.filter(row => row.secondary.includes('已绑定 linux.do')).length)
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="用户目录" description="查看全部账号及其状态、角色、linux.do 绑定和活跃时间。举报、纠纷与申诉在独立案件处理台管理。" />

    <div class="grid gap-3 md:grid-cols-3">
      <Card class="p-4">
        <div class="flex items-center gap-2 text-sm text-muted-foreground"><UsersRound class="h-4 w-4" />全部账号</div>
        <div class="mt-2 text-2xl font-semibold">{{ rows.length }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-sm text-muted-foreground">管理员账号</div>
        <div class="mt-2 text-2xl font-semibold">{{ adminCount }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-sm text-muted-foreground">已绑定 linux.do</div>
        <div class="mt-2 text-2xl font-semibold">{{ linuxDoBoundCount }}</div>
      </Card>
    </div>

    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <StatusTabs v-model="activeStatus" :items="statusTabs" />
      <div class="relative w-full sm:max-w-xs">
        <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input v-model="keyword" class="pl-9" placeholder="搜索用户名或显示名称" />
      </div>
    </div>

    <AdminReputationAuditPanel
      v-if="selectedRow"
      :user-id="selectedRow.id"
      :username="selectedRow.primary"
      :user-version="selectedRow.backendVersion"
    />

    <SkeletonTable v-if="isLoading" :rows="6" :columns="6" />
    <ErrorState
      v-else-if="error"
      title="用户目录加载失败"
      description="当前无法读取管理员用户目录。"
      @retry="refetch()"
    />
    <SoftTable v-else :columns="['账号', '资料与绑定', '角色', '账号状态', '注册 / 活跃', '操作']">
      <tr v-for="row in pagination.paginatedRows.value" :key="row.id">
        <td class="font-medium">{{ row.primary }}</td>
        <td class="text-muted-foreground">{{ row.secondary }}</td>
        <td>{{ row.owner }}</td>
        <td><Badge :variant="row.status === '正常' ? 'default' : 'secondary'">{{ row.status }}</Badge></td>
        <td>{{ row.risk }}</td>
        <td>
          <div class="flex flex-wrap gap-2">
            <Button size="sm" :variant="selectedUserId === row.id ? 'secondary' : 'outline'" @click="selectedUserId = selectedUserId === row.id ? '' : row.id">
              <ShieldCheck class="h-3.5 w-3.5" />信誉审计
            </Button>
            <Button v-if="row.targetTo" as-child size="sm" variant="ghost">
              <RouterLink :to="row.targetTo"><Eye class="h-3.5 w-3.5" />公开主页</RouterLink>
            </Button>
          </div>
        </td>
      </tr>
      <tr v-if="visibleRows.length === 0">
        <td colspan="6" class="py-10 text-center text-sm text-muted-foreground">没有符合当前筛选的账号。</td>
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
