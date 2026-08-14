export type ApiModelTesterCredentialSource =
  | { kind: 'manual', baseUrl: string, apiKey: string, acknowledgeInsecureHttp: boolean }
  | { kind: 'order', orderId: string, acknowledgeInsecureHttp: boolean }

export type ApiModelTesterOrderSource = {
  orderId: string
  orderNo: string
  serviceTitle: string
  baseUrl: string
  deliveredAt: string
}

export type ApiModelTesterDiscovery = {
  baseUrl: string
  models: string[]
  discoveredAt: string
}

export type ApiModelTesterErrorCode =
  | ''
  | 'authentication_failed'
  | 'protocol_unsupported'
  | 'rate_limited'
  | 'request_rejected'
  | 'upstream_error'
  | 'timeout'
  | 'blocked_target'
  | 'dns_failed'
  | 'connect_failed'
  | 'tls_failed'
  | 'response_too_large'
  | 'invalid_response'
  | 'internal'

export type ApiModelTesterProtocolResult = {
  succeeded: boolean
  httpStatusClass: number
  durationMs: number
  errorCode: ApiModelTesterErrorCode
}

export type ApiModelTesterModelResult = {
  model: string
  responsesResult: ApiModelTesterProtocolResult
  chatCompletionsResult: ApiModelTesterProtocolResult
  testedAt: string
}

export type ApiModelTesterRowState = {
  state: 'pending' | 'completed' | 'cancelled' | 'failed'
  result?: ApiModelTesterModelResult
  message?: string
}
