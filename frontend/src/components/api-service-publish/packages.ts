import { sellingModeLabels, type ApiServicePackage } from './types'
import { defaultApiQuotaUsagePolicyInput } from '@/lib/apiQuotaPolicy'

const cloneQuotaUsagePolicy = (item: ApiServicePackage['quotaUsagePolicy']): ApiServicePackage['quotaUsagePolicy'] => ({
  fiveHour: { ...item.fiveHour },
  daily: { ...item.daily },
})

export const createDefaultApiServicePackage = (modelCatalogIds: string[]): ApiServicePackage => ({
  id: globalThis.crypto?.randomUUID?.() ?? `package-${Date.now()}`,
  name: `3 天${sellingModeLabels.package}`,
  priceCny: 5,
  panelAllowance: 50,
  durationDays: 3,
  stockTotal: 10,
  description: '商户提交交付后开始计算有效期。',
  enabled: true,
  modelCatalogIds: [...modelCatalogIds],
  quotaUsagePolicy: defaultApiQuotaUsagePolicyInput(),
})

export const cloneApiServicePackageDraft = (item: ApiServicePackage): ApiServicePackage => ({
  ...item,
  modelCatalogIds: [...item.modelCatalogIds],
  quotaUsagePolicy: cloneQuotaUsagePolicy(item.quotaUsagePolicy),
})

export const applyApiServicePackageDraft = (target: ApiServicePackage, draft: ApiServicePackage) => {
  Object.assign(target, {
    name: draft.name,
    priceCny: draft.priceCny,
    panelAllowance: draft.panelAllowance,
    durationDays: draft.durationDays,
    stockTotal: draft.stockTotal,
    description: draft.description,
    enabled: draft.enabled,
    quotaUsagePolicy: cloneQuotaUsagePolicy(draft.quotaUsagePolicy),
  })
}
