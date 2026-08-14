import type {
  ApiModelBulkMutationResult as GeneratedApiModelBulkMutationResult,
  ApiModelSyncItem as GeneratedApiModelSyncItem,
  ApiModelSyncPreview as GeneratedApiModelSyncPreview,
  ApiModelSyncSelection as GeneratedApiModelSyncSelection,
  ApiModelSyncStatus as GeneratedApiModelSyncStatus,
} from '@/api/generated/openapi'

export type ApiModelProviderCategory = string
export type CatalogStatus = 'active' | 'deprecated' | 'blocked'
export type CatalogLifecycleAction = 'deprecate' | 'block' | 'reactivate' | 'unblock'
export type CatalogCoreKey = 'gpt' | 'claude' | 'grok'

export type CatalogLifecycleFields = {
  coreKey?: CatalogCoreKey
  status: CatalogStatus
  effectiveStatus: CatalogStatus
  effectiveStatusSource: 'self' | 'parent'
  statusChangedAt: string
  statusChangedBy?: string
  statusReason?: string
  version: number
  identityLocked: boolean
  identityLockReason?: string
  active: boolean
}

export type ApiModelCapability =
  | 'text'
  | 'chat'
  | 'vision'
  | 'image_generation'
  | 'image_edit'
  | 'reasoning'

export type AdminApiModel = CatalogLifecycleFields & {
  id: string
  providerId: string
  providerCategory: ApiModelProviderCategory
  providerCode: string
  provider: string
  providerActive: boolean
  modelKey: string
  capabilities: ApiModelCapability[]
  active: boolean
  currentPriceVersionId?: string
  currentPriceSourceUrl?: string
  currentPriceSourceVersion?: string
  currentPriceValidFrom?: string
  inputPricePerMillion?: string
  cachedInputPricePerMillion?: string
  outputPricePerMillion?: string
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export type AdminApiModelProvider = CatalogLifecycleFields & {
  id: string
  providerCategory: ApiModelProviderCategory
  code: string
  displayName: string
  active: boolean
  sortOrder: number
  createdAt: string
  updatedAt: string
}

export type ApiModelProviderInput = {
  providerCategory: ApiModelProviderCategory
  code: string
  displayName: string
  sortOrder: number
}

export type ApiModelInput = {
  providerId: string
  modelKey: string
  capabilities: ApiModelCapability[]
  inputTokenPrice: string
  cachedInputTokenPrice: string
  outputTokenPrice: string
  sourceUrl: string
  sourceVersion: string
  sortOrder: number
}

export type ModelsDevProviderCode = 'openai' | 'anthropic' | 'xai'
export type ApiModelSyncStatus = GeneratedApiModelSyncStatus
export type ApiModelSyncItem = GeneratedApiModelSyncItem
export type ApiModelSyncPreview = GeneratedApiModelSyncPreview
export type ApiModelSyncSelection = GeneratedApiModelSyncSelection
export type ApiModelBulkMutationResult = GeneratedApiModelBulkMutationResult

export const apiModelProviderCategories: Array<{ value: ApiModelProviderCategory, label: string }> = [
  { value: 'gpt', label: 'GPT' },
  { value: 'claude', label: 'Claude' },
  { value: 'grok', label: 'Grok' },
]

export const apiModelCapabilities: Array<{ value: ApiModelCapability, label: string }> = [
  { value: 'text', label: '文本' },
  { value: 'chat', label: '对话' },
  { value: 'vision', label: '视觉' },
  { value: 'image_generation', label: '文生图' },
  { value: 'image_edit', label: '图像编辑' },
  { value: 'reasoning', label: '推理' },
]
