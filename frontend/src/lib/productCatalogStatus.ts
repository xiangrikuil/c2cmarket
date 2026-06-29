import type { ProductPlan, ProductPlanInput } from '@/types/productCatalog'

export const DEFAULT_BOUNDARY_NOTICE_CODE = 'openai_subscription_carpool'

export const catalogBusinessStatusOptions = [
  { value: 'publishable', label: '可发布', badgeVariant: 'verified' },
  { value: 'publishable_with_boundary_confirmation', label: '发布前需确认边界', badgeVariant: 'secondary' },
  { value: 'info_only', label: '仅信息展示', badgeVariant: 'outline' },
  { value: 'blocked', label: '禁止发布', badgeVariant: 'destructive' },
] as const

export type CatalogBusinessStatus = typeof catalogBusinessStatusOptions[number]['value']

export function getCatalogBusinessStatus(plan: Pick<ProductPlan, 'publishPolicy' | 'riskAckRequired'>): CatalogBusinessStatus {
  if (plan.publishPolicy === 'blocked') return 'blocked'
  if (plan.publishPolicy === 'info_only') return 'info_only'
  return plan.riskAckRequired ? 'publishable_with_boundary_confirmation' : 'publishable'
}

export function getCatalogBusinessStatusMeta(status: CatalogBusinessStatus) {
  return catalogBusinessStatusOptions.find(item => item.value === status) ?? catalogBusinessStatusOptions[0]
}

export function applyCatalogBusinessStatus(input: ProductPlanInput, status: CatalogBusinessStatus): ProductPlanInput {
  const riskAckRequired = status === 'publishable_with_boundary_confirmation'
  const publishPolicy = status === 'blocked'
    ? 'blocked'
    : status === 'info_only'
      ? 'info_only'
      : 'allowed'

  return {
    ...input,
    publishPolicy,
    riskAckRequired,
    providerPolicyStatus: input.providerPolicyStatus || 'unknown',
    riskLevel: input.riskLevel || 'normal',
    riskNoticeCode: riskAckRequired ? input.riskNoticeCode.trim() || DEFAULT_BOUNDARY_NOTICE_CODE : '',
  }
}
