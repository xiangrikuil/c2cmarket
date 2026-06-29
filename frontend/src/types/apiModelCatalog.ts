export type ApiModelProviderCategory = 'gpt' | 'claude' | 'cursor' | 'gemini' | 'perplexity' | 'other'

export type ApiModelCapability =
  | 'text'
  | 'chat'
  | 'vision'
  | 'image_generation'
  | 'image_edit'
  | 'reasoning'

export type AdminApiModel = {
  id: string
  providerId: string
  providerCategory: ApiModelProviderCategory
  providerCode: string
  provider: string
  providerActive: boolean
  modelKey: string
  displayName: string
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

export type AdminApiModelProvider = {
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
  active: boolean
  sortOrder: number
}

export type ApiModelInput = {
  providerId: string
  modelKey: string
  displayName: string
  capabilities: ApiModelCapability[]
  inputTokenPrice: string
  cachedInputTokenPrice: string
  outputTokenPrice: string
  sourceUrl: string
  sourceVersion: string
  active: boolean
  sortOrder: number
}

export const apiModelProviderCategories: Array<{ value: ApiModelProviderCategory, label: string }> = [
  { value: 'gpt', label: 'GPT' },
  { value: 'claude', label: 'Claude' },
  { value: 'cursor', label: 'Cursor' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'perplexity', label: 'Perplexity' },
  { value: 'other', label: '其他' },
]

export const apiModelCapabilities: Array<{ value: ApiModelCapability, label: string }> = [
  { value: 'text', label: '文本' },
  { value: 'chat', label: '对话' },
  { value: 'vision', label: '视觉' },
  { value: 'image_generation', label: '文生图' },
  { value: 'image_edit', label: '图像编辑' },
  { value: 'reasoning', label: '推理' },
]
