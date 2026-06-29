import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  createProductCategory,
  createProductPlan,
  getAdminProductCategories,
  getAdminProductPlans,
  getProductCategories,
  setProductCategoryActive,
  setProductPlanActive,
  updateProductCategory,
  updateProductPlan,
} from '@/lib/productCatalogBackend'
import { clearBackendCarpoolProductCatalogCache } from '@/lib/carpoolBackend'
import type { ProductCategoryCode, ProductCategoryInput, ProductPlanInput } from '@/types/productCatalog'

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

export function useSetProductCategoryActive() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, active }: { id: string, active: boolean }) => setProductCategoryActive(id, active),
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

export function useSetProductPlanActive() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, active }: { id: string, active: boolean }) => setProductPlanActive(id, active),
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
