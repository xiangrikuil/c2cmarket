<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import ArrowRight from 'lucide-vue-next/dist/esm/icons/arrow-right.js'
import RotateCcw from 'lucide-vue-next/dist/esm/icons/rotate-ccw.js'
import Siren from 'lucide-vue-next/dist/esm/icons/siren.js'
import X from 'lucide-vue-next/dist/esm/icons/x.js'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { AnnouncementDelivery } from '@/types/announcement'

const props = defineProps<{
  announcement: AnnouncementDelivery
  dismissing?: boolean
  error?: string
}>()

const emit = defineEmits<{
  dismiss: [announcementId: string]
  retry: [announcementId: string]
}>()

const detailTo = computed(() => `/announcements/${props.announcement.slug}`)
</script>

<template>
  <div class="fixed inset-x-0 top-0 z-[70] min-h-[3.25rem] border-b border-amber-300/70 bg-amber-50 text-zinc-950 shadow-sm dark:border-amber-500/35 dark:bg-zinc-950 dark:text-zinc-50" role="status" aria-live="polite">
    <div class="mx-auto flex min-h-[3.25rem] max-w-[1600px] items-center gap-3 px-3 py-2 sm:px-5">
      <Siren class="h-4 w-4 shrink-0 text-amber-700 dark:text-amber-400" aria-hidden="true" />
      <Badge :variant="announcement.level === 'critical' ? 'destructive' : 'outline'" class="shrink-0">
        {{ announcement.level === 'critical' ? '紧急' : '重要' }}
      </Badge>
      <div class="min-w-0 flex-1 sm:flex sm:items-baseline sm:gap-3">
        <strong class="block truncate text-sm font-semibold">{{ announcement.title }}</strong>
        <span class="hidden min-w-0 truncate text-xs text-zinc-600 sm:block dark:text-zinc-300">{{ announcement.summary }}</span>
      </div>
      <Button as-child size="sm" variant="ghost" class="shrink-0 px-2 text-zinc-900 hover:bg-amber-100 dark:text-zinc-100 dark:hover:bg-zinc-800">
        <RouterLink :to="detailTo">
          <span class="hidden sm:inline">查看详情</span>
          <ArrowRight class="h-4 w-4" aria-hidden="true" />
        </RouterLink>
      </Button>
      <Button
        v-if="error"
        size="icon"
        variant="ghost"
        class="h-8 w-8 shrink-0 text-destructive"
        :aria-label="`${error} 点击重试`"
        :title="error"
        @click="emit('retry', announcement.id)"
      >
        <RotateCcw class="h-4 w-4" />
      </Button>
      <Button
        v-if="announcement.isDismissible"
        size="icon"
        variant="ghost"
        class="h-8 w-8 shrink-0"
        aria-label="关闭全站通知"
        :disabled="dismissing"
        @click="emit('dismiss', announcement.id)"
      >
        <X class="h-4 w-4" />
      </Button>
    </div>
  </div>
  <div class="h-[3.25rem]" aria-hidden="true" />
</template>
