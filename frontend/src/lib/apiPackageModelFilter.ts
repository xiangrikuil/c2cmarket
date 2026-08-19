import type { ApiPackageFilterModel } from '@/lib/api'

export type ApiPackageProviderGroupKey = 'openai' | 'xai' | 'anthropic' | 'google' | 'other'

export type ApiPackageProviderGroup = {
  key: ApiPackageProviderGroupKey
  label: string
  models: ApiPackageFilterModel[]
}

const providerGroups: Array<{ key: ApiPackageProviderGroupKey, label: string, aliases: ReadonlySet<string> }> = [
  { key: 'openai', label: 'OpenAI', aliases: new Set(['openai', 'gpt']) },
  { key: 'xai', label: 'xAI (Grok)', aliases: new Set(['xai', 'grok']) },
  { key: 'anthropic', label: 'Anthropic', aliases: new Set(['anthropic', 'claude']) },
  { key: 'google', label: 'Google', aliases: new Set(['google', 'gemini']) },
  { key: 'other', label: 'Other', aliases: new Set() },
]

const providerGroupKey = (model: ApiPackageFilterModel): ApiPackageProviderGroupKey => {
  const values = [model.providerCode, model.providerCategory].map(value => value.trim().toLowerCase())
  return providerGroups.find(group => group.key !== 'other' && values.some(value => group.aliases.has(value)))?.key ?? 'other'
}

export const groupApiPackageFilterModels = (models: ApiPackageFilterModel[]): ApiPackageProviderGroup[] => {
  return providerGroups
    .map(group => ({
      key: group.key,
      label: group.label,
      models: models
        .filter(model => providerGroupKey(model) === group.key)
        .sort((left, right) => left.sortOrder - right.sortOrder || left.modelKey.localeCompare(right.modelKey)),
    }))
    .filter(group => group.models.length > 0)
}

export const normalizeApiPackageModelQuery = (value: unknown): string[] => {
  const values = Array.isArray(value) ? value : [value]
  const seen = new Set<string>()
  for (const item of values) {
    if (typeof item !== 'string') continue
    const normalized = item.trim()
    if (normalized) seen.add(normalized)
  }
  return [...seen]
}
