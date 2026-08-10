import type { AdminRow, Carpool } from '@/lib/api'

const normalPublicCarpoolStatuses = new Set(['可上车', '已满', '已通过', '已验证', '已恢复'])
const adminActionCarpoolStatuses = new Set(['待处理', '审核中', '暂停'])

export type CarpoolModerationSource = Pick<
  Carpool,
  'id' | 'product' | 'region' | 'monthly' | 'status' | 'owner' | 'trustLevel' | 'hasInfoConflict' | 'hasUnresolvedDispute'
>

export function isCarpoolExceptionStatus(status: string) {
  return !normalPublicCarpoolStatuses.has(status.trim())
}

export function isCarpoolAdminActionStatus(status: string) {
  return adminActionCarpoolStatuses.has(status.trim())
}

export function createCarpoolModerationRow(carpool: CarpoolModerationSource): AdminRow {
  return {
    id: carpool.id,
    primary: carpool.product,
    secondary: `${carpool.region} · ¥${carpool.monthly}/月 · ${carpool.status}`,
    owner: `${carpool.owner} · ${carpool.trustLevel === null ? '信任等级暂无数据' : `信任等级${carpool.trustLevel}`}`,
    status: carpool.status,
    risk: carpool.hasInfoConflict
      ? '信息冲突'
      : carpool.hasUnresolvedDispute === true
        ? '存在未解决纠纷'
        : carpool.hasUnresolvedDispute === null
          ? '风险数据暂无'
          : '未发现公开风险',
    targetType: 'carpool',
    targetTo: `/carpools/${carpool.id}`,
  }
}
