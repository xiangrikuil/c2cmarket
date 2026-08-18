import { afterEach, describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import type { ApiPackageFilterModel } from '@/lib/api'
import { groupApiPackageFilterModels, normalizeApiPackageModelQuery } from '@/lib/apiPackageModelFilter'
import { providerFromBackend } from '@/lib/apiMarketBackend'

const selectorSource = readFileSync(new URL('../../components/api-market/ApiPackageModelFilter.vue', import.meta.url), 'utf8')
const marketPageSource = readFileSync(new URL('../../pages/ApiMarketPage.vue', import.meta.url), 'utf8')

const model = (id: string, providerCode: string, providerCategory: string, sortOrder = 0): ApiPackageFilterModel => ({
  id,
  modelKey: id,
  providerCode,
  providerCategory,
  providerName: providerCode,
  providerSortOrder: 0,
  sortOrder,
})

const createStorage = () => {
  const store = new Map<string, string>()
  return {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, value),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.resetModules()
})

describe('API package model filters', () => {
  it('normalizes scalar and repeated URL values with stable deduplication', () => {
    expect(normalizeApiPackageModelQuery(' model-1 ')).toEqual(['model-1'])
    expect(normalizeApiPackageModelQuery(['model-2', 'model-1', 'model-2', '', null])).toEqual(['model-2', 'model-1'])
  })

  it('groups models by provider metadata in the required order', () => {
    const groups = groupApiPackageFilterModels([
      model('other-1', 'deepseek', 'other'),
      model('gemini-1', 'google', 'gemini'),
      model('claude-1', 'anthropic', 'claude'),
      model('grok-1', 'xai', 'grok'),
      model('gpt-1', 'openai', 'gpt'),
    ])

    expect(groups.map(group => group.label)).toEqual(['OpenAI', 'xAI (Grok)', 'Anthropic', 'Google', 'Other'])
    expect(groups.map(group => group.models[0]?.id)).toEqual(['gpt-1', 'grok-1', 'claude-1', 'gemini-1', 'other-1'])
  })

  it('preserves xAI/Grok and Google/Gemini provider identities from the backend', () => {
    expect(providerFromBackend('xai')).toBe('xai')
    expect(providerFromBackend('grok')).toBe('xai')
    expect(providerFromBackend('google')).toBe('google')
    expect(providerFromBackend('gemini')).toBe('google')
  })

  it('projects the in-stock Mock package models into OpenAI and Google groups', async () => {
    vi.stubGlobal('window', { sessionStorage: createStorage(), localStorage: createStorage() })
    const client = await import('@/lib/backendClient')
    client.setBackendRuntimeConfig({ apiMode: 'mock', apiBaseUrl: '' })
    const { getApiPackageFilterOptions } = await import('@/lib/api')

    const options = await getApiPackageFilterOptions()
    expect(options.models.map(item => item.id)).toEqual(['gpt-5-5', 'gemini-flash'])
    expect(groupApiPackageFilterModels(options.models).map(group => group.label)).toEqual(['OpenAI', 'Google'])
    expect(options.durations).toEqual([3, 7])
  })

  it('keeps group selection separate from expand and collapse controls', () => {
    expect(selectorSource).toMatch(/:model-value="groupState\(group\)"/)
    expect(selectorSource).toMatch(/@update:model-value="toggleGroup\(group\)"/)
    expect(selectorSource).toMatch(/@click="toggleOpen\(group\.key\)"/)
    expect(selectorSource).toContain('indeterminate')
    expect(selectorSource).toContain('<ApiPackageModelFilter :model-value="modelValue" :options="options" inline')
  })

  it('uses repeated URL model state and removes stale filter IDs after options load', () => {
    expect(marketPageSource).toMatch(/packageModels\.value = view === 'packages' \? normalizeApiPackageModelQuery\(route\.query\.model\) : \[\]/)
    expect(marketPageSource).toMatch(/query\.model = packageModels\.value/)
    expect(marketPageSource).toMatch(/packageModels\.value\.filter\(id => availableIds\.has\(id\)\)/)
    expect(marketPageSource).not.toContain('先选择精确模型和有效期')
  })
})
