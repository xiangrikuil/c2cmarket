<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { CheckCircle2, Eye, MoreHorizontal, ShieldAlert } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Textarea } from '@/components/ui/textarea'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { runAdminModerationAction, updateAdminRowStatus, type AdminRow, type AdminSection } from '@/lib/api'
import { useAdminSectionRows } from '@/queries/useMarketQueries'
import { toast } from 'vue-sonner'

const route = useRoute()
const queryClient = useQueryClient()

const title = computed(() => String(route.meta.title ?? '管理页面'))
const description = computed(() => String(route.meta.description ?? '管理当前模块的数据、状态和审核记录。'))
const section = computed(() => String(route.meta.section ?? 'official-prices') as AdminSection)
const { data } = useAdminSectionRows(section)
const localRows = ref<AdminRow[]>([])
const activeStatus = ref('全部')
const reason = ref('')
const confirmedRiskAction = ref(false)
const actionBusy = ref('')
const selectedRowId = ref('')
const drawerOpen = ref(false)
const drawerMode = ref<DrawerMode>('detail')
const confirmOpen = ref(false)
const confirmAction = ref<QuickAction>('approve')
const confirmRowId = ref('')

type ModerationAction = 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban'
type DrawerMode = 'detail' | ModerationAction
type QuickAction = 'approve' | 'recheck'
type ModerationActionItem = {
  action: ModerationAction
  label: string
  disabled: boolean
  danger?: boolean
}

watch(data, rows => {
  localRows.value = rows ? [...rows] : []
  if (!localRows.value.some(row => row.id === selectedRowId.value)) {
    selectedRowId.value = ''
  }
  if (!localRows.value.some(row => row.id === confirmRowId.value)) {
    confirmOpen.value = false
    confirmRowId.value = ''
  }
}, { immediate: true })

watch(drawerOpen, open => {
  if (!open) {
    reason.value = ''
    confirmedRiskAction.value = false
    drawerMode.value = 'detail'
  }
})

const visibleRows = computed(() => {
  if (activeStatus.value === '全部') return localRows.value
  if (activeStatus.value === '待处理') {
    return localRows.value.filter(row => !['已通过', '待复核', '已关闭'].includes(row.status))
  }
  if (activeStatus.value === '需复核') {
    return localRows.value.filter(row => row.status.includes('复核'))
  }
  return localRows.value.filter(row => row.status === activeStatus.value)
})

const pendingCount = computed(() => localRows.value.filter(row => !['已通过', '待复核', '已关闭'].includes(row.status)).length)
const reviewCount = computed(() => localRows.value.filter(row => row.status.includes('复核')).length)
const pagination = usePagination(visibleRows)
const selectedRow = computed(() => localRows.value.find(row => row.id === selectedRowId.value) ?? null)
const drawerRow = computed(() => selectedRow.value)
const drawerAction = computed(() => drawerMode.value === 'detail' ? null : drawerMode.value)
const confirmRow = computed(() => localRows.value.find(row => row.id === confirmRowId.value) ?? null)
const panelCopy = computed(() => {
  const map: Partial<Record<AdminSection, { title: string, description: string }>> = {
    'official-prices': { title: '官网价格维护', description: '维护地区、渠道、原币价格、折合人民币和来源记录，再决定通过、复核或下架。' },
    'price-leads': { title: '价格记录维护', description: '维护地区、渠道、原币价格、折合人民币和来源记录，再决定通过、复核或下架。' },
    carpools: { title: '车源治理', description: '查看车主、席位资格、价格、车主承诺和原帖状态，支持下架、恢复和遗留审核队列处理。' },
    demands: { title: '求车管理', description: '查看预算、地区、车主偏好和原帖状态，支持关闭或恢复匹配。' },
    'api-services': { title: 'API 服务审核', description: '核对模型价格、最低意向金额、交易说明和商户身份展示。' },
    'api-merchants': { title: 'API 商户审核', description: '店铺名模式只在管理台展示真实用户映射，公开页继续隐藏。' },
    'trade-intents': { title: '购买意向监控', description: '关注意向状态、商户响应、取消责任和纠纷标记；管理员不查看完整联系方式。' },
    'carpool-applications': { title: '上车申请监控', description: '查看申请人、车主、席位、预留和当前状态，辅助纠纷判断。' },
    reports: { title: '举报纠纷处理', description: '只展示脱敏上下文；必要联系方式仍限制在联系快照流程内。' },
    appeals: { title: '申诉处理', description: '结合关联记录和未解决纠纷判断是否恢复能力。' },
    'audit-logs': { title: '审计日志', description: '只读查看管理动作、前后状态和原因。' },
    logs: { title: '审计日志', description: '只读查看管理动作、前后状态和原因。' },
  }
  return map[section.value] ?? { title: '管理处理', description: '查看当前对象上下文并执行管理动作。' }
})

const drawerTitle = computed(() => {
  if (!drawerRow.value) return '对象详情'
  if (!drawerAction.value) return `${drawerRow.value.primary} 详情`
  return `${moderationActionLabel(drawerAction.value, drawerRow.value)}确认`
})

const drawerDescription = computed(() => {
  if (!drawerAction.value) return '查看当前对象上下文，危险处理从本抽屉内完成。'
  return '填写操作原因并完成二次确认后，管理动作会写入审计日志。'
})

const confirmTitle = computed(() => {
  const row = confirmRow.value
  if (!row) return '确认操作'
  const label = confirmAction.value === 'approve' ? primaryActionLabel(row) : secondaryActionLabel(row)
  return `确认${label}`
})

const confirmDescription = computed(() => {
  const row = confirmRow.value
  if (!row) return '该操作会写入管理记录。'
  const label = confirmAction.value === 'approve' ? primaryActionLabel(row) : secondaryActionLabel(row)
  return `将 ${row.primary} 执行“${label}”，并写入本地审计记录。`
})

function openDetailDrawer(row: AdminRow) {
  selectedRowId.value = row.id
  drawerMode.value = 'detail'
  drawerOpen.value = true
}

function openModerationDrawer(row: AdminRow, action: ModerationAction) {
  if (action === 'restore' && !canRestore(row)) {
    toast.warning('当前记录未下架或限制，不能恢复。')
    return
  }
  if (action === 'take_down' && !canTakeDown(row)) {
    toast.warning('当前状态不适合下架，请先复核。')
    return
  }
  selectedRowId.value = row.id
  drawerMode.value = action
  reason.value = ''
  confirmedRiskAction.value = false
  drawerOpen.value = true
}

function openQuickConfirm(row: AdminRow, action: QuickAction) {
  if (action === 'approve' && !canApprove(row)) {
    toast.warning('当前记录已处于通过或在线状态，不能重复标记通过。')
    return
  }
  if (action === 'recheck' && !canRequestRecheck(row)) {
    toast.warning('当前记录已经在复核队列。')
    return
  }
  selectedRowId.value = row.id
  confirmRowId.value = row.id
  confirmAction.value = action
  confirmOpen.value = true
}

async function setRowStatus(row: AdminRow, status: string, auditReason: string) {
  const updated = await updateAdminRowStatus(row, status, auditReason)
  localRows.value = localRows.value.map(item => item.id === row.id ? updated : item)
  selectedRowId.value = updated.id
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  status === '已通过'
    ? toast.success(`${row.primary} 已标记通过。`)
    : toast.warning(`${row.primary} 已执行：${updated.status}`)
}

function canRestore(row: AdminRow | null) {
  if (!row) return false
  if (['report', 'dispute', 'appeal'].includes(row.targetType ?? '')) return false
  return ['已下架', '已限制', '暂停', '离线', '临时封禁', '永久封禁', '申诉复核中', '需要补充信息', 'partially_restricted', 'temporarily_suspended', 'permanently_banned', 'under_review'].some(status => row.status.includes(status))
}

function canTakeDown(row: AdminRow | null) {
  if (!row) return false
  if (row.targetType === 'report') return ['待处理', '已分诊'].includes(row.status)
  if (row.targetType === 'dispute') return row.status !== '已关闭'
  if (row.targetType === 'appeal') return row.status === '申诉复核中'
  return ['已验证', '已通过', '可上车', '在线', '匹配中', 'normal'].some(status => row.status.includes(status))
}

function canApprove(row: AdminRow | null) {
  if (!row) return false
  if (row.targetType === 'report') return row.status === '待处理'
  if (row.targetType === 'dispute') return ['处理中', '需要补充信息'].includes(row.status)
  if (row.targetType === 'appeal') return row.status === '申诉复核中'
  return !['已通过', '已验证', '在线', '可上车', '匹配中'].some(status => row.status.includes(status))
}

function canRequestRecheck(row: AdminRow | null) {
  if (!row) return false
  if (row.targetType === 'report') return ['待处理', '已分诊'].includes(row.status)
  if (row.targetType === 'dispute') return row.status === '处理中'
  if (row.targetType === 'appeal') return row.status === '申诉复核中'
  return !row.status.includes('复核')
}

function primaryActionLabel(row: AdminRow | null) {
  if (row?.targetType === 'report') return '标记分诊'
  if (row?.targetType === 'dispute') return '标记处理'
  if (row?.targetType === 'appeal') return '通过申诉'
  return '标记通过'
}

function secondaryActionLabel(row: AdminRow | null) {
  if (row?.targetType === 'report') return '打开纠纷'
  if (row?.targetType === 'dispute') return '要求补充'
  if (row?.targetType === 'appeal') return '驳回申诉'
  return '标记复核'
}

function negativeActionLabel(row: AdminRow | null) {
  if (row?.targetType === 'report') return '拒绝'
  if (row?.targetType === 'dispute') return '关闭'
  if (row?.targetType === 'appeal') return '驳回'
  return '下架'
}

function moderationActionLabel(action: ModerationAction, row: AdminRow | null) {
  const labels: Record<Exclude<ModerationAction, 'take_down'>, string> = {
    restore: '恢复',
    restrict: '限制能力',
    warn: '警告',
    suspend: '临时封禁',
    ban: '永久封禁',
  }
  return action === 'take_down' ? negativeActionLabel(row) : labels[action]
}

function moderationActionItems(row: AdminRow | null): ModerationActionItem[] {
  if (!row) return []
  const items: ModerationActionItem[] = [
    { action: 'take_down', label: negativeActionLabel(row), disabled: !canTakeDown(row), danger: true },
    { action: 'restore', label: '恢复', disabled: !canRestore(row) },
  ]
  if (showDangerActions.value) {
    items.push(
      { action: 'restrict', label: '限制能力', disabled: false, danger: true },
      { action: 'warn', label: '警告', disabled: false },
      { action: 'suspend', label: '临时封禁', disabled: false, danger: true },
      { action: 'ban', label: '永久封禁', disabled: false, danger: true },
    )
  }
  return items
}

async function approveRow(row: AdminRow) {
  if (!canApprove(row)) {
    toast.warning('当前记录已处于通过或在线状态，不能重复标记通过。')
    return false
  }
  actionBusy.value = `${row.id}-approve`
  try {
    await setRowStatus(row, '已通过', `管理台轻量确认：${primaryActionLabel(row)}`)
    return true
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '操作失败')
    return false
  } finally {
    actionBusy.value = ''
  }
}

async function requestRecheck(row: AdminRow) {
  if (!canRequestRecheck(row)) {
    toast.warning('当前记录已经在复核队列。')
    return false
  }
  actionBusy.value = `${row.id}-recheck`
  try {
    await setRowStatus(row, '待复核', `管理台轻量确认：${secondaryActionLabel(row)}`)
    return true
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '操作失败')
    return false
  } finally {
    actionBusy.value = ''
  }
}

async function confirmQuickAction() {
  const row = confirmRow.value
  if (!row) return
  const completed = confirmAction.value === 'approve'
    ? await approveRow(row)
    : await requestRecheck(row)
  if (completed) {
    confirmOpen.value = false
  }
}

async function runAction(row: AdminRow, action: ModerationAction) {
  if (!reason.value.trim()) {
    toast.warning('请先填写操作原因。')
    return false
  }
  if (!confirmedRiskAction.value) {
    toast.warning('请先勾选二次确认。')
    return false
  }
  if (action === 'restore' && !canRestore(row)) {
    toast.warning('当前记录未下架或限制，不能恢复。')
    return false
  }
  if (action === 'take_down' && !canTakeDown(row)) {
    toast.warning('当前状态不适合下架，请先复核。')
    return false
  }
  actionBusy.value = `${row.id}-${action}`
  try {
    const updated = await runAdminModerationAction(row, action, reason.value.trim())
    localRows.value = localRows.value.map(item => item.id === row.id ? updated : item)
    selectedRowId.value = updated.id
    confirmedRiskAction.value = false
    await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    toast.success(`${row.primary} 已执行：${updated.status}`)
    return true
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '操作失败')
    return false
  } finally {
    actionBusy.value = ''
  }
}

async function confirmModerationAction() {
  const row = drawerRow.value
  const action = drawerAction.value
  if (!row || !action) return
  const completed = await runAction(row, action)
  if (completed) {
    drawerOpen.value = false
  }
}

const showDangerActions = computed(() => ['users', 'restrictions'].includes(section.value))
const showContentActions = computed(() => !['logs', 'audit-logs'].includes(section.value))
</script>

<template>
  <div>
    <PageTitle :title="title" :description="description" />
    <div class="mb-5 grid gap-3 md:grid-cols-4">
      <Card class="p-4"><div class="text-sm text-muted-foreground">待处理</div><div class="mt-2 text-2xl font-semibold">{{ pendingCount }}</div></Card>
      <Card class="p-4"><div class="text-sm text-muted-foreground">需复核</div><div class="mt-2 text-2xl font-semibold">{{ reviewCount }}</div></Card>
      <Card class="p-4"><div class="text-sm text-muted-foreground">今日更新</div><div class="mt-2 text-2xl font-semibold">3</div></Card>
      <Card class="p-4"><div class="text-sm text-muted-foreground">操作记录</div><div class="mt-2 text-2xl font-semibold">12</div></Card>
    </div>
    <StatusTabs v-model="activeStatus" :items="['全部', '待处理', '已通过', '需复核', '已关闭']" />
    <SoftTable :columns="['对象', '详情', '提交 / 关联人', '状态', '风险 / 备注', '操作']">
      <tr
        v-for="row in pagination.paginatedRows.value"
        :key="row.id"
        :class="row.id === selectedRow?.id ? 'bg-accent/60' : ''"
      >
        <td class="font-medium">{{ row.primary }}</td>
        <td class="text-muted-foreground">{{ row.secondary }}</td>
        <td>{{ row.owner }}</td>
        <td><Badge :variant="row.status.includes('离线') || row.status.includes('取消') ? 'secondary' : 'default'">{{ row.status }}</Badge></td>
        <td>{{ row.risk }}</td>
        <td>
          <div v-if="showContentActions" class="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" @click="openDetailDrawer(row)">
              <Eye class="h-4 w-4" />
              详情
            </Button>
            <Button size="sm" :disabled="!canApprove(row) || actionBusy === `${row.id}-approve`" @click="openQuickConfirm(row, 'approve')">
              <CheckCircle2 class="h-4 w-4" />
              {{ primaryActionLabel(row) }}
            </Button>
            <Button size="sm" variant="outline" :disabled="!canRequestRecheck(row) || actionBusy === `${row.id}-recheck`" @click="openQuickConfirm(row, 'recheck')">
              {{ secondaryActionLabel(row) }}
            </Button>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button size="sm" variant="outline">
                  <MoreHorizontal class="h-4 w-4" />
                  更多
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-36">
                <DropdownMenuItem
                  v-for="item in moderationActionItems(row)"
                  :key="item.action"
                  :variant="item.danger ? 'destructive' : 'default'"
                  :disabled="item.disabled || actionBusy === `${row.id}-${item.action}`"
                  @click="openModerationDrawer(row, item.action)"
                >
                  {{ item.label }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
          <div v-else class="flex items-center gap-2">
            <Button size="sm" variant="outline" @click="openDetailDrawer(row)">
              <Eye class="h-4 w-4" />
              详情
            </Button>
            <span class="text-xs text-muted-foreground">只读记录</span>
          </div>
        </td>
      </tr>
      <tr v-if="visibleRows.length === 0">
        <td colspan="6" class="py-10 text-center text-sm text-muted-foreground">当前筛选下暂无记录。</td>
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

    <Dialog v-model:open="drawerOpen">
      <DialogContent class="bottom-0 left-auto right-0 top-0 flex h-dvh max-h-dvh w-full max-w-full translate-x-0 translate-y-0 grid-cols-1 gap-0 overflow-hidden rounded-none border-l border-r-0 p-0 shadow-xl duration-200 data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right data-[state=closed]:zoom-out-100 data-[state=open]:zoom-in-100 sm:max-w-xl">
        <div class="flex h-full min-h-0 flex-col">
          <DialogHeader class="border-b border-border px-5 py-4 pr-12">
            <div class="flex flex-wrap items-center gap-2">
              <DialogTitle>{{ drawerTitle }}</DialogTitle>
              <Badge v-if="drawerRow" variant="secondary">{{ drawerRow.status }}</Badge>
            </div>
            <DialogDescription>{{ drawerDescription }}</DialogDescription>
          </DialogHeader>

          <div v-if="drawerRow" class="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-5">
            <section class="rounded-lg border border-border bg-muted/30 p-4">
              <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <div class="text-base font-semibold">{{ drawerRow.primary }}</div>
                  <div class="mt-1 text-sm text-muted-foreground">{{ drawerRow.secondary }}</div>
                </div>
                <RouterLink v-if="drawerRow.targetTo" :to="drawerRow.targetTo">
                  <Button size="sm" variant="outline">打开关联页</Button>
                </RouterLink>
              </div>
              <div class="mt-4 grid gap-3 sm:grid-cols-2">
                <div>
                  <div class="text-xs text-muted-foreground">提交 / 关联人</div>
                  <div class="mt-1 text-sm font-medium">{{ drawerRow.owner }}</div>
                </div>
                <div>
                  <div class="text-xs text-muted-foreground">风险 / 备注</div>
                  <div class="mt-1 text-sm font-medium">{{ drawerRow.risk }}</div>
                </div>
                <div v-for="detail in drawerRow.detailItems ?? []" :key="detail.label">
                  <div class="text-xs text-muted-foreground">{{ detail.label }}</div>
                  <div class="mt-1 text-sm font-medium">{{ detail.value }}</div>
                </div>
              </div>
            </section>

            <section v-if="!drawerAction && showContentActions" class="space-y-3">
              <div>
                <h2 class="text-sm font-semibold">{{ panelCopy.title }}</h2>
                <p class="mt-1 text-sm text-muted-foreground">{{ panelCopy.description }}</p>
              </div>
              <div class="grid gap-2 sm:grid-cols-2">
                <Button
                  v-for="item in moderationActionItems(drawerRow)"
                  :key="item.action"
                  :variant="item.danger ? 'destructive' : 'outline'"
                  :disabled="item.disabled || actionBusy === `${drawerRow.id}-${item.action}`"
                  class="justify-start"
                  @click="openModerationDrawer(drawerRow, item.action)"
                >
                  <ShieldAlert v-if="item.danger" class="h-4 w-4" />
                  {{ item.label }}
                </Button>
              </div>
            </section>

            <section v-if="drawerAction" class="space-y-3">
              <label class="space-y-2">
                <span class="text-sm font-medium">操作原因</span>
                <Textarea v-model="reason" class="min-h-28" placeholder="填写下架、恢复、限制或封禁原因；审计日志会记录该说明。" />
              </label>
              <label class="flex items-start gap-2 rounded-lg border border-border bg-muted/30 p-3 text-xs leading-5 text-muted-foreground">
                <input v-model="confirmedRiskAction" type="checkbox" class="mt-1 h-4 w-4 accent-primary" />
                <span>二次确认：我已核对关联页、证据和当前状态，确认本次{{ moderationActionLabel(drawerAction, drawerRow) }}动作应写入审计日志。</span>
              </label>
              <p class="text-xs leading-5 text-muted-foreground">管理动作只更新状态并写入审计日志，不会删除记录、不会查看意向记录外的完整联系方式。</p>
            </section>
          </div>

          <div v-else class="flex-1 px-5 py-8 text-sm text-muted-foreground">
            当前模块暂无可查看记录。
          </div>

          <DialogFooter class="border-t border-border px-5 py-4">
            <Button variant="outline" @click="drawerOpen = false">取消</Button>
            <Button
              v-if="drawerAction && drawerRow"
              :variant="['take_down', 'restrict', 'suspend', 'ban'].includes(drawerAction) ? 'destructive' : 'default'"
              :disabled="actionBusy === `${drawerRow.id}-${drawerAction}`"
              @click="confirmModerationAction"
            >
              确认{{ moderationActionLabel(drawerAction, drawerRow) }}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="confirmOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ confirmTitle }}</DialogTitle>
          <DialogDescription>{{ confirmDescription }}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="confirmOpen = false">取消</Button>
          <Button
            v-if="confirmRow"
            :disabled="actionBusy === `${confirmRow.id}-${confirmAction === 'approve' ? 'approve' : 'recheck'}`"
            @click="confirmQuickAction"
          >
            确认执行
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
