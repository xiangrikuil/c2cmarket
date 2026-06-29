<script setup lang="ts">
import { computed } from 'vue'
import { ArrowRight, Pin } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { announcementCategoryLabels, announcementLevelLabels, isAnnouncementUnread } from '@/lib/announcementUtils'
import { getAnnouncementReceipt } from '@/lib/announcementStorage'
import type { Announcement } from '@/types/announcement'

const props = defineProps<{
  announcement: Announcement
}>()

const receipt = computed(() => props.announcement.receipt ?? getAnnouncementReceipt(props.announcement.id))
const unread = computed(() => isAnnouncementUnread(props.announcement, receipt.value))
const detailTo = computed(() => `/announcements/${props.announcement.slug}`)
const publishedAt = computed(() => new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
}).format(new Date(props.announcement.publishAt)))
</script>

<template>
  <article class="flex flex-col gap-3 p-4 md:flex-row md:items-center md:justify-between">
    <RouterLink :to="detailTo" class="min-w-0 flex-1 hover:underline">
      <div class="flex flex-wrap items-center gap-2">
        <span v-if="unread" class="h-2 w-2 rounded-full bg-primary" aria-label="未读公告"></span>
        <Badge variant="outline">{{ announcementCategoryLabels[announcement.category] }}</Badge>
        <Badge :variant="announcement.level === 'important' ? 'default' : 'secondary'">{{ announcementLevelLabels[announcement.level] }}</Badge>
        <Badge v-if="announcement.isPinned" variant="secondary" class="gap-1"><Pin class="h-3 w-3" />置顶</Badge>
        <h3 class="min-w-0 text-sm leading-5" :class="unread ? 'font-semibold' : 'font-medium text-muted-foreground'">
          {{ announcement.title }}
        </h3>
      </div>
      <p class="mt-1 line-clamp-1 text-sm text-muted-foreground">{{ announcement.summary }}</p>
    </RouterLink>
    <div class="flex shrink-0 items-center gap-3 md:justify-end">
      <span class="text-xs text-muted-foreground">{{ publishedAt }}</span>
      <RouterLink :to="detailTo">
        <Button size="sm" variant="outline">
          查看详情
          <ArrowRight class="h-4 w-4" />
        </Button>
      </RouterLink>
    </div>
  </article>
</template>
