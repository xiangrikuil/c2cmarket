import { computed, type Ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { shouldUseRealBackend } from '@/lib/backendClient'
import type { ApiServiceSalesView } from '@/data/mock'

function valueOf<T>(value: Ref<T> | T): T {
  return typeof value === 'object' && value !== null && 'value' in value ? value.value : value
}

async function getMyCarpools() {
  if (shouldUseRealBackend()) {
    const { backendOwnerCarpools } = await import('@/lib/carpoolBackend')
    return backendOwnerCarpools()
  }
  const api = await import('@/lib/api')
  return api.getMyCarpools()
}

async function getMyApiServices(salesView: ApiServiceSalesView) {
  if (shouldUseRealBackend()) {
    const { backendOwnerAPIServices } = await import('@/lib/apiMarketBackend')
    return backendOwnerAPIServices(salesView)
  }
  const api = await import('@/lib/api')
  return api.getMyApiServices(salesView)
}

async function getMyProfile() {
  if (shouldUseRealBackend()) {
    const { backendMyProfile } = await import('@/lib/profileBackend')
    return backendMyProfile()
  }
  const api = await import('@/lib/api')
  return api.getMyProfile()
}

async function getNotifications() {
  if (shouldUseRealBackend()) {
    const { backendNotifications } = await import('@/lib/notificationBackend')
    return backendNotifications()
  }
  const api = await import('@/lib/api')
  return api.getNotifications()
}

async function getMyContactMethods() {
  if (shouldUseRealBackend()) {
    const { backendMyContactMethods } = await import('@/lib/profileBackend')
    return backendMyContactMethods()
  }
  const api = await import('@/lib/api')
  return api.getMyContactMethods()
}

export function myProfileQueryKey() {
  return ['my-profile'] as const
}

export function myContactMethodsQueryKey() {
  return ['my-contact-methods'] as const
}

export function useMyCarpools(enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: ['my-carpools'],
    queryFn: getMyCarpools,
    enabled: computed(() => valueOf(enabled)),
    refetchOnMount: 'always',
  })
}

export function useMyApiServices(
  salesView: Ref<ApiServiceSalesView> | ApiServiceSalesView = 'active',
  enabled: Ref<boolean> | boolean = true,
) {
  return useQuery({
    queryKey: computed(() => ['my-api-services', valueOf(salesView)]),
    queryFn: () => getMyApiServices(valueOf(salesView)),
    enabled: computed(() => valueOf(enabled)),
    refetchOnMount: 'always',
  })
}

export function useMyProfileQuery(enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: myProfileQueryKey(),
    queryFn: getMyProfile,
    enabled: computed(() => valueOf(enabled)),
    retry: false,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function useMyContactMethodsQuery(
  enabled: Ref<boolean> | boolean = true,
  userId: Ref<string> | string = '',
) {
  return useQuery({
    queryKey: computed(() => [...myContactMethodsQueryKey(), valueOf(userId)]),
    queryFn: getMyContactMethods,
    enabled: computed(() => valueOf(enabled)),
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function useNotifications(enabled: Ref<boolean> | boolean = true) {
  return useQuery({
    queryKey: ['notifications'],
    queryFn: getNotifications,
    enabled: computed(() => valueOf(enabled)),
    refetchOnMount: 'always',
  })
}
