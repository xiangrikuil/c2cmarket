import type { ApiService } from '@/lib/api'

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
