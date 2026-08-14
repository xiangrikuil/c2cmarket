<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Bell, CheckCircle2, Clock3, ShieldAlert } from 'lucide-vue-next'
import AnnouncementListItem from '@/components/announcements/AnnouncementListItem.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import PageTitle from '@/components/market/PageTitle.vue'
import CompactStats from '@/components/market/CompactStats.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
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
type NotificationTab = 'todo' | 'transactions' | 'system' | 'announcements'
const activeTab = computed<NotificationTab>(() => {
  if (route.query.tab === 'transactions') return 'transactions'
  if (route.query.tab === 'system') return 'system'
  if (route.query.tab === 'announcements') return 'announcements'
  return 'todo'
})
const notificationCategory = (item: typeof notifications.value[number]): NotificationTab => {
  if (['审核结果', '管理操作', '边界提醒'].includes(item.type)) return 'system'
  if (item.unread) return 'todo'
  return 'transactions'
}
const todoCount = computed(() => notifications.value.filter(item => notificationCategory(item) === 'todo').length)
const transactionCount = computed(() => notifications.value.filter(item => notificationCategory(item) === 'transactions').length)
const systemCount = computed(() => notifications.value.filter(item => notificationCategory(item) === 'system').length)
const visibleNotifications = computed(() => notifications.value.filter(item => notificationCategory(item) === activeTab.value))
const stats = computed(() => [
  { label: '待办未读', value: todoCount.value },
  { label: '交易通知', value: transactionCount.value },
  { label: '系统通知', value: systemCount.value },
  { label: '平台公告', value: announcementUnreadCount.value ?? 0 },
])

watch(() => route.query.tab, tab => {
  if (tab && !['todo', 'transactions', 'system', 'business', 'announcements'].includes(String(tab))) router.replace({ query: { ...route.query, tab: 'todo' } })
}, { immediate: true })

function iconFor(type: string, title: string) {
  if (type === '边界提醒') return ShieldAlert
  if (type === '审核结果' || type === '管理操作') return CheckCircle2
  if (title.includes('窗口') || title.includes('预留')) return Clock3
  return Bell
}

function notificationTypeLabel(type: string) {
  return type === 'API 意向' ? 'API 订单' : type
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

function setTab(tab: NotificationTab) {
  if (activeTab.value === tab) return
  router.replace({ query: { ...route.query, tab } })
}
</script>

<template>
  <div class="notification-reference-page space-y-5">
    <div class="notification-reference-heading rounded-xl border px-5 py-4">
      <PageTitle
        :title="activeTab === 'announcements' ? '平台公告' : '通知中心'"
        :description="activeTab === 'announcements' ? '查看平台规则、功能更新与治理公告。' : '查看后端记录的站内业务通知。'"
      />
    </div>

    <CompactStats :items="stats" />

    <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <Tabs :model-value="activeTab" @update:model-value="value => setTab(value as NotificationTab)">
        <TabsList>
          <TabsTrigger value="todo">待办 {{ todoCount }}</TabsTrigger>
          <TabsTrigger value="transactions">交易 {{ transactionCount }}</TabsTrigger>
          <TabsTrigger value="system">系统 {{ systemCount }}</TabsTrigger>
          <TabsTrigger value="announcements">平台公告 {{ announcementUnreadCount ?? 0 }}</TabsTrigger>
        </TabsList>
      </Tabs>
      <Button
        v-if="activeTab === 'todo' || activeTab === 'transactions'"
        variant="outline"
        :disabled="!unreadCount || markAllReadMutation.isPending.value"
        @click="markAllRead"
      >
        全部标记已读
      </Button>
    </div>

    <Card v-if="activeTab === 'todo' || activeTab === 'transactions'" class="notification-reference-list divide-y divide-border p-0">
      <div v-for="item in visibleNotifications" :key="item.id" class="flex flex-col gap-3 p-4 md:flex-row md:items-center md:justify-between">
        <div class="flex min-w-0 gap-3">
          <div class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-accent">
            <component :is="iconFor(item.type, item.title)" class="h-5 w-5 text-primary" />
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <RouterLink :to="item.to" class="font-semibold hover:underline">{{ item.title }}</RouterLink>
              <Badge :variant="item.unread ? 'default' : 'secondary'">{{ notificationTypeLabel(item.type) }}</Badge>
            </div>
            <p class="mt-1 text-sm text-muted-foreground">{{ item.detail }}</p>
          </div>
        </div>
        <div class="flex shrink-0 items-center gap-3 md:justify-end">
          <span class="text-xs text-muted-foreground"><LocalTime :value="item.time" /></span>
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
      <EmptyState v-if="visibleNotifications.length === 0" title="当前分类暂无通知" description="状态变化和下一动作会在对应分类中显示。" />
    </Card>

    <Card v-else-if="activeTab === 'system'" class="divide-y divide-border p-0">
      <div v-for="item in visibleNotifications" :key="item.id" class="p-4">
        <RouterLink :to="item.to" class="font-semibold hover:underline">{{ item.title }}</RouterLink>
        <p class="mt-1 text-sm text-muted-foreground">{{ item.detail }}</p>
      </div>
      <EmptyState v-if="visibleNotifications.length === 0" title="暂无系统通知" description="审核结果、管理操作和边界提醒会显示在这里。" />
    </Card>

    <Card v-else class="divide-y divide-border p-0">
      <AnnouncementListItem v-for="item in announcementRows" :key="item.id" :announcement="item" />
      <EmptyState v-if="announcementRows.length === 0" title="暂无平台公告" description="平台更新和治理公告会显示在这里。" />
    </Card>
  </div>
</template>
