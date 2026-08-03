<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  Activity,
  Archive,
  Ban,
  CheckCircle2,
  Eye,
  History,
  KeyRound,
  Link2,
  MoreHorizontal,
  PauseCircle,
  RefreshCcw,
  RotateCcw,
  Search,
  ShieldCheck,
  TriangleAlert,
  UserRoundCog,
  UsersRound,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ErrorState from '@/components/market/ErrorState.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import AdminReputationAuditPanel from '@/components/reputation/AdminReputationAuditPanel.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { BackendProblemError, backendErrorMessage } from '@/lib/backendClient'
import {
  adminUserDirectoryRouteQuery,
  normalizeAdminUserDirectoryQuery,
  type AdminUser,
  type AdminUserDirectoryLinuxDo,
  type AdminUserDirectoryQuery,
  type AdminUserDirectoryRole,
  type AdminUserDirectorySort,
  type AdminUserDirectoryStatus,
  type AdminUserGovernanceAction,
  type AdminUserStatus,
} from '@/lib/adminUserBackend'
import {
  useAdminUserDetail,
  useAdminUserDirectory,
  useUpdateAdminUserPermissionMutation,
  useUpdateAdminUserStatusMutation,
} from '@/queries/useAdminUserQueries'

const route = useRoute()
const router = useRouter()
const directoryQuery = computed(() => normalizeAdminUserDirectoryQuery(route.query as Record<string, unknown>))
const directoryRequest = useAdminUserDirectory(directoryQuery)
const statusMutation = useUpdateAdminUserStatusMutation()
const permissionMutation = useUpdateAdminUserPermissionMutation()

const managementOpen = ref(false)
const selectedUserId = ref('')
const searchDraft = ref(directoryQuery.value.search)
const activeDrawerTab = ref('overview')
const pendingAction = ref<AdminUserGovernanceAction | null>(null)
const reason = ref('')
const confirmed = ref(false)
const detailQuery = useAdminUserDetail(selectedUserId)

const rows = computed(() => directoryRequest.data.value?.items ?? [])
const pagination = computed(() => directoryRequest.data.value?.pagination)
const summary = computed(() => directoryRequest.data.value?.summary)
const pageCount = computed(() => Math.max(1, pagination.value?.totalPages ?? 1))
const startItem = computed(() => pagination.value?.totalItems
  ? (pagination.value.page - 1) * pagination.value.limit + 1
  : 0)
const endItem = computed(() => pagination.value?.totalItems
  ? Math.min(pagination.value.page * pagination.value.limit, pagination.value.totalItems)
  : 0)
const selectedDetail = computed(() => detailQuery.data.value)
const selectedUser = computed(() =>
  selectedDetail.value?.user ?? rows.value.find(item => item.id === selectedUserId.value) ?? null,
)
const actionPending = computed(() => statusMutation.isPending.value || permissionMutation.isPending.value)
const statusActions = computed(() =>
  selectedDetail.value?.availableActions.filter(action => action.kind === 'status') ?? [],
)
const permissionAction = computed(() =>
  selectedDetail.value?.availableActions.find(action => action.kind === 'permission') ?? null,
)
const primaryStatusAction = computed(() => {
  const preferred = selectedDetail.value?.user.accountStatus === 'active' ? 'suspend' : 'restore'
  return statusActions.value.find(action => action.action === preferred) ?? statusActions.value[0] ?? null
})
const secondaryStatusActions = computed(() =>
  statusActions.value.filter(action => action.action !== primaryStatusAction.value?.action),
)
const blockedActions = computed(() =>
  selectedDetail.value?.availableActions.filter(action => !action.allowed && action.blockedReason) ?? [],
)
const capabilityRows = computed(() => {
  const capabilities = selectedDetail.value?.accountCapabilities
  if (!capabilities) return []
  return [
    { label: '登录账号', allowed: capabilities.canLogin },
    { label: '公开主页与市场内容', allowed: capabilities.publiclyVisible },
    { label: '发布新内容', allowed: capabilities.canPublish },
    { label: '申请与创建订单', allowed: capabilities.canCreateOrders },
    { label: '披露联系方式', allowed: capabilities.canRevealContact },
    { label: '历史订单与纠纷', allowed: capabilities.canAccessHistoricalTransactions },
  ]
})

const statusFilter = computed({
  get: () => directoryQuery.value.status,
  set: value => replaceDirectoryQuery({ status: value as AdminUserDirectoryStatus }, true),
})
const roleFilter = computed({
  get: () => directoryQuery.value.role,
  set: value => replaceDirectoryQuery({ role: value as AdminUserDirectoryRole }, true),
})
const linuxDoFilter = computed({
  get: () => directoryQuery.value.linuxDo,
  set: value => replaceDirectoryQuery({ linuxDo: value as AdminUserDirectoryLinuxDo }, true),
})
const sortFilter = computed({
  get: () => directoryQuery.value.sort,
  set: value => replaceDirectoryQuery({ sort: value as AdminUserDirectorySort }, true),
})
const limitFilter = computed({
  get: () => String(directoryQuery.value.limit),
  set: value => replaceDirectoryQuery({ limit: Number(value) as AdminUserDirectoryQuery['limit'] }, true),
})

const actionTitle = computed(() => pendingAction.value ? governanceActionLabel(pendingAction.value) : '')
const actionDescription = computed(() => {
  const action = pendingAction.value
  const detail = selectedDetail.value
  if (!action || !detail) return ''
  if (action.kind === 'permission') {
    return action.targetIsAdmin
      ? '该账号将获得管理端访问权限，账号状态不会改变。'
      : '该账号将失去管理端访问权限，账号状态和存量业务保持不变。'
  }
  if (action.targetStatus === 'active') {
    return detail.user.accountStatus === 'banned'
      ? '解除封禁后账号重新获得正常能力，已撤销的旧会话不会恢复。'
      : '账号重新启用后恢复正常能力，已撤销的旧会话不会恢复。'
  }
  return '账号离开正常状态后将立即撤销有效会话，并停止新的市场活动。'
})

watch(
  () => directoryQuery.value.search,
  value => {
    searchDraft.value = value
  },
)

watch(
  [() => directoryRequest.data.value?.pagination, () => directoryRequest.isPlaceholderData.value],
  ([metadata, isPlaceholder]) => {
    if (!metadata || isPlaceholder) return
    const lastValidPage = Math.max(1, metadata.totalPages)
    if (directoryQuery.value.page > lastValidPage) replaceDirectoryQuery({ page: lastValidPage })
  },
)

function replaceDirectoryQuery(patch: Partial<AdminUserDirectoryQuery>, resetPage = false) {
  const next = {
    ...directoryQuery.value,
    ...patch,
    page: resetPage ? 1 : (patch.page ?? directoryQuery.value.page),
  }
  router.replace({ query: adminUserDirectoryRouteQuery(next) })
}

function submitSearch() {
  replaceDirectoryQuery({ search: searchDraft.value.trim() }, true)
}

function clearFilters() {
  searchDraft.value = ''
  router.replace({ query: {} })
}

function filterByStatus(status: AdminUserDirectoryStatus) {
  statusFilter.value = status
}

function openManagement(user: AdminUser, tab = 'overview') {
  selectedUserId.value = user.id
  activeDrawerTab.value = tab
  pendingAction.value = null
  reason.value = ''
  confirmed.value = false
  managementOpen.value = true
}

function setManagementOpen(open: boolean) {
  managementOpen.value = open
  if (!open) resetPendingAction()
}

function resetPendingAction() {
  pendingAction.value = null
  reason.value = ''
  confirmed.value = false
}

function openAction(action: AdminUserGovernanceAction) {
  if (!action.allowed) {
    toast.warning(action.blockedReason || '当前不能执行该操作。')
    return
  }
  pendingAction.value = action
  reason.value = ''
  confirmed.value = false
}

async function confirmAction() {
  const action = pendingAction.value
  const detail = selectedDetail.value
  if (!action || !detail) return
  if (!reason.value.trim()) {
    toast.warning('请填写操作原因。')
    return
  }
  if (!confirmed.value) {
    toast.warning('请完成二次确认。')
    return
  }
  try {
    if (action.kind === 'status' && action.targetStatus) {
      await statusMutation.mutateAsync({
        userId: detail.user.id,
        version: detail.user.version,
        status: action.targetStatus,
        reason: reason.value,
      })
    } else if (action.kind === 'permission' && action.targetIsAdmin !== undefined) {
      await permissionMutation.mutateAsync({
        userId: detail.user.id,
        version: detail.user.version,
        isAdmin: action.targetIsAdmin,
        reason: reason.value,
      })
    }
    toast.success(`${actionTitle.value}已完成。`)
    resetPendingAction()
  } catch (error) {
    if (error instanceof BackendProblemError && error.code === 'VERSION_CONFLICT') {
      resetPendingAction()
      toast.error('该账号刚刚被其他管理员修改，详情已刷新，请重新核对后再提交。')
      return
    }
    toast.error(backendErrorMessage(error, '账号治理操作失败。'))
  }
}

function governanceActionLabel(action: AdminUserGovernanceAction) {
  if (action.action === 'grant_admin') return '授予管理员权限'
  if (action.action === 'revoke_admin') return '撤销管理员权限'
  if (action.action === 'suspend') return '暂停账号'
  if (action.action === 'ban') return '封禁账号'
  if (action.action === 'archive') return '归档账号'
  if (selectedDetail.value?.user.accountStatus === 'banned') return '解除封禁'
  if (selectedDetail.value?.user.accountStatus === 'archived') return '重新启用账号'
  return '恢复账号'
}

function actionIcon(action: AdminUserGovernanceAction) {
  if (action.action === 'suspend') return PauseCircle
  if (action.action === 'ban') return Ban
  if (action.action === 'archive') return Archive
  if (action.action === 'restore') return RotateCcw
  return ShieldCheck
}

function statusDisplay(status: AdminUserStatus) {
  return {
    active: '正常',
    suspended: '已暂停',
    banned: '已封禁',
    archived: '已归档',
  }[status]
}

function statusIcon(status: AdminUserStatus) {
  return {
    active: CheckCircle2,
    suspended: PauseCircle,
    banned: Ban,
    archived: Archive,
  }[status]
}

function statusBadgeClass(status: AdminUserStatus) {
  return {
    active: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    suspended: 'border-amber-200 bg-amber-50 text-amber-700',
    banned: 'border-red-200 bg-red-50 text-red-700',
    archived: 'border-zinc-200 bg-zinc-100 text-zinc-600',
  }[status]
}

function formatTime(value: string | undefined | null) {
  if (!value) return '暂无'
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function auditActionLabel(action: string) {
  if (action === 'user.account_status_changed') return '账号状态变更'
  if (action === 'user.admin_permission_changed') return '管理员权限变更'
  return action
}

function roleDisplay(isAdmin: boolean) {
  return isAdmin ? '管理员' : '普通用户'
}

function auditTransition(entry: {
  beforeStatus?: AdminUserStatus
  afterStatus?: AdminUserStatus
  beforeIsAdmin?: boolean
  afterIsAdmin?: boolean
}) {
  if (entry.beforeStatus && entry.afterStatus) {
    return `${statusDisplay(entry.beforeStatus)} → ${statusDisplay(entry.afterStatus)}`
  }
  if (entry.beforeIsAdmin !== undefined && entry.afterIsAdmin !== undefined) {
    return `${roleDisplay(entry.beforeIsAdmin)} → ${roleDisplay(entry.afterIsAdmin)}`
  }
  return ''
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="用户目录" description="全站账号状态、权限与治理记录" />

    <section class="border-y border-border" aria-label="全站账号概览">
      <div class="grid grid-cols-2 divide-x divide-y divide-border sm:grid-cols-4 xl:grid-cols-7 xl:divide-y-0">
        <button type="button" class="min-w-0 px-4 py-3 text-left hover:bg-muted/50" :class="{ 'bg-muted/60': statusFilter === 'all' }" @click="filterByStatus('all')">
          <span class="flex items-center gap-2 text-xs text-muted-foreground"><UsersRound class="h-3.5 w-3.5" />全部账号</span>
          <strong class="mt-1 block text-xl">{{ summary?.totalUsers ?? '-' }}</strong>
        </button>
        <button type="button" class="min-w-0 px-4 py-3 text-left hover:bg-emerald-50/60" :class="{ 'bg-emerald-50/70': statusFilter === 'active' }" @click="filterByStatus('active')">
          <span class="flex items-center gap-2 text-xs text-emerald-700"><Activity class="h-3.5 w-3.5" />正常</span>
          <strong class="mt-1 block text-xl">{{ summary?.activeUsers ?? '-' }}</strong>
        </button>
        <button type="button" class="min-w-0 px-4 py-3 text-left hover:bg-amber-50/70" :class="{ 'bg-amber-50/80': statusFilter === 'suspended' }" @click="filterByStatus('suspended')">
          <span class="flex items-center gap-2 text-xs text-amber-700"><PauseCircle class="h-3.5 w-3.5" />已暂停</span>
          <strong class="mt-1 block text-xl">{{ summary?.suspendedUsers ?? '-' }}</strong>
        </button>
        <button type="button" class="min-w-0 px-4 py-3 text-left hover:bg-red-50/70" :class="{ 'bg-red-50/80': statusFilter === 'banned' }" @click="filterByStatus('banned')">
          <span class="flex items-center gap-2 text-xs text-red-700"><Ban class="h-3.5 w-3.5" />已封禁</span>
          <strong class="mt-1 block text-xl">{{ summary?.bannedUsers ?? '-' }}</strong>
        </button>
        <button type="button" class="min-w-0 px-4 py-3 text-left hover:bg-zinc-100/80" :class="{ 'bg-zinc-100': statusFilter === 'archived' }" @click="filterByStatus('archived')">
          <span class="flex items-center gap-2 text-xs text-zinc-600"><Archive class="h-3.5 w-3.5" />已归档</span>
          <strong class="mt-1 block text-xl">{{ summary?.archivedUsers ?? '-' }}</strong>
        </button>
        <button type="button" class="min-w-0 px-4 py-3 text-left hover:bg-muted/50" :class="{ 'bg-muted/60': roleFilter === 'admin' }" @click="roleFilter = roleFilter === 'admin' ? 'all' : 'admin'">
          <span class="flex items-center gap-2 text-xs text-muted-foreground"><KeyRound class="h-3.5 w-3.5" />管理员</span>
          <strong class="mt-1 block text-xl">{{ summary?.adminUsers ?? '-' }}</strong>
        </button>
        <button type="button" class="min-w-0 px-4 py-3 text-left hover:bg-muted/50" :class="{ 'bg-muted/60': linuxDoFilter === 'bound' }" @click="linuxDoFilter = linuxDoFilter === 'bound' ? 'all' : 'bound'">
          <span class="flex items-center gap-2 text-xs text-muted-foreground"><Link2 class="h-3.5 w-3.5" />linux.do</span>
          <strong class="mt-1 block text-xl">{{ summary?.linuxDoBoundUsers ?? '-' }}</strong>
        </button>
      </div>
    </section>

    <section class="space-y-3" aria-label="用户筛选">
      <form class="flex flex-col gap-2 sm:flex-row sm:items-center" @submit.prevent="submitSearch">
        <div class="relative min-w-0 flex-1 sm:max-w-md">
          <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input v-model="searchDraft" maxlength="100" class="pl-9" placeholder="搜索用户名或显示名称" />
        </div>
        <Button type="submit">搜索</Button>
        <Button type="button" variant="ghost" @click="clearFilters">清空</Button>
        <div class="flex-1" />
        <Select v-model="roleFilter">
          <SelectTrigger class="w-full sm:w-36"><SelectValue placeholder="账号角色" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部角色</SelectItem>
            <SelectItem value="admin">管理员</SelectItem>
            <SelectItem value="user">普通用户</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="linuxDoFilter">
          <SelectTrigger class="w-full sm:w-44"><SelectValue placeholder="绑定状态" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部绑定状态</SelectItem>
            <SelectItem value="bound">已绑定 linux.do</SelectItem>
            <SelectItem value="unbound">未绑定 linux.do</SelectItem>
          </SelectContent>
        </Select>
      </form>

      <div class="flex flex-wrap gap-1" role="group" aria-label="账号状态">
        <Button v-for="item in [
          { value: 'all', label: '全部' },
          { value: 'active', label: '正常' },
          { value: 'suspended', label: '已暂停' },
          { value: 'banned', label: '已封禁' },
          { value: 'archived', label: '已归档' },
        ]" :key="item.value" size="sm" :variant="statusFilter === item.value ? 'secondary' : 'ghost'" @click="filterByStatus(item.value as AdminUserDirectoryStatus)">
          {{ item.label }}
        </Button>
      </div>
    </section>

    <div class="flex flex-col gap-2 border-t border-border pt-4 sm:flex-row sm:items-center">
      <p class="text-sm text-muted-foreground">共 {{ pagination?.totalItems ?? 0 }} 个账号</p>
      <div class="flex-1" />
      <div class="flex flex-wrap items-center gap-2">
        <Select v-model="sortFilter">
          <SelectTrigger class="w-40"><SelectValue placeholder="排序" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="created_desc">注册时间最新</SelectItem>
            <SelectItem value="created_asc">注册时间最早</SelectItem>
            <SelectItem value="active_desc">最近活跃优先</SelectItem>
            <SelectItem value="username_asc">用户名升序</SelectItem>
            <SelectItem value="username_desc">用户名降序</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="limitFilter">
          <SelectTrigger class="w-28"><SelectValue placeholder="每页数量" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="20">20 条</SelectItem>
            <SelectItem value="50">50 条</SelectItem>
            <SelectItem value="100">100 条</SelectItem>
          </SelectContent>
        </Select>
        <Button size="icon" variant="outline" :disabled="directoryRequest.isFetching.value" title="刷新用户目录" aria-label="刷新用户目录" @click="directoryRequest.refetch()">
          <RefreshCcw class="h-4 w-4" :class="{ 'animate-spin': directoryRequest.isFetching.value }" />
        </Button>
      </div>
    </div>

    <SkeletonTable v-if="directoryRequest.isLoading.value" :rows="8" :columns="6" />
    <ErrorState
      v-else-if="directoryRequest.error.value"
      title="用户目录加载失败"
      :description="backendErrorMessage(directoryRequest.error.value, '当前无法读取管理员用户目录。')"
      @retry="directoryRequest.refetch()"
    />
    <div v-else class="admin-users-table">
      <SoftTable :columns="['账号', '资料与绑定', '角色', '账号状态', '注册 / 活跃', '操作']">
        <tr v-for="user in rows" :key="user.id">
          <td>
            <div class="font-medium">{{ user.username }}</div>
            <div class="mt-1 text-xs text-muted-foreground">{{ user.displayName }}</div>
          </td>
          <td class="text-muted-foreground">
            {{ user.linuxDoBound ? `已绑定 linux.do${user.trustLevel === undefined ? '' : ` · 信任等级 ${user.trustLevel}`}` : '未绑定 linux.do' }}
          </td>
          <td>{{ user.isAdmin ? '管理员' : '普通用户' }}</td>
          <td>
            <Badge variant="outline" :class="statusBadgeClass(user.accountStatus)">
              <component :is="statusIcon(user.accountStatus)" class="h-3.5 w-3.5" />
              {{ statusDisplay(user.accountStatus) }}
            </Badge>
          </td>
          <td class="text-sm text-muted-foreground">
            <div>注册 {{ formatTime(user.createdAt) }}</div>
            <div class="mt-1">活跃 {{ formatTime(user.lastActiveAt) }}</div>
          </td>
          <td>
            <div class="flex items-center gap-1">
              <Button size="sm" variant="outline" @click="openManagement(user)">
                <UserRoundCog class="h-3.5 w-3.5" />
                查看
              </Button>
              <Button as-child size="icon" variant="ghost" title="打开公开主页">
                <RouterLink :to="`/u/${user.username}`">
                  <Eye class="h-4 w-4" />
                  <span class="sr-only">打开公开主页</span>
                </RouterLink>
              </Button>
            </div>
          </td>
        </tr>
        <tr v-if="rows.length === 0">
          <td colspan="6" class="py-10 text-center text-sm text-muted-foreground">没有符合当前筛选的账号。</td>
        </tr>
        <template #footer>
          <TablePagination
            :page="pagination?.page ?? directoryQuery.page"
            :page-count="pageCount"
            :total="pagination?.totalItems ?? 0"
            :start-item="startItem"
            :end-item="endItem"
            @update:page="replaceDirectoryQuery({ page: $event })"
          />
        </template>
      </SoftTable>
    </div>

    <Dialog :open="managementOpen" @update:open="setManagementOpen">
      <DialogContent class="bottom-0 left-auto right-0 top-0 flex h-dvh max-h-dvh w-full max-w-full translate-x-0 translate-y-0 grid-cols-1 gap-0 overflow-hidden rounded-none border-l border-r-0 p-0 shadow-xl duration-200 data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right data-[state=closed]:zoom-out-100 data-[state=open]:zoom-in-100 sm:max-w-3xl">
        <div class="flex h-full min-h-0 flex-col">
          <DialogHeader class="border-b border-border px-5 py-4 pr-12">
            <div class="flex flex-wrap items-center gap-2">
              <DialogTitle>{{ selectedUser?.username || '账号详情' }}</DialogTitle>
              <Badge v-if="selectedUser" variant="outline" :class="statusBadgeClass(selectedUser.accountStatus)">
                <component :is="statusIcon(selectedUser.accountStatus)" class="h-3.5 w-3.5" />
                {{ statusDisplay(selectedUser.accountStatus) }}
              </Badge>
              <Badge v-if="selectedUser?.isAdmin" variant="secondary">管理员</Badge>
            </div>
            <DialogDescription>{{ selectedUser?.displayName || '加载账号资料中' }}</DialogDescription>
          </DialogHeader>

          <div class="min-h-0 flex-1 overflow-y-auto px-5 py-5">
            <SkeletonTable v-if="detailQuery.isLoading.value" :rows="5" :columns="2" />
            <ErrorState
              v-else-if="detailQuery.error.value"
              title="账号详情加载失败"
              :description="backendErrorMessage(detailQuery.error.value, '当前无法读取账号详情。')"
              @retry="detailQuery.refetch()"
            />
            <Tabs v-else-if="selectedDetail" v-model="activeDrawerTab" class="space-y-5">
              <TabsList class="grid h-auto w-full grid-cols-4">
                <TabsTrigger value="overview">概览</TabsTrigger>
                <TabsTrigger value="governance">账号治理</TabsTrigger>
                <TabsTrigger value="capabilities">能力限制</TabsTrigger>
                <TabsTrigger value="audit">审计记录</TabsTrigger>
              </TabsList>

              <TabsContent value="overview" class="mt-0 space-y-5">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <h2 class="text-sm font-semibold">账号资料</h2>
                    <p class="mt-1 text-xs text-muted-foreground">更新于 {{ formatTime(selectedDetail.updatedAt) }}</p>
                  </div>
                  <Button as-child size="sm" variant="outline">
                    <RouterLink :to="`/u/${selectedDetail.user.username}`"><Eye class="h-4 w-4" />公开主页</RouterLink>
                  </Button>
                </div>
                <dl class="grid gap-x-5 gap-y-4 border-y border-border py-4 text-sm sm:grid-cols-2">
                  <div><dt class="text-xs text-muted-foreground">linux.do</dt><dd class="mt-1 font-medium">{{ selectedDetail.linuxDoBinding.bound ? `已绑定${selectedDetail.linuxDoBinding.username ? ` · ${selectedDetail.linuxDoBinding.username}` : ''}` : '未绑定' }}</dd></div>
                  <div><dt class="text-xs text-muted-foreground">信任等级</dt><dd class="mt-1 font-medium">{{ selectedDetail.linuxDoBinding.trustLevel ?? '暂无' }}</dd></div>
                  <div><dt class="text-xs text-muted-foreground">邮箱验证</dt><dd class="mt-1 font-medium">{{ selectedDetail.emailVerified ? '已验证' : '未验证' }}</dd></div>
                  <div><dt class="text-xs text-muted-foreground">备用密码</dt><dd class="mt-1 font-medium">{{ selectedDetail.backupPasswordConfigured ? '已配置' : '未配置' }}</dd></div>
                  <div><dt class="text-xs text-muted-foreground">有效会话</dt><dd class="mt-1 font-medium">{{ selectedDetail.sessions.activeCount }} 个</dd></div>
                  <div><dt class="text-xs text-muted-foreground">最近会话活动</dt><dd class="mt-1 font-medium">{{ formatTime(selectedDetail.sessions.latestActivityAt) }}</dd></div>
                  <div class="sm:col-span-2"><dt class="text-xs text-muted-foreground">认证方式</dt><dd class="mt-1 font-medium">{{ selectedDetail.providers.map(item => item.provider).join('、') || '无第三方认证方式' }}</dd></div>
                </dl>

                <section class="space-y-3">
                  <h2 class="text-sm font-semibold">当前业务影响</h2>
                  <div class="grid grid-cols-2 gap-px overflow-hidden border border-border bg-border sm:grid-cols-3">
                    <div class="bg-background p-3"><div class="text-xs text-muted-foreground">有效会话</div><div class="mt-1 font-semibold">{{ selectedDetail.impactPreview.activeSessions }}</div></div>
                    <div class="bg-background p-3"><div class="text-xs text-muted-foreground">在架车源</div><div class="mt-1 font-semibold">{{ selectedDetail.impactPreview.activeCarpoolListings }}</div></div>
                    <div class="bg-background p-3"><div class="text-xs text-muted-foreground">在线 API 服务</div><div class="mt-1 font-semibold">{{ selectedDetail.impactPreview.onlineApiServices }}</div></div>
                    <div class="bg-background p-3"><div class="text-xs text-muted-foreground">进行中申请</div><div class="mt-1 font-semibold">{{ selectedDetail.impactPreview.openCarpoolApplications }}</div></div>
                    <div class="bg-background p-3"><div class="text-xs text-muted-foreground">进行中 API 订单</div><div class="mt-1 font-semibold">{{ selectedDetail.impactPreview.openApiOrders }}</div></div>
                    <div class="bg-background p-3"><div class="text-xs text-muted-foreground">待处理纠纷</div><div class="mt-1 font-semibold">{{ selectedDetail.impactPreview.openDisputes }}</div></div>
                  </div>
                </section>
              </TabsContent>

              <TabsContent value="governance" class="mt-0 space-y-5">
                <Alert v-if="blockedActions.length">
                  <TriangleAlert class="h-4 w-4" />
                  <AlertTitle>部分操作当前不可执行</AlertTitle>
                  <AlertDescription>
                    <ul class="mt-1 space-y-1">
                      <li v-for="action in blockedActions" :key="action.action">{{ governanceActionLabel(action) }}：{{ action.blockedReason }}</li>
                    </ul>
                  </AlertDescription>
                </Alert>

                <section v-if="!pendingAction" class="space-y-5">
                  <div class="space-y-3">
                    <div>
                      <h2 class="text-sm font-semibold">账号状态</h2>
                      <p class="mt-1 text-sm text-muted-foreground">当前为{{ statusDisplay(selectedDetail.user.accountStatus) }}</p>
                    </div>
                    <div class="flex flex-wrap gap-2">
                      <Button
                        v-if="primaryStatusAction"
                        :variant="primaryStatusAction.action === 'restore' ? 'default' : 'outline'"
                        :disabled="!primaryStatusAction.allowed"
                        :title="primaryStatusAction.blockedReason"
                        @click="openAction(primaryStatusAction)"
                      >
                        <component :is="actionIcon(primaryStatusAction)" class="h-4 w-4" />
                        {{ governanceActionLabel(primaryStatusAction) }}
                      </Button>
                      <DropdownMenu v-if="secondaryStatusActions.length">
                        <DropdownMenuTrigger as-child>
                          <Button variant="outline"><MoreHorizontal class="h-4 w-4" />更多操作</Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="start">
                          <DropdownMenuItem
                            v-for="action in secondaryStatusActions"
                            :key="action.action"
                            :disabled="!action.allowed"
                            :class="{ 'text-destructive focus:text-destructive': action.severity === 'danger' }"
                            @select="openAction(action)"
                          >
                            <component :is="actionIcon(action)" class="h-4 w-4" />
                            {{ governanceActionLabel(action) }}
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </div>

                  <div class="border-t border-border pt-5">
                    <h2 class="text-sm font-semibold">管理员权限</h2>
                    <p class="mt-1 text-sm text-muted-foreground">{{ selectedDetail.user.isAdmin ? '当前拥有管理端访问权限' : '当前为普通用户' }}</p>
                    <Button
                      v-if="permissionAction"
                      class="mt-3"
                      variant="outline"
                      :disabled="!permissionAction.allowed"
                      :title="permissionAction.blockedReason"
                      @click="openAction(permissionAction)"
                    >
                      <ShieldCheck class="h-4 w-4" />
                      {{ governanceActionLabel(permissionAction) }}
                    </Button>
                  </div>
                </section>

                <section v-else class="space-y-4">
                  <div>
                    <h2 class="text-sm font-semibold">{{ actionTitle }}</h2>
                    <p class="mt-1 text-sm text-muted-foreground">{{ actionDescription }}</p>
                  </div>
                  <Alert>
                    <TriangleAlert class="h-4 w-4" />
                    <AlertTitle>操作影响</AlertTitle>
                    <AlertDescription>
                      <p>有效会话 {{ selectedDetail.impactPreview.activeSessions }} 个，在架车源 {{ selectedDetail.impactPreview.activeCarpoolListings }} 个，在线 API 服务 {{ selectedDetail.impactPreview.onlineApiServices }} 个。</p>
                      <p class="mt-1">进行中申请 {{ selectedDetail.impactPreview.openCarpoolApplications }} 个，进行中 API 订单 {{ selectedDetail.impactPreview.openApiOrders }} 个，待处理纠纷 {{ selectedDetail.impactPreview.openDisputes }} 个；历史订单与纠纷仍保留可查。</p>
                    </AlertDescription>
                  </Alert>
                  <label class="block space-y-2">
                    <span class="text-sm font-medium">操作原因</span>
                    <Textarea v-model="reason" maxlength="500" class="min-h-28" placeholder="填写具体、可审计的操作原因" />
                    <span class="block text-right text-xs text-muted-foreground">{{ reason.length }}/500</span>
                  </label>
                  <label class="flex items-start gap-3 border border-border p-3 text-sm leading-5">
                    <Checkbox v-model="confirmed" class="mt-0.5" />
                    <span>我已核对目标账号和操作影响，确认执行“{{ actionTitle }}”。</span>
                  </label>
                  <div class="flex flex-wrap gap-2">
                    <Button variant="outline" :disabled="actionPending" @click="resetPendingAction">返回</Button>
                    <Button
                      :variant="pendingAction.severity === 'danger' ? 'destructive' : 'default'"
                      :disabled="actionPending || !reason.trim() || !confirmed"
                      @click="confirmAction"
                    >
                      {{ actionPending ? '正在提交...' : `确认${actionTitle}` }}
                    </Button>
                  </div>
                </section>
              </TabsContent>

              <TabsContent value="capabilities" class="mt-0 space-y-6">
                <section class="space-y-3">
                  <div>
                    <h2 class="text-sm font-semibold">账号状态能力</h2>
                    <p class="mt-1 text-sm text-muted-foreground">{{ statusDisplay(selectedDetail.user.accountStatus) }}</p>
                  </div>
                  <div class="divide-y divide-border border-y border-border">
                    <div v-for="capability in capabilityRows" :key="capability.label" class="flex items-center justify-between gap-3 py-3 text-sm">
                      <span>{{ capability.label }}</span>
                      <Badge variant="outline" :class="capability.allowed ? 'border-emerald-200 bg-emerald-50 text-emerald-700' : 'border-red-200 bg-red-50 text-red-700'">
                        <CheckCircle2 v-if="capability.allowed" class="h-3.5 w-3.5" />
                        <Ban v-else class="h-3.5 w-3.5" />
                        {{ capability.allowed ? '允许' : '禁止' }}
                      </Badge>
                    </div>
                  </div>
                </section>
                <section class="border-t border-border pt-6">
                  <AdminReputationAuditPanel
                    embedded
                    :user-id="selectedDetail.user.id"
                    :username="selectedDetail.user.username"
                    :user-version="selectedDetail.user.version"
                  />
                </section>
              </TabsContent>

              <TabsContent value="audit" class="mt-0 space-y-3">
                <div class="flex items-center gap-2">
                  <History class="h-4 w-4 text-muted-foreground" />
                  <h2 class="text-sm font-semibold">账号治理记录</h2>
                </div>
                <div v-if="selectedDetail.recentAuditEntries.length" class="divide-y divide-border border-y border-border">
                  <article v-for="entry in selectedDetail.recentAuditEntries" :key="entry.id" class="py-4 text-sm">
                    <div class="flex flex-wrap items-start justify-between gap-2">
                      <div>
                        <div class="font-medium">{{ auditActionLabel(entry.action) }}</div>
                        <div v-if="auditTransition(entry)" class="mt-1 font-medium text-primary">{{ auditTransition(entry) }}</div>
                      </div>
                      <time class="text-xs text-muted-foreground">{{ formatTime(entry.createdAt) }}</time>
                    </div>
                    <p class="mt-2 text-muted-foreground">{{ entry.reason }}</p>
                    <p class="mt-1 text-xs text-muted-foreground">操作者：{{ entry.adminUsername }}</p>
                  </article>
                </div>
                <p v-else class="py-8 text-center text-sm text-muted-foreground">暂无账号治理记录。</p>
              </TabsContent>
            </Tabs>
          </div>

          <DialogFooter class="border-t border-border px-5 py-4">
            <Button variant="outline" @click="setManagementOpen(false)">关闭</Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.admin-users-table :deep(.c2c-soft-table) {
  min-width: 960px;
}

.admin-users-table :deep(.c2c-soft-table th:nth-child(1)),
.admin-users-table :deep(.c2c-soft-table td:nth-child(1)) {
  width: 150px;
}

.admin-users-table :deep(.c2c-soft-table th:nth-child(2)),
.admin-users-table :deep(.c2c-soft-table td:nth-child(2)) {
  width: 210px;
}

.admin-users-table :deep(.c2c-soft-table th:nth-child(3)),
.admin-users-table :deep(.c2c-soft-table td:nth-child(3)),
.admin-users-table :deep(.c2c-soft-table th:nth-child(4)),
.admin-users-table :deep(.c2c-soft-table td:nth-child(4)) {
  width: 110px;
}

.admin-users-table :deep(.c2c-soft-table th:nth-child(5)),
.admin-users-table :deep(.c2c-soft-table td:nth-child(5)) {
  width: 220px;
}

.admin-users-table :deep(.c2c-soft-table th:nth-child(6)),
.admin-users-table :deep(.c2c-soft-table td:nth-child(6)) {
  width: 130px;
}
</style>
