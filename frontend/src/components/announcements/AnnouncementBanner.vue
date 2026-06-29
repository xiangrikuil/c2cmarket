<script setup lang="ts">
import { computed } from 'vue'
import { Megaphone, X } from 'lucide-vue-next'
import { RouterLink } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { announcementCategoryLabels, announcementLevelLabels } from '@/lib/announcementUtils'
import type { Announcement } from '@/types/announcement'

const props = defineProps<{
  announcement: Announcement
  dismissing?: boolean
}>()

const emit = defineEmits<{
  dismiss: [announcementId: string]
}>()

const detailTo = computed(() => `/announcements/${props.announcement.slug}`)
const canDismiss = computed(() => props.announcement.isDismissible)
</script>

<template>
  <section
    class="rounded-md border px-3 py-2.5 shadow-sm"
    :class="announcement.level === 'important' ? 'border-primary/35 bg-primary/8' : 'border-border bg-card'"
    aria-label="平台公告"
  >
    <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
      <RouterLink :to="detailTo" class="flex min-w-0 gap-2 hover:underline">
        <span class="mt-0.5 grid h-6 w-6 shrink-0 place-items-center rounded-md bg-background text-primary">
          <Megaphone class="h-4 w-4" />
        </span>
        <span class="min-w-0">
          <span class="flex flex-wrap items-center gap-1.5">
            <Badge :variant="announcement.level === 'important' ? 'default' : 'secondary'">{{ announcementLevelLabels[announcement.level] }}</Badge>
            <Badge variant="outline">{{ announcementCategoryLabels[announcement.category] }}</Badge>
            <span class="line-clamp-2 text-sm font-semibold leading-5">{{ announcement.title }}</span>
          </span>
          <span class="mt-0.5 line-clamp-1 text-xs text-muted-foreground">{{ announcement.summary }}</span>
        </span>
      </RouterLink>
      <div class="flex shrink-0 items-center gap-2 sm:justify-end">
        <RouterLink :to="detailTo">
          <Button size="sm" variant="outline">查看详情</Button>
        </RouterLink>
        <Button
          v-if="canDismiss"
          size="icon"
          variant="ghost"
          class="h-8 w-8"
          aria-label="关闭首页公告"
          :disabled="dismissing"
          @click="emit('dismiss', announcement.id)"
        >
          <X class="h-4 w-4" />
        </Button>
      </div>
    </div>
  </section>
</template>
