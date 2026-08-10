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

test('models.dev Mock 预览无写入，新增默认停用并可批量启用', async () => {
  const backend = await loadMockBackend()
  const providers = await backend.getAdminAPIModelProviders()
  const openai = providers.find(provider => provider.code === 'openai')
  assert.ok(openai)

  const before = await backend.getAdminAPIModels()
  const preview = await backend.previewAPIModelsDevSync([openai.id])
  const afterPreview = await backend.getAdminAPIModels()
  assert.equal(afterPreview.length, before.length)
  assert.equal(preview.counts.new, 1)
  assert.ok(preview.counts.priceChanged >= 1)

  const newItem = preview.items.find(item => item.modelKey === 'gpt-4.1-mini')
  assert.equal(newItem?.status, 'new')
  const applied = await backend.applyAPIModelsDevSync([selectionFrom(newItem!)])
  assert.equal(applied.created, 1)

  const afterApply = await backend.getAdminAPIModels()
  const imported = afterApply.find(model => model.modelKey === 'gpt-4.1-mini')
  assert.ok(imported)
  assert.equal(imported.active, false)
  assert.equal(backend.getMockPublicAPIModels().some(model => model.name === 'gpt-4.1-mini'), false)

  const activated = await backend.setAPIModelsBulkStatus({ modelIds: [imported.id], active: true })
  assert.equal(activated.changed, 1)
  assert.equal(backend.getMockPublicAPIModels().some(model => model.name === 'gpt-4.1-mini'), true)
})

test('models.dev Mock 改价保留原状态并拒绝过期预览', async () => {
  const backend = await loadMockBackend()
  const providers = await backend.getAdminAPIModelProviders()
  const openai = providers.find(provider => provider.code === 'openai')
  assert.ok(openai)

  const preview = await backend.previewAPIModelsDevSync([openai.id])
  const changed = preview.items.find(item => item.modelKey === 'gpt-5-mini')
  assert.equal(changed?.status, 'price_changed')

  const before = (await backend.getAdminAPIModels()).find(model => model.modelKey === 'gpt-5-mini')
  assert.ok(before)

  const tampered = selectionFrom(changed!, false)
  tampered.outputPricePerMillion = '0.000001'
  await assert.rejects(
    () => backend.applyAPIModelsDevSync([tampered]),
    /同步候选项已变化/,
  )
  const afterTamperedApply = (await backend.getAdminAPIModels()).find(model => model.modelKey === 'gpt-5-mini')
  assert.equal(afterTamperedApply?.outputPricePerMillion, before.outputPricePerMillion)

  await backend.applyAPIModelsDevSync([selectionFrom(changed!, false)])
  const after = (await backend.getAdminAPIModels()).find(model => model.modelKey === 'gpt-5-mini')
  assert.ok(after)
  assert.equal(after.active, before.active)
  assert.equal(after.outputPricePerMillion, '2.100000')

  await assert.rejects(
    () => backend.applyAPIModelsDevSync([selectionFrom(changed!, false)]),
    /同步候选项已变化/,
  )
})
