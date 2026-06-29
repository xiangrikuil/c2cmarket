<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Bell, CheckCircle2, Clock3, ShieldAlert } from 'lucide-vue-next'
import AnnouncementListItem from '@/components/announcements/AnnouncementListItem.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import { useMarkAllNotificationsReadMutation, useMarkNotificationReadMutation, useNotifications } from '@/queries/useMarketQueries'
import { useAnnouncementUnreadCount, useAnnouncements } from '@/queries/useAnnouncementQueries'
import { toast } from 'vue-sonner'

const route = useRoute()
const router = useRouter()
const { data: notificationRows } = useNotifications()
const { data: announcements } = useAnnouncements()
const { data: announcementUnreadCount } = useAnnouncementUnreadCount()
const markReadMutation = useMarkNotificationReadMutation()
const markAllReadMutation = useMarkAllNotificationsReadMutation()

const notifications = computed(() => notificationRows.value ?? [])
const announcementRows = computed(() => announcements.value ?? [])
const unreadCount = computed(() => notifications.value.filter(item => item.unread).length)
const activeTab = computed(() => route.query.tab === 'announcements' ? 'announcements' : 'business')
const reviewCount = computed(() => notifications.value.filter(item => item.type === '审核结果' || item.type === '管理操作').length)
const carpoolCount = computed(() => notifications.value.filter(item => item.type === '上车申请').length)
const apiCount = computed(() => notifications.value.filter(item => item.type === 'API 意向').length)

watch(() => route.query.tab, tab => {
  if (tab && tab !== 'business' && tab !== 'announcements') {
    router.replace({ query: { ...route.query, tab: 'business' } })
  }
}, { immediate: true })

function iconFor(type: string, title: string) {
  if (type === '边界提醒') return ShieldAlert
  if (type === '审核结果' || type === '管理操作') return CheckCircle2
  if (title.includes('窗口') || title.includes('预留')) return Clock3
  return Bell
}

function markRead(id: string) {
  markReadMutation.mutate(id, {
    onSuccess: () => toast.success('通知已标记为已读。'),
    onError: error => toast.error(error instanceof Error ? error.message : '操作失败'),
  })
}

function markAllRead() {
  markAllReadMutation.mutate(undefined, {
    onSuccess: () => toast.success('全部通知已标记为已读。'),
    onError: error => toast.error(error instanceof Error ? error.message : '操作失败'),
  })
}

function setTab(tab: 'business' | 'announcements') {
  if (activeTab.value === tab) return
  router.replace({ query: { ...route.query, tab } })
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="通知中心" description="查看后端记录的站内业务通知；平台公告继续在独立标签中展示。" />

    <div class="grid gap-3 md:grid-cols-4">
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">未读通知</div>
        <div class="mt-2 text-2xl font-semibold">{{ unreadCount }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">审核 / 管理</div>
        <div class="mt-2 text-2xl font-semibold">{{ reviewCount }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">上车申请</div>
        <div class="mt-2 text-2xl font-semibold">{{ carpoolCount }}</div>
      </Card>
      <Card class="p-4">
        <div class="text-xs text-muted-foreground">API 意向</div>
        <div class="mt-2 text-2xl font-semibold">{{ apiCount }}</div>
      </Card>
    </div>

    <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div class="inline-flex rounded-md border border-border bg-card p-1">
        <button
          type="button"
          class="rounded px-3 py-1.5 text-sm transition"
          :class="activeTab === 'business' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
          @click="setTab('business')"
        >
          业务通知 {{ unreadCount }}
        </button>
        <button
          type="button"
          class="rounded px-3 py-1.5 text-sm transition"
          :class="activeTab === 'announcements' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
          @click="setTab('announcements')"
        >
          平台公告 {{ announcementUnreadCount ?? 0 }}
        </button>
      </div>
      <Button v-if="activeTab === 'business'" variant="outline" :disabled="!unreadCount || markAllReadMutation.isPending.value" @click="markAllRead">全部标记已读</Button>
    </div>

    <Card v-if="activeTab === 'business'" class="divide-y divide-border p-0">
      <div v-for="item in notifications" :key="item.id" class="flex flex-col gap-3 p-4 md:flex-row md:items-center md:justify-between">
        <div class="flex min-w-0 gap-3">
          <div class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-accent">
            <component :is="iconFor(item.type, item.title)" class="h-5 w-5 text-primary" />
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <RouterLink :to="item.to" class="font-semibold hover:underline">{{ item.title }}</RouterLink>
              <Badge :variant="item.unread ? 'default' : 'secondary'">{{ item.type }}</Badge>
            </div>
            <p class="mt-1 text-sm text-muted-foreground">{{ item.detail }}</p>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-3 md:justify-end">
          <span class="text-xs text-muted-foreground">{{ item.time }}</span>
          <RouterLink :to="item.to">
            <Button size="sm" variant="outline">
              查看详情
              <ArrowRight class="h-4 w-4" />
            </Button>
          </RouterLink>
          <Button size="sm" variant="outline" :disabled="!item.unread || markReadMutation.isPending.value" @click="markRead(item.id)">
            {{ item.unread ? '标记已读' : '已读' }}
          </Button>
        </div>
      </div>
      <div v-if="notifications.length === 0" class="p-8 text-center text-sm text-muted-foreground">暂无通知。</div>
    </Card>

    <Card v-else class="divide-y divide-border p-0">
      <AnnouncementListItem v-for="item in announcementRows" :key="item.id" :announcement="item" />
      <div v-if="announcementRows.length === 0" class="p-8 text-center text-sm text-muted-foreground">暂无平台公告。</div>
    </Card>
  </div>
</template>
