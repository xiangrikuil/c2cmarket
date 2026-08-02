import { backendAPIServices } from '@/lib/apiMarketBackend'
import { shouldUseRealBackend } from '@/lib/backendClient'
import { backendGetCarpools } from '@/lib/carpoolBackend'
import { backendOfficialPrices } from '@/lib/officialPriceBackend'
import { isApiServicePubliclyOrderable } from '@/lib/apiServicePresentation'

const developmentCacheDurationMs = 2_000

async function getRealHomeMarket() {
  const [officialPrices, carpools, apiServices] = await Promise.all([
    backendOfficialPrices(),
    backendGetCarpools(),
    backendAPIServices({ online: true }),
  ])

  return {
    officialPrices,
    carpools,
    apiServices: apiServices.filter(isApiServicePubliclyOrderable),
  }
}

let developmentCache: {
  expiresAt: number
  data: Awaited<ReturnType<typeof getRealHomeMarket>>
} | null = null

export async function getHomeMarket() {
  if (!shouldUseRealBackend()) {
    const api = await import('@/lib/api')
    return api.getHomeMarket()
  }

  if (import.meta.dev && developmentCache && developmentCache.expiresAt > Date.now()) {
    return developmentCache.data
  }

  const data = await getRealHomeMarket()
  if (import.meta.dev) {
    developmentCache = {
      expiresAt: Date.now() + developmentCacheDurationMs,
      data,
    }
  }
  return data
}
