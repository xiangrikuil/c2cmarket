<script setup lang="ts">
import { computed } from 'vue'
import { Download, Plus, Send, Pencil } from 'lucide-vue-next'
import { useOfficialPrices } from '@/queries/useMarketQueries'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { toast } from 'vue-sonner'
import { shouldUseRealBackend } from '@/lib/backendClient'

const { data } = useOfficialPrices()
const rows = computed(() => data.value ?? [])
const pagination = usePagination(rows)
const realBackend = shouldUseRealBackend()

function setReferenceLabel(product: string, plan: string) {
  if (realBackend) {
    toast.warning('真实模式下当前参考价由管理员审核通过的后端记录决定。')
    return
  }
  toast.success(`${product} ${plan} 已标记为当前在售参考，当前为前端本地反馈。`)
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight">官网公开价格管理</h1>
        <p class="mt-2 text-muted-foreground">维护每个产品、地区和渠道的官网公开价、社区低价线索、编辑记录和提交人。</p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" @click="toast('CSV 导入入口已打开，后续接入文件解析。')">
          <Download class="h-4 w-4" />导入 CSV
        </Button>
        <Button @click="toast.success('已新增一条参考价草稿。')">
          <Plus class="h-4 w-4" />新增参考价
        </Button>
      </div>
    </div>

    <Card class="p-4">
      <div class="grid gap-3 md:grid-cols-4">
        <Input placeholder="产品：Claude Pro" />
        <Input placeholder="地区：菲律宾区" />
        <Input placeholder="参考价：¥988" />
        <Input placeholder="说明：含汇率税费提醒" />
      </div>
    </Card>

    <SoftTable :columns="['产品', '地区', '参考价', '提交人', '状态', '更新时间', '操作']">
      <tr v-for="item in pagination.paginatedRows.value" :key="item.id">
        <td class="font-medium">{{ item.product }} {{ item.plan }}</td>
        <td class="text-muted-foreground">{{ item.region }} · {{ item.channel }}</td>
        <td class="font-semibold">{{ item.cny ? `¥${item.cny}` : '待验证' }}</td>
        <td class="text-muted-foreground">{{ item.submitter }} · 信任等级{{ item.submitterTrust }}</td>
        <td><Badge :variant="item.status === '已验证' ? 'default' : 'secondary'">{{ item.status }}</Badge></td>
        <td class="text-muted-foreground">{{ item.updatedAt }}</td>
        <td>
          <div class="flex gap-2">
            <Button size="sm" variant="outline" @click="toast(`正在编辑 ${item.product} ${item.plan}。`)">
              <Pencil class="h-3.5 w-3.5" />编辑
            </Button>
            <Button size="sm" @click="setReferenceLabel(item.product, item.plan)">
              <Send class="h-3.5 w-3.5" />设参考
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
