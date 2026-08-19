<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { ArrowLeft, Check, FileText, MessageCircle, Settings2, ShieldAlert, UsersRound, X } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import CompactStats from '@/components/market/CompactStats.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import ShortId from '@/components/market/ShortId.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import StatusTabs from '@/components/market/StatusTabs.vue'
import {
  acceptCarpoolApplication,
  getCarpoolApplicationStatusLabel,
  rejectCarpoolApplication,
  removeCarpoolMember,
  updateCarpoolMembershipOwnerNote,
  type CarpoolApplicationWithMeta,
  type CarpoolWithMeta,
} from '@/lib/api'
import { useCarpool, useMerchantCarpoolApplications } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const carpoolId = computed(() => String(route.params.id ?? ''))
const activeTab = ref('概览')
const actionId = ref('')
const noteSavingId = ref('')
const noteDrafts = reactive<Record<string, string>>({})
const removeDialogOpen = ref(false)
const selectedMember = ref<CarpoolApplicationWithMeta | null>(null)
const removeReason = ref('')

const carpoolQuery = useCarpool(carpoolId)
const applicationFilters = computed(() => ({ carpoolId: carpoolId.value, sort: 'default_owner' as const }))
const applicationsQuery = useMerchantCarpoolApplications(applicationFilters, computed(() => Boolean(carpoolId.value)))
const carpool = computed(() => carpoolQuery.data.value)
const applications = computed(() => applicationsQuery.data.value ?? [])
const isLoading = computed(() => carpoolQuery.isLoading.value || applicationsQuery.isLoading.value)
const pendingApplications = computed(() => applications.value.filter(item => item.status === 'pending_owner'))
const currentMembers = computed(() => applications.value.filter(isCurrentMember))
const historyMembers = computed(() => applications.value.filter(isHistoryMember))
const occupiedSeats = computed(() => carpool.value?.seatSummary?.occupiedSeatCount ?? carpool.value?.currentConfirmedMembers ?? 0)
const totalSeats = computed(() => carpool.value?.seatSummary?.totalSeats ?? carpool.value?.maxMembers ?? 0)
const offlineSeats = computed(() => (carpool.value as CarpoolWithMeta | null)?.offlineOccupiedSeats ?? 0)
const availableSeats = computed(() => carpool.value?.seatSummary?.availableSeats ?? Math.max(0, totalSeats.value - occupiedSeats.value))

watch(applications, rows => {
  for (const row of rows) {
    if (!(row.id in noteDrafts)) noteDrafts[row.id] = row.ownerNote ?? ''
  }
}, { immediate: true })

function isCurrentMember(item: CarpoolApplicationWithMeta) {
  return item.backendMembershipId !== undefined && (item.backendStatus === 'active' || item.status === 'active')
}

function isHistoryMember(item: CarpoolApplicationWithMeta) {
  return item.backendMembershipId !== undefined
    && (item.backendStatus === 'left' || item.backendStatus === 'removed' || item.status === 'cancelled_by_buyer' || item.status === 'cancelled_by_owner')
}

function joinedAt(item: CarpoolApplicationWithMeta) {
  return item.backendMembershipJoinedAt ?? item.startedAt ?? item.createdAt
}

function noteDirty(item: CarpoolApplicationWithMeta) {
  return (noteDrafts[item.id] ?? '').trim() !== (item.ownerNote ?? '').trim()
}

async function refresh() {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ['merchant-carpool-applications'] }),
    queryClient.invalidateQueries({ queryKey: ['my-carpools'] }),
    queryClient.invalidateQueries({ queryKey: ['carpools', carpoolId.value] }),
    queryClient.invalidateQueries({ queryKey: ['carpools'] }),
    queryClient.invalidateQueries({ queryKey: ['navigation-badges'] }),
    queryClient.invalidateQueries({ queryKey: ['carpool-application'] }),
  ])
}

async function runApplicationAction(item: CarpoolApplicationWithMeta, action: () => Promise<unknown>, message: string) {
  actionId.value = item.id
  try {
    await action()
    await refresh()
    toast.success(message)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '操作失败')
  } finally {
    actionId.value = ''
  }
}

function acceptApplication(item: CarpoolApplicationWithMeta) {
  void runApplicationAction(item, () => acceptCarpoolApplication(item.id), '已确认上车，成员关系已生效。')
}

function rejectApplication(item: CarpoolApplicationWithMeta) {
  void runApplicationAction(item, () => rejectCarpoolApplication(item.id, '车主暂不接受该申请'), '已拒绝申请。')
}

async function saveNote(item: CarpoolApplicationWithMeta) {
  noteSavingId.value = item.id
  try {
    const updated = await updateCarpoolMembershipOwnerNote(item, noteDrafts[item.id] ?? '')
    noteDrafts[item.id] = updated.ownerNote ?? ''
    await refresh()
    toast.success('私有备注已保存。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '备注保存失败，请刷新后重试。')
  } finally {
    noteSavingId.value = ''
  }
}

function openRemoveDialog(item: CarpoolApplicationWithMeta) {
  selectedMember.value = item
  removeReason.value = ''
  removeDialogOpen.value = true
}

async function confirmRemoveMember() {
  const item = selectedMember.value
  if (!item) return
  actionId.value = item.id
  try {
    await removeCarpoolMember(item.id, removeReason.value.trim())
    removeDialogOpen.value = false
    selectedMember.value = null
    await refresh()
    toast.success('成员已移除，并已进入历史成员。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '移除成员失败，请刷新后重试。')
  } finally {
    actionId.value = ''
  }
}
</script>

<template>
  <div v-if="isLoading" class="space-y-5">
    <div class="h-24 animate-pulse rounded-xl bg-muted" />
    <SkeletonTable :rows="5" :columns="7" />
  </div>
  <EmptyState v-else-if="!carpool" title="未找到车源" description="车源不存在、已下架或当前账号无权查看。">
    <template #action><RouterLink to="/my/carpools"><Button variant="outline">返回拼车管理</Button></RouterLink></template>
  </EmptyState>
  <div v-else class="space-y-5">
    <header class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div class="min-w-0">
        <Button class="mb-3 px-0" variant="ghost" @click="router.push('/my/carpools')"><ArrowLeft class="h-4 w-4" />返回拼车管理</Button>
        <div class="flex flex-wrap items-center gap-2">
          <h1 class="text-2xl font-semibold md:text-3xl">{{ carpool.product }}</h1>
          <Badge variant="secondary">{{ carpool.region }}</Badge>
          <Badge>{{ carpool.status }}</Badge>
        </div>
        <div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <ShortId :value="carpool.id" prefix="CAR" />
          <span>· {{ carpool.owner }}</span>
          <span>· 更新于 <LocalTime :value="carpool.confirmedAt" /></span>
        </div>
      </div>
      <div class="flex flex-wrap gap-2">
        <RouterLink :to="`/my/carpools/${carpool.id}/edit`"><Button variant="outline"><Settings2 class="h-4 w-4" />车源设置</Button></RouterLink>
        <RouterLink to="/my/carpools?view=applications"><Button variant="outline"><ShieldAlert class="h-4 w-4" />纠纷与异常</Button></RouterLink>
      </div>
    </header>

    <CompactStats :items="[
      { label: '已占席位', value: `${occupiedSeats} / ${totalSeats}` },
      { label: '平台成员', value: currentMembers.length },
      { label: '线下占用', value: `${offlineSeats} 席` },
      { label: '剩余席位', value: `${availableSeats} 席` },
      { label: '待处理申请', value: pendingApplications.length },
    ]" :loading="applicationsQuery.isFetching.value" />

    <StatusTabs v-model="activeTab" :items="['概览', '上车申请', '当前成员', '历史成员']" />

    <section v-if="activeTab === '概览'" class="grid gap-5 lg:grid-cols-[1.2fr_0.8fr]">
      <Card class="p-5">
        <div class="flex items-start gap-3">
          <span class="rounded-lg bg-primary/10 p-2 text-primary"><UsersRound class="h-5 w-5" /></span>
          <div><h2 class="text-lg font-semibold">成员关系概览</h2><p class="mt-1 text-sm text-muted-foreground">申请通过后才占用席位；当前成员和历史成员都以这条车源为上下文管理。</p></div>
        </div>
        <dl class="mt-5 grid gap-4 sm:grid-cols-2">
          <div class="rounded-lg border border-border p-4"><dt class="text-xs text-muted-foreground">当前成员</dt><dd class="mt-1 text-xl font-semibold">{{ currentMembers.length }} 人</dd><p class="mt-1 text-xs text-muted-foreground">已建立有效成员关系</p></div>
          <div class="rounded-lg border border-border p-4"><dt class="text-xs text-muted-foreground">历史成员</dt><dd class="mt-1 text-xl font-semibold">{{ historyMembers.length }} 人</dd><p class="mt-1 text-xs text-muted-foreground">主动退出或车主移除</p></div>
        </dl>
      </Card>
      <Card class="p-5">
        <div class="flex items-start gap-3"><FileText class="mt-0.5 h-5 w-5 text-muted-foreground" /><div><h2 class="text-lg font-semibold">快捷处理</h2><p class="mt-1 text-sm text-muted-foreground">申请记录和成员关系分开保存，详情页保留完整条件快照。</p></div></div>
        <div class="mt-5 grid gap-2 sm:grid-cols-2 lg:grid-cols-1">
          <Button class="justify-start" variant="outline" @click="activeTab = '上车申请'"><MessageCircle class="h-4 w-4" />查看待处理申请<span class="ml-auto">{{ pendingApplications.length }}</span></Button>
          <Button class="justify-start" variant="outline" @click="activeTab = '当前成员'"><UsersRound class="h-4 w-4" />查看当前成员<span class="ml-auto">{{ currentMembers.length }}</span></Button>
        </div>
      </Card>
    </section>

    <section v-else-if="activeTab === '上车申请'">
      <EmptyState v-if="pendingApplications.length === 0" title="暂无待处理申请" description="新的申请会出现在这里，待处理申请不占用席位。" />
      <SoftTable v-else animate-rows :columns="['申请人', '条件快照', '申请席位', '申请时间', '操作']">
        <tr v-for="item in pendingApplications" :key="item.id">
          <td><RouterLink :to="`/u/${item.applicantUsername}`" class="font-medium hover:underline">{{ item.applicantUsername }}</RouterLink><div class="mt-1 text-xs text-muted-foreground"><ShortId :value="item.id" prefix="RIDE" /></div></td>
          <td><div class="font-medium">{{ item.snapshot.priceLabel }} ¥{{ item.snapshot.monthlyPriceCny }}</div><div class="mt-1 text-xs text-muted-foreground">{{ item.snapshot.regionName }} · 条件版本 {{ item.conditionsVersionSnapshot ?? item.snapshot.rulesVersion }}</div></td>
          <td>{{ item.seatsRequested }} 席</td>
          <td class="text-muted-foreground"><LocalTime :value="item.createdAt" /></td>
          <td><div class="flex flex-wrap gap-2"><Button size="sm" :disabled="actionId === item.id" @click="acceptApplication(item)"><Check class="h-4 w-4" />确认上车</Button><Button size="sm" variant="outline" :disabled="actionId === item.id" @click="rejectApplication(item)"><X class="h-4 w-4" />拒绝</Button><RouterLink :to="`/merchant/carpool-applications/${item.id}`"><Button size="sm" variant="ghost">详情</Button></RouterLink></div></td>
        </tr>
      </SoftTable>
    </section>

    <section v-else>
      <EmptyState v-if="(activeTab === '当前成员' ? currentMembers : historyMembers).length === 0" :title="activeTab === '当前成员' ? '暂无当前成员' : '暂无历史成员'" :description="activeTab === '当前成员' ? '确认上车后，成员会出现在这里。' : '主动退出或被移除的成员会保留在这里；被拒绝申请不会进入历史成员。'" />
      <SoftTable v-else :columns="['成员', '加入时间', '占用席位', '加入时价格', '条件快照', '联系方式', '车主私有备注', '操作']">
        <tr v-for="item in activeTab === '当前成员' ? currentMembers : historyMembers" :key="item.id" class="align-top">
          <td><RouterLink :to="`/u/${item.applicantUsername}`" class="font-medium hover:underline">{{ item.applicantUsername }}</RouterLink><div class="mt-1 text-xs text-muted-foreground"><ShortId :value="item.id" prefix="RIDE" /></div></td>
          <td class="whitespace-nowrap text-muted-foreground"><LocalTime :value="joinedAt(item)" /></td>
          <td>{{ item.seatsRequested }} 席</td>
          <td><div class="font-semibold">¥{{ item.snapshot.monthlyPriceCny }}</div><div class="mt-1 text-xs text-muted-foreground">{{ item.snapshot.priceLabel }}</div></td>
          <td><RouterLink :to="`/merchant/carpool-applications/${item.id}`"><Button size="sm" variant="outline"><FileText class="h-4 w-4" />查看快照</Button></RouterLink></td>
          <td><RouterLink :to="`/merchant/carpool-applications/${item.id}`"><Button size="sm" variant="ghost"><MessageCircle class="h-4 w-4" />查看联系方式</Button></RouterLink></td>
          <td class="min-w-56"><Textarea v-model="noteDrafts[item.id]" class="min-h-20 text-sm" maxlength="500" placeholder="仅自己可见，可留空" /><div class="mt-2 flex items-center justify-between gap-2"><span class="text-xs text-muted-foreground">{{ (noteDrafts[item.id] ?? '').length }} / 500</span><Button size="sm" variant="outline" :disabled="!noteDirty(item) || noteSavingId === item.id" @click="saveNote(item)">{{ noteSavingId === item.id ? '保存中…' : '保存备注' }}</Button></div></td>
          <td><Button v-if="activeTab === '当前成员'" size="sm" variant="outline" :disabled="actionId === item.id" @click="openRemoveDialog(item)"><ShieldAlert class="h-4 w-4" />移除成员</Button><Badge v-else variant="secondary">{{ item.status === 'cancelled_by_buyer' ? '主动退出' : '车主移除' }}</Badge></td>
        </tr>
      </SoftTable>
    </section>

    <Dialog v-model:open="removeDialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>移除成员</DialogTitle>
          <DialogDescription>移除后会释放该成员占用的席位，并保留在历史成员中。原因可以留空。</DialogDescription>
        </DialogHeader>
        <div v-if="selectedMember" class="space-y-3">
          <div class="rounded-lg border border-border bg-muted/30 p-3 text-sm"><span class="text-muted-foreground">成员：</span>{{ selectedMember.applicantUsername }}<span class="mx-2 text-muted-foreground">·</span>{{ selectedMember.snapshot.productName }}</div>
          <label class="space-y-2 text-sm"><span class="font-medium">移除备注（选填）</span><Textarea v-model="removeReason" class="min-h-24" maxlength="500" placeholder="可以不填写原因" /></label>
        </div>
        <DialogFooter><Button variant="outline" @click="removeDialogOpen = false">取消</Button><Button variant="destructive" :disabled="!selectedMember || Boolean(actionId)" @click="confirmRemoveMember"><ShieldAlert class="h-4 w-4" />确认移除</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
