import { computed, type Ref } from 'vue'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  getAdminUserDetail,
  getAdminUserDirectory,
  updateAdminUserPermission,
  updateAdminUserStatus,
} from '@/lib/api'
import { ensureBackendSession } from '@/lib/backendClient'
import type { AdminUserDirectoryQuery } from '@/lib/adminUserBackend'

function valueOf<T>(value: Ref<T> | T): T {
  return typeof value === 'object' && value !== null && 'value' in value ? value.value : value
}

export const adminUserQueryKeys = {
  all: ['admin-users'] as const,
  directory: (query: AdminUserDirectoryQuery) => ['admin-users', 'directory', query] as const,
  detail: (userId: string) => ['admin-users', 'detail', userId] as const,
  session: ['admin-users', 'session'] as const,
}

export function useAdminUserDirectory(query: Ref<AdminUserDirectoryQuery> | AdminUserDirectoryQuery) {
  return useQuery({
    queryKey: computed(() => adminUserQueryKeys.directory(valueOf(query))),
    queryFn: () => getAdminUserDirectory(valueOf(query)),
    placeholderData: keepPreviousData,
    refetchOnMount: 'always',
  })
}

export function useAdminUserDetail(userId: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => adminUserQueryKeys.detail(valueOf(userId))),
    queryFn: () => getAdminUserDetail(valueOf(userId)),
    enabled: computed(() => Boolean(valueOf(userId))),
    refetchOnMount: 'always',
  })
}

export function useCurrentAdminSession() {
  return useQuery({
    queryKey: adminUserQueryKeys.session,
    queryFn: () => ensureBackendSession('admin', true),
  })
}

export function useUpdateAdminUserStatusMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: updateAdminUserStatus,
    async onSuccess(detail) {
      queryClient.setQueryData(adminUserQueryKeys.detail(detail.user.id), detail)
      await queryClient.invalidateQueries({ queryKey: adminUserQueryKeys.all })
    },
    async onError(_error, input) {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: adminUserQueryKeys.detail(input.userId) }),
        queryClient.invalidateQueries({ queryKey: adminUserQueryKeys.all }),
      ])
    },
  })
}

export function useUpdateAdminUserPermissionMutation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: updateAdminUserPermission,
    async onSuccess(detail) {
      queryClient.setQueryData(adminUserQueryKeys.detail(detail.user.id), detail)
      await queryClient.invalidateQueries({ queryKey: adminUserQueryKeys.all })
    },
    async onError(_error, input) {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: adminUserQueryKeys.detail(input.userId) }),
        queryClient.invalidateQueries({ queryKey: adminUserQueryKeys.all }),
      ])
    },
  })
}
