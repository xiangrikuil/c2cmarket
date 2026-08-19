import type { DisputeEvidenceAsset, DisputeEvidenceReference } from '@/api/generated/openapi'
import { BackendProblemError, backendBaseURL, backendMutation, ensureBackendSession, getBackendCSRFToken } from '@/lib/backendClient'

export type DisputeEvidenceKind = 'payment_result' | 'refund_result' | 'api_error' | 'quota_insufficient' | 'expired_early' | 'description_mismatch' | 'other_redacted_fact'

export type { DisputeEvidenceAsset, DisputeEvidenceReference }

export const disputeEvidenceKindLabels: Record<DisputeEvidenceKind, string> = {
  payment_result: '付款结果',
  refund_result: '退款结果',
  api_error: 'API 错误',
  quota_insufficient: '额度不足',
  expired_early: '提前失效',
  description_mismatch: '描述不符',
  other_redacted_fact: '其他脱敏事实',
}

export const disputeEvidenceUsageLabels: Record<DisputeEvidenceReference['usage'], string> = {
  dispute_initial: '发起纠纷',
  platform_escalation: '申请平台介入',
  formal_response: '正式答复',
  message: '沟通补充',
  info_supplement: '定向补件',
  remedy_claim: '履行声明',
  remedy_contest: '履行异议',
  appeal: '申诉材料',
}

export type AdminDisputeEvidenceQuarantineResult = {
  id: string
  status: 'quarantined'
  quarantinedExpiresAt: string
  version: number
}

export async function uploadDisputeEvidence(orderId: string, kind: DisputeEvidenceKind, files: File[], redactionConfirmed: boolean, onProgress?: (value: number) => void) {
  if (!redactionConfirmed) throw new Error('请先确认图片已遮挡敏感内容和二维码。')
  await ensureBackendSession('buyer', false)
  const form = new FormData()
  form.set('kind', kind)
  form.set('redactionConfirmed', 'true')
  files.forEach(file => form.append('files', file))
  return new Promise<DisputeEvidenceAsset[]>((resolve, reject) => {
    const request = new XMLHttpRequest()
    request.open('POST', `${backendBaseURL()}/api/v1/me/api-orders/${encodeURIComponent(orderId)}/dispute-evidence`)
    request.withCredentials = true
    request.setRequestHeader('Accept', 'application/json')
    const csrf = getBackendCSRFToken()
    if (csrf) request.setRequestHeader('X-CSRF-Token', csrf)
    request.upload.onprogress = event => {
      if (event.lengthComputable) onProgress?.(Math.round(event.loaded / event.total * 100))
    }
    request.onerror = () => reject(new Error('图片上传网络异常，请重试。'))
    request.onload = () => {
      let payload: { items?: DisputeEvidenceAsset[], detail?: string, title?: string, code?: string, errors?: Array<{ field?: string, code?: string, message?: string }> } = {}
      try {
        payload = request.responseText ? JSON.parse(request.responseText) : {}
      } catch {
        reject(new Error('图片上传响应无效。'))
        return
      }
      if (request.status < 200 || request.status >= 300) {
        reject(new BackendProblemError(payload, request.status))
        return
      }
      resolve(payload.items ?? [])
    }
    request.send(form)
  })
}

export async function quarantineDisputeEvidence(assetId: string, expectedVersion: number, reason: string) {
  await ensureBackendSession('admin', true)
  return backendMutation<AdminDisputeEvidenceQuarantineResult>(
    `/api/v1/admin/dispute-evidence/${encodeURIComponent(assetId)}/quarantine`,
    { reason: reason.trim() },
    {
      idempotencyPrefix: 'admin-dispute-evidence-quarantine',
      ifMatch: expectedVersion,
    },
  )
}

export function disputeEvidenceContentURL(path: string) {
  return `${backendBaseURL()}${path}`
}

export async function fetchDisputeEvidenceContent(path: string) {
  await ensureBackendSession('buyer', false)
  const response = await fetch(disputeEvidenceContentURL(path), {
    credentials: 'include',
    headers: { Accept: 'image/jpeg,image/png' },
  })
  if (!response.ok) {
    let payload: Record<string, unknown> = {}
    try {
      payload = await response.json()
    } catch {
      // 图片端点可能返回空错误体。
    }
    throw new BackendProblemError(payload, response.status)
  }
  return response.blob()
}
