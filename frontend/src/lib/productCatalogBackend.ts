import { carpoolProductCatalog } from '@/data/mock'
import { backendMutation, backendRequest, ensureBackendSession, shouldUseRealBackend } from '@/lib/backendClient'
import type { ProductCategory, ProductCategoryCode, ProductCategoryInput, ProductPlan, ProductPlanInput } from '@/types/productCatalog'

type ListResponse<T> = { items: T[] }

const productCatalogCategoryAdminStorageKey = 'marketplace.admin.product-categories'
const productCatalogAdminStorageKey = 'marketplace.admin.product-plans'

const categoryRows: ProductCategory[] = [
  { id: '00000000-0000-0000-0000-000000000101', code: 'gpt', displayName: 'GPT', sortOrder: 10, active: true },
  { id: '00000000-0000-0000-0000-000000000102', code: 'claude', displayName: 'Claude', sortOrder: 20, active: true },
  { id: '00000000-0000-0000-0000-000000000103', code: 'cursor', displayName: 'Cursor', sortOrder: 30, active: true },
  { id: '00000000-0000-0000-0000-000000000104', code: 'gemini', displayName: 'Gemini', sortOrder: 40, active: true },
  { id: '00000000-0000-0000-0000-000000000105', code: 'perplexity', displayName: 'Perplexity', sortOrder: 50, active: true },
  { id: '00000000-0000-0000-0000-000000000199', code: 'other', displayName: '其他', sortOrder: 999, active: true },
]

function readMockProductCategories(): ProductCategory[] {
  if (typeof window === 'undefined') return sortProductCategories(categoryRows)
  try {
    const raw = window.sessionStorage.getItem(productCatalogCategoryAdminStorageKey)
    if (!raw) return sortProductCategories(categoryRows)
    const stored = JSON.parse(raw) as ProductCategory[]
    const storedIds = new Set(stored.map(item => item.id))
    return sortProductCategories([
      ...stored,
      ...categoryRows.filter(item => !storedIds.has(item.id)),
    ])
  } catch {
    return sortProductCategories(categoryRows)
  }
}

function writeMockProductCategories(items: ProductCategory[]) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(productCatalogCategoryAdminStorageKey, JSON.stringify(sortProductCategories(items)))
}

function readMockProductPlans(): ProductPlan[] {
  if (typeof window === 'undefined') return seedProductPlans()
  try {
    const raw = window.sessionStorage.getItem(productCatalogAdminStorageKey)
    if (!raw) return seedProductPlans()
    const stored = JSON.parse(raw) as ProductPlan[]
    const storedIds = new Set(stored.map(item => item.id))
    return sortProductPlans([
      ...stored,
      ...seedProductPlans().filter(item => !storedIds.has(item.id)),
    ])
  } catch {
    return seedProductPlans()
  }
}

function writeMockProductPlans(items: ProductPlan[]) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(productCatalogAdminStorageKey, JSON.stringify(sortProductPlans(items)))
}

function seedProductPlans(): ProductPlan[] {
  const categories = readMockProductCategories()
  return sortProductPlans(carpoolProductCatalog.map(item => {
    const category = categories.find(row => row.code === item.categoryCode) ?? categories[categories.length - 1]
    return {
      id: item.id,
      categoryId: category.id,
      categoryCode: category.code,
      providerCode: item.providerCode,
      slug: item.slug,
      displayName: item.displayName,
      description: item.description ?? '',
      publishPolicy: item.publishPolicy,
      accessMode: item.accessMode,
      providerPolicyStatus: item.providerPolicyStatus,
      riskLevel: item.riskLevel,
      riskAckRequired: item.riskAckRequired,
      riskNoticeCode: item.riskNoticeCode,
      policyVersion: item.policyVersion,
      policyNote: item.policyNote,
      quotaLabel: item.quotaLabel,
      quotaUnit: item.quotaUnit,
      quotaPeriod: item.quotaPeriod,
      active: item.active,
      allowCustomVariant: item.allowCustomVariant,
      sortOrder: item.sortOrder,
      createdAt: item.createdAt,
      updatedAt: item.updatedAt,
    }
  }))
}

function sortProductPlans(items: ProductPlan[]) {
  const categories = readMockProductCategories()
  return [...items].sort((left, right) => {
    const leftCategory = categories.find(item => item.code === left.categoryCode)?.sortOrder ?? 999
    const rightCategory = categories.find(item => item.code === right.categoryCode)?.sortOrder ?? 999
    if (leftCategory !== rightCategory) return leftCategory - rightCategory
    if (left.sortOrder !== right.sortOrder) return left.sortOrder - right.sortOrder
    return left.displayName.localeCompare(right.displayName)
  })
}

function sortProductCategories(items: ProductCategory[]) {
  return [...items].sort((left, right) => {
    if (left.sortOrder !== right.sortOrder) return left.sortOrder - right.sortOrder
    return left.displayName.localeCompare(right.displayName)
  })
}

function normalizeCategoryInput(input: ProductCategoryInput): ProductCategoryInput {
  return {
    ...input,
    code: input.code.trim().toLowerCase(),
    displayName: input.displayName.trim(),
  }
}

function normalizeInput(input: ProductPlanInput): ProductPlanInput {
  return {
    ...input,
    providerCode: input.providerCode.trim().toLowerCase(),
    slug: input.slug.trim().toLowerCase(),
    displayName: input.displayName.trim(),
    description: input.description.trim(),
    riskNoticeCode: input.riskNoticeCode.trim(),
    policyNote: input.policyNote.trim(),
    quotaLabel: input.quotaLabel.trim() || '额度',
    quotaUnit: input.quotaUnit.trim() || 'USD',
    quotaPeriod: 'monthly',
  }
}

function categoryById(id: string) {
  const categories = readMockProductCategories()
  return categories.find(item => item.id === id) ?? categories[categories.length - 1]
}

function fromInput(id: string, input: ProductPlanInput, previous?: ProductPlan): ProductPlan {
  const normalized = normalizeInput(input)
  const category = categoryById(normalized.categoryId)
  const policyChanged = previous
    ? previous.publishPolicy !== normalized.publishPolicy
      || previous.accessMode !== normalized.accessMode
      || previous.providerPolicyStatus !== normalized.providerPolicyStatus
      || previous.riskLevel !== normalized.riskLevel
      || previous.riskAckRequired !== normalized.riskAckRequired
      || (previous.riskNoticeCode ?? '') !== normalized.riskNoticeCode
      || previous.policyNote !== normalized.policyNote
      || previous.quotaLabel !== normalized.quotaLabel
      || previous.quotaUnit !== normalized.quotaUnit
      || previous.quotaPeriod !== normalized.quotaPeriod
    : false
  return {
    id,
    categoryId: normalized.categoryId,
    categoryCode: category.code,
    providerCode: normalized.providerCode,
    slug: normalized.slug,
    displayName: normalized.displayName,
    description: normalized.description,
    publishPolicy: normalized.publishPolicy,
    accessMode: normalized.accessMode,
    providerPolicyStatus: normalized.providerPolicyStatus,
    riskLevel: normalized.riskLevel,
    riskAckRequired: normalized.riskAckRequired,
    riskNoticeCode: normalized.riskNoticeCode || undefined,
    policyVersion: previous ? previous.policyVersion + (policyChanged ? 1 : 0) : 1,
    policyNote: normalized.policyNote,
    quotaLabel: normalized.quotaLabel,
    quotaUnit: normalized.quotaUnit,
    quotaPeriod: normalized.quotaPeriod,
    active: normalized.active,
    allowCustomVariant: normalized.allowCustomVariant,
    sortOrder: normalized.sortOrder,
    createdAt: previous?.createdAt ?? new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  }
}

export async function getProductCategories(): Promise<ProductCategory[]> {
  if (shouldUseRealBackend()) {
    const response = await backendRequest<ListResponse<ProductCategory>>('/api/v1/product-categories')
    return response.items
  }
  return readMockProductCategories().filter(item => item.active)
}

export async function getAdminProductCategories(): Promise<ProductCategory[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const response = await backendRequest<ListResponse<ProductCategory>>('/api/v1/admin/product-categories')
    return response.items
  }
  return readMockProductCategories()
}

export async function createProductCategory(input: ProductCategoryInput): Promise<ProductCategory> {
  const normalized = normalizeCategoryInput(input)
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ProductCategory>('/api/v1/admin/product-categories', normalized, {
      idempotencyPrefix: 'product-category-create',
    })
  }
  const rows = readMockProductCategories()
  if (rows.some(item => item.code === normalized.code)) throw new Error('分类 code 已被占用。')
  const created: ProductCategory = {
    id: `category-${Date.now()}`,
    ...normalized,
  }
  writeMockProductCategories([...rows, created])
  return created
}

export async function updateProductCategory(id: string, input: ProductCategoryInput): Promise<ProductCategory> {
  const normalized = normalizeCategoryInput(input)
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ProductCategory>(`/api/v1/admin/product-categories/${encodeURIComponent(id)}`, normalized, {
      method: 'PATCH',
    })
  }
  const rows = readMockProductCategories()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('产品分类不存在。')
  if (rows.some(item => item.id !== id && item.code === normalized.code)) throw new Error('分类 code 已被占用。')
  const updated: ProductCategory = { id, ...normalized }
  writeMockProductCategories(rows.map(item => item.id === id ? updated : item))
  const plans = readMockProductPlans()
  writeMockProductPlans(plans.map(item => item.categoryId === id ? { ...item, categoryCode: updated.code, updatedAt: new Date().toISOString() } : item))
  return updated
}

export async function setProductCategoryActive(id: string, active: boolean): Promise<ProductCategory> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ProductCategory>(`/api/v1/admin/product-categories/${encodeURIComponent(id)}/${active ? 'activate' : 'deactivate'}`, {})
  }
  const rows = readMockProductCategories()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('产品分类不存在。')
  const updated = { ...previous, active }
  writeMockProductCategories(rows.map(item => item.id === id ? updated : item))
  return updated
}

export async function getAdminProductPlans(category?: ProductCategoryCode | 'all'): Promise<ProductPlan[]> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    const query = category && category !== 'all' ? `?category=${encodeURIComponent(category)}` : ''
    const response = await backendRequest<ListResponse<ProductPlan>>(`/api/v1/admin/product-plans${query}`)
    return response.items
  }
  const rows = readMockProductPlans()
  return category && category !== 'all' ? rows.filter(item => item.categoryCode === category) : rows
}

export async function createProductPlan(input: ProductPlanInput): Promise<ProductPlan> {
  const normalized = normalizeInput(input)
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ProductPlan>('/api/v1/admin/product-plans', normalized, {
      idempotencyPrefix: 'product-plan-create',
    })
  }
  const rows = readMockProductPlans()
  if (rows.some(item => item.slug === normalized.slug)) throw new Error('套餐 slug 已被占用。')
  const created = fromInput(`plan-${Date.now()}`, normalized)
  writeMockProductPlans([...rows, created])
  return created
}

export async function updateProductPlan(id: string, input: ProductPlanInput): Promise<ProductPlan> {
  const normalized = normalizeInput(input)
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ProductPlan>(`/api/v1/admin/product-plans/${encodeURIComponent(id)}`, normalized, {
      method: 'PATCH',
    })
  }
  const rows = readMockProductPlans()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('产品套餐不存在。')
  if (rows.some(item => item.id !== id && item.slug === normalized.slug)) throw new Error('套餐 slug 已被占用。')
  const updated = fromInput(id, normalized, previous)
  writeMockProductPlans(rows.map(item => item.id === id ? updated : item))
  return updated
}

export async function setProductPlanActive(id: string, active: boolean): Promise<ProductPlan> {
  if (shouldUseRealBackend()) {
    await ensureBackendSession('admin', true)
    return backendMutation<ProductPlan>(`/api/v1/admin/product-plans/${encodeURIComponent(id)}/${active ? 'activate' : 'deactivate'}`, {})
  }
  const rows = readMockProductPlans()
  const previous = rows.find(item => item.id === id)
  if (!previous) throw new Error('产品套餐不存在。')
  const updated = { ...previous, active, updatedAt: new Date().toISOString() }
  writeMockProductPlans(rows.map(item => item.id === id ? updated : item))
  return updated
}
