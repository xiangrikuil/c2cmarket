import { modelCatalog, type ModelCatalogItem } from '@/data/mock'
import { backendJSON, backendMutation, backendRequest, ensureBackendSession, shouldUseRealBackend } from '@/lib/backendClient'
import {
  apiModelCapabilities,
  type AdminApiModel,
  type AdminApiModelProvider,
  type ApiModelCapability,
  type ApiModelBulkMutationResult,
  type CatalogLifecycleAction,
  type ApiModelInput,
  type ApiModelProviderCategory,
  type ApiModelProviderInput,
  type ApiModelSyncItem,
  type ApiModelSyncPreview,
  type ApiModelSyncSelection,
  type ModelsDevProviderCode,
} from '@/types/apiModelCatalog'

type ListResponse<T> = { items: T[] }

const apiModelProviderStorageKey = 'marketplace.admin.api-model-providers'
const apiModelAdminStorageKey = 'marketplace.admin.api-models'

const capabilityOrder = apiModelCapabilities.map(item => item.value)

export async function getAdminAPIModelProviders(): Promise<AdminApiModelProvider[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<AdminApiModelProvider>>('/api/v1/admin/api-model-providers')
    return response.items
  }
  return readMockAPIModelProviders()
}

export async function createAPIModelProvider(input: ApiModelProviderInput): Promise<AdminApiModelProvider> {
  const normalized = normalizeProviderInput(input)
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<AdminApiModelProvider>('/api/v1/admin/api-model-providers', normalized, {
      idempotencyPrefix: 'api-model-provider-create',
    })
  }
  const rows = readMockAPIModelProviders()
  if (rows.some(item => item.code === normalized.code)) throw new Error('提供商 code 已被占用。')
  const created = fromProviderInput(stableProviderId(normalized.code, rows), normalized)
  writeMockAPIModelProviders([...rows, created])
  return created
}

export async function updateAPIModelProvider(id: string, input: ApiModelProviderInput): Promise<AdminApiModelProvider> {
  const normalized = normalizeProviderInput(input)
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<AdminApiModelProvider>(`/api/v1/admin/api-model-providers/${encodeURIComponent(id)}`, normalized, {
      method: 'PATCH',
    })
  }
  const rows = readMockAPIModelProviders()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('API 提供商不存在。')
  if (rows.some(item => item.id !== id && item.code === normalized.code)) throw new Error('提供商 code 已被占用。')
  const updated = fromProviderInput(id, normalized, previous)
  writeMockAPIModelProviders(rows.map(item => item.id === id ? updated : item))
  writeMockAdminAPIModels(readMockAdminAPIModels().map(item => item.providerId === id ? withProvider(item, updated) : item))
  return updated
}

export async function applyAPIModelProviderLifecycle(id: string, version: number, action: CatalogLifecycleAction, reason: string, targetStatus?: 'active' | 'deprecated'): Promise<AdminApiModelProvider> {
  return applyCatalogLifecycle('api-model-providers', id, version, action, reason, targetStatus) as Promise<AdminApiModelProvider>
}

export async function getAdminAPIModels(): Promise<AdminApiModel[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<AdminApiModel>>('/api/v1/admin/api-models')
    return response.items
  }
  return readMockAdminAPIModels()
}

export async function createAPIModel(input: ApiModelInput): Promise<AdminApiModel> {
  const normalized = normalizeModelInput(input)
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<AdminApiModel>('/api/v1/admin/api-models', normalized, {
      idempotencyPrefix: 'api-model-create',
    })
  }
  const rows = readMockAdminAPIModels()
  const provider = activeProviderOrThrow(normalized.providerId)
  if (rows.some(item => item.modelKey === normalized.modelKey)) throw new Error('模型标识已被占用。')
  const created = withProvider(fromModelInput(stableModelId(normalized.modelKey, rows), normalized), provider)
  writeMockAdminAPIModels([...rows, created])
  return created
}

export async function updateAPIModel(id: string, input: ApiModelInput): Promise<AdminApiModel> {
  const normalized = normalizeModelInput(input)
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<AdminApiModel>(`/api/v1/admin/api-models/${encodeURIComponent(id)}`, normalized, {
      method: 'PATCH',
    })
  }
  const rows = readMockAdminAPIModels()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('API 模型不存在。')
  const provider = activeProviderOrThrow(normalized.providerId)
  if (rows.some(item => item.id !== id && item.modelKey === normalized.modelKey)) throw new Error('模型标识已被占用。')
  const updated = withProvider(fromModelInput(id, normalized, previous), provider)
  writeMockAdminAPIModels(rows.map(item => item.id === id ? updated : item))
  return updated
}

export async function applyAPIModelLifecycle(id: string, version: number, action: CatalogLifecycleAction, reason: string, targetStatus?: 'active' | 'deprecated'): Promise<AdminApiModel> {
  return applyCatalogLifecycle('api-models', id, version, action, reason, targetStatus) as Promise<AdminApiModel>
}

export async function previewAPIModelsDevSync(providerIds: string[]): Promise<ApiModelSyncPreview> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendJSON<ApiModelSyncPreview>('/api/v1/admin/api-models/models-dev/preview', { providerIds })
  }
  return buildMockModelsDevPreview(providerIds)
}

export async function applyAPIModelsDevSync(items: ApiModelSyncSelection[]): Promise<ApiModelBulkMutationResult> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ApiModelBulkMutationResult>('/api/v1/admin/api-models/models-dev/apply', { items }, {
      idempotencyPrefix: 'api-model-models-dev-apply',
    })
  }
  return applyMockModelsDevSync(items)
}

async function applyCatalogLifecycle(resource: 'api-model-providers' | 'api-models', id: string, version: number, action: CatalogLifecycleAction, reason: string, targetStatus?: 'active' | 'deprecated') {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<AdminApiModelProvider | AdminApiModel>(`/api/v1/admin/${resource}/${encodeURIComponent(id)}/${action}`, {
      reason: reason.trim(),
      ...(action === 'unblock' ? { targetStatus } : {}),
    }, {
      idempotencyPrefix: `catalog-${resource}-${action}`,
      ifMatch: version,
    })
  }
  if (resource === 'api-model-providers') return applyMockProviderLifecycle(id, version, action, reason, targetStatus)
  return applyMockModelLifecycle(id, version, action, reason, targetStatus)
}

export function getMockPublicAPIModels(): ModelCatalogItem[] {
  return readMockAdminAPIModels()
    .filter(item => item.active && item.providerActive)
    .map(toPublicModel)
}

function readMockAPIModelProviders(): AdminApiModelProvider[] {
  if (typeof window === 'undefined') return seedAPIModelProviders()
  try {
    const raw = window.sessionStorage.getItem(apiModelProviderStorageKey)
    if (!raw) return seedAPIModelProviders()
    const stored = JSON.parse(raw) as AdminApiModelProvider[]
    const storedIds = new Set(stored.map(item => item.id))
    return sortAPIModelProviders([
      ...stored,
      ...seedAPIModelProviders().filter(item => !storedIds.has(item.id)),
    ])
  } catch {
    return seedAPIModelProviders()
  }
}

function writeMockAPIModelProviders(items: AdminApiModelProvider[]) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(apiModelProviderStorageKey, JSON.stringify(sortAPIModelProviders(items)))
}

function readMockAdminAPIModels(): AdminApiModel[] {
  const providers = readMockAPIModelProviders()
  if (typeof window === 'undefined') return seedAdminAPIModels(providers)
  try {
    const raw = window.sessionStorage.getItem(apiModelAdminStorageKey)
    if (!raw) return seedAdminAPIModels(providers)
    const stored = JSON.parse(raw) as AdminApiModel[]
    const storedIds = new Set(stored.map(item => item.id))
    return sortAdminAPIModels([
      ...stored.map(item => withProvider(item, providerById(item.providerId, providers))),
      ...seedAdminAPIModels(providers).filter(item => !storedIds.has(item.id)),
    ])
  } catch {
    return seedAdminAPIModels(providers)
  }
}

function writeMockAdminAPIModels(items: AdminApiModel[]) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(apiModelAdminStorageKey, JSON.stringify(sortAdminAPIModels(items)))
}

function seedAPIModelProviders(): AdminApiModelProvider[] {
  const now = '2026-08-14T00:00:00.000Z'
  return sortAPIModelProviders([
    { id: '00000000-0000-0000-0000-000000000c01', providerCategory: 'gpt', code: 'openai', displayName: 'OpenAI', coreKey: 'gpt', ...mockLifecycle(now, true), sortOrder: 10, createdAt: now, updatedAt: now },
    { id: '00000000-0000-0000-0000-000000000c02', providerCategory: 'claude', code: 'anthropic', displayName: 'Anthropic', coreKey: 'claude', ...mockLifecycle(now, true), sortOrder: 20, createdAt: now, updatedAt: now },
    { id: '00000000-0000-0000-0000-000000000c06', providerCategory: 'grok', code: 'xai', displayName: 'xAI', coreKey: 'grok', ...mockLifecycle(now, true), sortOrder: 30, createdAt: now, updatedAt: now },
  ])
}

function seedAdminAPIModels(providers: AdminApiModelProvider[]): AdminApiModel[] {
  const now = '2026-08-14T00:00:00.000Z'
  const seeds = [
    { id: '00000000-0000-0000-0000-000000000a01', providerCode: 'openai', coreKey: 'gpt' as const, modelKey: 'gpt-4.1', sortOrder: 10, inputPricePerMillion: '2.000000', cachedInputPricePerMillion: '0.500000', outputPricePerMillion: '8.000000' },
    { id: '00000000-0000-0000-0000-000000000a02', providerCode: 'openai', coreKey: undefined, modelKey: 'gpt-4.1-mini', sortOrder: 20, inputPricePerMillion: '0.400000', cachedInputPricePerMillion: '0.100000', outputPricePerMillion: '1.600000' },
    { id: '00000000-0000-0000-0000-000000000a31', providerCode: 'xai', coreKey: 'grok' as const, modelKey: 'grok-4', sortOrder: 310 },
  ]
  return sortAdminAPIModels(seeds.map((seed) => {
    const provider = providerByCode(seed.providerCode, providers)
    return withProvider({
      id: seed.id,
      providerId: provider.id,
      providerCategory: provider.providerCategory,
      providerCode: provider.code,
      provider: provider.displayName,
      providerActive: true,
      coreKey: seed.coreKey,
      ...mockLifecycle(now, true),
      modelKey: seed.modelKey,
      capabilities: ['text'],
      currentPriceSourceVersion: 'mock-seed-2026-08-14',
      inputPricePerMillion: seed.inputPricePerMillion,
      cachedInputPricePerMillion: seed.cachedInputPricePerMillion,
      outputPricePerMillion: seed.outputPricePerMillion,
      sortOrder: seed.sortOrder,
      createdAt: now,
      updatedAt: now,
    }, provider)
  }))
}

type MockModelsDevModel = {
  providerCode: ModelsDevProviderCode
  modelKey: string
  capabilities: Array<'text' | 'vision' | 'reasoning'>
  inputPricePerMillion: string
  cachedInputPricePerMillion: string
  outputPricePerMillion: string
  sourceVersion: string
  unavailableReason?: string
}

const mockModelsDevModels: MockModelsDevModel[] = [
  { providerCode: 'openai', modelKey: 'gpt-5-mini', capabilities: ['text', 'vision', 'reasoning'], inputPricePerMillion: '0.250000', cachedInputPricePerMillion: '0.025000', outputPricePerMillion: '2.100000', sourceVersion: 'models.dev:2026-08-08' },
  { providerCode: 'openai', modelKey: 'gpt-4.1-mini', capabilities: ['text', 'vision'], inputPricePerMillion: '0.400000', cachedInputPricePerMillion: '0.100000', outputPricePerMillion: '1.600000', sourceVersion: 'models.dev:2026-08-08' },
  { providerCode: 'openai', modelKey: 'gpt-audio-preview', capabilities: ['text'], inputPricePerMillion: '', cachedInputPricePerMillion: '', outputPricePerMillion: '', sourceVersion: '', unavailableReason: '当前目录不支持音频模型计价。' },
  { providerCode: 'anthropic', modelKey: 'claude-sonnet', capabilities: ['text', 'vision'], inputPricePerMillion: '3.000000', cachedInputPricePerMillion: '', outputPricePerMillion: '15.000000', sourceVersion: 'models.dev:2026-08-08' },
  { providerCode: 'anthropic', modelKey: 'claude-3-5-haiku', capabilities: ['text', 'vision'], inputPricePerMillion: '0.800000', cachedInputPricePerMillion: '0.080000', outputPricePerMillion: '4.000000', sourceVersion: 'models.dev:2026-08-08' },
  { providerCode: 'xai', modelKey: 'grok-4.5', capabilities: ['text', 'vision', 'reasoning'], inputPricePerMillion: '2.000000', cachedInputPricePerMillion: '0.300000', outputPricePerMillion: '6.000000', sourceVersion: 'models.dev:2026-08-14' },
]

function buildMockModelsDevPreview(providerIds: string[]): ApiModelSyncPreview {
  const providers = readMockAPIModelProviders()
  const uniqueProviderIds = Array.from(new Set(providerIds))
  const selectedProviders = uniqueProviderIds.map(id => providers.find(item => item.id === id))
  if (selectedProviders.length === 0 || selectedProviders.some(provider => !provider || !isModelsDevProviderCode(provider.code))) {
    throw new Error('请至少选择一个支持 models.dev 的官方提供商。')
  }
  const localModels = readMockAdminAPIModels()
  const items: ApiModelSyncItem[] = []
  for (const provider of selectedProviders as AdminApiModelProvider[]) {
    const externalModels = mockModelsDevModels.filter(item => item.providerCode === provider.code)
    const externalKeys = new Set(externalModels.map(item => item.modelKey))
    for (const external of externalModels) {
      const local = localModels.find(item => item.modelKey === external.modelKey)
      if (external.unavailableReason) {
        items.push(mockSyncItem(provider, external, 'unavailable', undefined, 'UNSUPPORTED_MODEL_TYPE', external.unavailableReason))
        continue
      }
      if (local && local.providerId !== provider.id) {
        items.push(mockSyncItem(provider, external, 'unavailable', undefined, 'MODEL_KEY_PROVIDER_CONFLICT', '该模型标识已属于其他提供商。'))
        continue
      }
      const status = !local ? 'new' : mockPriceChanged(local, external) ? 'price_changed' : 'unchanged'
      items.push(mockSyncItem(provider, external, status, local))
    }
    for (const local of localModels.filter(item => item.providerId === provider.id && !externalKeys.has(item.modelKey))) {
      items.push({
        candidateKey: `${provider.code}:${local.modelKey}`,
        fingerprint: '', status: 'source_missing', reasonCode: 'SOURCE_MISSING',
        reason: 'models.dev 本次未返回该模型，本地数据保持不变。',
        providerId: provider.id, providerCode: provider.code, provider: provider.displayName,
        modelKey: local.modelKey, capabilities: local.capabilities,
        localModelId: local.id, localPriceVersionId: local.currentPriceVersionId,
        localInputPricePerMillion: local.inputPricePerMillion,
        localCachedInputPricePerMillion: local.cachedInputPricePerMillion,
        localOutputPricePerMillion: local.outputPricePerMillion,
        localSourceUrl: local.currentPriceSourceUrl, localSourceVersion: local.currentPriceSourceVersion,
      })
    }
  }
  const statusOrder = ['new', 'price_changed', 'unchanged', 'source_missing', 'unavailable']
  items.sort((left, right) => statusOrder.indexOf(left.status) - statusOrder.indexOf(right.status) || left.candidateKey.localeCompare(right.candidateKey))
  return {
    fingerprint: mockFingerprint(`preview:${uniqueProviderIds.join(',')}:${items.map(item => item.fingerprint).join(',')}`),
    fetchedAt: new Date().toISOString(),
    counts: {
      new: items.filter(item => item.status === 'new').length,
      priceChanged: items.filter(item => item.status === 'price_changed').length,
      unchanged: items.filter(item => item.status === 'unchanged').length,
      sourceMissing: items.filter(item => item.status === 'source_missing').length,
      unavailable: items.filter(item => item.status === 'unavailable').length,
    },
    items,
  }
}

function mockSyncItem(provider: AdminApiModelProvider, external: MockModelsDevModel, status: ApiModelSyncItem['status'], local?: AdminApiModel, reasonCode?: string, reason?: string): ApiModelSyncItem {
  const candidateKey = `${provider.code}:${external.modelKey}`
  const item: ApiModelSyncItem = {
    candidateKey, fingerprint: '', status, reasonCode, reason,
    providerId: provider.id, providerCode: provider.code, provider: provider.displayName,
    modelKey: external.modelKey, capabilities: external.capabilities,
    sourceUrl: status === 'unavailable' ? undefined : 'https://models.dev/api.json',
    sourceVersion: external.sourceVersion || undefined,
    inputPricePerMillion: external.inputPricePerMillion || undefined,
    cachedInputPricePerMillion: external.cachedInputPricePerMillion || undefined,
    outputPricePerMillion: external.outputPricePerMillion || undefined,
    localModelId: local?.id, localPriceVersionId: local?.currentPriceVersionId,
    localInputPricePerMillion: local?.inputPricePerMillion,
    localCachedInputPricePerMillion: local?.cachedInputPricePerMillion,
    localOutputPricePerMillion: local?.outputPricePerMillion,
    localSourceUrl: local?.currentPriceSourceUrl, localSourceVersion: local?.currentPriceSourceVersion,
  }
  if (status === 'new' || status === 'price_changed') item.fingerprint = mockSyncSelectionFingerprint(item)
  return item
}

function applyMockModelsDevSync(items: ApiModelSyncSelection[]): ApiModelBulkMutationResult {
  if (items.length === 0) throw new Error('请至少选择一个模型变化。')
  const rows = readMockAdminAPIModels()
  const providers = readMockAPIModelProviders()
  const seen = new Set<string>()
  for (const item of items) {
    const provider = providers.find(candidate => candidate.id === item.providerId)
    const key = `${item.providerCode}:${item.modelKey}`
    if (!provider || provider.code !== item.providerCode || seen.has(key)) throw new Error('同步候选项已变化，请重新获取预览。')
    const currentCandidate = buildMockModelsDevPreview([provider.id]).items.find(candidate => candidate.candidateKey === key)
    if (!currentCandidate
      || currentCandidate.fingerprint !== item.fingerprint
      || currentCandidate.status !== item.status
      || mockSyncSelectionFingerprint(item) !== item.fingerprint) {
      throw new Error('同步候选项已变化，请重新获取预览。')
    }
    seen.add(key)
    const local = rows.find(candidate => candidate.id === item.localModelId)
    if (item.status === 'new' && rows.some(candidate => candidate.modelKey === item.modelKey)) throw new Error('模型目录已变化，请重新获取预览。')
    if (item.status === 'price_changed' && (!local || local.providerId !== item.providerId || local.modelKey !== item.modelKey || (local.currentPriceVersionId ?? '') !== item.localPriceVersionId)) {
      throw new Error('模型价格已变化，请重新获取预览。')
    }
  }
  const now = new Date().toISOString()
  let nextSortOrder = Math.max(0, ...rows.map(item => item.sortOrder)) + 10
  let nextRows = [...rows]
  const ids: string[] = []
  let created = 0
  let updated = 0
  for (const item of items) {
    const provider = providers.find(candidate => candidate.id === item.providerId)!
    if (item.status === 'new') {
      const id = stableModelId(item.modelKey, nextRows)
      const createdModel = withProvider({
        id, providerId: provider.id, providerCategory: provider.providerCategory,
        providerCode: provider.code, provider: provider.displayName, providerActive: provider.active,
        ...mockLifecycle(now, false, item.active ? 'active' : 'deprecated'),
        modelKey: item.modelKey, capabilities: item.capabilities,
        currentPriceVersionId: `mock-price-${id}-${Date.now()}`,
        currentPriceSourceUrl: item.sourceUrl, currentPriceSourceVersion: item.sourceVersion,
        currentPriceValidFrom: now, inputPricePerMillion: item.inputPricePerMillion,
        cachedInputPricePerMillion: item.cachedInputPricePerMillion,
        outputPricePerMillion: item.outputPricePerMillion, sortOrder: nextSortOrder,
        createdAt: now, updatedAt: now,
      }, provider)
      nextRows.push(createdModel)
      nextSortOrder += 10
      ids.push(id)
      created += 1
      continue
    }
    nextRows = nextRows.map(local => local.id === item.localModelId ? {
      ...local,
      currentPriceVersionId: `mock-price-${local.id}-${Date.now()}`,
      currentPriceSourceUrl: item.sourceUrl,
      currentPriceSourceVersion: item.sourceVersion,
      currentPriceValidFrom: now,
      inputPricePerMillion: item.inputPricePerMillion,
      cachedInputPricePerMillion: item.cachedInputPricePerMillion,
      outputPricePerMillion: item.outputPricePerMillion,
      updatedAt: now,
    } : local)
    ids.push(item.localModelId)
    updated += 1
  }
  writeMockAdminAPIModels(nextRows)
  return { created, updated, changed: 0, ids }
}

function mockPriceChanged(local: AdminApiModel, external: MockModelsDevModel) {
  return (local.currentPriceSourceUrl ?? '') !== 'https://models.dev/api.json'
    || (local.currentPriceSourceVersion ?? '') !== external.sourceVersion
    || (local.inputPricePerMillion ?? '') !== external.inputPricePerMillion
    || (local.cachedInputPricePerMillion ?? '') !== external.cachedInputPricePerMillion
    || (local.outputPricePerMillion ?? '') !== external.outputPricePerMillion
}

function isModelsDevProviderCode(value: string): value is ModelsDevProviderCode {
  return value === 'openai' || value === 'anthropic' || value === 'xai'
}

function mockFingerprint(value: string) {
  let hash = 2166136261
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return Math.abs(hash >>> 0).toString(16).padStart(8, '0').repeat(8).slice(0, 64)
}

function mockSyncSelectionFingerprint(item: Pick<ApiModelSyncSelection, 'status' | 'providerId' | 'providerCode' | 'modelKey' | 'capabilities' | 'sourceUrl' | 'sourceVersion' | 'inputPricePerMillion' | 'cachedInputPricePerMillion' | 'outputPricePerMillion' | 'localModelId' | 'localPriceVersionId'> | ApiModelSyncItem) {
  return mockFingerprint(JSON.stringify({
    status: item.status,
    providerId: item.providerId,
    providerCode: item.providerCode,
    modelKey: item.modelKey,
    capabilities: item.capabilities,
    sourceUrl: item.sourceUrl ?? '',
    sourceVersion: item.sourceVersion ?? '',
    inputPricePerMillion: item.inputPricePerMillion ?? '',
    cachedInputPricePerMillion: item.cachedInputPricePerMillion ?? '',
    outputPricePerMillion: item.outputPricePerMillion ?? '',
    localModelId: item.localModelId ?? '',
    localPriceVersionId: item.localPriceVersionId ?? '',
  }))
}

function normalizeProviderInput(input: ApiModelProviderInput): ApiModelProviderInput {
  return {
    providerCategory: input.providerCategory.trim().toLowerCase(),
    code: input.code.trim().toLowerCase(),
    displayName: input.displayName.trim(),
    sortOrder: input.sortOrder,
  }
}

function normalizeModelInput(input: ApiModelInput): ApiModelInput {
  return {
    providerId: input.providerId.trim(),
    modelKey: input.modelKey.trim(),
    capabilities: normalizeCapabilities(input.capabilities),
    inputTokenPrice: normalizePriceInput(input.inputTokenPrice),
    cachedInputTokenPrice: normalizePriceInput(input.cachedInputTokenPrice),
    outputTokenPrice: normalizePriceInput(input.outputTokenPrice),
    sourceUrl: input.sourceUrl.trim(),
    sourceVersion: input.sourceVersion.trim(),
    sortOrder: input.sortOrder,
  }
}

function fromProviderInput(id: string, input: ApiModelProviderInput, previous?: AdminApiModelProvider): AdminApiModelProvider {
  const now = new Date().toISOString()
  return {
    ...(previous ? lifecycleFields(previous) : mockLifecycle(now, false)),
    id,
    providerCategory: input.providerCategory,
    code: input.code,
    displayName: input.displayName,
    sortOrder: input.sortOrder,
    createdAt: previous?.createdAt ?? now,
    updatedAt: now,
  }
}

function fromModelInput(id: string, input: ApiModelInput, previous?: AdminApiModel): AdminApiModel {
  const now = new Date().toISOString()
  const priceChanged = previous
    ? (previous.currentPriceSourceUrl ?? '') !== input.sourceUrl
      || (previous.currentPriceSourceVersion ?? '') !== input.sourceVersion
      || (previous.inputPricePerMillion ?? '') !== input.inputTokenPrice
      || (previous.cachedInputPricePerMillion ?? '') !== input.cachedInputTokenPrice
      || (previous.outputPricePerMillion ?? '') !== input.outputTokenPrice
    : priceInputPresent(input)
  const priceVersionId = priceChanged
    ? `mock-price-${id}-${Date.now()}`
    : previous?.currentPriceVersionId
  return {
    ...(previous ? lifecycleFields(previous) : mockLifecycle(now, priceVersionId !== undefined)),
    id,
    providerId: input.providerId,
    providerCategory: previous?.providerCategory ?? 'other',
    providerCode: previous?.providerCode ?? '',
    provider: previous?.provider ?? '',
    providerActive: previous?.providerActive ?? true,
    modelKey: input.modelKey,
    capabilities: input.capabilities,
    currentPriceVersionId: priceVersionId,
    currentPriceSourceUrl: input.sourceUrl,
    currentPriceSourceVersion: input.sourceVersion,
    currentPriceValidFrom: priceVersionId ? now : undefined,
    inputPricePerMillion: input.inputTokenPrice,
    cachedInputPricePerMillion: input.cachedInputTokenPrice,
    outputPricePerMillion: input.outputTokenPrice,
    sortOrder: input.sortOrder,
    createdAt: previous?.createdAt ?? now,
    updatedAt: now,
  }
}

function mockLifecycle(now: string, identityLocked: boolean, status: 'active' | 'deprecated' | 'blocked' = 'active') {
  return {
    status,
    effectiveStatus: status,
    effectiveStatusSource: 'self' as const,
    statusChangedAt: now,
    statusReason: '',
    version: 1,
    identityLocked,
    identityLockReason: identityLocked ? '目录身份由系统管理或已被业务引用，不能修改。' : '',
    active: status === 'active',
  }
}

function lifecycleFields(item: AdminApiModelProvider | AdminApiModel) {
  return {
    coreKey: item.coreKey,
    status: item.status,
    effectiveStatus: item.effectiveStatus,
    effectiveStatusSource: item.effectiveStatusSource,
    statusChangedAt: item.statusChangedAt,
    statusChangedBy: item.statusChangedBy,
    statusReason: item.statusReason,
    version: item.version,
    identityLocked: item.identityLocked,
    identityLockReason: item.identityLockReason,
    active: item.active,
  }
}

function lifecycleTarget(action: CatalogLifecycleAction, targetStatus?: 'active' | 'deprecated') {
  if (action === 'deprecate') return 'deprecated' as const
  if (action === 'block') return 'blocked' as const
  if (action === 'reactivate') return 'active' as const
  if (action === 'unblock' && targetStatus) return targetStatus
  throw new Error('目录状态动作无效。')
}

function assertLifecycleTransition(status: AdminApiModelProvider['status'], action: CatalogLifecycleAction) {
  const allowed = (action === 'deprecate' && status === 'active')
    || (action === 'block' && (status === 'active' || status === 'deprecated'))
    || (action === 'reactivate' && status === 'deprecated')
    || (action === 'unblock' && status === 'blocked')
  if (!allowed) throw new Error('当前目录状态不允许执行该动作。')
}

function applyMockProviderLifecycle(id: string, version: number, action: CatalogLifecycleAction, reason: string, targetStatus?: 'active' | 'deprecated') {
  const rows = readMockAPIModelProviders()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('API 提供商不存在。')
  if (previous.version !== version) throw new Error('目录版本已变化，请刷新后重试。')
  assertLifecycleTransition(previous.status, action)
  const status = lifecycleTarget(action, targetStatus)
  const now = new Date().toISOString()
  const updated: AdminApiModelProvider = {
    ...previous,
    status,
    effectiveStatus: status,
    effectiveStatusSource: 'self',
    statusChangedAt: now,
    statusReason: reason.trim(),
    version: previous.version + 1,
    active: status === 'active',
    updatedAt: now,
  }
  writeMockAPIModelProviders(rows.map(item => item.id === id ? updated : item))
  writeMockAdminAPIModels(readMockAdminAPIModels().map(item => item.providerId === id ? withProvider(item, updated) : item))
  return updated
}

function applyMockModelLifecycle(id: string, version: number, action: CatalogLifecycleAction, reason: string, targetStatus?: 'active' | 'deprecated') {
  const rows = readMockAdminAPIModels()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('API 模型不存在。')
  if (previous.version !== version) throw new Error('目录版本已变化，请刷新后重试。')
  assertLifecycleTransition(previous.status, action)
  const status = lifecycleTarget(action, targetStatus)
  const provider = providerById(previous.providerId, readMockAPIModelProviders())
  if (status === 'active' && provider.effectiveStatus !== 'active') throw new Error('父级提供商当前不可用，不能恢复该模型。')
  const now = new Date().toISOString()
  const updated = withProvider({
    ...previous,
    status,
    effectiveStatus: status,
    effectiveStatusSource: 'self',
    statusChangedAt: now,
    statusReason: reason.trim(),
    version: previous.version + 1,
    active: status === 'active',
    updatedAt: now,
  }, provider)
  writeMockAdminAPIModels(rows.map(item => item.id === id ? updated : item))
  return updated
}

function withProvider(item: AdminApiModel, provider: AdminApiModelProvider): AdminApiModel {
  const effectiveStatus = item.status === 'blocked' || provider.status === 'blocked'
    ? 'blocked'
    : item.status === 'deprecated' || provider.status === 'deprecated'
      ? 'deprecated'
      : 'active'
  return {
    ...item,
    providerId: provider.id,
    providerCategory: provider.providerCategory,
    providerCode: provider.code,
    provider: provider.displayName,
    providerActive: provider.effectiveStatus === 'active',
    effectiveStatus,
    effectiveStatusSource: effectiveStatus !== item.status ? 'parent' : 'self',
    active: effectiveStatus === 'active',
  }
}

function toPublicModel(item: AdminApiModel): ModelCatalogItem {
  return {
    id: item.id,
    provider: publicProvider(item),
    name: item.modelKey,
    capabilities: item.capabilities.filter(isPublicCapability),
    officialInputPricePerMillion: priceToNumber(item.inputPricePerMillion),
    officialCachedInputPricePerMillion: priceToNumber(item.cachedInputPricePerMillion),
    officialOutputPricePerMillion: priceToNumber(item.outputPricePerMillion),
    active: item.active,
    sortOrder: item.sortOrder,
    createdAt: item.createdAt,
    updatedAt: item.updatedAt,
  }
}

function publicProvider(item: AdminApiModel): ModelCatalogItem['provider'] {
  return item.providerCode
}

function activeProviderOrThrow(providerId: string) {
  const provider = providerById(providerId, readMockAPIModelProviders())
  if (!provider.active) throw new Error('API 提供商已停用。')
  return provider
}

function providerById(providerId: string, providers: AdminApiModelProvider[]) {
  return providers.find(item => item.id === providerId) ?? providers[0]
}

function providerByCode(code: string, providers: AdminApiModelProvider[]) {
  return providers.find(item => item.code === code) ?? providers[0]
}

function normalizeCapabilities(values: string[]): ApiModelCapability[] {
  const seen = new Set(values.map(value => value.trim()).filter((value): value is ApiModelCapability => capabilityOrder.includes(value as ApiModelCapability)))
  return capabilityOrder.filter(value => seen.has(value))
}

function isPublicCapability(value: ApiModelCapability): value is ModelCatalogItem['capabilities'][number] {
  return value !== 'text'
}

function normalizePriceInput(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return ''
  const numeric = Number(trimmed)
  if (!Number.isFinite(numeric) || numeric < 0) return trimmed
  return numeric.toFixed(6)
}

function priceToString(value: number | null | undefined) {
  return value == null ? '' : value.toFixed(6)
}

function priceToNumber(value: string | undefined) {
  if (!value) return null
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : null
}

function priceInputPresent(input: ApiModelInput) {
  return Boolean(input.sourceUrl || input.sourceVersion || input.inputTokenPrice || input.cachedInputTokenPrice || input.outputTokenPrice)
}

function stableProviderId(code: string, rows: AdminApiModelProvider[]) {
  let id = `mock-api-provider-${code || 'provider'}`
  let suffix = 2
  while (rows.some(item => item.id === id)) {
    id = `mock-api-provider-${code || 'provider'}-${suffix}`
    suffix += 1
  }
  return id
}

function stableModelId(modelKey: string, rows: AdminApiModel[]) {
  const base = modelKey.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'api-model'
  let id = `mock-api-model-${base}`
  let suffix = 2
  while (rows.some(item => item.id === id)) {
    id = `mock-api-model-${base}-${suffix}`
    suffix += 1
  }
  return id
}

function sortAPIModelProviders(items: AdminApiModelProvider[]) {
  return [...items].sort((left, right) => {
    if (left.sortOrder !== right.sortOrder) return left.sortOrder - right.sortOrder
    return left.displayName.localeCompare(right.displayName)
  })
}

function sortAdminAPIModels(items: AdminApiModel[]) {
  return [...items].sort((left, right) => {
    if (left.sortOrder !== right.sortOrder) return left.sortOrder - right.sortOrder
    return left.modelKey.localeCompare(right.modelKey)
  })
}
