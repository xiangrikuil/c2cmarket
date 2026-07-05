<script setup lang="ts">
import { computed } from 'vue'
import { BadgeCheck, Car, Code2, ListChecks } from 'lucide-vue-next'
import { useAdminOverview, useApiServices, useCarpools, useOfficialPrices } from '@/queries/useMarketQueries'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import StatCard from '@/components/market/StatCard.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { getApiMerchantDisplayName } from '@/lib/api'

const { data } = useAdminOverview()
const { data: officialPrices } = useOfficialPrices()
const { data: carpools } = useCarpools()
const { data: apiServices } = useApiServices()

const officialRows = computed(() => officialPrices.value ?? [])
const carpoolRows = computed(() => carpools.value ?? [])
const apiServiceRows = computed(() => apiServices.value ?? [])
const officialPagination = usePagination(officialRows)
const carpoolPagination = usePagination(carpoolRows)
const apiServicePagination = usePagination(apiServiceRows)
const statIcons = [BadgeCheck, Car, Code2, ListChecks]
const overviewCards = computed(() => (data.value ?? []).map((card, index) => ({
  ...card,
  icon: statIcons[index % statIcons.length],
})))
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight">管理台</h1>
        <p class="mt-2 text-muted-foreground">审核线索、治理车源、管理求车、API 商户和交易意向。所有关键操作保留日志。</p>
      </div>
      <Badge>管理员</Badge>
    </div>
    <div class="grid gap-2 md:grid-cols-5">
      <RouterLink v-for="item in [
        ['官网公开价', '/admin/official-prices'],
        ['套餐目录', '/admin/product-plans'],
        ['API 模型目录', '/admin/api-models'],
        ['低价线索', '/admin/price-leads'],
        ['车源治理', '/admin/carpools'],
        ['求车管理', '/admin/demands'],
        ['API 商户', '/admin/api-merchants'],
        ['API 服务', '/admin/api-services'],
        ['公告管理', '/admin/announcements'],
        ['交易意向', '/admin/trade-intents'],
        ['认证铭牌', '/admin/certifications'],
        ['问题反馈', '/admin/feedback'],
        ['举报纠纷', '/admin/reports'],
        ['操作日志', '/admin/logs'],
      ]" :key="item[1]" :to="item[1]" class="rounded-lg border border-border bg-card px-3 py-2 text-sm font-medium hover:bg-accent">
        {{ item[0] }}
      </RouterLink>
    </div>
    <div class="grid gap-3 md:grid-cols-4">
      <StatCard v-for="card in overviewCards" :key="card.label" :label="card.label" :value="card.value" :hint="card.hint" :icon="card.icon" :accent="card.label.includes('低价')" />
    </div>
    <div class="grid gap-5 xl:grid-cols-2">
      <section class="space-y-3">
        <h2 class="font-semibold">官网公开价格管理</h2>
        <SoftTable :columns="['产品', '地区', '价格', '状态', '操作']">
          <tr v-for="item in officialPagination.paginatedRows.value" :key="item.id">
            <td>{{ item.product }} {{ item.plan }}</td>
            <td>{{ item.region }}</td>
            <td class="font-semibold">{{ item.cny ? `¥${item.cny}` : '待验证' }}</td>
            <td><Badge :variant="item.status === '已验证' ? 'default' : 'secondary'">{{ item.status }}</Badge></td>
            <td><Button size="sm" @click="toast(`正在编辑 ${item.product} ${item.plan} 的价格记录。`)">编辑</Button></td>
          </tr>
          <template #footer>
            <TablePagination
              v-model:page="officialPagination.page.value"
              :page-count="officialPagination.pageCount.value"
              :total="officialPagination.total.value"
              :start-item="officialPagination.startItem.value"
              :end-item="officialPagination.endItem.value"
            />
          </template>
        </SoftTable>
      </section>
      <section class="space-y-3">
        <h2 class="font-semibold">车源治理</h2>
        <SoftTable :columns="['车源', '车主', '状态', '操作']">
          <tr v-for="item in carpoolPagination.paginatedRows.value" :key="item.id">
            <td>{{ item.product }} · {{ item.region }}</td>
            <td>{{ item.owner }} · 信任等级{{ item.trustLevel }}</td>
            <td><Badge>{{ item.status }}</Badge></td>
            <td><Button size="sm" @click="toast(`已打开 ${item.product} 的审核详情。`)">查看</Button></td>
          </tr>
          <template #footer>
            <TablePagination
              v-model:page="carpoolPagination.page.value"
              :page-count="carpoolPagination.pageCount.value"
              :total="carpoolPagination.total.value"
              :start-item="carpoolPagination.startItem.value"
              :end-item="carpoolPagination.endItem.value"
            />
          </template>
        </SoftTable>
      </section>
      <section class="space-y-3">
        <h2 class="font-semibold">API 商户监控</h2>
        <SoftTable :columns="['商户', '服务', '在线', '操作']">
          <tr v-for="item in apiServicePagination.paginatedRows.value" :key="item.id">
            <td>
              <div>{{ getApiMerchantDisplayName(item) }} · 信任等级{{ item.trustLevel }}</div>
              <div v-if="item.merchantIdentityMode === 'store_alias'" class="text-xs text-muted-foreground">真实用户 {{ item.merchantUsername }}</div>
            </td>
            <td>{{ item.title }}</td>
            <td><Badge :variant="item.online ? 'default' : 'secondary'">{{ item.online ? '在线' : '离线' }}</Badge></td>
            <td><Button size="sm" @click="toast.warning(`${getApiMerchantDisplayName(item)} 已标记为待下线复核。`)">强制下线</Button></td>
          </tr>
          <template #footer>
            <TablePagination
              v-model:page="apiServicePagination.page.value"
              :page-count="apiServicePagination.pageCount.value"
              :total="apiServicePagination.total.value"
              :start-item="apiServicePagination.startItem.value"
              :end-item="apiServicePagination.endItem.value"
            />
          </template>
        </SoftTable>
      </section>
      <Card class="p-5">
        <h3 class="font-semibold">操作日志</h3>
        <div class="mt-4 space-y-3 text-sm text-muted-foreground">
          <p>16:30 管理员将 ChatGPT Pro 菲律宾区标记为当前在售参考。</p>
          <p>15:42 车源 ChatGPT Business 美国区补充席位机制，进入复审。</p>
          <p>14:10 API 商户小葵 API 因 2 次未响应被自动下线。</p>
        </div>
      </Card>
    </div>
  </div>
</template>
