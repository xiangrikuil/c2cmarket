import type {
  ApiService,
  ApiServiceSalesChannel,
  ApiServiceSalesChannelKind,
  ApiServiceSalesState,
  ApiServiceSalesSummary,
  ApiServiceSalesView,
} from '@/lib/api'
import type { ApiHealthAvailabilityReason, ApiHealthState, ApiServiceHealthSummary } from '@/types/apiHealth'
import { formatDecimal } from '@/lib/decimal'

export type ApiServiceOwnerStatusTone = 'success' | 'waiting' | 'warning' | 'neutral'

export type ApiServiceOwnerStatus = {
  label: '接单中' | '审核中' | '已暂停' | '未上线'
  tone: ApiServiceOwnerStatusTone
}

export function getApiServiceOwnerStatus(
  service: Pick<ApiService, 'online' | 'state'>,
): ApiServiceOwnerStatus {
  if (service.online) return { label: '接单中', tone: 'success' }
  if (service.state === 'reviewing') return { label: '审核中', tone: 'waiting' }
  if (service.state === 'paused') return { label: '已暂停', tone: 'warning' }
  return { label: '未上线', tone: 'neutral' }
}

export type ApiServiceProbeStatus = {
  label: string
  description: string
  tone: 'success' | 'waiting' | 'warning' | 'risk' | 'neutral'
}

const probeAvailabilityPresentation: Record<Exclude<ApiHealthAvailabilityReason, null>, ApiServiceProbeStatus> = {
  unconfigured: { label: '未绑定', description: '尚未绑定探针连接', tone: 'neutral' },
  disabled: { label: '已停用', description: '探针连接已停用', tone: 'warning' },
  unverified: { label: '待验证', description: '探针连接尚未通过鉴权验证', tone: 'waiting' },
  insufficient: { label: '样本不足', description: '最近一小时样本不足', tone: 'waiting' },
  stale: { label: '样本过期', description: '最近样本已过期', tone: 'warning' },
  temporarily_unavailable: { label: '暂不可用', description: '探针连接暂时不可用', tone: 'risk' },
}

const probeStatePresentation: Record<ApiHealthState, ApiServiceProbeStatus> = {
  normal: { label: '正常', description: '探针连接鉴权正常', tone: 'success' },
  fluctuating: { label: '波动', description: '探针连接近期存在波动', tone: 'warning' },
  abnormal: { label: '异常', description: '探针连接近期异常', tone: 'risk' },
  no_sample: { label: '暂无数据', description: '探针连接暂无有效数据', tone: 'neutral' },
}

export function getApiServiceProbeStatus(
  summary: ApiServiceHealthSummary,
): ApiServiceProbeStatus {
  if (summary.availabilityReason) return probeAvailabilityPresentation[summary.availabilityReason]
  return probeStatePresentation[summary.state]
}

export type ApiServiceSalesStatus = {
  label: string
  tone: 'success' | 'waiting' | 'warning' | 'risk' | 'neutral'
}

const salesStatePresentation: Record<ApiServiceSalesState, ApiServiceSalesStatus> = {
  selling: { label: '销售中', tone: 'success' },
  upcoming: { label: '待开始', tone: 'waiting' },
  paused: { label: '已暂停', tone: 'warning' },
  sold_out: { label: '已售罄', tone: 'neutral' },
  expired: { label: '已过期', tone: 'risk' },
  draft: { label: '草稿', tone: 'neutral' },
  offline: { label: '未上线', tone: 'neutral' },
  archived: { label: '已归档', tone: 'neutral' },
}

const salesChannelLabels: Record<ApiServiceSalesChannelKind, string> = {
  flexible_quota: '自由额度',
  limited_quota: '限时额度包',
}

export const apiServiceSalesViewOptions: Array<{
  value: ApiServiceSalesView
  label: string
  description: string
}> = [
  {
    value: 'active',
    label: '有效销售',
    description: '包含当前销售中和已发布待开始的服务',
  },
  {
    value: 'expired',
    label: '已过期',
    description: '销售窗口或额度有效期已经结束',
  },
  {
    value: 'paused',
    label: '已暂停',
    description: '服务或销售计划当前暂停',
  },
  {
    value: 'draft',
    label: '草稿/离线',
    description: '尚未发布或当前没有上线',
  },
  {
    value: 'all',
    label: '全部',
    description: '查看所有当前和历史服务',
  },
]

export function getInitialApiServiceSalesView(intent: unknown): ApiServiceSalesView {
  return intent === 'quota' ? 'all' : 'active'
}

export function getApiServiceSalesStatus(state: ApiServiceSalesState): ApiServiceSalesStatus {
  return salesStatePresentation[state]
}

export function getApiServiceSalesChannelLabel(kind: ApiServiceSalesChannelKind) {
  return salesChannelLabels[kind]
}

export function getApiServiceSalesMethodLabels(summary: ApiServiceSalesSummary) {
  return summary.channels.map(channel => getApiServiceSalesChannelLabel(channel.kind))
}

export function getApiServiceSalesAvailabilitySummary(channel: ApiServiceSalesChannel) {
  if (channel.kind === 'flexible_quota' && channel.availableUsdAllowance !== undefined) {
    return `可售 $${formatDecimal(channel.availableUsdAllowance, 0, 6)}`
  }
  if (channel.kind === 'limited_quota' && channel.availableCopies !== undefined) {
    return `剩余 ${channel.availableCopies} 份`
  }
  return ''
}

function formatSalesTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

export function getApiServiceSalesTimeSummary(channel: ApiServiceSalesChannel) {
  if (channel.state === 'upcoming' && channel.nextStartsAt) {
    return `开售 ${formatSalesTime(channel.nextStartsAt)}`
  }
  if (channel.state === 'expired') {
    const endedAt = formatSalesTime(channel.saleCutoffAt ?? channel.expiresAt)
    return endedAt ? `已于 ${endedAt} 结束` : '销售已结束'
  }
  if (channel.kind === 'limited_quota') {
    const cutoff = formatSalesTime(channel.saleCutoffAt)
    const expires = formatSalesTime(channel.expiresAt)
    if (cutoff && expires) return `停售 ${cutoff} · 失效 ${expires}`
    if (cutoff) return `停售 ${cutoff}`
    if (expires) return `失效 ${expires}`
  }
  const expires = formatSalesTime(channel.expiresAt)
  return expires ? `有效至 ${expires}` : '长期服务'
}
