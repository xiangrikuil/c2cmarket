import type { ContactMethodType, ContactUsageScope, SaveContactMethodRequest, UserContactMethod } from '@/lib/api'

export type ContactUsageScopeOption = {
  value: ContactUsageScope
  label: string
  description: string
}

export const CONTACT_USAGE_SCOPE_OPTIONS: ContactUsageScopeOption[] = [
  { value: 'carpool_owner', label: '拼车车主', description: '用于拼车交易联系' },
  { value: 'api_merchant', label: 'API 商户', description: '用于 API 订单联系' },
  { value: 'buyer', label: '买家', description: '用于购买时联系' },
  { value: 'dispute', label: '纠纷联系', description: '用于售后纠纷联系' },
]

export function contactUsageScopeOptionsForCapabilities(capabilities: {
  canPublishCarpool: boolean
  canPublishApiService: boolean
}): ContactUsageScopeOption[] {
  return CONTACT_USAGE_SCOPE_OPTIONS.filter(option => {
    if (option.value === 'carpool_owner') return capabilities.canPublishCarpool
    if (option.value === 'api_merchant') return capabilities.canPublishApiService
    return true
  })
}

export function normalizeContactUsageScopes(scopes: ContactUsageScope[]): ContactUsageScope[] {
  const selected = new Set(scopes)
  return CONTACT_USAGE_SCOPE_OPTIONS.map(option => option.value).filter(scope => selected.has(scope))
}

export function initialContactUsageScopes(
  current: UserContactMethod | null,
  defaults: ContactUsageScope[],
): ContactUsageScope[] {
  return normalizeContactUsageScopes(current?.usageScopes.length ? current.usageScopes : defaults)
}

export function sameContactUsageScopes(left: ContactUsageScope[], right: ContactUsageScope[]): boolean {
  const normalizedLeft = normalizeContactUsageScopes(left)
  const normalizedRight = normalizeContactUsageScopes(right)
  return normalizedLeft.length === normalizedRight.length
    && normalizedLeft.every((scope, index) => scope === normalizedRight[index])
}

export function buildContactMethodPayload(input: {
  type: ContactMethodType
  label: string
  displayValue: string
  usageScopes: ContactUsageScope[]
  current: UserContactMethod | null
}): SaveContactMethodRequest {
  return {
    type: input.type,
    label: input.label,
    displayValue: input.displayValue.trim(),
    usageScopes: normalizeContactUsageScopes(input.usageScopes),
    isDefault: input.current?.isDefault ?? false,
    enabled: true,
  }
}
