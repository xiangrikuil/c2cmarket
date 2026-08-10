<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import { CheckCircle2, Eye, Gavel, MoreHorizontal, RotateCcw, ShieldAlert } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
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
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import PageTitle from '@/components/market/PageTitle.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import CursorTablePagination from '@/components/market/CursorTablePagination.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import CompactStats from '@/components/market/CompactStats.vue'
import AdminDisputeResolutionDialog from '@/components/admin/AdminDisputeResolutionDialog.vue'
import { useCursorPagination } from '@/composables/useCursorPagination'
import { runAdminModerationAction, updateAdminRowStatus, type AdminRow, type AdminSection, type ApiOrderStatus } from '@/lib/api'
import { backendAdminModerationDetailRow } from '@/lib/reportBackend'
import { isCarpoolAdminActionStatus, isCarpoolExceptionStatus } from '@/lib/carpoolModeration'
import { isApiServiceAdminActionStatus, isApiServiceExceptionStatus, isApiServicePublicStatus } from '@/lib/apiServiceModeration'
import { matchesApiOrderSearch, normalizeApiOrderAmountFilter } from '@/lib/apiOrderUi'
import { useAdminSectionRows, useAdminSectionRowsPage } from '@/queries/useMarketQueries'
import { toast } from 'vue-sonner'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()

const title = computed(() => String(route.meta.title ?? '管理页面'))
const description = computed(() => String(route.meta.description ?? '管理当前模块的数据、状态和审核记录。'))
const section = computed(() => String(route.meta.section ?? 'official-prices') as AdminSection)
const localRows = ref<AdminRow[]>([])
const activeStatus = ref('全部')
const carpoolView = ref<CarpoolView>(route.query.view === 'exceptions' ? 'exceptions' : 'public')
const apiServiceView = ref<ApiServiceView>(route.query.view === 'exceptions' ? 'exceptions' : 'public')
const keyword = ref('')
const riskFilter = ref<'all' | 'high' | 'has_note'>('all')
const orderStatus = ref<ApiOrderStatus | 'all'>('all')
const orderDateRange = ref<'all' | 'today' | '7d' | '30d'>('all')
const orderBuyerId = ref('')
const orderSellerId = ref('')
const orderServiceId = ref('')
const orderDispute = ref<'all' | 'active' | 'none'>('all')
const orderMinAmount = ref<string | number>('')
const orderMaxAmount = ref<string | number>('')
const orderSort = ref<'updated_desc' | 'created_desc' | 'amount_desc' | 'amount_asc'>('updated_desc')
const normalizedOrderMinAmount = computed(() => normalizeApiOrderAmountFilter(orderMinAmount.value))
const normalizedOrderMaxAmount = computed(() => normalizeApiOrderAmountFilter(orderMaxAmount.value))
const reason = ref('')
const requestedFromUserId = ref('')
const confirmedRiskAction = ref(false)
const actionBusy = ref('')
const selectedRowId = ref('')
const drawerOpen = ref(false)
const drawerMode = ref<DrawerMode>('detail')
const confirmOpen = ref(false)
const confirmAction = ref<QuickAction>('approve')
const confirmRowId = ref('')
const disputeDialogOpen = ref(false)
const disputeDialogId = ref('')

type ModerationAction = 'request_info' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban'
type DrawerMode = 'detail' | ModerationAction
type QuickAction = 'approve' | 'recheck'
type CarpoolView = 'public' | 'exceptions'
type ApiServiceView = 'public' | 'exceptions'
type ModerationActionItem = {
  action: ModerationAction
  label: string
  disabled: boolean
  danger?: boolean
}

const serverPagedSections: AdminSection[] = ['carpools', 'api-services', 'official-prices', 'price-leads', 'trade-intents', 'logs']
const supportsServerPagination = computed(() => serverPagedSections.includes(section.value))
const pageFilters = computed(() => ({
  q: keyword.value.trim() || undefined,
  view: section.value === 'carpools' ? carpoolView.value : section.value === 'api-services' ? apiServiceView.value : undefined,
  activeStatus: activeStatus.value,
  risk: section.value === 'trade-intents' ? undefined : riskFilter.value,
  orderStatus: section.value === 'trade-intents' ? orderStatus.value : undefined,
  orderDateRange: section.value === 'trade-intents' ? orderDateRange.value : undefined,
  orderBuyerId: section.value === 'trade-intents' ? orderBuyerId.value.trim() || undefined : undefined,
  orderSellerId: section.value === 'trade-intents' ? orderSellerId.value.trim() || undefined : undefined,
  orderServiceId: section.value === 'trade-intents' ? orderServiceId.value.trim() || undefined : undefined,
  orderDispute: section.value === 'trade-intents' ? orderDispute.value : undefined,
  orderMinAmount: section.value === 'trade-intents' ? normalizedOrderMinAmount.value || undefined : undefined,
  orderMaxAmount: section.value === 'trade-intents' ? normalizedOrderMaxAmount.value || undefined : undefined,
  orderSort: section.value === 'trade-intents' ? orderSort.value : undefined,
}))
const pagination = useCursorPagination([
  section,
  activeStatus,
  keyword,
  riskFilter,
  carpoolView,
  apiServiceView,
  orderStatus,
  orderDateRange,
  orderBuyerId,
  orderSellerId,
  orderServiceId,
  orderDispute,
  orderMinAmount,
  orderMaxAmount,
  orderSort,
])
const pageRequest = computed(() => ({ limit: pagination.pageSize, cursor: pagination.cursor.value }))
const fullRowsQuery = useAdminSectionRows(section, computed(() => !supportsServerPagination.value))
const pageRowsQuery = useAdminSectionRowsPage(section, pageFilters, pageRequest, supportsServerPagination)
const data = computed(() => supportsServerPagination.value ? pageRowsQuery.data.value?.items : fullRowsQuery.data.value)
const error = computed(() => supportsServerPagination.value ? pageRowsQuery.error.value : fullRowsQuery.error.value)
const isFetching = computed(() => supportsServerPagination.value ? pageRowsQuery.isFetching.value : fullRowsQuery.isFetching.value)
const isLoading = computed(() => supportsServerPagination.value ? pageRowsQuery.isLoading.value : fullRowsQuery.isLoading.value)
const refetch = () => supportsServerPagination.value ? pageRowsQuery.refetch() : fullRowsQuery.refetch()

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
    requestedFromUserId.value = ''
    confirmedRiskAction.value = false
    drawerMode.value = 'detail'
  }
})

watch(() => route.query.view, value => {
  if (section.value === 'carpools') carpoolView.value = value === 'exceptions' ? 'exceptions' : 'public'
  if (section.value === 'api-services') apiServiceView.value = value === 'exceptions' ? 'exceptions' : 'public'
  activeStatus.value = '全部'
})

function setCarpoolView(value: string | number) {
  if (value !== 'public' && value !== 'exceptions') return
  carpoolView.value = value
  activeStatus.value = '全部'
  void router.replace({ query: { ...route.query, view: value === 'exceptions' ? 'exceptions' : undefined } })
}

function setApiServiceView(value: string | number) {
  if (value !== 'public' && value !== 'exceptions') return
  apiServiceView.value = value
  activeStatus.value = '全部'
  void router.replace({ query: { ...route.query, view: value === 'exceptions' ? 'exceptions' : undefined } })
}

const exceptionRows = computed(() => localRows.value.filter(row => isCarpoolExceptionStatus(row.status)))
const apiServiceExceptionRows = computed(() => localRows.value.filter(row => isApiServiceExceptionStatus(row.status)))
const sectionRows = computed(() => {
  if (section.value === 'carpools') {
    return carpoolView.value === 'exceptions'
      ? exceptionRows.value
      : localRows.value.filter(row => !isCarpoolExceptionStatus(row.status))
  }
  if (section.value === 'api-services') {
    return apiServiceView.value === 'exceptions'
      ? apiServiceExceptionRows.value
      : localRows.value.filter(row => isApiServicePublicStatus(row.status))
  }
  return localRows.value
})

function requiresAdminAction(row: AdminRow) {
  if (row.targetType === 'carpool') return isCarpoolAdminActionStatus(row.status)
  if (row.targetType === 'api-service' || row.targetType === 'api-merchant') return isApiServiceAdminActionStatus(row.status)
  if (section.value === 'reports') {
    if (row.targetType === 'report') return ['待处理', '已分诊', '需要补充信息'].includes(row.status)
    if (row.targetType === 'dispute') return ['处理中', '需要补充信息'].includes(row.status)
    if (row.targetType === 'appeal') return row.status === '申诉复核中'
    return false
  }
  return !['已通过', '待复核', '已关闭'].includes(row.status)
}

const visibleRows = computed(() => {
  if (supportsServerPagination.value) return localRows.value
  let rows = sectionRows.value
  if (activeStatus.value === '待处理') {
    rows = rows.filter(requiresAdminAction)
  } else if (activeStatus.value === '需复核') {
    rows = rows.filter(row => row.status.includes('复核'))
  } else if (activeStatus.value !== '全部') {
    rows = rows.filter(row => row.status === activeStatus.value)
  }
  const q = keyword.value.trim()
  if (q) rows = rows.filter(row => matchesApiOrderSearch(q, [row.id, row.primary, row.secondary, row.owner, row.status, row.risk, ...(row.detailItems ?? []).flatMap(item => [item.label, item.value])]))
  if (riskFilter.value === 'high') rows = rows.filter(row => /高风险|纠纷|举报|封禁|异常|超时|未解决|危险/i.test(`${row.risk} ${row.status}`))
  if (riskFilter.value === 'has_note') rows = rows.filter(row => Boolean(row.risk.trim()))
  return rows
})

const pendingCount = computed(() => sectionRows.value.filter(requiresAdminAction).length)
const reviewCount = computed(() => sectionRows.value.filter(row => row.status.includes('复核')).length)
const summaryStats = computed(() => {
  if (supportsServerPagination.value) {
    return [
      { label: '本页记录', value: localRows.value.length },
      { label: '待处理', value: localRows.value.filter(requiresAdminAction).length },
      { label: '需复核', value: localRows.value.filter(row => row.status.includes('复核')).length },
      { label: '当前筛选', value: visibleRows.value.length },
    ]
  }
  if (section.value === 'carpools' && carpoolView.value === 'public') {
    return [
      { label: '公开车源', value: sectionRows.value.length },
      { label: '异常车源', value: exceptionRows.value.length },
      { label: '管理记录', value: localRows.value.length },
      { label: '当前筛选', value: visibleRows.value.length },
    ]
  }
  if (section.value === 'api-services' && apiServiceView.value === 'public') {
    return [
      { label: '公开服务', value: sectionRows.value.length },
      { label: '异常服务', value: apiServiceExceptionRows.value.length },
      { label: '管理记录', value: localRows.value.length },
      { label: '当前筛选', value: visibleRows.value.length },
    ]
  }
  return [
    { label: '待处理', value: pendingCount.value },
    { label: '需复核', value: reviewCount.value },
    { label: '本页记录', value: sectionRows.value.length },
    { label: '当前筛选', value: visibleRows.value.length },
  ]
})
const errorMessage = computed(() => error.value instanceof Error ? error.value.message : '管理数据读取失败，请稍后重试。')
const statusTabs = computed(() => {
  if (section.value === 'carpools') return carpoolView.value === 'exceptions' ? ['全部', '待处理', '需复核'] : ['全部']
  if (section.value === 'api-services') return apiServiceView.value === 'exceptions' ? ['全部', '待处理', '需复核'] : ['全部']
  if (['official-prices', 'price-leads', 'trade-intents', 'logs'].includes(section.value)) return ['全部']
  return ['全部', '待处理', '已通过', '需复核', '已关闭']
})
const selectedRow = computed(() => localRows.value.find(row => row.id === selectedRowId.value) ?? null)
const drawerRow = computed(() => selectedRow.value)
const drawerAction = computed(() => drawerMode.value === 'detail' ? null : drawerMode.value)
const requestInfoParticipants = computed(() => drawerRow.value?.moderationParticipants ?? [])
const confirmRow = computed(() => localRows.value.find(row => row.id === confirmRowId.value) ?? null)
const panelCopy = computed(() => {
  const map: Partial<Record<AdminSection, { title: string, description: string }>> = {
    'official-prices': { title: '官网价格维护', description: '维护地区、渠道、原币价格、折合人民币和来源记录，再决定通过、复核或下架。' },
    'price-leads': { title: '价格记录维护', description: '维护地区、渠道、原币价格、折合人民币和来源记录，再决定通过、复核或下架。' },
    carpools: { title: '车源管理', description: '集中巡查公开车源，并处理暂停、待复核和遗留审核记录。' },
    'api-services': { title: 'API 服务管理', description: '集中巡查公开服务，并处理遗留待审、下架和其他异常记录。' },
    'trade-intents': { title: 'API 订单监管', description: '查看 API 订单状态、参与方、金额快照、取消责任与纠纷标记；管理摘要不展示联系方式或原始交付凭证。' },
    reports: { title: '举报纠纷处理', description: '只展示脱敏上下文；必要联系方式仍限制在联系快照流程内。' },
    appeals: { title: '申诉处理', description: '结合关联记录和未解决纠纷判断是否恢复能力。' },
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

async function openDetailDrawer(row: AdminRow) {
  if (row.targetType === 'dispute') {
    openDisputeResolution(row)
    return
  }
  selectedRowId.value = row.id
  drawerMode.value = 'detail'
  drawerOpen.value = true
  if (row.targetType === 'report') {
    try {
      const refreshed = await backendAdminModerationDetailRow(row)
      localRows.value = localRows.value.map(item => item.id === row.id ? refreshed : item)
      selectedRowId.value = refreshed.id
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '无法读取最新举报详情。')
    }
  }
}

function openDisputeResolution(row: AdminRow) {
  disputeDialogId.value = row.id
  disputeDialogOpen.value = true
}

async function handleDisputeUpdated(updated: AdminRow) {
  localRows.value = localRows.value.map(item => item.id === updated.id ? updated : item)
  selectedRowId.value = updated.id
  await queryClient.invalidateQueries({ queryKey: ['admin-section', 'reports'] })
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
  requestedFromUserId.value = ''
  if (action === 'request_info' && row.moderationParticipants?.length === 1) {
    requestedFromUserId.value = row.moderationParticipants[0]?.userId ?? ''
  }
  confirmedRiskAction.value = false
  drawerOpen.value = true
}

function openQuickConfirm(row: AdminRow, action: QuickAction) {
  if (action === 'recheck' && row.targetType === 'dispute') {
    openModerationDrawer(row, 'request_info')
    return
  }
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
  await queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
  status === '已通过'
    ? toast.success(`${row.primary} 已标记通过。`)
    : toast.warning(`${row.primary} 已执行：${updated.status}`)
}

function canRestore(row: AdminRow | null) {
  if (!row) return false
  if (['report', 'dispute', 'appeal'].includes(row.targetType ?? '')) return false
  if (row.targetType === 'api-service' || row.targetType === 'api-merchant') return row.status === '已下架'
  return ['已下架', '已限制', '暂停', '离线', '临时封禁', '永久封禁', '申诉复核中', '需要补充信息', 'partially_restricted', 'temporarily_suspended', 'permanently_banned', 'under_review'].some(status => row.status.includes(status))
}

function canTakeDown(row: AdminRow | null) {
  if (!row) return false
  if (row.targetType === 'report') return ['待处理', '已分诊'].includes(row.status)
  if (row.targetType === 'dispute') return ['处理中', '需要补充信息'].includes(row.status)
  if (row.targetType === 'appeal') return row.status === '申诉复核中'
  if (row.targetType === 'api-service' || row.targetType === 'api-merchant') return row.status === '在线'
  return ['已验证', '已通过', '可上车', '已满', '在线', '匹配中', 'normal'].some(status => row.status.includes(status))
}

function canApprove(row: AdminRow | null) {
  if (!row) return false
  if (row.targetType === 'carpool') return ['待处理', '审核中'].includes(row.status)
  if (row.targetType === 'api-service' || row.targetType === 'api-merchant') return row.status === '待处理'
  if (row.targetType === 'report') return row.status === '待处理'
  if (row.targetType === 'dispute') return false
  if (row.targetType === 'appeal') return row.status === '申诉复核中'
  return !['已通过', '已验证', '在线', '可上车', '匹配中'].some(status => row.status.includes(status))
}

function canRequestRecheck(row: AdminRow | null) {
  if (!row) return false
  if (row.targetType === 'carpool') return ['待处理', '审核中', '可上车', '已满', '已通过', '已验证', '已恢复'].includes(row.status)
  if (row.targetType === 'api-service' || row.targetType === 'api-merchant') return row.status === '待处理'
  if (row.targetType === 'report') return ['待处理', '已分诊', '需要补充信息'].includes(row.status)
  if (row.targetType === 'dispute') return row.status === '处理中'
  if (row.targetType === 'appeal') return row.status === '申诉复核中'
  return !row.status.includes('复核')
}

function primaryActionLabel(row: AdminRow | null) {
  if (row?.targetType === 'report') return '标记分诊'
  if (row?.targetType === 'appeal') return '通过申诉'
  return '标记通过'
}

function disputeActionLabel(row: AdminRow) {
  if (row.status === '已处理') return '责任认定'
  if (row.status === '已关闭') return '查看案件'
  return '裁决'
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
    request_info: '要求补充',
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
  if (row.targetType === 'report') {
    items.unshift({ action: 'request_info', label: '要求举报人补充', disabled: !['待处理', '已分诊'].includes(row.status) })
  }
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

function hasModerationAction(row: AdminRow) {
  return moderationActionItems(row).some(item => !item.disabled)
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
    if (row.targetType === 'report') {
      const updated = await runAdminModerationAction(row, 'restore', '管理台打开纠纷并进入人工裁决。')
      localRows.value = localRows.value.map(item => item.id === row.id ? updated : item)
      selectedRowId.value = updated.id
      await queryClient.invalidateQueries({ queryKey: ['admin-section', 'reports'] })
      toast.success(`${row.primary} 已打开纠纷。`)
    } else {
      await setRowStatus(row, '待复核', `管理台轻量确认：${secondaryActionLabel(row)}`)
    }
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
  if (action === 'request_info' && !requestedFromUserId.value) {
    toast.warning('请选择需要补充信息的案件参与者。')
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
    const backendAction = action === 'request_info' ? 'request_changes' : action
    const updated = await runAdminModerationAction(row, backendAction, reason.value.trim(), requestedFromUserId.value)
    localRows.value = localRows.value.map(item => item.id === row.id ? updated : item)
    selectedRowId.value = updated.id
    confirmedRiskAction.value = false
    await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    await queryClient.invalidateQueries({ queryKey: ['navigation-badges'] })
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

const showDangerActions = computed(() => false)
const showContentActions = computed(() => !['logs', 'trade-intents'].includes(section.value))
const hasOrderFilters = computed(() => keyword.value.trim() !== ''
  || orderStatus.value !== 'all'
  || orderDateRange.value !== 'all'
  || orderBuyerId.value.trim() !== ''
  || orderSellerId.value.trim() !== ''
  || orderServiceId.value.trim() !== ''
  || orderDispute.value !== 'all'
  || normalizedOrderMinAmount.value !== ''
  || normalizedOrderMaxAmount.value !== ''
  || orderSort.value !== 'updated_desc')

function clearOrderFilters() {
  keyword.value = ''
  orderStatus.value = 'all'
  orderDateRange.value = 'all'
  orderBuyerId.value = ''
  orderSellerId.value = ''
  orderServiceId.value = ''
  orderDispute.value = 'all'
  orderMinAmount.value = ''
  orderMaxAmount.value = ''
  orderSort.value = 'updated_desc'
}
</script>

<template>
  <div>
    <PageTitle :title="title" :description="description" />
    <CompactStats class="mb-5" :items="summaryStats" :loading="isLoading" />
    <Tabs v-if="section === 'carpools'" :model-value="carpoolView" class="mb-4" @update:model-value="setCarpoolView">
      <TabsList class="grid h-10 w-full max-w-sm grid-cols-2" aria-label="车源管理视图">
        <TabsTrigger value="public">公开车源</TabsTrigger>
        <TabsTrigger value="exceptions">异常车源</TabsTrigger>
      </TabsList>
    </Tabs>
    <Tabs v-else-if="section === 'api-services'" :model-value="apiServiceView" class="mb-4" @update:model-value="setApiServiceView">
      <TabsList class="grid h-10 w-full max-w-sm grid-cols-2" aria-label="API 服务管理视图">
        <TabsTrigger value="public">公开服务</TabsTrigger>
        <TabsTrigger value="exceptions">异常服务</TabsTrigger>
      </TabsList>
    </Tabs>
    <div v-if="section === 'trade-intents'" class="mb-4 space-y-2">
      <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-[minmax(240px,1fr)_180px_160px_180px]">
        <Input v-model="keyword" placeholder="订单号、订单 ID、服务或参与方 ID" aria-label="订单关键词" />
        <Select v-model="orderStatus">
          <SelectTrigger class="w-full" aria-label="订单状态"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部状态</SelectItem>
            <SelectItem value="pending_payment">待买家付款</SelectItem>
            <SelectItem value="payment_submitted">待确认收款</SelectItem>
            <SelectItem value="payment_issue">等待买家补充</SelectItem>
            <SelectItem value="paid_confirmed">待商户交付</SelectItem>
            <SelectItem value="delivery_submitted">买家核验期</SelectItem>
            <SelectItem value="completed">已完成</SelectItem>
            <SelectItem value="cancelled">已取消</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="orderDateRange">
          <SelectTrigger class="w-full" aria-label="创建时间"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部时间</SelectItem>
            <SelectItem value="today">今天</SelectItem>
            <SelectItem value="7d">近 7 天</SelectItem>
            <SelectItem value="30d">近 30 天</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="orderSort">
          <SelectTrigger class="w-full" aria-label="订单排序"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="updated_desc">最近更新</SelectItem>
            <SelectItem value="created_desc">最新创建</SelectItem>
            <SelectItem value="amount_desc">金额从高到低</SelectItem>
            <SelectItem value="amount_asc">金额从低到高</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-4 2xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1fr)_160px_130px_130px_auto]">
        <Input v-model="orderBuyerId" placeholder="买家用户 ID" aria-label="买家用户 ID" />
        <Input v-model="orderSellerId" placeholder="商户用户 ID" aria-label="商户用户 ID" />
        <Input v-model="orderServiceId" placeholder="API 服务 ID" aria-label="API 服务 ID" />
        <Select v-model="orderDispute">
          <SelectTrigger class="w-full" aria-label="纠纷状态"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部纠纷</SelectItem>
            <SelectItem value="active">有进行中纠纷</SelectItem>
            <SelectItem value="none">无进行中纠纷</SelectItem>
          </SelectContent>
        </Select>
        <Input v-model="orderMinAmount" type="text" inputmode="decimal" placeholder="最低金额" aria-label="最低金额" />
        <Input v-model="orderMaxAmount" type="text" inputmode="decimal" placeholder="最高金额" aria-label="最高金额" />
        <Button type="button" variant="outline" size="sm" class="h-9 whitespace-nowrap" :disabled="!hasOrderFilters" @click="clearOrderFilters">
          <RotateCcw class="h-4 w-4" />
          清空筛选
        </Button>
      </div>
    </div>
    <div v-else class="mb-4 grid gap-2" :class="section === 'logs' ? '' : 'md:grid-cols-[minmax(0,1fr)_180px]'">
      <Input v-model="keyword" placeholder="搜索对象、管理员、动作、状态或请求追踪" />
      <Select v-if="section !== 'logs'" v-model="riskFilter">
        <SelectTrigger class="w-full" aria-label="风险筛选"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem value="all">全部风险</SelectItem>
          <SelectItem value="high">仅高风险</SelectItem>
          <SelectItem value="has_note">有风险/备注</SelectItem>
        </SelectContent>
      </Select>
    </div>
    <StatusTabs v-if="statusTabs.length > 1" v-model="activeStatus" :items="statusTabs" />
    <SkeletonTable v-if="isLoading" :rows="6" :columns="6" />
    <Alert v-else-if="error" variant="destructive" class="mb-4">
      <ShieldAlert class="h-4 w-4" />
      <AlertTitle>管理数据读取失败</AlertTitle>
      <AlertDescription class="flex flex-wrap items-center justify-between gap-3">
        <span>{{ errorMessage }}</span>
        <Button size="sm" variant="outline" :disabled="isFetching" @click="refetch()">重新读取</Button>
      </AlertDescription>
    </Alert>
    <EmptyState v-else-if="visibleRows.length === 0" title="当前筛选下暂无记录" description="调整状态筛选，或等待新的管理记录进入队列。" />
    <SoftTable v-else class="[&_table]:min-w-[760px]" :columns="['对象', '详情', '提交 / 关联人', '状态', '风险 / 备注', '操作']">
      <tr
        v-for="row in visibleRows"
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
            <Button v-if="row.targetType === 'dispute'" size="sm" @click="openDisputeResolution(row)">
              <Gavel class="h-4 w-4" />
              {{ disputeActionLabel(row) }}
            </Button>
            <Button v-else-if="!['carpools', 'api-services'].includes(section) || canApprove(row)" size="sm" :disabled="!canApprove(row) || actionBusy === `${row.id}-approve`" @click="openQuickConfirm(row, 'approve')">
              <CheckCircle2 class="h-4 w-4" />
              {{ primaryActionLabel(row) }}
            </Button>
            <Button v-if="!['carpools', 'api-services'].includes(section) || canRequestRecheck(row)" size="sm" variant="outline" :disabled="!canRequestRecheck(row) || actionBusy === `${row.id}-recheck`" @click="openQuickConfirm(row, 'recheck')">
              {{ secondaryActionLabel(row) }}
            </Button>
            <DropdownMenu v-if="!['carpools', 'api-services'].includes(section) || hasModerationAction(row)">
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
      <template v-if="supportsServerPagination" #footer>
        <CursorTablePagination
          :page="pagination.page.value"
          :item-count="visibleRows.length"
          :has-next-page="Boolean(pageRowsQuery.data.value?.nextCursor)"
          :loading="pageRowsQuery.isFetching.value"
          @previous="pagination.previous"
          @next="pagination.next(pageRowsQuery.data.value?.nextCursor)"
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

            <section v-if="drawerRow.moderationSupplements?.length" class="space-y-3">
              <div>
                <h2 class="text-sm font-semibold">用户补充材料</h2>
                <p class="mt-1 text-sm text-muted-foreground">仅管理员可见，按提交时间排列。</p>
              </div>
              <div
                v-for="supplement in drawerRow.moderationSupplements"
                :key="supplement.id"
                class="rounded-lg border border-border bg-muted/30 p-4"
              >
                <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
                  <span>{{ supplement.submittedByName || supplement.submittedByUsername || supplement.submittedByUserId }}</span>
                  <span>{{ new Date(supplement.createdAt).toLocaleString('zh-CN') }}</span>
                </div>
                <p class="mt-3 whitespace-pre-wrap break-words text-sm leading-6">{{ supplement.body }}</p>
              </div>
            </section>

            <section v-if="!drawerAction && showContentActions && (section !== 'api-services' || hasModerationAction(drawerRow))" class="space-y-3">
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
              <label v-if="drawerAction === 'request_info'" class="space-y-2">
                <span class="text-sm font-medium">需要补充的用户</span>
                <div v-if="requestInfoParticipants.length === 1" class="rounded-md border border-input bg-muted/30 px-3 py-2 text-sm">
                  {{ requestInfoParticipants[0]?.label }}
                </div>
                <Select v-else v-model="requestedFromUserId">
                  <SelectTrigger class="w-full">
                    <SelectValue placeholder="选择案件参与者" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="participant in requestInfoParticipants" :key="participant.userId" :value="participant.userId">
                      {{ participant.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </label>
              <label class="space-y-2">
                <span class="text-sm font-medium">操作原因</span>
                <Textarea v-model="reason" class="min-h-28" :placeholder="drawerAction === 'request_info' ? '说明需要该用户补充的脱敏事实；该说明仅管理员可见。' : '填写下架、恢复、限制或封禁原因；审计日志会记录该说明。'" />
              </label>
              <label class="flex items-start gap-2 rounded-lg border border-border bg-muted/30 p-3 text-xs leading-5 text-muted-foreground">
                <Checkbox v-model="confirmedRiskAction" class="mt-1" />
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

    <AdminDisputeResolutionDialog
      v-model:open="disputeDialogOpen"
      :dispute-id="disputeDialogId"
      @updated="handleDisputeUpdated"
    />
  </div>
</template>
