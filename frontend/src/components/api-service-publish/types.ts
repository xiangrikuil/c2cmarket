import type { ApiMerchantIdentityMode, ModelCatalogItem } from '@/lib/api'
import type { ApiPaymentMethod } from '@/lib/apiPaymentSettings'
import type { ApiQuotaUsagePolicyInput } from '@/types/apiQuota'

export type DistributionSystem = 'sub2api' | 'new_api_proxy' | 'other'
export type ApiProviderCategory = 'gpt' | 'claude' | 'other'
export type BillingMode = 'metered_credit' | 'fixed_package'
export type SellingMode = 'free' | 'package' | 'limited'
export const sellingModeLabels = {
  free: '自由额度',
  package: '限时流量包',
  limited: '限时额度包',
} as const satisfies Record<SellingMode, string>
export type PublishDeliveryMode = 'api_key_endpoint' | 'sub2api_panel_account'
export type PublishPaymentMethod = ApiPaymentMethod
export type UsageVisibility = 'panel_realtime' | 'panel_balance_only' | 'merchant_confirmed' | 'fixed_package_only' | 'not_available'
export type ValidityMode = 'days' | 'permanent'
export type AccountPoolType = 'gpt_pro_20x' | 'gpt_pro_5x' | 'gpt_plus' | 'custom'
export type WarrantyMode = '' | 'no_warranty' | 'merchant_full_refund'

export type SelectedServiceModel = {
  modelId: string
  enabled: boolean
}

export type ImageCapabilityConfig = {
  enabled: boolean
  supportsTextToImage: boolean
  supportsImageToImage: boolean
  pricingMode: 'same_multiplier' | 'custom_multiplier'
  customMultiplier: number | null
  note: string | null
}

export type ApiServicePackage = {
  id: string
  name: string
  priceCny: number
  panelAllowance: number
  durationDays: 1 | 3 | 7 | 30
  stockTotal: number
  description: string
  enabled: boolean
  modelCatalogIds: string[]
  quotaUsagePolicy: ApiQuotaUsagePolicyInput
}

export type ApiServicePaymentOption = {
  paymentMethod: PublishPaymentMethod
  enabled: boolean
  paymentInstructions: string
  paymentQrCodeDataUrl: string | null
}

export type WarrantyConfig = {
  mode: WarrantyMode
}

export type ApiServicePublishForm = {
	probeConnectionId: string
	ownerContactMethodIds: string[]
  merchantIdentityMode: ApiMerchantIdentityMode
  merchantDisplayName: string
  distributionSystem: DistributionSystem
  distributionSystemNote: string
  providerCategory: ApiProviderCategory
  billingMode: BillingMode
  deliveryModes: PublishDeliveryMode[]
  shortDescription: string
  cnyPerUsdCredit: number | null
  manualBillingNote: string
  defaultMultiplier: number
  selectedModels: SelectedServiceModel[]
  imageCapability: ImageCapabilityConfig
  availableCreditUsd: number | null
  quotaExpiresAt: string
  quotaUsagePolicy: ApiQuotaUsagePolicyInput
  minimumPurchaseCny: number | null
  maximumPurchaseCny: number | null
  paymentWindowMinutes: number
  paymentOptions: ApiServicePaymentOption[]
  declaredMaxConcurrency: number
  promptAuditEnabled: boolean | null
  packages: ApiServicePackage[]
  validity: {
    mode: ValidityMode
    days: number | null
    startsAt: 'delivered_at'
  }
  usageVisibility: UsageVisibility
  accountPoolType: AccountPoolType | ''
  accountPoolCustomName: string
  warranty: WarrantyConfig
  merchantNote: string
}

export type CatalogById = Map<string, ModelCatalogItem>
