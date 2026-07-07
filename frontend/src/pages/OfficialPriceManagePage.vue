<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { Archive, ExternalLink, Pencil, Plus, RefreshCw, Save } from 'lucide-vue-next'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { usePagination } from '@/composables/usePagination'
import {
  backendAdminOfficialPriceRecords,
  backendCreateAdminOfficialPriceRecord,
  backendTakeDownAdminOfficialPriceRecord,
  backendUpdateAdminOfficialPriceRecord,
  type OfficialPriceAdminRecord,
  type OfficialPriceAdminRecordPayload,
} from '@/lib/officialPriceBackend'
import { useCarpoolProductCatalog } from '@/queries/useMarketQueries'

type FormMode = 'create' | 'edit'
type StatusFilter = '全部' | '生效中' | '历史记录' | '已下架'

const queryClient = useQueryClient()
const recordsQuery = useQuery({
  queryKey: ['admin-official-price-records'],
  queryFn: backendAdminOfficialPriceRecords,
  refetchOnMount: 'always',
})
const productCatalogQuery = useCarpoolProductCatalog()
const search = ref('')
const statusFilter = ref<StatusFilter>('全部')
const formOpen = ref(false)
const takeDownOpen = ref(false)
const formMode = ref<FormMode>('create')
const editingRecord = ref<OfficialPriceAdminRecord | null>(null)
const actionRecord = ref<OfficialPriceAdminRecord | null>(null)
const takeDownReason = ref('官网公开价格已失效。')
const form = reactive({
  productPlanId: '',
  productText: '',
  planText: '',
  regionCode: 'ph',
  channel: 'web',
  openingMethod: 'official_web',
  sourceUrl: '',
  observedAt: dateTimeLocalValue(),
  billingPeriod: 'monthly',
  currency: 'PHP',
  originalAmount: '',
  taxIncluded: 'true',
  fxRateToCny: '0.12210000',
  fxSource: 'admin_configured_snapshot',
  fxObservedAt: dateTimeLocalValue(),
  validFrom: dateTimeLocalValue(),
  reason: '管理员维护官网公开价格。',
})

const records = computed(() => recordsQuery.data.value ?? [])
const productPlans = computed(() => productCatalogQuery.data.value ?? [])
const activeProductPlans = computed(() => productPlans.value.filter(item => item.active !== false))
const isLoading = computed(() => recordsQuery.isLoading.value)
const errorMessage = computed(() => {
  const error = recordsQuery.error.value
  return error instanceof Error ? error.message : '官网价格记录读取失败。'
})
const activeCount = computed(() => records.value.filter(item => item.status === 'active').length)
const hiddenCount = computed(() => records.value.filter(item => item.status !== 'active').length)
const lowestCny = computed(() => {
  const prices = records.value
    .filter(item => item.status === 'active')
    .map(item => Number(item.normalizedMonthlyCny))
    .filter(value => Number.isFinite(value))
  if (!prices.length) return '暂无'
  return `¥${Math.min(...prices).toFixed(2)}`
})
const filteredRows = computed(() => {
  const keyword = search.value.trim().toLowerCase()
  return records.value.filter(item => {
    const statusMatched = statusFilter.value === '全部'
      || (statusFilter.value === '生效中' && item.status === 'active')
      || (statusFilter.value === '历史记录' && item.status === 'superseded')
      || (statusFilter.value === '已下架' && item.status === 'taken_down')
    if (!statusMatched) return false
    if (!keyword) return true
    return [
      item.product,
      item.plan,
      item.productName,
      item.region,
      item.regionCode,
      item.channel,
      item.currency,
      item.sourceUrl,
    ].some(value => value.toLowerCase().includes(keyword))
  })
})
const pagination = usePagination(filteredRows)
const saveMutation = useMutation({
  mutationFn: async () => {
    const payload = formPayload()
    if (formMode.value === 'edit' && editingRecord.value) {
      return backendUpdateAdminOfficialPriceRecord(editingRecord.value.id, editingRecord.value.version, payload)
    }
    return backendCreateAdminOfficialPriceRecord(payload)
  },
  onSuccess: async () => {
    toast.success(formMode.value === 'edit' ? '官网价格记录已更新。' : '官网价格记录已新增。')
    formOpen.value = false
    await invalidateOfficialPriceQueries()
  },
  onError: error => toast.error(error instanceof Error ? error.message : '保存失败。'),
})
const takeDownMutation = useMutation({
  mutationFn: async () => {
    if (!actionRecord.value) throw new Error('请选择要下架的记录。')
    return backendTakeDownAdminOfficialPriceRecord(actionRecord.value.id, actionRecord.value.version, takeDownReason.value)
  },
  onSuccess: async () => {
    toast.success('官网价格记录已下架。')
    takeDownOpen.value = false
    await invalidateOfficialPriceQueries()
  },
  onError: error => toast.error(error instanceof Error ? error.message : '下架失败。'),
})
const saving = computed(() => saveMutation.isPending.value)
const takingDown = computed(() => takeDownMutation.isPending.value)
const dialogTitle = computed(() => formMode.value === 'edit' ? '编辑官网价格记录' : '新增官网价格记录')

watch(activeProductPlans, value => {
  if (form.productPlanId || value.length === 0) return
  form.productPlanId = value[0].id
}, { immediate: true })

function dateTimeLocalValue(value: string | Date = new Date()) {
  const date = value instanceof Date ? value : new Date(value)
  if (!Number.isFinite(date.getTime())) return dateTimeLocalValue(new Date())
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function dateTimeLocalToISOString(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return new Date().toISOString()
  return date.toISOString()
}

function formatDateTime(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function statusVariant(status: string) {
  if (status === 'active') return 'verified'
  if (status === 'taken_down') return 'destructive'
  return 'secondary'
}

function resetForm(record: OfficialPriceAdminRecord | null = null) {
  editingRecord.value = record
  formMode.value = record ? 'edit' : 'create'
  form.productPlanId = record?.productPlanId || activeProductPlans.value[0]?.id || ''
  form.productText = record?.product || ''
  form.planText = record?.plan || ''
  form.regionCode = record?.regionCode || 'ph'
  form.channel = record?.channel || 'web'
  form.openingMethod = record?.openingMethod || 'official_web'
  form.sourceUrl = record?.sourceUrl || ''
  form.observedAt = dateTimeLocalValue(record?.observedAt)
  form.billingPeriod = record?.billingPeriod || 'monthly'
  form.currency = record?.currency || 'PHP'
  form.originalAmount = record?.originalAmount || ''
  form.taxIncluded = record?.taxIncluded === false ? 'false' : 'true'
  form.fxRateToCny = record?.fxRateToCny || '0.12210000'
  form.fxSource = record?.fxSource || 'admin_configured_snapshot'
  form.fxObservedAt = dateTimeLocalValue(record?.fxObservedAt)
  form.validFrom = dateTimeLocalValue(record?.validFrom)
  form.reason = record ? '管理员编辑官网公开价格。' : '管理员维护官网公开价格。'
}

function openCreateForm() {
  resetForm()
  formOpen.value = true
}

function openEditForm(record: OfficialPriceAdminRecord) {
  resetForm(record)
  formOpen.value = true
}

function openTakeDownDialog(record: OfficialPriceAdminRecord) {
  actionRecord.value = record
  takeDownReason.value = '官网公开价格已失效。'
  takeDownOpen.value = true
}

function formPayload(): OfficialPriceAdminRecordPayload {
  return {
    productPlanId: form.productPlanId,
    productText: form.productText,
    planText: form.planText,
    regionCode: form.regionCode,
    channel: form.channel,
    openingMethod: form.openingMethod,
    sourceUrl: form.sourceUrl,
    observedAt: dateTimeLocalToISOString(form.observedAt),
    billingPeriod: form.billingPeriod,
    currency: form.currency,
    originalAmount: form.originalAmount,
    taxIncluded: form.taxIncluded === 'true',
    fxRateToCny: form.fxRateToCny,
    fxSource: form.fxSource,
    fxObservedAt: dateTimeLocalToISOString(form.fxObservedAt),
    validFrom: dateTimeLocalToISOString(form.validFrom),
    reason: form.reason,
  }
}

async function invalidateOfficialPriceQueries() {
  await queryClient.invalidateQueries({ queryKey: ['admin-official-price-records'] })
  await queryClient.invalidateQueries({ queryKey: ['official-prices'] })
  await queryClient.invalidateQueries({ queryKey: ['home-market'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight">官网价格记录维护</h1>
        <p class="mt-2 text-muted-foreground">由管理员维护产品、地区、渠道、官网公开价、折合人民币和来源。</p>
      </div>
      <div class="flex gap-2">
        <Button variant="outline" :disabled="recordsQuery.isFetching.value" @click="recordsQuery.refetch()">
          <RefreshCw class="h-4 w-4" />刷新
        </Button>
        <Button @click="openCreateForm">
          <Plus class="h-4 w-4" />新增记录
        </Button>
      </div>
    </div>

    <div class="grid gap-3 md:grid-cols-3">
      <div class="rounded-md border bg-card px-4 py-3">
        <div class="text-sm text-muted-foreground">生效记录</div>
        <div class="mt-1 text-2xl font-semibold">{{ activeCount }}</div>
      </div>
      <div class="rounded-md border bg-card px-4 py-3">
        <div class="text-sm text-muted-foreground">最低折合人民币</div>
        <div class="mt-1 text-2xl font-semibold">{{ lowestCny }}</div>
      </div>
      <div class="rounded-md border bg-card px-4 py-3">
        <div class="text-sm text-muted-foreground">历史 / 下架</div>
        <div class="mt-1 text-2xl font-semibold">{{ hiddenCount }}</div>
      </div>
    </div>

    <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_180px]">
      <Input v-model="search" placeholder="搜索产品、地区、渠道或来源" />
      <Select v-model="statusFilter">
        <SelectTrigger class="w-full bg-background"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="全部">全部</SelectItem>
          <SelectItem value="生效中">生效中</SelectItem>
          <SelectItem value="历史记录">历史记录</SelectItem>
          <SelectItem value="已下架">已下架</SelectItem>
        </SelectContent>
      </Select>
    </div>

    <div v-if="recordsQuery.isError.value" class="rounded-md border border-destructive/20 bg-destructive/5 px-4 py-3 text-sm text-destructive">
      {{ errorMessage }}
    </div>

    <SoftTable :columns="['产品', '地区 / 渠道', '官网公开价', '折合人民币', '来源', '状态', '维护时间', '操作']">
      <tr v-if="isLoading">
        <td colspan="8" class="px-4 py-8 text-center text-sm text-muted-foreground">正在读取官网价格记录</td>
      </tr>
      <tr v-else-if="pagination.paginatedRows.value.length === 0">
        <td colspan="8" class="px-4 py-8 text-center text-sm text-muted-foreground">暂无记录</td>
      </tr>
      <tr v-for="item in pagination.paginatedRows.value" v-else :key="item.id">
        <td>
          <div class="font-medium">{{ item.product }} {{ item.plan }}</div>
          <div class="mt-1 text-xs text-muted-foreground">{{ item.productName }}</div>
        </td>
        <td class="text-muted-foreground">
          <div>{{ item.region }} · {{ item.channel }}</div>
          <div class="mt-1 text-xs">{{ item.openingMethod }}</div>
        </td>
        <td class="font-medium">{{ item.originalPrice }}</td>
        <td class="font-semibold">¥{{ item.normalizedMonthlyCny }}</td>
        <td>
          <a :href="item.sourceUrl" target="_blank" rel="noreferrer" class="inline-flex items-center gap-1 text-sm text-primary hover:underline">
            来源 <ExternalLink class="h-3.5 w-3.5" />
          </a>
        </td>
        <td>
          <Badge :variant="statusVariant(item.status)">{{ item.statusLabel }}</Badge>
        </td>
        <td class="text-sm text-muted-foreground">
          <div>{{ formatDateTime(item.validFrom) }}</div>
          <div class="mt-1 text-xs">v{{ item.version }}</div>
        </td>
        <td>
          <div class="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" :disabled="item.status !== 'active'" @click="openEditForm(item)">
              <Pencil class="h-3.5 w-3.5" />编辑
            </Button>
            <Button size="sm" variant="outline" :disabled="item.status !== 'active'" @click="openTakeDownDialog(item)">
              <Archive class="h-3.5 w-3.5" />下架
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

    <Dialog v-model:open="formOpen">
      <DialogContent class="max-h-[90vh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>{{ dialogTitle }}</DialogTitle>
          <DialogDescription>保存后会刷新公开官网价格表。</DialogDescription>
        </DialogHeader>
        <div class="grid gap-4 md:grid-cols-2">
          <label class="space-y-2">
            <span class="text-sm font-medium">产品套餐</span>
            <Select v-model="form.productPlanId">
              <SelectTrigger class="w-full bg-background"><SelectValue placeholder="选择产品套餐" /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="plan in activeProductPlans" :key="plan.id" :value="plan.id">{{ plan.displayName }}</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">地区代码</span>
            <Input v-model="form.regionCode" placeholder="ph" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">渠道</span>
            <Input v-model="form.channel" placeholder="web" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">开通方式</span>
            <Input v-model="form.openingMethod" placeholder="official_web" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">产品文本</span>
            <Input v-model="form.productText" placeholder="ChatGPT" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">套餐文本</span>
            <Input v-model="form.planText" placeholder="Pro" />
          </label>
          <label class="space-y-2 md:col-span-2">
            <span class="text-sm font-medium">来源 URL</span>
            <Input v-model="form.sourceUrl" placeholder="https://..." />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">观察时间</span>
            <Input v-model="form.observedAt" type="datetime-local" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">生效时间</span>
            <Input v-model="form.validFrom" type="datetime-local" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">币种</span>
            <Input v-model="form.currency" maxlength="3" placeholder="PHP" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">原币价格</span>
            <Input v-model="form.originalAmount" inputmode="decimal" placeholder="799.00" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">汇率到人民币</span>
            <Input v-model="form.fxRateToCny" inputmode="decimal" placeholder="0.12210000" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">汇率来源</span>
            <Input v-model="form.fxSource" placeholder="admin_configured_snapshot" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">汇率时间</span>
            <Input v-model="form.fxObservedAt" type="datetime-local" />
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">税费</span>
            <Select v-model="form.taxIncluded">
              <SelectTrigger class="w-full bg-background"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="true">含税</SelectItem>
                <SelectItem value="false">未含税</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="space-y-2 md:col-span-2">
            <span class="text-sm font-medium">维护原因</span>
            <Textarea v-model="form.reason" class="min-h-24" />
          </label>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="formOpen = false">取消</Button>
          <Button :disabled="saving" @click="saveMutation.mutate()">
            <Save class="h-4 w-4" />保存
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="takeDownOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>下架官网价格记录</DialogTitle>
          <DialogDescription>{{ actionRecord ? `${actionRecord.product} ${actionRecord.plan} · ${actionRecord.region}` : '' }}</DialogDescription>
        </DialogHeader>
        <label class="space-y-2">
          <span class="text-sm font-medium">下架原因</span>
          <Textarea v-model="takeDownReason" class="min-h-24" />
        </label>
        <DialogFooter>
          <Button variant="outline" @click="takeDownOpen = false">取消</Button>
          <Button variant="destructive" :disabled="takingDown" @click="takeDownMutation.mutate()">
            <Archive class="h-4 w-4" />下架
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
