import type { ApiMerchantIdentityMode, ModelCatalogItem } from '@/lib/api'

export type DistributionSystem = 'sub2api' | 'new_api_proxy' | 'other'
export type ApiProviderCategory = 'gpt' | 'claude' | 'other'
export type BillingMode = 'metered_credit' | 'manual_credit' | 'fixed_package'
export type PublishDeliveryMode = 'api_key_endpoint' | 'sub2api_panel_account'
export type UsageVisibility = 'panel_realtime' | 'panel_balance_only' | 'merchant_confirmed' | 'fixed_package_only' | 'not_available'
export type ValidityMode = 'days' | 'permanent'
export type WarrantyMode = 'no_warranty' | 'upstream_refund_only' | 'merchant_warranty'

export type SelectedServiceModel = {
  modelId: string
  multiplierOverride: number | null
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
  durationDays: number | null
  description: string
  inventory: number | null
}

export type WarrantyConfig = {
  mode: WarrantyMode
  warrantyDays: number | null
  coverage: string | null
  compensation: string | null
  exclusions: string | null
  refundNote: string | null
}

export type ApiServicePublishForm = {
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
  minimumPurchaseCny: number | null
  maximumPurchaseCny: number | null
  packages: ApiServicePackage[]
  validity: {
    mode: ValidityMode
    days: number | null
    startsAt: 'delivered_at'
  }
  usageVisibility: UsageVisibility
  warranty: WarrantyConfig
  merchantNote: string
}

export type CatalogById = Map<string, ModelCatalogItem>
