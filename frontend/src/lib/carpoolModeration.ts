import type { AdminRow, Carpool } from '@/lib/api'

const normalPublicCarpoolStatuses = new Set(['可上车', '已满', '已通过', '已验证', '已恢复'])

export type CarpoolModerationSource = Pick<
  Carpool,
  'id' | 'product' | 'region' | 'monthly' | 'status' | 'owner' | 'trustLevel' | 'linuxdoBound'
>

export function isCarpoolExceptionStatus(status: string) {
  return !normalPublicCarpoolStatuses.has(status.trim())
}

export function createCarpoolModerationRow(carpool: CarpoolModerationSource): AdminRow {
  return {
    id: carpool.id,
    primary: carpool.product,
    secondary: `${carpool.region} · ¥${carpool.monthly}/月 · ${carpool.status}`,
    owner: `${carpool.owner} · 信任等级${carpool.trustLevel}`,
    status: carpool.status,
    risk: carpool.linuxdoBound ? '原帖已绑定' : '缺少原帖',
    targetType: 'carpool',
    targetTo: `/carpools/${carpool.id}`,
  }
}
