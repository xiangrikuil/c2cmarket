import type { AdminRow, Carpool } from '@/lib/api'
import { sourceAuthorVerificationLabel } from '@/lib/sourceAuthorVerification'

const normalPublicCarpoolStatuses = new Set(['可上车', '已满', '已通过', '已验证', '已恢复'])

export type CarpoolModerationSource = Pick<
  Carpool,
  'id' | 'product' | 'region' | 'monthly' | 'status' | 'owner' | 'trustLevel' | 'sourceAuthorVerification'
>

export function isCarpoolExceptionStatus(status: string) {
  return !normalPublicCarpoolStatuses.has(status.trim())
}

export function createCarpoolModerationRow(carpool: CarpoolModerationSource): AdminRow {
  return {
    id: carpool.id,
    primary: carpool.product,
    secondary: `${carpool.region} · ¥${carpool.monthly}/月 · ${carpool.status}`,
    owner: `${carpool.owner} · ${carpool.trustLevel === null ? '信任等级暂无数据' : `信任等级${carpool.trustLevel}`}`,
    status: carpool.status,
    risk: sourceAuthorVerificationLabel(carpool.sourceAuthorVerification),
    targetType: 'carpool',
    targetTo: `/carpools/${carpool.id}`,
  }
}
