import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import type {
  ApiModelTesterCredentialSource,
  ApiModelTesterDiscovery,
  ApiModelTesterModelResult,
  ApiModelTesterOrderSource,
} from '@/types/apiModelTester'

type OrderSourcesResponse = { items: ApiModelTesterOrderSource[] }

function sourceBody(source: ApiModelTesterCredentialSource) {
  if (source.kind === 'order') {
    return {
      kind: 'order',
      orderId: source.orderId,
      acknowledgeInsecureHttp: source.acknowledgeInsecureHttp,
    }
  }
  return {
    kind: 'manual',
    baseUrl: source.baseUrl.trim(),
    apiKey: source.apiKey.trim(),
    acknowledgeInsecureHttp: source.acknowledgeInsecureHttp,
  }
}

export async function backendAPIModelTesterOrderSources() {
  await ensureBackendSession()
  const response = await backendRequest<OrderSourcesResponse>('/api/v1/tools/api-model-tester/order-sources')
  return response.items
}

export async function backendDiscoverAPIModels(source: ApiModelTesterCredentialSource, signal?: AbortSignal) {
  await ensureBackendSession()
  return backendMutation<ApiModelTesterDiscovery>(
    '/api/v1/tools/api-model-tester/discover',
    { credentialSource: sourceBody(source) },
    { signal },
  )
}

export async function backendTestAPIModel(source: ApiModelTesterCredentialSource, model: string, signal?: AbortSignal) {
  await ensureBackendSession()
  return backendMutation<ApiModelTesterModelResult>(
    '/api/v1/tools/api-model-tester/test',
    { credentialSource: sourceBody(source), model },
    { signal },
  )
}
