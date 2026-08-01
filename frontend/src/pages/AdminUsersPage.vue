<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import {
  Activity,
  Eye,
  KeyRound,
  Link2,
  RefreshCcw,
  Search,
  ShieldCheck,
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
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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
  type AdminUserStatus,
} from '@/lib/adminUserBackend'
import {
  useAdminUserDetail,
  useAdminUserDirectory,
  useCurrentAdminSession,
  useUpdateAdminUserPermissionMutation,
  useUpdateAdminUserStatusMutation,
} from '@/queries/useAdminUserQueries'

type PendingAction =
  | { kind: 'status', status: AdminUserStatus }
  | { kind: 'permission', isAdmin: boolean }

const route = useRoute()
const router = useRouter()
const directoryQuery = computed(() => normalizeAdminUserDirectoryQuery(route.query as Record<string, unknown>))
const directoryRequest = useAdminUserDirectory(directoryQuery)
const currentAdminSession = useCurrentAdminSession()
const statusMutation = useUpdateAdminUserStatusMutation()
const permissionMutation = useUpdateAdminUserPermissionMutation()

const managementOpen = ref(false)
const selectedUserId = ref('')
const reputationUser = ref<AdminUser | null>(null)
const searchDraft = ref(directoryQuery.value.search)
const pendingAction = ref<PendingAction | null>(null)
const reason = ref('')
const confirmed = ref(false)
const detailQuery = useAdminUserDetail(selectedUserId)

const rows = computed(() => directoryRequest.data.value?.items ?? [])
const pagination = computed(() => directoryRequest.data.value?.pagination)
const summary = computed(() => directoryRequest.data.value?.summary)
const pageCount = computed(() => Math.max(1, pagination.value?.totalPages ?? 1))
const startItem = computed(() => {
  if (!pagination.value?.totalItems) return 0
  return (pagination.value.page - 1) * pagination.value.limit + 1
})
const endItem = computed(() => {
  if (!pagination.value?.totalItems) return 0
  return Math.min(pagination.value.page * pagination.value.limit, pagination.value.totalItems)
})
const selectedDetail = computed(() => detailQuery.data.value)
const selectedUser = computed(() => selectedDetail.value?.user ?? rows.value.find(item => item.id === selectedUserId.value) ?? null)
const isSelfTarget = computed(() => selectedUser.value?.id === currentAdminSession.data.value?.user.id)
const isKnownLastAdmin = computed(() => Boolean(selectedUser.value?.isAdmin && summary.value?.adminUsers === 1))
const actionPending = computed(() => statusMutation.isPending.value || permissionMutation.isPending.value)
const reputationVersion = computed(() => {
  if (!reputationUser.value) return undefined
  if (selectedDetail.value?.user.id === reputationUser.value.id) return selectedDetail.value.user.version
  return rows.value.find(item => item.id === reputationUser.value?.id)?.version ?? reputationUser.value.version
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

const statusActions = computed(() => {
  const current = selectedUser.value?.accountStatus
  if (!current) return []
  const allowed: Record<AdminUserStatus, AdminUserStatus[]> = {
    active: ['suspended', 'banned', 'archived'],
    suspended: ['active', 'banned', 'archived'],
    banned: ['active', 'archived'],
    archived: ['active'],
  }
  return allowed[current]
})

const actionTitle = computed(() => {
  if (!pendingAction.value) return ''
  if (pendingAction.value.kind === 'permission') return pendingAction.value.isAdmin ? '授予管理员权限' : '撤销管理员权限'
  return `${statusLabel(pendingAction.value.status)}账号`
})

const actionDescription = computed(() => {
  if (!pendingAction.value) return ''
  if (pendingAction.value.kind === 'permission') {
    return pendingAction.value.isAdmin
      ? '该账号将获得管理员权限。系统会记录操作者、原因和变更前后状态。'
      : '该账号将失去管理员权限。最后一个有效管理员不能被撤销。'
  }
  if (pendingAction.value.status === 'active') return '账号将恢复正常使用，原有会话不会自动恢复。'
  return '账号离开正常状态后，当前登录会话会立即全部撤销。'
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

watch(rows, items => {
  if (!reputationUser.value) return
  const refreshed = items.find(item => item.id === reputationUser.value?.id)
  if (refreshed) reputationUser.value = refreshed
})

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

function openManagement(user: AdminUser) {
  selectedUserId.value = user.id
  pendingAction.value = null
  reason.value = ''
  confirmed.value = false
  managementOpen.value = true
}

function setManagementOpen(open: boolean) {
  managementOpen.value = open
  if (!open) {
    pendingAction.value = null
    reason.value = ''
    confirmed.value = false
  }
}

function openAction(action: PendingAction) {
  if (isSelfTarget.value) {
    toast.warning('不能修改自己的账号状态或管理员权限。')
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
    if (action.kind === 'status') {
      await statusMutation.mutateAsync({
        userId: detail.user.id,
        version: detail.user.version,
        status: action.status,
        reason: reason.value,
      })
    } else {
      await permissionMutation.mutateAsync({
        userId: detail.user.id,
        version: detail.user.version,
        isAdmin: action.isAdmin,
        reason: reason.value,
      })
    }
    toast.success(`${actionTitle.value}已完成。`)
    pendingAction.value = null
    reason.value = ''
    confirmed.value = false
  } catch (error) {
    const message = backendErrorMessage(error, '账号治理操作失败。')
    toast.error(message)
    if (error instanceof BackendProblemError && error.code === 'VERSION_CONFLICT') {
      toast.warning('已刷新账号详情，请核对最新状态后重新操作。')
    }
  }
}

function statusLabel(status: AdminUserStatus) {
  return {
    active: '恢复',
    suspended: '暂停',
    banned: '封禁',
    archived: '归档',
  }[status]
}

function statusDisplay(status: AdminUserStatus) {
  return {
    active: '正常',
    suspended: '已暂停',
    banned: '已封禁',
    archived: '已归档',
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
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="用户目录" description="按账号状态、角色和 linux.do 绑定管理全站用户；分页、筛选和统计均以服务端结果为准。" />

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Card class="p-4">
        <div class="flex items-center justify-between gap-3">
          <div><div class="text-xs text-muted-foreground">全部账号</div><div class="mt-1 text-2xl font-semibold">{{ summary?.totalUsers ?? '-' }}</div></div>
          <UsersRound class="h-5 w-5 text-primary" />
        </div>
      </Card>
      <Card class="p-4">
        <div class="flex items-center justify-between gap-3">
          <div><div class="text-xs text-muted-foreground">正常账号</div><div class="mt-1 text-2xl font-semibold">{{ summary?.activeUsers ?? '-' }}</div></div>
          <Activity class="h-5 w-5 text-emerald-600" />
        </div>
        <div class="mt-2 text-xs text-muted-foreground">暂停 {{ summary?.suspendedUsers ?? '-' }} · 封禁 {{ summary?.bannedUsers ?? '-' }} · 归档 {{ summary?.archivedUsers ?? '-' }}</div>
      </Card>
      <Card class="p-4">
        <div class="flex items-center justify-between gap-3">
          <div><div class="text-xs text-muted-foreground">管理员账号</div><div class="mt-1 text-2xl font-semibold">{{ summary?.adminUsers ?? '-' }}</div></div>
          <KeyRound class="h-5 w-5 text-amber-600" />
        </div>
      </Card>
      <Card class="p-4">
        <div class="flex items-center justify-between gap-3">
          <div><div class="text-xs text-muted-foreground">已绑定 linux.do</div><div class="mt-1 text-2xl font-semibold">{{ summary?.linuxDoBoundUsers ?? '-' }}</div></div>
          <Link2 class="h-5 w-5 text-sky-600" />
        </div>
      </Card>
    </div>

    <div class="space-y-3 border-y border-border py-4">
      <form class="flex flex-col gap-2 sm:flex-row" @submit.prevent="submitSearch">
        <div class="relative min-w-0 flex-1 sm:max-w-md">
          <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input v-model="searchDraft" maxlength="100" class="pl-9" placeholder="搜索用户名或显示名称" />
        </div>
        <Button type="submit">搜索</Button>
        <Button type="button" variant="ghost" @click="clearFilters">清空</Button>
      </form>

      <div class="grid gap-2 sm:grid-cols-2 xl:grid-cols-[repeat(5,minmax(0,1fr))_auto]">
        <Select v-model="statusFilter">
          <SelectTrigger><SelectValue placeholder="账号状态" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部状态</SelectItem><SelectItem value="active">正常</SelectItem><SelectItem value="suspended">已暂停</SelectItem><SelectItem value="banned">已封禁</SelectItem><SelectItem value="archived">已归档</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="roleFilter">
          <SelectTrigger><SelectValue placeholder="账号角色" /></SelectTrigger>
          <SelectContent><SelectItem value="all">全部角色</SelectItem><SelectItem value="admin">管理员</SelectItem><SelectItem value="user">普通用户</SelectItem></SelectContent>
        </Select>
        <Select v-model="linuxDoFilter">
          <SelectTrigger><SelectValue placeholder="linux.do 绑定" /></SelectTrigger>
          <SelectContent><SelectItem value="all">全部绑定状态</SelectItem><SelectItem value="bound">已绑定 linux.do</SelectItem><SelectItem value="unbound">未绑定 linux.do</SelectItem></SelectContent>
        </Select>
        <Select v-model="sortFilter">
          <SelectTrigger><SelectValue placeholder="排序" /></SelectTrigger>
          <SelectContent><SelectItem value="created_desc">注册时间最新</SelectItem><SelectItem value="created_asc">注册时间最早</SelectItem><SelectItem value="active_desc">最近活跃优先</SelectItem><SelectItem value="username_asc">用户名升序</SelectItem><SelectItem value="username_desc">用户名降序</SelectItem></SelectContent>
        </Select>
        <Select v-model="limitFilter">
          <SelectTrigger><SelectValue placeholder="每页数量" /></SelectTrigger>
          <SelectContent><SelectItem value="20">每页 20 条</SelectItem><SelectItem value="50">每页 50 条</SelectItem><SelectItem value="100">每页 100 条</SelectItem></SelectContent>
        </Select>
        <Button variant="outline" :disabled="directoryRequest.isFetching.value" title="刷新用户目录" @click="directoryRequest.refetch()">
          <RefreshCcw class="h-4 w-4" :class="{ 'animate-spin': directoryRequest.isFetching.value }" />
          刷新
        </Button>
      </div>
      <p v-if="directoryRequest.isFetching.value && !directoryRequest.isLoading.value" class="text-xs text-muted-foreground">正在更新当前筛选结果...</p>
    </div>

    <AdminReputationAuditPanel
      v-if="reputationUser"
      :user-id="reputationUser.id"
      :username="reputationUser.username"
      :user-version="reputationVersion"
    />

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
        <td><Badge :variant="user.accountStatus === 'active' ? 'default' : 'secondary'">{{ statusDisplay(user.accountStatus) }}</Badge></td>
        <td class="text-sm text-muted-foreground">
          <div>注册 {{ formatTime(user.createdAt) }}</div><div class="mt-1">活跃 {{ formatTime(user.lastActiveAt) }}</div>
        </td>
        <td>
          <div class="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" @click="openManagement(user)"><UserRoundCog class="h-3.5 w-3.5" />账号管理</Button>
            <Button size="sm" :variant="reputationUser?.id === user.id ? 'secondary' : 'outline'" @click="reputationUser = reputationUser?.id === user.id ? null : user"><ShieldCheck class="h-3.5 w-3.5" />信誉审计</Button>
            <Button as-child size="icon" variant="ghost" title="打开公开主页"><RouterLink :to="`/u/${user.username}`"><Eye class="h-4 w-4" /><span class="sr-only">打开公开主页</span></RouterLink></Button>
          </div>
        </td>
      </tr>
      <tr v-if="rows.length === 0"><td colspan="6" class="py-10 text-center text-sm text-muted-foreground">没有符合当前筛选的账号。</td></tr>
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
      <DialogContent class="bottom-0 left-auto right-0 top-0 flex h-dvh max-h-dvh w-full max-w-full translate-x-0 translate-y-0 grid-cols-1 gap-0 overflow-hidden rounded-none border-l border-r-0 p-0 shadow-xl duration-200 data-[state=closed]:slide-out-to-right data-[state=open]:slide-in-from-right data-[state=closed]:zoom-out-100 data-[state=open]:zoom-in-100 sm:max-w-2xl">
        <div class="flex h-full min-h-0 flex-col">
          <DialogHeader class="border-b border-border px-5 py-4 pr-12">
            <div class="flex flex-wrap items-center gap-2"><DialogTitle>账号管理</DialogTitle><Badge v-if="selectedUser" variant="secondary">{{ statusDisplay(selectedUser.accountStatus) }}</Badge></div>
            <DialogDescription>查看必要账号事实，并执行有审计记录的账号状态或管理员权限变更。</DialogDescription>
          </DialogHeader>

          <div class="min-h-0 flex-1 overflow-y-auto px-5 py-5">
            <SkeletonTable v-if="detailQuery.isLoading.value" :rows="5" :columns="2" />
            <ErrorState v-else-if="detailQuery.error.value" title="账号详情加载失败" :description="backendErrorMessage(detailQuery.error.value, '当前无法读取账号详情。')" @retry="detailQuery.refetch()" />
            <div v-else-if="selectedDetail" class="space-y-6">
              <section class="space-y-3">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div><h2 class="text-base font-semibold">{{ selectedDetail.user.username }}</h2><p class="mt-1 text-sm text-muted-foreground">{{ selectedDetail.user.displayName }} · {{ selectedDetail.user.isAdmin ? '管理员' : '普通用户' }}</p></div>
                  <Button as-child size="sm" variant="outline"><RouterLink :to="`/u/${selectedDetail.user.username}`"><Eye class="h-4 w-4" />公开主页</RouterLink></Button>
                </div>
                <div class="grid gap-x-5 gap-y-3 border-y border-border py-4 text-sm sm:grid-cols-2">
                  <div><div class="text-xs text-muted-foreground">账号状态</div><div class="mt-1 font-medium">{{ statusDisplay(selectedDetail.user.accountStatus) }}</div></div>
                  <div><div class="text-xs text-muted-foreground">账号版本</div><div class="mt-1 font-medium">{{ selectedDetail.user.version }}</div></div>
                  <div><div class="text-xs text-muted-foreground">linux.do</div><div class="mt-1 font-medium">{{ selectedDetail.linuxDoBinding.bound ? `已绑定${selectedDetail.linuxDoBinding.username ? ` · ${selectedDetail.linuxDoBinding.username}` : ''}` : '未绑定' }}</div></div>
                  <div><div class="text-xs text-muted-foreground">信任等级</div><div class="mt-1 font-medium">{{ selectedDetail.linuxDoBinding.trustLevel ?? '暂无' }}</div></div>
                  <div><div class="text-xs text-muted-foreground">邮箱验证</div><div class="mt-1 font-medium">{{ selectedDetail.emailVerified ? '已验证' : '未验证' }}</div></div>
                  <div><div class="text-xs text-muted-foreground">备用密码</div><div class="mt-1 font-medium">{{ selectedDetail.backupPasswordConfigured ? '已配置' : '未配置' }}</div></div>
                  <div><div class="text-xs text-muted-foreground">有效会话</div><div class="mt-1 font-medium">{{ selectedDetail.sessions.activeCount }} 个</div></div>
                  <div><div class="text-xs text-muted-foreground">最近会话活动</div><div class="mt-1 font-medium">{{ formatTime(selectedDetail.sessions.latestActivityAt) }}</div></div>
                  <div><div class="text-xs text-muted-foreground">认证方式</div><div class="mt-1 font-medium">{{ selectedDetail.providers.map(item => item.provider).join('、') || '无第三方认证方式' }}</div></div>
                  <div><div class="text-xs text-muted-foreground">详情更新时间</div><div class="mt-1 font-medium">{{ formatTime(selectedDetail.updatedAt) }}</div></div>
                </div>
              </section>

              <section v-if="!pendingAction" class="space-y-3">
                <div><h2 class="text-sm font-semibold">账号治理</h2><p class="mt-1 text-sm text-muted-foreground">状态变更会写入审计记录；账号离开正常状态时会撤销全部有效会话。</p></div>
                <div v-if="isSelfTarget" class="border-l-2 border-amber-500 pl-3 text-sm text-muted-foreground">当前账号不能修改自己的状态或管理员权限。</div>
                <div v-if="isKnownLastAdmin" class="border-l-2 border-amber-500 pl-3 text-sm text-muted-foreground">这是目录中最后一个管理员，停用或撤销权限会被系统拒绝。</div>
                <div class="flex flex-wrap gap-2">
                  <Button v-for="status in statusActions" :key="status" size="sm" :variant="status === 'active' ? 'outline' : 'destructive'" :disabled="isSelfTarget || (isKnownLastAdmin && selectedDetail.user.accountStatus === 'active' && status !== 'active')" @click="openAction({ kind: 'status', status })">{{ statusLabel(status) }}账号</Button>
                  <Button size="sm" :variant="selectedDetail.user.isAdmin ? 'destructive' : 'outline'" :disabled="isSelfTarget || (selectedDetail.user.isAdmin && isKnownLastAdmin) || (!selectedDetail.user.isAdmin && selectedDetail.user.accountStatus !== 'active')" @click="openAction({ kind: 'permission', isAdmin: !selectedDetail.user.isAdmin })">{{ selectedDetail.user.isAdmin ? '撤销管理员权限' : '授予管理员权限' }}</Button>
                </div>
                <p class="text-xs leading-5 text-muted-foreground">系统会在事务内再次检查最后一个有效管理员，页面判断只用于提前阻止明确不可执行的操作。</p>
              </section>

              <section v-else class="space-y-4 border-t border-border pt-5">
                <div><h2 class="text-sm font-semibold">{{ actionTitle }}</h2><p class="mt-1 text-sm text-muted-foreground">{{ actionDescription }}</p></div>
                <label class="block space-y-2"><span class="text-sm font-medium">操作原因</span><Textarea v-model="reason" maxlength="500" class="min-h-28" placeholder="填写具体、可审计的操作原因。" /><span class="block text-right text-xs text-muted-foreground">{{ reason.length }}/500</span></label>
                <label class="flex items-start gap-3 border border-border p-3 text-sm leading-5">
                  <input v-model="confirmed" type="checkbox" class="mt-0.5 h-4 w-4 accent-primary" />
                  <span>二次确认：我已核对目标账号、当前状态和操作原因，确认执行“{{ actionTitle }}”。</span>
                </label>
                <div class="flex flex-wrap gap-2"><Button variant="outline" :disabled="actionPending" @click="pendingAction = null">返回</Button><Button :variant="pendingAction.kind === 'status' && pendingAction.status !== 'active' || pendingAction.kind === 'permission' && !pendingAction.isAdmin ? 'destructive' : 'default'" :disabled="actionPending || !reason.trim() || !confirmed" @click="confirmAction">{{ actionPending ? '正在提交...' : `确认${actionTitle}` }}</Button></div>
              </section>

              <section class="space-y-3 border-t border-border pt-5">
                <div><h2 class="text-sm font-semibold">最近账号审计</h2><p class="mt-1 text-sm text-muted-foreground">仅展示账号状态和管理员权限治理记录。</p></div>
                <div v-if="selectedDetail.recentAuditEntries.length" class="divide-y divide-border border-y border-border">
                  <div v-for="entry in selectedDetail.recentAuditEntries" :key="entry.id" class="py-3 text-sm">
                    <div class="flex flex-wrap items-center justify-between gap-2"><span class="font-medium">{{ auditActionLabel(entry.action) }}</span><span class="text-xs text-muted-foreground">{{ formatTime(entry.createdAt) }}</span></div>
                    <p class="mt-1 text-muted-foreground">{{ entry.reason }}</p><p class="mt-1 text-xs text-muted-foreground">操作者：{{ entry.adminUsername }}</p>
                  </div>
                </div>
                <p v-else class="text-sm text-muted-foreground">暂无账号治理记录。</p>
              </section>
            </div>
          </div>

          <DialogFooter class="border-t border-border px-5 py-4"><Button variant="outline" @click="setManagementOpen(false)">关闭</Button></DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  </div>
</template>

<style scoped>
.admin-users-table :deep(.c2c-soft-table) {
  min-width: 1080px;
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
  width: 280px;
}
</style>
