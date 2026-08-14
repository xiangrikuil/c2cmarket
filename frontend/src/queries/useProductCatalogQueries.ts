import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  applyProductCategoryLifecycle,
  applyProductPlanLifecycle,
  createProductCategory,
  createProductPlan,
  getAdminProductCategories,
  getAdminProductPlans,
  getProductCategories,
  updateProductCategory,
  updateProductPlan,
} from '@/lib/productCatalogBackend'
import { clearBackendCarpoolProductCatalogCache } from '@/lib/carpoolBackend'
import type { CatalogLifecycleAction, ProductCategoryCode, ProductCategoryInput, ProductPlanInput } from '@/types/productCatalog'

export const productCatalogQueryKeys = {
  categories: ['product-categories', 'active'] as const,
  adminCategories: ['admin-product-categories'] as const,
  adminPlans: ['admin-product-plans'] as const,
  activePlans: ['carpool-product-catalog', 'active'] as const,
}

export function useProductCategories() {
  return useQuery({
    queryKey: productCatalogQueryKeys.categories,
    queryFn: getProductCategories,
  })
}

export function useAdminProductCategories() {
  return useQuery({
    queryKey: productCatalogQueryKeys.adminCategories,
    queryFn: getAdminProductCategories,
    retry: false,
  })
}

export function useAdminProductPlans(category?: ProductCategoryCode | 'all') {
  return useQuery({
    queryKey: [...productCatalogQueryKeys.adminPlans, category ?? 'all'],
    queryFn: () => getAdminProductPlans(category),
    retry: false,
  })
}

export function useCreateProductCategory() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ProductCategoryInput) => createProductCategory(input),
    onSuccess() {
      invalidateProductCategoryQueries(queryClient)
    },
  })
}

export function useUpdateProductCategory() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string, input: ProductCategoryInput }) => updateProductCategory(id, input),
    onSuccess() {
      invalidateProductCategoryQueries(queryClient)
    },
  })
}

export function useApplyProductCategoryLifecycle() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, version, action, reason, targetStatus }: { id: string, version: number, action: CatalogLifecycleAction, reason: string, targetStatus?: 'active' | 'deprecated' }) => applyProductCategoryLifecycle(id, version, action, reason, targetStatus),
    onSuccess() {
      invalidateProductCategoryQueries(queryClient)
    },
  })
}

export function useCreateProductPlan() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: ProductPlanInput) => createProductPlan(input),
    onSuccess() {
      invalidateProductPlanQueries(queryClient)
    },
  })
}

export function useUpdateProductPlan() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string, input: ProductPlanInput }) => updateProductPlan(id, input),
    onSuccess() {
      invalidateProductPlanQueries(queryClient)
    },
  })
}

export function useApplyProductPlanLifecycle() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, version, action, reason, targetStatus }: { id: string, version: number, action: CatalogLifecycleAction, reason: string, targetStatus?: 'active' | 'deprecated' }) => applyProductPlanLifecycle(id, version, action, reason, targetStatus),
    onSuccess() {
      invalidateProductPlanQueries(queryClient)
    },
  })
}

function invalidateProductPlanQueries(queryClient: ReturnType<typeof useQueryClient>) {
  clearBackendCarpoolProductCatalogCache()
  queryClient.invalidateQueries({ queryKey: productCatalogQueryKeys.adminPlans })
  queryClient.invalidateQueries({ queryKey: productCatalogQueryKeys.activePlans })
  queryClient.invalidateQueries({ queryKey: productCatalogQueryKeys.categories })
}

function invalidateProductCategoryQueries(queryClient: ReturnType<typeof useQueryClient>) {
  clearBackendCarpoolProductCatalogCache()
  queryClient.invalidateQueries({ queryKey: productCatalogQueryKeys.adminCategories })
  queryClient.invalidateQueries({ queryKey: productCatalogQueryKeys.categories })
  queryClient.invalidateQueries({ queryKey: productCatalogQueryKeys.adminPlans })
  queryClient.invalidateQueries({ queryKey: productCatalogQueryKeys.activePlans })
}
