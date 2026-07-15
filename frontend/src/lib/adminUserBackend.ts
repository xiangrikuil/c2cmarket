import type { AdminRow } from '@/lib/api'
import { backendRequest, ensureBackendSession } from '@/lib/backendClient'

type ListResponse<T> = {
  items: T[]
}

export type BackendAdminUser = {
  id: string
  username: string
  displayName: string
  accountStatus: 'active' | 'suspended' | 'banned' | 'archived'
  isAdmin: boolean
  linuxDoBound: boolean
  trustLevel?: number
  createdAt: string
  lastActiveAt?: string | null
}

function formatTime(value: string | undefined | null) {
  if (!value) return '从未'
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('sv-SE', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function accountStatusLabel(value: BackendAdminUser['accountStatus']) {
  const labels: Record<BackendAdminUser['accountStatus'], string> = {
    active: '正常',
    suspended: '已暂停',
    banned: '已封禁',
    archived: '已归档',
  }
  return labels[value]
}

export function mapBackendAdminUser(item: BackendAdminUser): AdminRow {
  const trust = item.linuxDoBound ? `已绑定 linux.do · 信任等级${item.trustLevel ?? 0}` : '未绑定 linux.do'
  return {
    id: item.id,
    primary: item.username,
    secondary: `${item.displayName} · ${trust}`,
    owner: item.isAdmin ? '管理员账号' : '普通账号',
    status: accountStatusLabel(item.accountStatus),
    risk: `注册 ${formatTime(item.createdAt)} · 最近活跃 ${formatTime(item.lastActiveAt)}`,
    targetType: 'user',
    backendKind: 'admin-user',
    detailItems: [
      { label: '显示名称', value: item.displayName },
      { label: '账号状态', value: accountStatusLabel(item.accountStatus) },
      { label: '账号角色', value: item.isAdmin ? '管理员' : '普通用户' },
      { label: 'linux.do 绑定', value: item.linuxDoBound ? `已绑定，信任等级${item.trustLevel ?? 0}` : '未绑定' },
      { label: '注册时间', value: formatTime(item.createdAt) },
      { label: '最近活跃', value: formatTime(item.lastActiveAt) },
    ],
    targetTo: `/u/${item.username}`,
  }
}

export async function backendAdminUserRows() {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<BackendAdminUser>>('/api/v1/admin/users')
  return response.items.map(mapBackendAdminUser)
}
