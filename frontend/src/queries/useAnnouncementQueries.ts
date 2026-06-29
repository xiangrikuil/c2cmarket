import { computed, type Ref } from 'vue'
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import {
  createAnnouncement,
  dismissAnnouncement,
  duplicateAnnouncement,
  getActiveAnnouncements,
  getActiveHomeAnnouncement,
  getAdminAnnouncements,
  getAnnouncementAuditLogs,
  getAnnouncementById,
  getAnnouncementBySlug,
  getAnnouncementUnreadCount,
  getAnnouncements,
  getImportantAnnouncementUnreadCount,
  markAnnouncementRead,
  markAnnouncementSeen,
  offlineAnnouncement,
  publishAnnouncement,
  updateAnnouncement,
} from '@/lib/announcementsApi'
import type { AnnouncementChannel, AnnouncementFormInput } from '@/types/announcement'

function valueOf<T>(value: Ref<T> | T): T {
  return typeof value === 'object' && value !== null && 'value' in value ? value.value : value
}

export const announcementQueryKeys = {
  all: ['announcements'] as const,
  active: (channel?: AnnouncementChannel) => ['announcements', 'active', channel ?? 'all'] as const,
  home: ['announcements', 'home'] as const,
  detail: (slug: string) => ['announcements', 'detail', slug] as const,
  unreadCount: ['announcements', 'unread-count'] as const,
  importantUnreadCount: ['announcements', 'important-unread-count'] as const,
  adminList: ['admin-announcements'] as const,
  adminDetail: (id: string) => ['admin-announcements', id] as const,
  auditLogs: ['admin-announcements', 'audit-logs'] as const,
}

export function useAnnouncements() {
  return useQuery({
    queryKey: announcementQueryKeys.all,
    queryFn: getAnnouncements,
    refetchOnMount: 'always',
  })
}

export function useActiveAnnouncements(channel?: Ref<AnnouncementChannel | undefined> | AnnouncementChannel) {
  return useQuery({
    queryKey: computed(() => announcementQueryKeys.active(channel ? valueOf(channel) : undefined)),
    queryFn: () => getActiveAnnouncements(channel ? valueOf(channel) : undefined),
    refetchOnMount: 'always',
  })
}

export function useActiveHomeAnnouncement() {
  return useQuery({
    queryKey: announcementQueryKeys.home,
    queryFn: getActiveHomeAnnouncement,
    refetchOnMount: 'always',
  })
}

export function useAnnouncementDetail(slug: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => announcementQueryKeys.detail(valueOf(slug))),
    queryFn: () => getAnnouncementBySlug(valueOf(slug)),
    enabled: computed(() => Boolean(valueOf(slug))),
  })
}

export function useAnnouncementUnreadCount() {
  return useQuery({
    queryKey: announcementQueryKeys.unreadCount,
    queryFn: getAnnouncementUnreadCount,
    refetchOnMount: 'always',
  })
}

export function useImportantAnnouncementUnreadCount() {
  return useQuery({
    queryKey: announcementQueryKeys.importantUnreadCount,
    queryFn: getImportantAnnouncementUnreadCount,
    refetchOnMount: 'always',
  })
}

export function useAdminAnnouncements() {
  return useQuery({
    queryKey: announcementQueryKeys.adminList,
    queryFn: getAdminAnnouncements,
    refetchOnMount: 'always',
  })
}

export function useAdminAnnouncement(id: Ref<string> | string) {
  return useQuery({
    queryKey: computed(() => announcementQueryKeys.adminDetail(valueOf(id))),
    queryFn: () => getAnnouncementById(valueOf(id)),
    enabled: computed(() => Boolean(valueOf(id))),
  })
}

export function useAnnouncementAuditLogs() {
  return useQuery({
    queryKey: announcementQueryKeys.auditLogs,
    queryFn: getAnnouncementAuditLogs,
    refetchOnMount: 'always',
  })
}

export function useMarkAnnouncementRead() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (announcementId: string) => markAnnouncementRead(announcementId),
    onSuccess() {
      invalidateUserAnnouncementQueries(queryClient)
    },
  })
}

export function useMarkAnnouncementSeen() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (announcementId: string) => markAnnouncementSeen(announcementId),
    onSuccess() {
      invalidateUserAnnouncementQueries(queryClient)
    },
  })
}

export function useDismissAnnouncement() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (announcementId: string) => dismissAnnouncement(announcementId),
    onSuccess() {
      queryClient.invalidateQueries({ queryKey: announcementQueryKeys.home })
      invalidateUserAnnouncementQueries(queryClient)
    },
  })
}

export function useCreateAnnouncement() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: AnnouncementFormInput) => createAnnouncement(input),
    onSuccess(data) {
      queryClient.setQueryData(announcementQueryKeys.adminDetail(data.id), data)
      invalidateAllAnnouncementQueries(queryClient)
    },
  })
}

export function useUpdateAnnouncement() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string, input: AnnouncementFormInput }) => updateAnnouncement(id, input),
    onSuccess(data) {
      queryClient.setQueryData(announcementQueryKeys.adminDetail(data.id), data)
      invalidateAllAnnouncementQueries(queryClient)
    },
  })
}

export function usePublishAnnouncement() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => publishAnnouncement(id),
    onSuccess(data) {
      queryClient.setQueryData(announcementQueryKeys.adminDetail(data.id), data)
      invalidateAllAnnouncementQueries(queryClient)
    },
  })
}

export function useOfflineAnnouncement() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, reason }: { id: string, reason: string }) => offlineAnnouncement(id, reason),
    onSuccess(data) {
      queryClient.setQueryData(announcementQueryKeys.adminDetail(data.id), data)
      invalidateAllAnnouncementQueries(queryClient)
    },
  })
}

export function useDuplicateAnnouncement() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => duplicateAnnouncement(id),
    onSuccess(data) {
      queryClient.setQueryData(announcementQueryKeys.adminDetail(data.id), data)
      invalidateAllAnnouncementQueries(queryClient)
    },
  })
}

function invalidateUserAnnouncementQueries(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: announcementQueryKeys.all })
  queryClient.invalidateQueries({ queryKey: ['announcements', 'active'] })
  queryClient.invalidateQueries({ queryKey: announcementQueryKeys.home })
  queryClient.invalidateQueries({ queryKey: announcementQueryKeys.unreadCount })
  queryClient.invalidateQueries({ queryKey: announcementQueryKeys.importantUnreadCount })
}

function invalidateAllAnnouncementQueries(queryClient: ReturnType<typeof useQueryClient>) {
  invalidateUserAnnouncementQueries(queryClient)
  queryClient.invalidateQueries({ queryKey: announcementQueryKeys.adminList })
  queryClient.invalidateQueries({ queryKey: announcementQueryKeys.auditLogs })
}
