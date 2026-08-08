import { shouldUseRealBackend } from '@/lib/backendClient'
import {
  backendAPIModelTesterOrderSources,
  backendDiscoverAPIModels,
  backendTestAPIModel,
} from '@/lib/apiModelTesterBackend'
import type {
  ApiModelTesterCredentialSource,
  ApiModelTesterDiscovery,
  ApiModelTesterModelResult,
  ApiModelTesterOrderSource,
  ApiModelTesterProtocolResult,
} from '@/types/apiModelTester'

const mockOrderSources: ApiModelTesterOrderSource[] = [{
  orderId: '00000000-0000-0000-0000-000000000901',
  orderNo: 'API-20260808-MDELTEST2X',
  serviceTitle: '开发环境 API 订单',
  baseUrl: 'https://api.example.com/v1',
  deliveredAt: '2026-08-08T08:00:00Z',
}]

function clone<T>(value: T): T {
  return structuredClone(value)
}

function mockModels(source: ApiModelTesterCredentialSource) {
  if (source.kind === 'manual' && source.apiKey.toLowerCase().includes('invalid')) {
    throw new Error('当前 API Key 无法获取模型列表。')
  }
  return ['gpt-4.1-mini', 'gpt-5-mini', 'claude-sonnet-4-5']
}

function mockProtocolResult(succeeded: boolean, errorCode: ApiModelTesterProtocolResult['errorCode'] = ''): ApiModelTesterProtocolResult {
  return {
    succeeded,
    httpStatusClass: succeeded ? 2 : 4,
    durationMs: succeeded ? 268 : 91,
    errorCode,
  }
}

export async function getAPIModelTesterOrderSources() {
  if (shouldUseRealBackend()) return backendAPIModelTesterOrderSources()
  return clone(mockOrderSources)
}

export async function discoverAPIModels(source: ApiModelTesterCredentialSource, signal?: AbortSignal): Promise<ApiModelTesterDiscovery> {
  if (shouldUseRealBackend()) return backendDiscoverAPIModels(source, signal)
  signal?.throwIfAborted()
  await Promise.resolve()
  signal?.throwIfAborted()
  return {
    baseUrl: source.kind === 'manual' ? source.baseUrl.trim() : mockOrderSources[0]?.baseUrl ?? '',
    models: mockModels(source),
    discoveredAt: new Date().toISOString(),
  }
}

export async function testAPIModel(source: ApiModelTesterCredentialSource, model: string, signal?: AbortSignal): Promise<ApiModelTesterModelResult> {
  if (shouldUseRealBackend()) return backendTestAPIModel(source, model, signal)
  signal?.throwIfAborted()
  await Promise.resolve()
  signal?.throwIfAborted()
  const responsesSupported = !model.startsWith('claude-')
  return {
    model,
    responsesResult: mockProtocolResult(responsesSupported, responsesSupported ? '' : 'protocol_unsupported'),
    chatCompletionsResult: mockProtocolResult(true),
    testedAt: new Date().toISOString(),
  }
}
