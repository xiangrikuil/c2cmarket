import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'
import type { ApiModelSyncItem, ApiModelSyncSelection, ModelsDevProviderCode } from '@/types/apiModelCatalog'

function createStorage() {
  const store = new Map<string, string>()
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

async function loadMockBackend() {
  vi.resetModules()
  vi.stubGlobal('window', {
    sessionStorage: createStorage(),
    localStorage: createStorage(),
  })
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: '' })
  return import('../apiModelCatalogBackend')
}

function selectionFrom(item: ApiModelSyncItem, active = false): ApiModelSyncSelection {
  return {
    fingerprint: item.fingerprint,
    status: item.status as 'new' | 'price_changed',
    providerId: item.providerId,
    providerCode: item.providerCode as ModelsDevProviderCode,
    modelKey: item.modelKey,
    capabilities: item.capabilities.filter((capability): capability is 'text' | 'vision' | 'reasoning' => capability === 'text' || capability === 'vision' || capability === 'reasoning'),
    sourceUrl: 'https://models.dev/api.json',
    sourceVersion: item.sourceVersion ?? '',
    inputPricePerMillion: item.inputPricePerMillion ?? '',
    cachedInputPricePerMillion: item.cachedInputPricePerMillion ?? '',
    outputPricePerMillion: item.outputPricePerMillion ?? '',
    localModelId: item.localModelId ?? '',
    localPriceVersionId: item.localPriceVersionId ?? '',
    active,
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('models.dev Mock 预览无写入，新增模型默认退役并通过专用动作启用', async () => {
  const backend = await loadMockBackend()
  const providers = await backend.getAdminAPIModelProviders()
  const openai = providers.find(provider => provider.code === 'openai')
  assert.ok(openai)

  const before = await backend.getAdminAPIModels()
  const preview = await backend.previewAPIModelsDevSync([openai.id])
  const afterPreview = await backend.getAdminAPIModels()
  assert.equal(afterPreview.length, before.length)
  assert.equal(preview.counts.new, 1)
  assert.ok(preview.counts.sourceMissing >= 1)

  const newItem = preview.items.find(item => item.modelKey === 'gpt-5-mini')
  assert.equal(newItem?.status, 'new')
  const applied = await backend.applyAPIModelsDevSync([selectionFrom(newItem!)])
  assert.equal(applied.created, 1)

  const afterApply = await backend.getAdminAPIModels()
  const imported = afterApply.find(model => model.modelKey === 'gpt-5-mini')
  assert.ok(imported)
  assert.equal(imported.active, false)
  assert.equal(backend.getMockPublicAPIModels().some(model => model.name === 'gpt-5-mini'), false)

  const activated = await backend.applyAPIModelLifecycle(imported.id, imported.version, 'reactivate', '审核模型后启用')
  assert.equal(activated.status, 'active')
  assert.equal(backend.getMockPublicAPIModels().some(model => model.name === 'gpt-5-mini'), true)
})

test('models.dev Mock 识别价格来源变化和来源缺失，不自动改写本地目录', async () => {
  const backend = await loadMockBackend()
  const providers = await backend.getAdminAPIModelProviders()
  const openai = providers.find(provider => provider.code === 'openai')
  assert.ok(openai)

  const preview = await backend.previewAPIModelsDevSync([openai.id])
  assert.equal(preview.items.find(item => item.modelKey === 'gpt-4.1-mini')?.status, 'price_changed')
  assert.equal(preview.items.find(item => item.modelKey === 'gpt-4.1')?.status, 'source_missing')
  const before = await backend.getAdminAPIModels()
  const after = await backend.getAdminAPIModels()
  assert.deepEqual(after, before)
})

test('models.dev Mock 可预览并导入 xAI Grok 模型', async () => {
  const backend = await loadMockBackend()
  const providers = await backend.getAdminAPIModelProviders()
  const xai = providers.find(provider => provider.code === 'xai')
  assert.ok(xai)

  const preview = await backend.previewAPIModelsDevSync([xai.id])
  const candidate = preview.items.find(item => item.modelKey === 'grok-4.5')
  assert.equal(candidate?.providerCode, 'xai')
  assert.equal(candidate?.status, 'new')

  const applied = await backend.applyAPIModelsDevSync([selectionFrom(candidate!)])
  assert.equal(applied.created, 1)
  const imported = (await backend.getAdminAPIModels()).find(model => model.modelKey === 'grok-4.5')
  assert.equal(imported?.providerCategory, 'grok')
  assert.equal(imported?.active, false)
})
