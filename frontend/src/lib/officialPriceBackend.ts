import type { AdminRow, OfficialPrice } from '@/lib/api'
import { backendMutation, backendRequest, ensureBackendSession } from '@/lib/backendClient'
import { backendCarpoolProductCatalog } from '@/lib/carpoolBackend'

type ListResponse<T> = { items: T[] }

type BackendOfficialPriceRecord = {
  id: string
  leadId: string
  productPlanId: string
  regionCode: string
  channel: string
  openingMethod: string
  sourceUrl: string
  status: string
  validFrom: string
  validTo?: string | null
  observedAt: string
  billingPeriod: string
  priceUnit: string
  currency: string
  originalAmount: string
  taxIncluded: boolean
  normalizedMonthlyCny: string
  fxRate: string
  fxSource: string
  fxObservedAt: string
  offerKey: string
  isLowestReference?: boolean
  createdAt: string
}

type BackendOfficialPriceLeadSummary = {
  id: string
  status: string
  productPlanId?: string
  productText: string
  regionCode: string
  sourceUrl: string
  observedAt: string
  billingPeriod: string
  currency: string
  originalAmount: string
  normalizedMonthlyCny?: string
  normalizationStatus?: string
  duplicateOfLeadId?: string | null
  version: number
  createdAt: string
}

type BackendOfficialPriceLeadAdmin = BackendOfficialPriceLeadSummary & {
  reviewReason?: string | null
  reviewedAt?: string | null
  submitterUserId: string
  planText?: string
  channel: string
  openingMethod: string
  sourceTitle?: string
  evidenceSummary: string
  note?: string
  priceUnit: string
  originalPriceText: string
  taxIncluded: boolean
  fxRate?: string
  fxSource?: string
  fxObservedAt?: string
  offerKey?: string
  reviewedByAdminId?: string
  conversionMode?: string
  normalizationRule?: string
}

type BackendApproveOfficialPriceLeadResponse = {
  lead: BackendOfficialPriceLeadSummary
  record: BackendOfficialPriceRecord
}

type SubmitOfficialPriceLeadPayload = {
  productPlanId?: unknown
  product?: unknown
  plan?: unknown
  region?: unknown
  channel?: unknown
  openingMethod?: unknown
  sourceUrl?: unknown
  note?: unknown
  originalPrice?: unknown
  originalPriceCurrency?: unknown
  originalPriceAmount?: unknown
}

const productPlanNames = new Map<string, string>()

function stringValue(value: unknown, fallback = '') {
  return typeof value === 'string' && value.trim() ? value.trim() : fallback
}

function decimalNumber(value: string | undefined, fallback: number | null = null) {
  if (!value) return fallback
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

function formatDate(value: string | undefined) {
  if (!value) return ''
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

function normalizeAmountText(value: string) {
  return value.trim().replace(/,/g, '')
}

function regionCode(value: string) {
  if (value === '菲律宾区' || value.includes('菲律宾')) return 'ph'
  if (value === '土耳其区' || value.includes('土耳其')) return 'tr'
  if (value === '香港区' || value.includes('香港')) return 'hk'
  if (value === '美国区' || value.includes('美国')) return 'us'
  if (value === '日本区' || value.includes('日本')) return 'jp'
  if (value === '新加坡区' || value.includes('新加坡')) return 'sg'
  return value.toLowerCase().replace(/[^a-z0-9_-]+/g, '_') || 'other'
}

function regionLabel(value: string) {
  const map: Record<string, string> = {
    ph: '菲律宾区',
    tr: '土耳其区',
    hk: '香港区',
    us: '美国区',
    jp: '日本区',
    sg: '新加坡区',
    other: '其他',
  }
  return map[value] ?? value
}

function backendLeadStatus(value: string): OfficialPrice['status'] {
  if (value === 'approved') return '已验证'
  if (value === 'changes_requested') return '需复核'
  if (value === 'rejected') return '有争议'
  return '待验证'
}

function backendRecordStatus(value: string): OfficialPrice['status'] {
  if (value === 'active') return '已验证'
  if (value === 'superseded' || value === 'expired' || value === 'taken_down') return '已过期'
  return '待验证'
}

function backendAdminStatus(value: string) {
  if (value === 'pending') return '待处理'
  if (value === 'changes_requested') return '待复核'
  if (value === 'approved') return '已通过'
  if (value === 'rejected') return '已拒绝'
  return value || '待处理'
}

function priceText(currency: string, amount: string) {
  return `${currency} ${amount}`.trim()
}

async function productPlanName(productPlanId: string | undefined) {
  if (!productPlanId) return ''
  if (productPlanNames.has(productPlanId)) return productPlanNames.get(productPlanId)!
  const catalog = await backendCarpoolProductCatalog()
  for (const item of catalog) {
    productPlanNames.set(item.id, item.displayName)
  }
  return productPlanNames.get(productPlanId) ?? ''
}

function splitProductPlanName(name: string) {
  const normalized = name.trim()
  if (!normalized) return { product: '官方价格记录', plan: '后端记录' }
  const known = ['ChatGPT', 'Claude', 'Cursor', 'Gemini', 'Perplexity']
  const prefix = known.find(item => normalized.includes(item))
  if (!prefix) return { product: normalized, plan: '官方记录' }
  return {
    product: prefix,
    plan: normalized.replace(prefix, '').trim() || normalized,
  }
}

export async function mapOfficialPriceRecord(record: BackendOfficialPriceRecord): Promise<OfficialPrice> {
  const planName = await productPlanName(record.productPlanId)
  const productPlan = splitProductPlanName(planName)
  return {
    id: record.id,
    product: productPlan.product,
    plan: productPlan.plan,
    region: regionLabel(record.regionCode),
    channel: record.channel || '其他',
    openingMethod: record.openingMethod || '其他',
    originalPrice: priceText(record.currency, record.originalAmount),
    cny: decimalNumber(record.normalizedMonthlyCny),
    status: backendRecordStatus(record.status),
    source: record.sourceUrl,
    submitter: '管理员审核',
    submitterTrust: 0,
    updatedAt: formatDate(record.validFrom || record.createdAt),
    isLowest: record.isLowestReference === true,
  }
}

async function mapLeadAsOfficialPrice(lead: BackendOfficialPriceLeadSummary): Promise<OfficialPrice> {
  const planName = await productPlanName(lead.productPlanId)
  const productPlan = splitProductPlanName(planName)
  return {
    id: lead.id,
    product: lead.productText || productPlan.product,
    plan: productPlan.plan,
    region: regionLabel(lead.regionCode),
    channel: '待审核',
    openingMethod: '待审核',
    originalPrice: priceText(lead.currency, lead.originalAmount),
    cny: decimalNumber(lead.normalizedMonthlyCny),
    status: backendLeadStatus(lead.status),
    source: lead.sourceUrl,
    submitter: '当前用户',
    submitterTrust: 0,
    updatedAt: formatDate(lead.createdAt),
    isLowest: false,
  }
}

export async function backendOfficialPrices() {
  const response = await backendRequest<ListResponse<BackendOfficialPriceRecord>>('/api/v1/official-prices')
  return Promise.all(response.items.map(mapOfficialPriceRecord))
}

export async function backendOfficialPriceById(id: string) {
  const record = await backendRequest<BackendOfficialPriceRecord>(`/api/v1/official-prices/${id}`)
  return mapOfficialPriceRecord(record)
}

function toSubmitRequest(payload: SubmitOfficialPriceLeadPayload) {
  const currency = stringValue(payload.originalPriceCurrency, 'CNY').toUpperCase()
  const amount = normalizeAmountText(stringValue(payload.originalPriceAmount, ''))
  const originalPrice = stringValue(payload.originalPrice, `${currency} ${amount}`)
  return {
    productPlanId: stringValue(payload.productPlanId),
    productText: stringValue(payload.product, '其他'),
    planText: stringValue(payload.plan, '自定义套餐'),
    regionCode: regionCode(stringValue(payload.region, '其他')),
    channel: stringValue(payload.channel, 'Web'),
    openingMethod: stringValue(payload.openingMethod, '其他'),
    sourceUrl: stringValue(payload.sourceUrl),
    sourceTitle: '用户提交低价线索',
    evidenceSummary: stringValue(payload.note, '用户提交来源链接和价格文本。'),
    note: stringValue(payload.note),
    observedAt: new Date().toISOString(),
    billingPeriod: 'monthly',
    currency,
    originalAmount: amount || '1',
    originalPriceText: originalPrice,
    taxIncluded: true,
  }
}

export async function backendSubmitOfficialPriceLead(payload: SubmitOfficialPriceLeadPayload) {
  await ensureBackendSession('buyer', false)
  const lead = await backendMutation<BackendOfficialPriceLeadSummary>('/api/v1/official-price-leads', toSubmitRequest(payload), {
    idempotencyPrefix: 'official-price-lead',
  })
  return mapLeadAsOfficialPrice(lead)
}

export async function backendMyOfficialPriceLeads() {
  await ensureBackendSession('buyer', false)
  const response = await backendRequest<ListResponse<BackendOfficialPriceLeadSummary>>('/api/v1/me/official-price-leads')
  return Promise.all(response.items.map(mapLeadAsOfficialPrice))
}

function leadAdminDetailItems(lead: BackendOfficialPriceLeadAdmin): AdminRow['detailItems'] {
  return [
    { label: '后端状态', value: lead.status },
    { label: '版本', value: String(lead.version) },
    { label: '来源', value: lead.sourceUrl },
    { label: '来源标题', value: lead.sourceTitle || '未提供' },
    { label: '证据摘要', value: lead.evidenceSummary || '未提供' },
    { label: '原始价格', value: lead.originalPriceText || priceText(lead.currency, lead.originalAmount) },
    { label: '计费周期', value: lead.billingPeriod },
    { label: '价格单位', value: lead.priceUnit },
    { label: '税费', value: lead.taxIncluded ? '含税' : '未含税' },
    { label: '复核原因', value: lead.reviewReason || '暂无' },
  ]
}

async function leadAdminRow(summary: BackendOfficialPriceLeadSummary): Promise<AdminRow> {
  const detail = await backendRequest<BackendOfficialPriceLeadAdmin>(`/api/v1/admin/official-price-leads/${summary.id}`)
  const planName = await productPlanName(detail.productPlanId)
  const productPlan = splitProductPlanName(planName || detail.planText || detail.productText)
  return {
    id: detail.id,
    primary: `${detail.productText || productPlan.product} ${detail.planText || productPlan.plan}`.trim(),
    secondary: `${regionLabel(detail.regionCode)} · ${detail.channel || '待补充'} · ${priceText(detail.currency, detail.originalAmount)}`,
    owner: `提交用户 ${detail.submitterUserId.slice(0, 8)} · 真实后端`,
    status: backendAdminStatus(detail.status),
    risk: detail.duplicateOfLeadId ? `疑似重复 ${detail.duplicateOfLeadId.slice(0, 8)}` : detail.normalizationStatus || '待管理员核价',
    targetType: 'official-price',
    backendKind: 'official-price-lead',
    backendVersion: detail.version,
    detailItems: leadAdminDetailItems(detail),
    targetTo: `/official-prices/${detail.id}`,
  }
}

export async function backendAdminOfficialPriceRows() {
  await ensureBackendSession('admin', true)
  const response = await backendRequest<ListResponse<BackendOfficialPriceLeadSummary>>('/api/v1/admin/official-price-leads')
  return Promise.all(response.items.map(leadAdminRow))
}

async function adminLead(id: string) {
  await ensureBackendSession('admin', true)
  return backendRequest<BackendOfficialPriceLeadAdmin>(`/api/v1/admin/official-price-leads/${id}`)
}

function fxRateToCny(currency: string) {
  const map: Record<string, string> = {
    CNY: '1.0000',
    USD: '7.2000',
    HKD: '0.9200',
    PHP: '0.1230',
    TRY: '0.2250',
    JPY: '0.0460',
    SGD: '5.3500',
  }
  return map[currency.toUpperCase()] ?? '1.0000'
}

async function firstProductPlanId(fallback: string | undefined) {
  if (fallback) return fallback
  const catalog = await backendCarpoolProductCatalog()
  const first = catalog.find(item => item.active !== false) ?? catalog[0]
  if (!first?.id) throw new Error('审核通过前需要可用产品套餐。')
  return first.id
}

async function approveLead(row: AdminRow, reason: string) {
  const detail = await adminLead(row.id)
  const now = new Date().toISOString()
  const response = await backendMutation<BackendApproveOfficialPriceLeadResponse>(`/api/v1/admin/official-price-leads/${row.id}/approve`, {
    reason: reason || '管理台审核通过',
    resolvedProductPlanId: await firstProductPlanId(detail.productPlanId),
    validFrom: now,
    fxSnapshot: {
      rateToCny: fxRateToCny(detail.currency),
      source: 'frontend-admin-demo',
      observedAt: now,
    },
  }, {
    idempotencyPrefix: 'official-price-approve',
    ifMatch: detail.version,
  })
  return leadAdminRow(response.lead)
}

async function updateLeadReviewStatus(row: AdminRow, action: 'reject' | 'request-changes', reason: string) {
  const detail = await adminLead(row.id)
  const response = await backendMutation<BackendOfficialPriceLeadSummary>(`/api/v1/admin/official-price-leads/${row.id}/${action}`, {
    reason: reason || '管理台审核操作',
  }, {
    idempotencyPrefix: `official-price-${action}`,
    ifMatch: detail.version,
  })
  return leadAdminRow(response)
}

export async function backendUpdateOfficialPriceAdminStatus(row: AdminRow, status: string, reason: string) {
  if (row.targetType !== 'official-price') return row
  if (status === '已通过') return approveLead(row, reason)
  return updateLeadReviewStatus(row, 'request-changes', reason)
}

export async function backendRunOfficialPriceAdminAction(row: AdminRow, action: 'approve' | 'request_changes' | 'take_down' | 'restore' | 'restrict' | 'warn' | 'suspend' | 'ban', reason: string) {
  if (row.targetType !== 'official-price') return row
  if (action === 'approve' || action === 'restore') return approveLead(row, reason)
  if (action === 'request_changes' || action === 'warn') return updateLeadReviewStatus(row, 'request-changes', reason)
  return updateLeadReviewStatus(row, 'reject', reason)
}
