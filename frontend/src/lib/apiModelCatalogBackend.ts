import { modelCatalog, type ModelCatalogItem } from '@/data/mock'
import { backendMutation, backendRequest, ensureBackendSession, shouldUseRealBackend } from '@/lib/backendClient'
import {
  apiModelCapabilities,
  type AdminApiModel,
  type AdminApiModelProvider,
  type ApiModelCapability,
  type ApiModelInput,
  type ApiModelProviderCategory,
  type ApiModelProviderInput,
} from '@/types/apiModelCatalog'

type ListResponse<T> = { items: T[] }

const apiModelProviderStorageKey = 'marketplace.admin.api-model-providers'
const apiModelAdminStorageKey = 'marketplace.admin.api-models'

const providerOrder: ApiModelProviderCategory[] = ['gpt', 'claude', 'cursor', 'gemini', 'perplexity', 'other']
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

export async function setAPIModelProviderActive(id: string, active: boolean): Promise<AdminApiModelProvider> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<AdminApiModelProvider>(`/api/v1/admin/api-model-providers/${encodeURIComponent(id)}/${active ? 'activate' : 'deactivate'}`, {})
  }
  const rows = readMockAPIModelProviders()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('API 提供商不存在。')
  const updated = { ...previous, active, updatedAt: new Date().toISOString() }
  writeMockAPIModelProviders(rows.map(item => item.id === id ? updated : item))
  writeMockAdminAPIModels(readMockAdminAPIModels().map(item => item.providerId === id ? withProvider(item, updated) : item))
  return updated
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

export async function setAPIModelActive(id: string, active: boolean): Promise<AdminApiModel> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<AdminApiModel>(`/api/v1/admin/api-models/${encodeURIComponent(id)}/${active ? 'activate' : 'deactivate'}`, {})
  }
  const rows = readMockAdminAPIModels()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('API 模型不存在。')
  const updated = { ...previous, active, updatedAt: new Date().toISOString() }
  writeMockAdminAPIModels(rows.map(item => item.id === id ? updated : item))
  return updated
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
  const now = '2026-06-29T00:00:00.000Z'
  return sortAPIModelProviders([
    { id: 'mock-api-provider-openai', providerCategory: 'gpt', code: 'openai', displayName: 'OpenAI', active: true, sortOrder: 10, createdAt: now, updatedAt: now },
    { id: 'mock-api-provider-anthropic', providerCategory: 'claude', code: 'anthropic', displayName: 'Anthropic', active: true, sortOrder: 20, createdAt: now, updatedAt: now },
    { id: 'mock-api-provider-google', providerCategory: 'gemini', code: 'google', displayName: 'Google', active: true, sortOrder: 30, createdAt: now, updatedAt: now },
    { id: 'mock-api-provider-perplexity', providerCategory: 'perplexity', code: 'perplexity', displayName: 'Perplexity', active: true, sortOrder: 40, createdAt: now, updatedAt: now },
    { id: 'mock-api-provider-openrouter', providerCategory: 'other', code: 'openrouter', displayName: 'OpenRouter', active: true, sortOrder: 50, createdAt: now, updatedAt: now },
  ])
}

function seedAdminAPIModels(providers: AdminApiModelProvider[]): AdminApiModel[] {
  const now = '2026-06-29T00:00:00.000Z'
  return sortAdminAPIModels(modelCatalog.map((item, index) => {
    const provider = seedProviderForPublicModel(item, providers)
    return withProvider({
      id: item.id,
      providerId: provider.id,
      providerCategory: provider.providerCategory,
      providerCode: provider.code,
      provider: provider.displayName,
      providerActive: provider.active,
      modelKey: item.name,
      displayName: item.displayName,
      capabilities: normalizeCapabilities(item.capabilities),
      active: item.active,
      currentPriceVersionId: item.officialInputPricePerMillion !== null || item.officialCachedInputPricePerMillion !== null || item.officialOutputPricePerMillion !== null ? `mock-price-${item.id}-seed` : undefined,
      currentPriceSourceUrl: '',
      currentPriceSourceVersion: 'mock-seed',
      currentPriceValidFrom: now,
      inputPricePerMillion: priceToString(item.officialInputPricePerMillion),
      cachedInputPricePerMillion: priceToString(item.officialCachedInputPricePerMillion),
      outputPricePerMillion: priceToString(item.officialOutputPricePerMillion),
      sortOrder: (index + 1) * 10,
      createdAt: now,
      updatedAt: now,
    }, provider)
  }))
}

function normalizeProviderInput(input: ApiModelProviderInput): ApiModelProviderInput {
  return {
    providerCategory: input.providerCategory,
    code: input.code.trim().toLowerCase(),
    displayName: input.displayName.trim(),
    active: input.active,
    sortOrder: input.sortOrder,
  }
}

function normalizeModelInput(input: ApiModelInput): ApiModelInput {
  return {
    providerId: input.providerId.trim(),
    modelKey: input.modelKey.trim(),
    displayName: input.displayName.trim(),
    capabilities: normalizeCapabilities(input.capabilities),
    inputTokenPrice: normalizePriceInput(input.inputTokenPrice),
    cachedInputTokenPrice: normalizePriceInput(input.cachedInputTokenPrice),
    outputTokenPrice: normalizePriceInput(input.outputTokenPrice),
    sourceUrl: input.sourceUrl.trim(),
    sourceVersion: input.sourceVersion.trim(),
    active: input.active,
    sortOrder: input.sortOrder,
  }
}

function fromProviderInput(id: string, input: ApiModelProviderInput, previous?: AdminApiModelProvider): AdminApiModelProvider {
  const now = new Date().toISOString()
  return {
    id,
    providerCategory: input.providerCategory,
    code: input.code,
    displayName: input.displayName,
    active: input.active,
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
    id,
    providerId: input.providerId,
    providerCategory: previous?.providerCategory ?? 'other',
    providerCode: previous?.providerCode ?? '',
    provider: previous?.provider ?? '',
    providerActive: previous?.providerActive ?? true,
    modelKey: input.modelKey,
    displayName: input.displayName,
    capabilities: input.capabilities,
    active: input.active,
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

function withProvider(item: AdminApiModel, provider: AdminApiModelProvider): AdminApiModel {
  return {
    ...item,
    providerId: provider.id,
    providerCategory: provider.providerCategory,
    providerCode: provider.code,
    provider: provider.displayName,
    providerActive: provider.active,
  }
}

function toPublicModel(item: AdminApiModel): ModelCatalogItem {
  return {
    id: item.id,
    provider: publicProvider(item),
    name: item.modelKey,
    displayName: item.displayName,
    capabilities: item.capabilities.filter(isPublicCapability),
    officialInputPricePerMillion: priceToNumber(item.inputPricePerMillion),
    officialCachedInputPricePerMillion: priceToNumber(item.cachedInputPricePerMillion),
    officialOutputPricePerMillion: priceToNumber(item.outputPricePerMillion),
    active: item.active,
  }
}

function publicProvider(item: AdminApiModel): ModelCatalogItem['provider'] {
  if (item.providerCategory === 'gpt' || item.providerCode === 'openai') return 'openai'
  if (item.providerCategory === 'claude' || item.providerCode === 'anthropic') return 'anthropic'
  return 'other'
}

function seedProviderForPublicModel(item: ModelCatalogItem, providers: AdminApiModelProvider[]) {
  if (item.provider === 'openai') return providerByCode('openai', providers)
  if (item.provider === 'anthropic') return providerByCode('anthropic', providers)
  if (item.id.includes('gemini')) return providerByCode('google', providers)
  return providerByCode('openrouter', providers)
}

function activeProviderOrThrow(providerId: string) {
  const provider = providerById(providerId, readMockAPIModelProviders())
  if (!provider.active) throw new Error('API 提供商已停用。')
  return provider
}

function providerById(providerId: string, providers: AdminApiModelProvider[]) {
  return providers.find(item => item.id === providerId) ?? providerByCode('openrouter', providers)
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
    const providerDelta = providerOrder.indexOf(left.providerCategory) - providerOrder.indexOf(right.providerCategory)
    if (providerDelta !== 0) return providerDelta
    if (left.sortOrder !== right.sortOrder) return left.sortOrder - right.sortOrder
    return left.displayName.localeCompare(right.displayName)
  })
}

function sortAdminAPIModels(items: AdminApiModel[]) {
  return [...items].sort((left, right) => {
    const providerDelta = providerOrder.indexOf(left.providerCategory) - providerOrder.indexOf(right.providerCategory)
    if (providerDelta !== 0) return providerDelta
    if (left.sortOrder !== right.sortOrder) return left.sortOrder - right.sortOrder
    return left.displayName.localeCompare(right.displayName)
  })
}
