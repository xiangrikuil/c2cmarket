import { computed, type Ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import {
  backendAdminAuditLogsPage,
  type AdminAuditLogFilters,
} from '@/lib/adminAuditBackend'
import type { CursorPageRequest } from '@/lib/cursorPagination'

function valueOf<T>(value: Ref<T> | T): T {
  return typeof value === 'object' && value !== null && 'value' in value ? value.value : value
}

export function useAdminAuditLogsPage(
  filters: Ref<AdminAuditLogFilters> | AdminAuditLogFilters,
  page: Ref<CursorPageRequest> | CursorPageRequest,
) {
  return useQuery({
    queryKey: computed(() => ['admin', 'audit-logs', valueOf(filters), valueOf(page)]),
    queryFn: () => backendAdminAuditLogsPage(valueOf(filters), valueOf(page)),
    retry: false,
    refetchOnMount: 'always',
  })
}
