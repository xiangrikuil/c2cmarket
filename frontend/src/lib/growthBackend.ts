import type { GrowthOverview } from '@/api/generated/openapi'
import { backendRequest, ensureBackendSession } from '@/lib/backendClient'

export type GrowthWindowDays = 7 | 30 | 90

export const growthWindowDays = [7, 30, 90] as const satisfies readonly GrowthWindowDays[]

export function normalizeGrowthWindowDays(value: unknown): GrowthWindowDays {
  const parsed = typeof value === 'number' ? value : Number(value)
  return growthWindowDays.includes(parsed as GrowthWindowDays) ? parsed as GrowthWindowDays : 30
}

export function validateGrowthOverview(value: GrowthOverview, requestedDays: GrowthWindowDays) {
  if (value.windowDays !== requestedDays) throw new Error('增长统计周期与请求不一致。')
  if (value.timezone !== 'Asia/Shanghai') throw new Error('增长统计时区不受支持。')
  if (!Array.isArray(value.registrationTrend) || !Array.isArray(value.activityTrend) || !Array.isArray(value.retentionCohorts)) {
    throw new Error('增长统计趋势数据不完整。')
  }
  return value
}

export async function backendAdminGrowthOverview(days: GrowthWindowDays) {
  await ensureBackendSession('admin', true)
  const overview = await backendRequest<GrowthOverview>(`/api/v1/admin/growth-overview?days=${days}`)
  return validateGrowthOverview(overview, days)
}
