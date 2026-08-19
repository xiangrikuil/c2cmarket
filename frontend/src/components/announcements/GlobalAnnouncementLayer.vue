<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { backendErrorMessage } from '@/lib/backendClient'
import {
  selectGlobalAnnouncementDeliveries,
} from '@/lib/announcementUtils'
import { readAnnouncementReceipts, upsertAnnouncementReceipt } from '@/lib/announcementStorage'
import {
  useAcknowledgeAnnouncement,
  useActiveAnnouncements,
  useDismissAnnouncement,
  useMarkAnnouncementSeen,
  usePublicActiveAnnouncements,
} from '@/queries/useAnnouncementQueries'
import { useMyProfileQuery } from '@/queries/useAppShellQueries'
import type { AnnouncementDelivery, AnnouncementReceipt } from '@/types/announcement'
import CriticalAnnouncementDialog from './CriticalAnnouncementDialog.vue'
import GlobalAnnouncementBar from './GlobalAnnouncementBar.vue'

const { data: profile, isPending: profilePending } = useMyProfileQuery(import.meta.client)
const authResolved = computed(() => import.meta.client && !profilePending.value)
const authenticated = computed(() => Boolean(profile.value))
const authenticatedQuery = useActiveAnnouncements(undefined, authenticated)
const anonymousEnabled = computed(() => authResolved.value && !authenticated.value)
const publicBarQuery = usePublicActiveAnnouncements('global_bar', anonymousEnabled)
const publicModalQuery = usePublicActiveAnnouncements('modal', anonymousEnabled)
const seenMutation = useMarkAnnouncementSeen()
const dismissMutation = useDismissAnnouncement()
const acknowledgeMutation = useAcknowledgeAnnouncement()
const localReceiptRevision = ref(0)
const seenKeys = new Set<string>()
const seenInFlight = new Set<string>()
const deliveryError = ref<{ announcementId: string, action: 'seen' | 'dismiss', message: string } | null>(null)
const acknowledgementError = ref('')

const announcements = computed(() => {
  if (authenticated.value) return authenticatedQuery.data.value ?? []
  const unique = new Map<string, AnnouncementDelivery>()
  for (const item of [...(publicBarQuery.data.value ?? []), ...(publicModalQuery.data.value ?? [])]) unique.set(item.id, item)
  return [...unique.values()]
})

const receipts = computed(() => {
  void localReceiptRevision.value
  return authenticated.value ? {} : readAnnouncementReceipts()
})

const deliveries = computed(() => selectGlobalAnnouncementDeliveries(announcements.value, receipts.value))
const criticalAnnouncement = computed(() => deliveries.value.critical)
const barAnnouncement = computed(() => deliveries.value.bar)
const barError = computed(() => {
  const error = deliveryError.value
  return error && error.announcementId === barAnnouncement.value?.id ? error.message : ''
})
const criticalError = computed(() => {
  const error = deliveryError.value
  return error && error.announcementId === criticalAnnouncement.value?.id ? error.message : ''
})

watch([criticalAnnouncement, barAnnouncement], ([critical, bar]) => {
  for (const item of [critical, bar]) {
    if (!item) continue
    const key = `${item.id}:${item.version}`
    const receipt = authenticated.value ? embeddedReceipt(item) : receipts.value[item.id]
    if (seenKeys.has(key) || (receipt?.announcementVersion === item.version && receipt.firstSeenAt)) continue
    void markRenderedSeen(item)
  }
}, { immediate: true })

watch(barAnnouncement, (item) => {
  if (typeof document === 'undefined') return
  document.documentElement.style.setProperty('--global-announcement-height', item ? '3.25rem' : '0rem')
}, { immediate: true })

async function dismiss(announcementId: string) {
  const item = announcements.value.find(candidate => candidate.id === announcementId)
  if (!item) return
  try {
    if (authenticated.value) await dismissMutation.mutateAsync(announcementId)
    else {
      const now = new Date().toISOString()
      if (!upsertAnnouncementReceipt(item, { firstSeenAt: now, dismissedAt: now })) throw new Error('公告关闭状态保存失败。')
      localReceiptRevision.value += 1
    }
    if (deliveryError.value?.announcementId === announcementId) deliveryError.value = null
  } catch (error) {
    deliveryError.value = {
      announcementId,
      action: 'dismiss',
      message: backendErrorMessage(error, '公告关闭状态未保存，请重试。'),
    }
  }
}

async function acknowledge(announcementId: string) {
  const item = announcements.value.find(candidate => candidate.id === announcementId)
  if (!item) return
  acknowledgementError.value = ''
  try {
    if (authenticated.value) await acknowledgeMutation.mutateAsync(announcementId)
    else {
      const now = new Date().toISOString()
      if (!upsertAnnouncementReceipt(item, { firstSeenAt: now, acknowledgedAt: now })) throw new Error('公告确认状态保存失败。')
      localReceiptRevision.value += 1
    }
  } catch (error) {
    acknowledgementError.value = backendErrorMessage(error, '确认未保存，请重试。')
  }
}

async function markRenderedSeen(item: AnnouncementDelivery) {
  const key = `${item.id}:${item.version}`
  if (seenKeys.has(key) || seenInFlight.has(key)) return
  seenInFlight.add(key)
  try {
    if (authenticated.value) await seenMutation.mutateAsync(item.id)
    else {
      if (!upsertAnnouncementReceipt(item, { firstSeenAt: new Date().toISOString() })) throw new Error('公告展示状态保存失败。')
      localReceiptRevision.value += 1
    }
    seenKeys.add(key)
    if (deliveryError.value?.announcementId === item.id && deliveryError.value.action === 'seen') deliveryError.value = null
  } catch (error) {
    deliveryError.value = {
      announcementId: item.id,
      action: 'seen',
      message: backendErrorMessage(error, '公告展示状态未保存，请重试。'),
    }
  } finally {
    seenInFlight.delete(key)
  }
}

function retryDelivery(announcementId: string) {
  const item = announcements.value.find(candidate => candidate.id === announcementId)
  if (!item) return
  if (deliveryError.value?.announcementId === announcementId && deliveryError.value.action === 'dismiss') {
    void dismiss(announcementId)
    return
  }
  void markRenderedSeen(item)
}

function embeddedReceipt(item: AnnouncementDelivery): AnnouncementReceipt | undefined {
  return 'receipt' in item ? item.receipt : undefined
}

watch(criticalAnnouncement, () => {
  acknowledgementError.value = ''
})

onBeforeUnmount(() => {
  if (typeof document !== 'undefined') document.documentElement.style.removeProperty('--global-announcement-height')
})
</script>

<template>
  <GlobalAnnouncementBar
    v-if="barAnnouncement"
    :announcement="barAnnouncement"
    :dismissing="dismissMutation.isPending.value"
    :error="barError"
    @dismiss="dismiss"
    @retry="retryDelivery"
  />
  <CriticalAnnouncementDialog
    v-if="criticalAnnouncement"
    :announcement="criticalAnnouncement"
    :acknowledging="acknowledgeMutation.isPending.value"
    :error="acknowledgementError || criticalError"
    @acknowledge="acknowledge"
  />
</template>
