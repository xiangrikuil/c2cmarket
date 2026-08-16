<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import ArrowRight from 'lucide-vue-next/dist/esm/icons/arrow-right.js'
import Siren from 'lucide-vue-next/dist/esm/icons/siren.js'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { formatAnnouncementDateTime } from '@/lib/announcementUtils'
import type { AnnouncementDelivery } from '@/types/announcement'

const props = defineProps<{
  announcement: AnnouncementDelivery
  acknowledging?: boolean
  error?: string
}>()

const emit = defineEmits<{
  acknowledge: [announcementId: string]
}>()

const detailTo = computed(() => `/announcements/${props.announcement.slug}`)
</script>

<template>
  <Dialog :open="true">
    <DialogContent
      :show-close-button="false"
      class="max-h-[calc(100dvh-2rem)] overflow-y-auto border-destructive/35 sm:max-w-xl"
      @escape-key-down="$event.preventDefault()"
      @pointer-down-outside="$event.preventDefault()"
      @interact-outside="$event.preventDefault()"
    >
      <DialogHeader class="gap-3 text-left">
        <div class="flex items-center gap-2">
          <span class="grid h-9 w-9 place-items-center rounded-md bg-destructive/10 text-destructive">
            <Siren class="h-5 w-5" aria-hidden="true" />
          </span>
          <Badge variant="destructive">紧急通知</Badge>
        </div>
        <DialogTitle class="text-xl leading-7">{{ announcement.title }}</DialogTitle>
        <DialogDescription class="text-sm leading-6 text-foreground/75">
          {{ announcement.summary }}
        </DialogDescription>
      </DialogHeader>

      <dl class="grid gap-2 border-y border-border py-3 text-xs text-muted-foreground sm:grid-cols-2">
        <div><dt class="inline">发布时间：</dt><dd class="inline">{{ formatAnnouncementDateTime(announcement.publishAt) }}</dd></div>
        <div><dt class="inline">内容更新：</dt><dd class="inline">{{ formatAnnouncementDateTime(announcement.contentUpdatedAt) }}</dd></div>
      </dl>

      <Alert v-if="error" variant="destructive">
        <AlertTitle>确认失败</AlertTitle>
        <AlertDescription>{{ error }}</AlertDescription>
      </Alert>

      <DialogFooter class="gap-2 sm:justify-between">
        <Button as-child variant="outline">
          <RouterLink :to="detailTo">
            查看完整内容
            <ArrowRight class="h-4 w-4" aria-hidden="true" />
          </RouterLink>
        </Button>
        <Button :disabled="acknowledging" @click="emit('acknowledge', announcement.id)">
          {{ acknowledging ? '正在确认' : '我已知悉' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
