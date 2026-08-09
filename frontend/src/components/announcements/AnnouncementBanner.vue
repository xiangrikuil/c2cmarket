<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import ArrowRight from 'lucide-vue-next/dist/esm/icons/arrow-right.js'
import Megaphone from 'lucide-vue-next/dist/esm/icons/megaphone.js'
import X from 'lucide-vue-next/dist/esm/icons/x.js'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { announcementCategoryLabels } from '@/lib/announcementUtils'
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
    class="relative overflow-hidden rounded-lg border"
    :class="announcement.level === 'important' ? 'border-primary/30 bg-primary/5' : 'border-border bg-card'"
    aria-label="平台公告"
  >
    <div class="grid min-h-12 grid-cols-[1.25rem_minmax(0,1fr)] items-center gap-x-3 gap-y-1.5 px-4 py-2 sm:grid-cols-[1.25rem_minmax(0,1fr)_auto] sm:gap-y-0 sm:px-5">
      <span class="grid h-5 w-5 shrink-0 place-items-center text-primary" aria-hidden="true">
        <Megaphone class="h-4 w-4" />
      </span>

      <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-center sm:gap-3">
        <div class="flex min-w-0 items-center gap-2">
          <Badge v-if="announcement.level === 'important'">重要</Badge>
          <Badge class="shrink-0 border border-primary/15 bg-primary/10 text-primary hover:bg-primary/10">
            {{ announcementCategoryLabels[announcement.category] }}
          </Badge>
          <strong class="min-w-0 line-clamp-1 text-sm font-semibold leading-5">
            {{ announcement.title }}
          </strong>
        </div>
        <p class="line-clamp-1 min-w-0 text-xs leading-5 text-muted-foreground">
          {{ announcement.summary }}
        </p>
      </div>

      <div class="col-start-2 flex shrink-0 items-center gap-1 sm:col-start-auto sm:justify-end">
        <Button as-child size="sm" variant="ghost" class="px-2.5 text-primary hover:text-primary">
          <RouterLink :to="detailTo">
            查看详情
            <ArrowRight class="h-4 w-4" aria-hidden="true" />
          </RouterLink>
        </Button>
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
