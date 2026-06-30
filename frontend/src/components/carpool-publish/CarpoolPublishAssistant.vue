<script setup lang="ts">
import { computed } from 'vue'
import { ClipboardCopy, Eye, RefreshCw, Send } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import type { PublishDefaultItem, PublishTask, TrustItem } from './types'

const props = defineProps<{
  tasks: PublishTask[]
  defaultItems: PublishDefaultItem[]
  trustItems: TrustItem[]
  reminders: string[]
  remainingSeats: number
  totalSeats: number
  copyEnabled: boolean
  copyDisabledReason: string
  postText: string
  submitPending: boolean
}>()

const emit = defineEmits<{
  saveDraft: []
  submitReview: []
  copyPostText: []
  jumpToTask: [key: string]
}>()

const doneItems = computed(() => props.tasks.filter(item => item.complete))
const pendingItems = computed(() => props.tasks.filter(item => !item.complete))
const completenessPercent = computed(() => {
  if (!props.tasks.length) return 0
  return Math.round((doneItems.value.length / props.tasks.length) * 100)
})
const topPendingText = computed(() => {
  const labels = pendingItems.value.map(item => item.shortLabel).slice(0, 3)
  if (!labels.length) return '发布必填项已完成，可提交审核'
  return `还差：${labels.join('、')}`
})
</script>

<template>
  <aside class="space-y-4">
    <Card class="max-h-[calc(100dvh-var(--app-header-height)-32px)] overflow-y-auto p-5 shadow-sm">
      <div class="flex items-start justify-between gap-3">
        <div>
          <div class="text-xs text-muted-foreground">发布助手</div>
          <h2 class="mt-1 text-lg font-semibold">发布必填 {{ doneItems.length }} / {{ tasks.length }}</h2>
        </div>
        <Badge :variant="pendingItems.length ? 'secondary' : 'verified'">
          {{ pendingItems.length ? `${pendingItems.length} 项待补` : '可发布' }}
        </Badge>
      </div>

      <div class="mt-4 h-2 overflow-hidden rounded-full bg-muted">
        <div class="h-full rounded-full bg-primary" :style="{ width: `${completenessPercent}%` }" />
      </div>
      <p class="mt-2 text-xs leading-5 text-muted-foreground">{{ topPendingText }}</p>

      <div v-if="pendingItems.length" class="mt-5">
        <h3 class="text-sm font-semibold">待补项</h3>
        <div class="mt-2 space-y-2">
          <button
            v-for="(item, index) in pendingItems"
            :key="item.key"
            type="button"
            class="flex w-full items-center gap-2 rounded-lg border border-border bg-background px-3 py-2 text-left text-sm transition hover:border-primary/40 hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            @click="emit('jumpToTask', item.key)"
          >
            <span
              class="grid h-5 w-5 place-items-center rounded-full text-[11px] font-semibold"
              :class="item.error ? 'bg-destructive/10 text-destructive' : 'bg-warning/10 text-warning'"
            >
              {{ index + 1 }}
            </span>
            <span class="min-w-0 flex-1">
              <span class="block font-medium">{{ item.label }}</span>
              <span class="mt-0.5 block text-xs text-muted-foreground">{{ item.description }}</span>
            </span>
            <span class="text-muted-foreground">→</span>
          </button>
        </div>
      </div>

      <div class="mt-5">
        <h3 class="text-sm font-semibold">已完成</h3>
        <div class="mt-2 space-y-2">
          <div v-for="item in doneItems" :key="item.key" class="flex items-center gap-2 text-sm">
            <span class="grid h-5 w-5 place-items-center rounded-full bg-success/10 text-[11px] font-semibold text-success">✓</span>
            <span>{{ item.label }}</span>
          </div>
        </div>
      </div>

      <div v-if="defaultItems.length" class="mt-5">
        <h3 class="text-sm font-semibold">系统默认</h3>
        <div class="mt-2 space-y-2">
          <div v-for="item in defaultItems" :key="item.key" class="flex items-start gap-2 text-sm">
            <span class="mt-0.5 grid h-5 w-5 place-items-center rounded-full bg-success/10 text-[11px] font-semibold text-success">✓</span>
            <span class="min-w-0">
              <span class="block">{{ item.label }}</span>
              <span class="mt-0.5 block text-xs leading-5 text-muted-foreground">{{ item.description }}</span>
            </span>
          </div>
        </div>
      </div>

      <div class="mt-5 rounded-lg border border-border bg-muted/35 p-3">
        <div class="flex items-center justify-between gap-3">
          <span class="text-sm font-semibold">剩余名额</span>
          <span class="text-lg font-bold text-primary">{{ remainingSeats }} / {{ totalSeats }}</span>
        </div>
        <p class="mt-1 text-xs text-muted-foreground">{{ remainingSeats > 0 ? '发布后前台显示可申请。' : '剩余为 0 时前台会显示已满。' }}</p>
      </div>

      <div class="mt-5">
        <h3 class="text-sm font-semibold">增信项</h3>
        <div class="mt-2 space-y-2">
          <div v-for="item in trustItems" :key="item.label" class="flex items-start gap-2 text-sm">
            <span
              class="mt-0.5 grid h-5 w-5 place-items-center rounded-full text-[11px] font-semibold"
              :class="item.status === 'done' ? 'bg-success/10 text-success' : 'bg-muted text-muted-foreground'"
            >
              {{ item.status === 'done' ? '✓' : '·' }}
            </span>
            <span class="min-w-0">
              <span class="block">{{ item.label }}</span>
              <span v-if="item.description" class="mt-0.5 block text-xs leading-5 text-muted-foreground">{{ item.description }}</span>
            </span>
          </div>
        </div>
      </div>

      <div class="mt-5 rounded-lg border border-primary/15 bg-primary/5 p-3">
        <div class="text-sm font-semibold">提效工具</div>
        <button
          type="button"
          class="mt-3 flex w-full items-center justify-between gap-3 rounded-md border border-border bg-background px-3 py-2 text-left text-sm transition hover:bg-muted"
          @click="emit('jumpToTask', 'linuxDoImport')"
        >
          <span class="min-w-0">
            <span class="block font-medium">导入 linux.do 原帖</span>
            <span class="mt-0.5 block text-xs text-muted-foreground">可选，不影响手动发布。</span>
          </span>
          <RefreshCw class="h-4 w-4 text-muted-foreground" />
        </button>
        <div class="mt-4 text-sm font-semibold">linux.do 发帖文案</div>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">
          {{ copyEnabled ? '可复制当前表单内容到 linux.do 发帖。' : copyDisabledReason }}
        </p>
        <div class="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
          <Dialog>
            <DialogTrigger as-child>
              <Button variant="outline" size="sm" :disabled="!copyEnabled">
                <Eye class="h-4 w-4" />预览文案
              </Button>
            </DialogTrigger>
            <DialogContent class="sm:max-w-2xl">
              <DialogHeader>
                <DialogTitle>linux.do 发帖文案</DialogTitle>
                <DialogDescription>复制前确认文案中没有账号、密码、token、Cookie、API Key 或付款凭据。</DialogDescription>
              </DialogHeader>
              <pre class="max-h-[60vh] overflow-auto whitespace-pre-wrap rounded-md border bg-muted/40 p-4 text-sm leading-6">{{ postText }}</pre>
            </DialogContent>
          </Dialog>
          <Button size="sm" :disabled="!copyEnabled" @click="emit('copyPostText')">
            <ClipboardCopy class="h-4 w-4" />复制文案
          </Button>
        </div>
      </div>

      <div class="mt-5 grid gap-2 xl:grid-cols-2">
        <Button variant="outline" @click="emit('saveDraft')">保存草稿</Button>
        <Button :disabled="submitPending" @click="emit('submitReview')">
          <Send class="h-4 w-4" />{{ submitPending ? '发布中' : '检查并发布' }}
        </Button>
      </div>
    </Card>

    <div v-if="reminders.length" class="space-y-2">
      <div v-for="reminder in reminders" :key="reminder" class="rounded-lg border border-warning/25 bg-warning/10 px-3 py-2 text-xs leading-5 text-warning">
        {{ reminder }}
      </div>
    </div>
  </aside>
</template>
