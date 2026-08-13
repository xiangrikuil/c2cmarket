import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, test, vi } from 'vitest'

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
  vi.stubGlobal('window', { sessionStorage: createStorage(), localStorage: createStorage() })
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: '' })
  return import('../productCatalogBackend')
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('默认公开目录只包含 GPT、Claude、Grok，旧品牌保持退役可管理', async () => {
  const backend = await loadMockBackend()
  const publicCategories = await backend.getProductCategories()
  assert.deepEqual(publicCategories.map(item => item.code), ['gpt', 'claude', 'grok'])

  const adminCategories = await backend.getAdminProductCategories()
  assert.equal(adminCategories.find(item => item.code === 'cursor')?.status, 'deprecated')
  assert.equal(adminCategories.find(item => item.code === 'other')?.status, 'deprecated')
})

test('首发品牌映射不会把 xAI 或未知供应商压缩为 Other', () => {
  const apiSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
  const officialPriceSource = readFileSync(new URL('../officialPriceBackend.ts', import.meta.url), 'utf8')

  assert.match(apiSource, /if \(provider === 'xai'\) return 'xAI'/)
  assert.match(apiSource, /return provider \|\| '未标注供应商'/)
  assert.doesNotMatch(apiSource, /model\?\.provider === 'anthropic' \? 'Anthropic' : 'Other'/)
  assert.match(officialPriceSource, /const known = \['ChatGPT', 'Claude', 'Grok'\]/)
})

test('未知品牌原样进入公开目录，父级退役会使其套餐失效', async () => {
  const backend = await loadMockBackend()
  const category = await backend.createProductCategory({ code: 'deepseek', displayName: 'DeepSeek', iconDataUrl: '', sortOrder: 40 })
  const plan = await backend.createProductPlan({
    categoryId: category.id,
    providerCode: 'deepseek',
    slug: 'deepseek-pro',
    displayName: 'DeepSeek Pro',
    description: '',
    publishPolicy: 'allowed',
    accessMode: 'owner_managed_access',
    providerPolicyStatus: 'unknown',
    riskLevel: 'normal',
    riskAckRequired: false,
    riskNoticeCode: '',
    policyNote: '',
    quotaLabel: '额度',
    quotaUnit: 'USD',
    quotaPeriod: 'monthly',
    allowCustomVariant: false,
    sortOrder: 10,
  })
  assert.equal(plan.categoryCode, 'deepseek')
  assert.equal(plan.providerCode, 'deepseek')

  const deprecated = await backend.applyProductCategoryLifecycle(category.id, category.version, 'deprecate', '目录正常退役')
  assert.equal(deprecated.status, 'deprecated')
  assert.equal((await backend.getProductCategories()).some(item => item.code === 'deepseek'), false)
  assert.equal((await backend.getAdminProductPlans()).find(item => item.id === plan.id)?.effectiveStatusSource, 'parent')
})
