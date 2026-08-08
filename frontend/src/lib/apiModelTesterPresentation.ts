import type { ApiModelTesterProtocolResult } from '@/types/apiModelTester'

export type ApiModelTesterResultTone = 'success' | 'waiting' | 'risk'

const errorLabels: Record<Exclude<ApiModelTesterProtocolResult['errorCode'], ''>, string> = {
  authentication_failed: '鉴权失败',
  protocol_unsupported: '协议不支持',
  rate_limited: '额度或频率受限',
  request_rejected: '请求被拒绝',
  upstream_error: '上游暂时异常',
  timeout: '请求超时',
  blocked_target: '目标不可访问',
  dns_failed: 'DNS 解析失败',
  connect_failed: '连接失败',
  tls_failed: 'TLS 连接失败',
  response_too_large: '响应过大',
  invalid_response: '响应格式无效',
  internal: '测试暂时失败',
}

export function apiModelTesterProtocolPresentation(result: ApiModelTesterProtocolResult) {
  if (result.succeeded) {
    return { label: '实际调用通过', tone: 'success' as const }
  }
  const warning = result.errorCode === 'rate_limited' || result.errorCode === 'upstream_error' || result.errorCode === 'timeout'
  return {
    label: result.errorCode ? errorLabels[result.errorCode] : '调用失败',
    tone: warning ? 'waiting' as const : 'risk' as const,
  }
}
