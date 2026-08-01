<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, toRef, watch } from 'vue'
import { RouterLink } from 'vue-router'
import {
  Archive,
  Box,
  Boxes,
  CalendarClock,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  CopyPlus,
  Download,
  FileKey2,
  FileUp,
  KeyRound,
  Layers3,
  LockKeyhole,
  MoreHorizontal,
  PackageOpen,
  PackagePlus,
  Pause,
  Play,
  Plus,
  Rocket,
  TimerReset,
  WalletCards,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import LocalTime from '@/components/market/LocalTime.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  getApiQuotaDeliveryModeLabel,
  getApiQuotaSaleModeLabel,
  type ApiOrderDeliveryKind,
  type ApiQuotaBatch,
  type ApiQuotaOffer,
  type ApiQuotaSourceType,
} from '@/lib/api'
import { formatDecimal, normalizeDecimal } from '@/lib/decimal'
import {
  findNextApiQuotaRound,
  getApiQuotaBatchStatus,
  getApiQuotaOfferStatus,
  getApiQuotaRoundStatus,
  type ApiQuotaStatusTone,
} from './apiQuotaOwnerPresentation'
import {
  useApiQuotaBatchActionMutation,
  useApiQuotaCredentialSummary,
  useCreateApiQuotaBatchMutation,
  useCreateApiQuotaOfferMutation,
  useCreateApiQuotaRoundMutation,
  useImportApiQuotaCredentialsMutation,
  useOwnerApiQuotaBatches,
  useOwnerApiQuotaOffers,
  useOwnerApiQuotaRounds,
} from '@/queries/useMarketQueries'

const props = defineProps<{
  apiServiceId: string
  distributionSystem: string
  defaultMultiplier: number
}>()

const apiServiceId = toRef(props, 'apiServiceId')
const isSub2Api = computed(() => props.distributionSystem === 'sub2api' || props.distributionSystem === 'Sub2API')
const serviceDefaultMultiplier = computed(() => Number.isFinite(props.defaultMultiplier) && props.defaultMultiplier > 0 ? props.defaultMultiplier : 1)
const serviceDefaultMultiplierDecimal = computed(() => normalizeDecimal(serviceDefaultMultiplier.value, 4))
const batchesQuery = useOwnerApiQuotaBatches(apiServiceId)
const selectedBatchId = ref('')
const salesTab = ref('batches')
const batchDialogOpen = ref(false)
const offerDialogOpen = ref(false)
const roundDialogOpen = ref(false)
const selectedCredentialOfferId = ref('')
const deliveryKind = ref<ApiOrderDeliveryKind>('api_key_endpoint')
const credentialFile = ref<File | null>(null)
const ownerNow = ref(Date.now())
let ownerClock: ReturnType<typeof setInterval> | undefined

const selectedBatch = computed(() => (batchesQuery.data.value ?? []).find(item => item.id === selectedBatchId.value) ?? null)
const offersQuery = useOwnerApiQuotaOffers(selectedBatchId)
const roundsQuery = useOwnerApiQuotaRounds(selectedBatchId)
const offers = computed(() => offersQuery.data.value ?? [])
const scheduledOffers = computed(() => offers.value.filter(item => item.saleMode === 'scheduled'))
const credentialOffers = computed(() => offers.value.filter(item => item.deliveryMode === 'preimported'))
const sortedRounds = computed(() => [...(roundsQuery.data.value ?? [])].sort((left, right) => Date.parse(left.startsAt) - Date.parse(right.startsAt)))
const nextRound = computed(() => findNextApiQuotaRound(sortedRounds.value, ownerNow.value))
const credentialSummaryQuery = useApiQuotaCredentialSummary(selectedCredentialOfferId)
const selectedSystemBatchLocked = computed(() => (roundsQuery.data.value ?? []).some(round =>
  Boolean(round.systemSlotKey)
  && ownerNow.value >= Date.parse(round.startsAt) - 60 * 60 * 1000,
))

onMounted(() => {
  ownerClock = setInterval(() => {
    ownerNow.value = Date.now()
  }, 1000)
})

onBeforeUnmount(() => {
  if (ownerClock) clearInterval(ownerClock)
})

watch(() => batchesQuery.data.value, rows => {
  if (!rows?.length) {
    selectedBatchId.value = ''
    return
  }
  if (!rows.some(item => item.id === selectedBatchId.value)) selectedBatchId.value = rows[0].id
}, { immediate: true })

watch(credentialOffers, rows => {
  if (!rows.some(item => item.id === selectedCredentialOfferId.value)) selectedCredentialOfferId.value = rows[0]?.id ?? ''
}, { immediate: true })

const batchForm = reactive({
  sourceType: (isSub2Api.value ? 'sub2api' : props.distributionSystem === 'new_api_proxy' || props.distributionSystem === 'NewAPI Proxy' ? 'new_api_proxy' : 'other') as ApiQuotaSourceType,
  sourceLabel: '',
  declaredTotalUsdAllowance: '1000',
  saleCutoffAt: '',
  expiresAt: '',
  sourceConfirmedAt: '',
})
const offerForm = reactive({
  name: '',
  usdAllowance: '50',
  priceCny: '5.00',
  deliveryMode: 'manual' as ApiQuotaOffer['deliveryMode'],
  deliveryEtaMinutes: 10,
  saleMode: 'continuous' as ApiQuotaOffer['saleMode'],
  continuousCopies: 10,
  sortOrder: 10,
})
const roundForm = reactive({
  name: '',
  startsAt: '',
  endsAt: '',
  copies: {} as Record<string, number>,
})

const createBatchMutation = useCreateApiQuotaBatchMutation()
const createOfferMutation = useCreateApiQuotaOfferMutation()
const createRoundMutation = useCreateApiQuotaRoundMutation()
const batchActionMutation = useApiQuotaBatchActionMutation()
const importMutation = useImportApiQuotaCredentialsMutation()

function toISO(value: string, label: string) {
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) throw new Error(`请填写有效的${label}。`)
  return parsed.toISOString()
}

function mutationMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

async function createBatch() {
  try {
    const created = await createBatchMutation.mutateAsync({
      apiServiceId: props.apiServiceId,
      sourceType: batchForm.sourceType,
      sourceLabel: batchForm.sourceType === 'other' ? batchForm.sourceLabel : undefined,
      declaredTotalUsdAllowance: batchForm.declaredTotalUsdAllowance,
      saleCutoffAt: toISO(batchForm.saleCutoffAt, '最晚下单时间'),
      expiresAt: toISO(batchForm.expiresAt, '额度失效时间'),
      sourceConfirmedAt: toISO(batchForm.sourceConfirmedAt, '来源确认时间'),
    })
    selectedBatchId.value = created.id
    batchDialogOpen.value = false
    toast.success('额度批次草稿已创建。')
  } catch (error) {
    toast.error(mutationMessage(error, '创建额度批次失败。'))
  }
}

async function createOffer() {
  if (!selectedBatch.value) return
  try {
    await createOfferMutation.mutateAsync({
      batchId: selectedBatch.value.id,
      name: offerForm.name,
      usdAllowance: offerForm.usdAllowance,
      priceCny: offerForm.priceCny,
      modelMultiplier: serviceDefaultMultiplierDecimal.value,
      deliveryMode: offerForm.deliveryMode,
      deliveryEtaMinutes: offerForm.deliveryEtaMinutes,
      saleMode: offerForm.saleMode,
      continuousCopies: offerForm.saleMode === 'continuous' ? offerForm.continuousCopies : 0,
      sortOrder: offerForm.sortOrder,
    })
    offerDialogOpen.value = false
    offerForm.name = ''
    toast.success('额度规格已加入草稿。')
  } catch (error) {
    toast.error(mutationMessage(error, '创建额度规格失败。'))
  }
}

function openRoundDialog() {
  roundForm.copies = Object.fromEntries(scheduledOffers.value.map(item => [item.id, 0]))
  roundDialogOpen.value = true
}

async function createRound() {
  if (!selectedBatch.value) return
  const allocations = scheduledOffers.value
    .map(item => ({ offerId: item.id, copies: Number(roundForm.copies[item.id] ?? 0) }))
    .filter(item => item.copies > 0)
  if (!allocations.length) {
    toast.error('至少为一个定时额度规格填写放量份数。')
    return
  }
  try {
    await createRoundMutation.mutateAsync({
      batchId: selectedBatch.value.id,
      name: roundForm.name,
      startsAt: toISO(roundForm.startsAt, '轮次开始时间'),
      endsAt: toISO(roundForm.endsAt, '轮次结束时间'),
      offers: allocations,
    })
    roundDialogOpen.value = false
    toast.success('放量轮次已创建。')
  } catch (error) {
    toast.error(mutationMessage(error, '创建放量轮次失败。'))
  }
}

async function runBatchAction(batch: ApiQuotaBatch, action: 'publish' | 'pause' | 'resume' | 'archive') {
  if ((action === 'pause' || action === 'archive') && !canPauseOrArchiveBatch(batch)) return
  if (action === 'pause' && !window.confirm('确认暂停这个额度批次？暂停后买家将无法从该批次创建新订单。')) return
  if (action === 'archive' && !window.confirm('确认归档这个额度批次？归档后不能继续销售。')) return
  try {
    await batchActionMutation.mutateAsync({ batchId: batch.id, action, version: batch.version })
    const labels = { publish: '已发布', pause: '已暂停', resume: '已恢复', archive: '已归档' }
    toast.success(`额度批次${labels[action]}。`)
  } catch (error) {
    toast.error(mutationMessage(error, '更新额度批次失败。'))
  }
}

function canPauseOrArchiveBatch(batch: ApiQuotaBatch) {
  return batch.id === selectedBatchId.value
    && !roundsQuery.isLoading.value
    && !selectedSystemBatchLocked.value
}

function batchActionDisabledReason(batch: ApiQuotaBatch) {
  if (batch.id !== selectedBatchId.value) return '选择批次后管理'
  if (roundsQuery.isLoading.value) return '正在读取放量锁定状态'
  if (selectedSystemBatchLocked.value) return '固定场次已进入锁定期'
  return ''
}

function statusDotClass(tone: ApiQuotaStatusTone) {
  return {
    success: 'bg-success',
    waiting: 'bg-primary',
    warning: 'bg-warning',
    neutral: 'bg-muted-foreground/60',
  }[tone]
}

function queryErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback
}

function copyOfferRoute(offer: ApiQuotaOffer) {
  return {
    path: '/api-market/quota/new',
    query: {
      serviceId: props.apiServiceId,
      copy: '1',
      name: offer.name,
      usdAllowance: offer.usdAllowance,
      priceCny: offer.priceCny,
      deliveryMode: offer.deliveryMode,
      deliveryEtaMinutes: String(offer.deliveryEtaMinutes),
    },
  }
}

function chooseCredentialFile(event: Event) {
  credentialFile.value = (event.target as HTMLInputElement).files?.[0] ?? null
}

async function importCredentials() {
  if (!selectedCredentialOfferId.value || !credentialFile.value) return
  try {
    const result = await importMutation.mutateAsync({ offerId: selectedCredentialOfferId.value, deliveryKind: deliveryKind.value, file: credentialFile.value })
    credentialFile.value = null
    toast.success(`已导入 ${result.imported} 条买家专属凭据。`)
  } catch (error) {
    toast.error(mutationMessage(error, '导入凭据失败。'))
  }
}

function downloadTemplate(kind: ApiOrderDeliveryKind) {
  const content = kind === 'api_key_endpoint'
    ? 'api_base_url,api_key,instructions\nhttps://api.example.com/v1,replace-with-buyer-key,optional note\n'
    : 'panel_login_url,username,password,instructions\nhttps://panel.example.com/login,buyer-account,initial-password,optional note\n'
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = kind === 'api_key_endpoint' ? 'api-key-credentials-template.csv' : 'login-account-credentials-template.csv'
  link.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <Card id="quota-offers" class="scroll-mt-20 rounded-lg p-5 shadow-xs md:p-6">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <div class="flex items-center gap-2">
          <Layers3 class="h-5 w-5 text-primary" />
          <h2 class="text-lg font-semibold">销售管理</h2>
        </div>
        <p class="mt-1 text-sm text-muted-foreground">统一管理额度来源、销售规格、放量节奏与交付凭据。</p>
      </div>

      <DropdownMenu>
        <DropdownMenuTrigger as-child>
          <Button>
            <Plus />
            新建销售计划
            <ChevronDown />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="w-52">
          <DropdownMenuItem @click="batchDialogOpen = true">
            <Layers3 />
            新建额度批次
          </DropdownMenuItem>
          <DropdownMenuItem as-child>
            <RouterLink :to="{ path: '/api-market/quota/new', query: { serviceId: apiServiceId } }">
              <PackagePlus />
              快速发布限时包
            </RouterLink>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>

    <Tabs v-model="salesTab" class="mt-5 gap-0">
      <div class="overflow-x-auto border-b border-border">
        <TabsList class="h-auto min-w-max rounded-none bg-transparent p-0">
          <TabsTrigger value="batches" class="rounded-none border-0 border-b-2 border-transparent px-3 py-2.5 shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-primary data-[state=active]:shadow-none">
            <Layers3 />
            额度批次
          </TabsTrigger>
          <TabsTrigger value="offers" class="rounded-none border-0 border-b-2 border-transparent px-3 py-2.5 shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-primary data-[state=active]:shadow-none">
            <PackageOpen />
            销售规格
          </TabsTrigger>
          <TabsTrigger value="rounds" class="rounded-none border-0 border-b-2 border-transparent px-3 py-2.5 shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-primary data-[state=active]:shadow-none">
            <CalendarClock />
            放量计划
          </TabsTrigger>
          <TabsTrigger value="credentials" class="rounded-none border-0 border-b-2 border-transparent px-3 py-2.5 shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-primary data-[state=active]:shadow-none">
            <KeyRound />
            交付凭据
          </TabsTrigger>
        </TabsList>
      </div>

      <TabsContent value="batches" class="mt-4">
        <SkeletonTable v-if="batchesQuery.isLoading.value" :rows="3" :columns="6" />
        <div v-else-if="batchesQuery.error.value" class="flex flex-col gap-3 rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm sm:flex-row sm:items-center sm:justify-between">
          <div class="flex items-start gap-2 text-destructive">
            <CircleAlert class="mt-0.5 h-4 w-4 shrink-0" />
            <span>{{ queryErrorMessage(batchesQuery.error.value, '额度批次暂时无法加载。') }}</span>
          </div>
          <Button size="sm" variant="outline" @click="batchesQuery.refetch()">重新加载</Button>
        </div>
        <div v-else-if="!batchesQuery.data.value?.length" class="flex flex-col items-start gap-3 rounded-lg border border-dashed border-border bg-muted/30 p-5">
          <span class="grid h-10 w-10 place-items-center rounded-lg bg-primary/10 text-primary"><Layers3 class="h-5 w-5" /></span>
          <div><h3 class="font-medium">还没有额度批次</h3><p class="mt-1 text-sm text-muted-foreground">先声明美元额度来源、最晚下单时间和绝对失效时间。</p></div>
          <Button size="sm" @click="batchDialogOpen = true"><Plus />新建额度批次</Button>
        </div>
        <template v-else>
          <div class="overflow-hidden rounded-lg border border-border">
            <Table class="min-w-[840px]">
              <TableHeader class="bg-muted/40">
                <TableRow>
                  <TableHead>批次</TableHead>
                  <TableHead>声明总额</TableHead>
                  <TableHead>尚未划拨</TableHead>
                  <TableHead>最晚下单</TableHead>
                  <TableHead>额度失效</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead class="w-14 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                <TableRow v-for="batch in batchesQuery.data.value" :key="batch.id" :data-state="selectedBatchId === batch.id ? 'selected' : undefined">
                  <TableCell>
                    <button type="button" class="inline-flex items-center gap-2 font-mono font-medium hover:text-primary" @click="selectedBatchId = batch.id">
                      <span class="grid h-7 w-7 place-items-center rounded-md bg-primary/10 text-primary"><Box class="h-3.5 w-3.5" /></span>
                      {{ batch.id.slice(-8) }}
                    </button>
                  </TableCell>
                  <TableCell class="font-medium">${{ formatDecimal(batch.declaredTotalUsdAllowance, 0, 6) }}</TableCell>
                  <TableCell>${{ formatDecimal(batch.unallocatedUsdAllowance, 0, 6) }}</TableCell>
                  <TableCell><LocalTime :value="batch.saleCutoffAt" /></TableCell>
                  <TableCell><LocalTime :value="batch.expiresAt" /></TableCell>
                  <TableCell>
                    <span class="inline-flex items-center gap-2 whitespace-nowrap">
                      <span class="h-2 w-2 rounded-full" :class="statusDotClass(getApiQuotaBatchStatus(batch.status).tone)" />
                      {{ getApiQuotaBatchStatus(batch.status).label }}
                    </span>
                  </TableCell>
                  <TableCell class="text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger as-child>
                        <Button variant="ghost" size="icon-sm" :aria-label="`管理批次 ${batch.id.slice(-8)}`">
                          <MoreHorizontal />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" class="w-52">
                        <DropdownMenuItem v-if="batch.status === 'draft'" :disabled="batchActionMutation.isPending.value" @click="runBatchAction(batch, 'publish')">
                          <Rocket />
                          发布批次
                        </DropdownMenuItem>
                        <DropdownMenuItem v-if="batch.status === 'published'" :disabled="Boolean(batchActionDisabledReason(batch)) || batchActionMutation.isPending.value" @click="runBatchAction(batch, 'pause')">
                          <Pause />
                          暂停销售
                        </DropdownMenuItem>
                        <DropdownMenuItem v-if="batch.status === 'paused'" :disabled="batchActionMutation.isPending.value" @click="runBatchAction(batch, 'resume')">
                          <Play />
                          恢复销售
                        </DropdownMenuItem>
                        <DropdownMenuSeparator v-if="batch.status !== 'archived'" />
                        <DropdownMenuItem v-if="batch.status !== 'archived'" variant="destructive" :disabled="Boolean(batchActionDisabledReason(batch)) || batchActionMutation.isPending.value" @click="runBatchAction(batch, 'archive')">
                          <Archive />
                          归档批次
                        </DropdownMenuItem>
                        <DropdownMenuItem v-if="batch.status === 'archived'" disabled>
                          <Archive />
                          批次已归档
                        </DropdownMenuItem>
                        <template v-if="batchActionDisabledReason(batch)">
                          <DropdownMenuSeparator />
                          <DropdownMenuItem disabled class="text-xs">{{ batchActionDisabledReason(batch) }}</DropdownMenuItem>
                        </template>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>

          <div v-if="selectedBatch" class="mt-4 grid overflow-hidden rounded-lg border border-border bg-muted/20 sm:grid-cols-3">
            <button type="button" class="flex min-w-0 items-center gap-3 p-3 text-left transition-colors hover:bg-muted/60 sm:border-r sm:border-border" @click="salesTab = 'offers'">
              <span class="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary"><Boxes class="h-4 w-4" /></span>
              <span class="min-w-0"><small class="block text-muted-foreground">销售规格</small><strong class="block truncate">{{ offers.length }} 个</strong></span>
            </button>
            <button type="button" class="flex min-w-0 items-center gap-3 border-t border-border p-3 text-left transition-colors hover:bg-muted/60 sm:border-t-0 sm:border-r" @click="salesTab = 'rounds'">
              <span class="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-warning/10 text-warning"><CalendarClock class="h-4 w-4" /></span>
              <span class="min-w-0"><small class="block text-muted-foreground">下次放量</small><strong class="block truncate"><LocalTime v-if="nextRound" :value="nextRound.startsAt" /><template v-else>暂无计划</template></strong></span>
            </button>
            <button type="button" class="flex min-w-0 items-center gap-3 border-t border-border p-3 text-left transition-colors hover:bg-muted/60 sm:border-t-0" @click="salesTab = 'credentials'">
              <span class="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-signal-soft text-signal"><KeyRound class="h-4 w-4" /></span>
              <span class="min-w-0"><small class="block text-muted-foreground">交付凭据</small><strong class="block truncate">{{ credentialSummaryQuery.data.value ? `库存 ${credentialSummaryQuery.data.value.available} 份` : credentialOffers.length ? '待读取库存' : '未配置' }}</strong></span>
            </button>
          </div>
        </template>
      </TabsContent>

      <TabsContent value="offers" class="mt-4">
        <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p class="text-sm text-muted-foreground">当前批次：{{ selectedBatch ? selectedBatch.id.slice(-8) : '未选择' }}</p>
          <Button v-if="selectedBatch?.status === 'draft'" size="sm" @click="offerDialogOpen = true"><Plus />新增规格</Button>
        </div>
        <div v-if="!selectedBatch" class="rounded-lg border border-dashed border-border p-5 text-sm text-muted-foreground">请先在“额度批次”中选择一个批次。</div>
        <SkeletonTable v-else-if="offersQuery.isLoading.value" :rows="3" :columns="7" />
        <div v-else-if="offersQuery.error.value" class="flex items-center justify-between gap-3 rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          <span>{{ queryErrorMessage(offersQuery.error.value, '销售规格暂时无法加载。') }}</span>
          <Button size="sm" variant="outline" @click="offersQuery.refetch()">重新加载</Button>
        </div>
        <div v-else-if="!offers.length" class="flex items-center gap-3 rounded-lg border border-dashed border-border p-5">
          <PackageOpen class="h-5 w-5 text-muted-foreground" />
          <p class="text-sm text-muted-foreground">当前批次还没有销售规格。</p>
        </div>
        <div v-else class="overflow-hidden rounded-lg border border-border">
          <Table class="min-w-[900px]">
            <TableHeader class="bg-muted/40"><TableRow><TableHead>规格</TableHead><TableHead>额度 / 总价</TableHead><TableHead>倍率</TableHead><TableHead>销售方式</TableHead><TableHead>交付方式</TableHead><TableHead>状态</TableHead><TableHead class="w-14 text-right">操作</TableHead></TableRow></TableHeader>
            <TableBody>
              <TableRow v-for="offer in offers" :key="offer.id">
                <TableCell class="font-medium">{{ offer.name }}</TableCell>
                <TableCell>${{ formatDecimal(offer.usdAllowance, 0, 6) }} / ¥{{ formatDecimal(offer.priceCny, 2, 2) }}</TableCell>
                <TableCell>{{ Number(offer.modelMultiplier).toFixed(2) }}x</TableCell>
                <TableCell>{{ getApiQuotaSaleModeLabel(offer.saleMode) }}</TableCell>
                <TableCell>{{ getApiQuotaDeliveryModeLabel(offer.deliveryMode) }} · ≤{{ offer.deliveryEtaMinutes }} 分钟</TableCell>
                <TableCell><span class="inline-flex items-center gap-2"><span class="h-2 w-2 rounded-full" :class="statusDotClass(getApiQuotaOfferStatus(offer.status).tone)" />{{ getApiQuotaOfferStatus(offer.status).label }}</span></TableCell>
                <TableCell class="text-right">
                  <DropdownMenu>
                    <DropdownMenuTrigger as-child><Button variant="ghost" size="icon-sm" :aria-label="`管理规格 ${offer.name}`"><MoreHorizontal /></Button></DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem v-if="offer.saleMode === 'scheduled'" as-child>
                        <RouterLink :to="copyOfferRoute(offer)"><CopyPlus />复制再发</RouterLink>
                      </DropdownMenuItem>
                      <DropdownMenuItem v-else disabled><PackageOpen />暂无可用操作</DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </TabsContent>

      <TabsContent value="rounds" class="mt-4">
        <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p class="text-sm text-muted-foreground">一个计划可为多个定时规格配置独立份数。</p>
          <Button v-if="selectedBatch?.status === 'draft' && scheduledOffers.length" size="sm" @click="openRoundDialog"><TimerReset />新增放量计划</Button>
        </div>
        <div v-if="!selectedBatch" class="rounded-lg border border-dashed border-border p-5 text-sm text-muted-foreground">请先在“额度批次”中选择一个批次。</div>
        <SkeletonTable v-else-if="roundsQuery.isLoading.value" :rows="3" :columns="5" />
        <div v-else-if="roundsQuery.error.value" class="flex items-center justify-between gap-3 rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
          <span>{{ queryErrorMessage(roundsQuery.error.value, '放量计划暂时无法加载。') }}</span>
          <Button size="sm" variant="outline" @click="roundsQuery.refetch()">重新加载</Button>
        </div>
        <div v-else-if="!sortedRounds.length" class="flex items-center gap-3 rounded-lg border border-dashed border-border p-5">
          <CalendarClock class="h-5 w-5 text-muted-foreground" />
          <p class="text-sm text-muted-foreground">当前批次没有放量计划；全天可买规格不需要计划。</p>
        </div>
        <div v-else class="overflow-hidden rounded-lg border border-border">
          <Table class="min-w-[780px]">
            <TableHeader class="bg-muted/40"><TableRow><TableHead>计划</TableHead><TableHead>开始时间</TableHead><TableHead>结束时间</TableHead><TableHead>规格份数</TableHead><TableHead>状态</TableHead></TableRow></TableHeader>
            <TableBody>
              <TableRow v-for="round in sortedRounds" :key="round.id">
                <TableCell class="font-medium">{{ round.name }}</TableCell>
                <TableCell><LocalTime :value="round.startsAt" /></TableCell>
                <TableCell><LocalTime :value="round.endsAt" /></TableCell>
                <TableCell class="max-w-80 whitespace-normal">{{ round.allocations.map(item => `${offers.find(offer => offer.id === item.offerId)?.name || item.offerId}: ${item.copyLimit} 份`).join(' / ') }}</TableCell>
                <TableCell><span class="inline-flex items-center gap-2 whitespace-nowrap"><span class="h-2 w-2 rounded-full" :class="statusDotClass(getApiQuotaRoundStatus(round, ownerNow).tone)" />{{ getApiQuotaRoundStatus(round, ownerNow).label }}</span></TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </TabsContent>

      <TabsContent value="credentials" class="mt-4">
        <div v-if="!selectedBatch" class="rounded-lg border border-dashed border-border p-5 text-sm text-muted-foreground">请先在“额度批次”中选择一个批次。</div>
        <div v-else-if="credentialOffers.length === 0" class="flex flex-col items-start gap-3 rounded-lg border border-dashed border-border bg-muted/20 p-5 sm:flex-row sm:items-center">
          <span class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-signal-soft text-signal"><FileKey2 class="h-5 w-5" /></span>
          <div class="min-w-0 flex-1"><h3 class="font-medium">暂未配置交付凭据</h3><p class="mt-1 text-sm text-muted-foreground">新增规格时选择“预导入凭据”，再导入对应的买家专属接入信息。</p></div>
          <Button v-if="selectedBatch.status === 'draft'" size="sm" variant="outline" @click="offerDialogOpen = true"><Plus />新增凭据规格</Button>
        </div>
        <template v-else>
          <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_180px_minmax(0,1fr)_auto] lg:items-end">
            <label class="space-y-2"><span class="text-sm font-medium">销售规格</span><Select v-model="selectedCredentialOfferId"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem v-for="offer in credentialOffers" :key="offer.id" :value="offer.id">{{ offer.name }}</SelectItem></SelectContent></Select></label>
            <label class="space-y-2"><span class="text-sm font-medium">CSV 模板</span><Select v-model="deliveryKind"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="api_key_endpoint">API Key</SelectItem><SelectItem value="login_account">登录账号</SelectItem></SelectContent></Select></label>
            <label class="space-y-2"><span class="text-sm font-medium">CSV 文件</span><Input type="file" accept=".csv,text/csv" @change="chooseCredentialFile" /></label>
            <Button :disabled="!credentialFile || importMutation.isPending.value" @click="importCredentials"><FileUp />{{ importMutation.isPending.value ? '导入中...' : '导入凭据' }}</Button>
          </div>
          <div class="mt-3 flex flex-wrap gap-2"><Button size="sm" variant="outline" @click="downloadTemplate('api_key_endpoint')"><Download />API Key 模板</Button><Button size="sm" variant="outline" @click="downloadTemplate('login_account')"><Download />登录账号模板</Button></div>
          <p v-if="credentialFile" class="mt-3 text-xs text-muted-foreground">待导入：{{ credentialFile.name }} · {{ Math.ceil(credentialFile.size / 1024) }} KiB。提交后由服务端整批校验、去重和加密。</p>
          <div v-if="credentialSummaryQuery.error.value" class="mt-4 flex items-center justify-between gap-3 rounded-lg border border-destructive/20 bg-destructive/5 p-4 text-sm text-destructive">
            <span>{{ queryErrorMessage(credentialSummaryQuery.error.value, '凭据库存暂时无法加载。') }}</span>
            <Button size="sm" variant="outline" @click="credentialSummaryQuery.refetch()">重新加载</Button>
          </div>
          <SkeletonTable v-else-if="credentialSummaryQuery.isLoading.value" class="mt-4" :rows="1" :columns="4" />
          <dl v-else-if="credentialSummaryQuery.data.value" class="mt-4 grid overflow-hidden rounded-lg border border-border sm:grid-cols-4">
            <div class="flex items-center gap-3 p-4 sm:border-r sm:border-border"><span class="grid h-9 w-9 place-items-center rounded-lg bg-success/10 text-success"><WalletCards class="h-4 w-4" /></span><div><dt class="text-xs text-muted-foreground">可用</dt><dd class="text-lg font-semibold">{{ credentialSummaryQuery.data.value.available }}</dd></div></div>
            <div class="flex items-center gap-3 border-t border-border p-4 sm:border-t-0 sm:border-r"><span class="grid h-9 w-9 place-items-center rounded-lg bg-primary/10 text-primary"><LockKeyhole class="h-4 w-4" /></span><div><dt class="text-xs text-muted-foreground">已预留</dt><dd class="text-lg font-semibold">{{ credentialSummaryQuery.data.value.reserved }}</dd></div></div>
            <div class="flex items-center gap-3 border-t border-border p-4 sm:border-t-0 sm:border-r"><span class="grid h-9 w-9 place-items-center rounded-lg bg-signal-soft text-signal"><CheckCircle2 class="h-4 w-4" /></span><div><dt class="text-xs text-muted-foreground">已交付</dt><dd class="text-lg font-semibold">{{ credentialSummaryQuery.data.value.delivered }}</dd></div></div>
            <div class="flex items-center gap-3 border-t border-border p-4 sm:border-t-0"><span class="grid h-9 w-9 place-items-center rounded-lg bg-muted text-muted-foreground"><Archive class="h-4 w-4" /></span><div><dt class="text-xs text-muted-foreground">已退役</dt><dd class="text-lg font-semibold">{{ credentialSummaryQuery.data.value.retired }}</dd></div></div>
          </dl>
          <p class="mt-3 text-xs leading-5 text-muted-foreground">原始凭据不会进入公开列表或管理摘要，只在卖家确认收款后向对应买家展示。</p>
        </template>
      </TabsContent>
    </Tabs>
  </Card>

    <Dialog v-model:open="batchDialogOpen">
      <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-[620px]"><DialogHeader><DialogTitle>新建额度批次</DialogTitle><DialogDescription>声明卖家站外控制的美元额度来源。最晚下单不能晚于失效前 1 小时。</DialogDescription></DialogHeader>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-2"><span class="text-sm font-medium">来源类型</span><Select v-model="batchForm.sourceType"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="sub2api">Sub2API</SelectItem><SelectItem value="new_api_proxy">NewAPI</SelectItem><SelectItem value="self_hosted">自建中转</SelectItem><SelectItem value="other">其他</SelectItem></SelectContent></Select></label>
          <label v-if="batchForm.sourceType === 'other'" class="space-y-2"><span class="text-sm font-medium">来源说明</span><Input v-model="batchForm.sourceLabel" maxlength="80" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">声明总美元额度</span><Input v-model="batchForm.declaredTotalUsdAllowance" inputmode="decimal" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">来源最近确认时间</span><Input v-model="batchForm.sourceConfirmedAt" type="datetime-local" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">最晚下单时间</span><Input v-model="batchForm.saleCutoffAt" type="datetime-local" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">额度失效时间</span><Input v-model="batchForm.expiresAt" type="datetime-local" /></label>
        </div>
        <DialogFooter><Button variant="outline" @click="batchDialogOpen = false">取消</Button><Button :disabled="createBatchMutation.isPending.value" @click="createBatch">创建草稿</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="offerDialogOpen">
      <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-[620px]"><DialogHeader><DialogTitle>新增固定额度规格</DialogTitle><DialogDescription>商业字段发布后不可原地修改；需要调整时创建新版本。</DialogDescription></DialogHeader>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-2 sm:col-span-2"><span class="text-sm font-medium">规格名称</span><Input v-model="offerForm.name" maxlength="80" placeholder="$50 日内额度" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">美元额度</span><Input v-model="offerForm.usdAllowance" inputmode="decimal" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">人民币总价</span><Input v-model="offerForm.priceCny" inputmode="decimal" /></label>
          <div class="space-y-2"><span class="text-sm font-medium">服务倍率</span><div class="flex h-10 items-center rounded-md border border-border bg-muted/40 px-3 text-sm font-semibold">{{ serviceDefaultMultiplier.toFixed(2) }}x</div><p class="text-xs text-muted-foreground">沿用基础服务，额度规格无需重复设置。</p></div>
          <label class="space-y-2"><span class="text-sm font-medium">最长交付分钟</span><Input v-model.number="offerForm.deliveryEtaMinutes" type="number" min="1" max="10" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">销售方式</span><Select v-model="offerForm.saleMode"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="continuous">全天可买</SelectItem><SelectItem value="scheduled">定时放量</SelectItem></SelectContent></Select></label>
          <label class="space-y-2"><span class="text-sm font-medium">交付方式</span><Select v-model="offerForm.deliveryMode"><SelectTrigger><SelectValue /></SelectTrigger><SelectContent><SelectItem value="manual">商户手工交付</SelectItem><SelectItem value="preimported">预导入凭据</SelectItem></SelectContent></Select></label>
          <label v-if="offerForm.saleMode === 'continuous'" class="space-y-2"><span class="text-sm font-medium">全天可售份数</span><Input v-model.number="offerForm.continuousCopies" type="number" min="1" max="100000" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">排序</span><Input v-model.number="offerForm.sortOrder" type="number" /></label>
        </div>
        <DialogFooter><Button variant="outline" @click="offerDialogOpen = false">取消</Button><Button :disabled="createOfferMutation.isPending.value" @click="createOffer">加入批次</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="roundDialogOpen">
      <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-[620px]"><DialogHeader><DialogTitle>新增放量轮次</DialogTitle><DialogDescription>为同一轮的每个定时额度规格分别设置权威份数。</DialogDescription></DialogHeader>
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-2 sm:col-span-2"><span class="text-sm font-medium">轮次名称</span><Input v-model="roundForm.name" maxlength="80" placeholder="09:00 上班场" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">开始时间</span><Input v-model="roundForm.startsAt" type="datetime-local" /></label>
          <label class="space-y-2"><span class="text-sm font-medium">结束时间</span><Input v-model="roundForm.endsAt" type="datetime-local" /></label>
        </div>
        <div class="space-y-2"><div class="text-sm font-medium">各规格放量份数</div><label v-for="offer in scheduledOffers" :key="offer.id" class="grid grid-cols-[1fr_140px] items-center gap-3 rounded-md border border-border px-3 py-2 text-sm"><span>{{ offer.name }} · ${{ formatDecimal(offer.usdAllowance, 0, 6) }}</span><Input v-model.number="roundForm.copies[offer.id]" type="number" min="0" max="100000" /></label></div>
        <DialogFooter><Button variant="outline" @click="roundDialogOpen = false">取消</Button><Button :disabled="createRoundMutation.isPending.value" @click="createRound">创建轮次</Button></DialogFooter>
      </DialogContent>
    </Dialog>
</template>
