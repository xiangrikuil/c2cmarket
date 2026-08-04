import type { ApiServicePackage } from './types'
import { defaultApiQuotaUsagePolicyInput } from '@/lib/apiQuotaPolicy'

export const createDefaultApiServicePackage = (modelCatalogIds: string[]): ApiServicePackage => ({
  id: globalThis.crypto?.randomUUID?.() ?? `package-${Date.now()}`,
  name: '3 天固定额度包',
  priceCny: 9.9,
  panelAllowance: 5,
  durationDays: 3,
  stockTotal: 10,
  description: '商户提交交付后开始计算有效期。',
  enabled: true,
  modelCatalogIds: [...modelCatalogIds],
  quotaUsagePolicy: defaultApiQuotaUsagePolicyInput(),
})
