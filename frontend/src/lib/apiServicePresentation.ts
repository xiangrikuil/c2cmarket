import type { ApiPurchaseIntent, ApiService } from '@/data/mock'

export type ApiMerchantIdentitySource = Pick<ApiService, 'merchant' | 'merchantIdentityMode' | 'merchantDisplayName'>
export type ApiIntentMerchantSource = Pick<ApiPurchaseIntent, 'merchant'> & {
  snapshot: Pick<ApiPurchaseIntent['snapshot'], 'merchant' | 'merchantDisplayName'>
}

export function getApiMerchantDisplayName(source: ApiMerchantIdentitySource | ApiIntentMerchantSource) {
  if ('snapshot' in source) {
    return source.snapshot.merchantDisplayName || source.snapshot.merchant
  }
  return source.merchantDisplayName || source.merchant
}

export function isApiServicePubliclyOrderable(service: Pick<ApiService, 'online' | 'publiclyOrderable'>) {
  return service.online && service.publiclyOrderable
}
