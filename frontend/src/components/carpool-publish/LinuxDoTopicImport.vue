<script setup lang="ts">
import { computed } from 'vue'
import { Check, RefreshCw } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { ParsedLinuxDoTopic } from './types'
import { formatConfidence } from './utils'
import PublishSectionCard from './PublishSectionCard.vue'

const props = defineProps<{
  topicUrl: string
  parsedTopic: ParsedLinuxDoTopic | null
  parsePending: boolean
  error?: string
  embedded?: boolean
}>()

const emit = defineEmits<{
  'update:topicUrl': [value: string]
  parse: []
}>()

const parseButtonLabel = computed(() => {
  if (props.parsePending) return '正在读取...'
  return props.parsedTopic ? '重新读取' : '读取并回填'
})

const confidenceRows = computed(() => {
  if (!props.parsedTopic) return []
  return [
    ['产品', props.parsedTopic.confidence.product],
    ['地区', props.parsedTopic.confidence.region],
    ['价格', props.parsedTopic.confidence.monthlyPrice],
    ['名额', props.parsedTopic.confidence.seats],
  ] as const
})
</script>

<template>
  <div v-if="embedded">
    <label class="text-sm font-medium" for="linuxdo-topic-url">linux.do 原帖链接 <span class="text-xs text-muted-foreground">可选</span></label>
    <div class="mt-2 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
      <Input
        id="linuxdo-topic-url"
        :model-value="topicUrl"
        placeholder="https://linux.do/t/topic/123456"
        @update:model-value="value => emit('update:topicUrl', String(value))"
      />
      <Button class="sm:min-w-32" :disabled="parsePending" @click="emit('parse')">
        <RefreshCw :class="['h-4 w-4', parsePending ? 'animate-spin' : '']" />
        {{ parseButtonLabel }}
      </Button>
    </div>
    <p class="mt-2 text-xs text-muted-foreground">仅提取发布所需的结构化信息，不在本站复制帖子全文。没有原帖也可以跳过此步。</p>
    <p v-if="!topicUrl.trim()" class="mt-2 rounded-md border border-info/20 bg-info/10 px-3 py-2 text-xs leading-5 text-info">
      没有原帖？你可以先手动填写车源，发布后复制发帖文案到 linux.do。
    </p>
    <p v-if="error" class="mt-2 text-xs text-destructive">{{ error }}</p>

    <div v-if="parsedTopic" class="mt-3 grid gap-3 rounded-lg border border-success/25 bg-success/10 p-3 md:grid-cols-[auto_1fr] md:items-center">
      <span class="grid h-7 w-7 place-items-center rounded-full bg-success text-xs font-bold text-success-foreground"><Check class="h-4 w-4" /></span>
      <div class="min-w-0">
        <div class="text-sm font-semibold">原帖读取成功 · 作者 {{ parsedTopic.authorUsername }}</div>
        <div class="mt-1 text-xs leading-5 text-muted-foreground">
          已识别 {{ parsedTopic.detected.productText || '产品待确认' }}、{{ parsedTopic.detected.regionText || '地区待确认' }}、¥{{ parsedTopic.detected.monthlyPriceCny ?? '-' }}/月、总名额 {{ parsedTopic.detected.totalSeats ?? '-' }}、剩余名额 {{ parsedTopic.detected.availableSeats ?? '-' }}
        </div>
        <div class="mt-1 text-xs text-muted-foreground">最后更新 {{ parsedTopic.updatedAt }} · {{ parsedTopic.authorMatchesBoundUser ? '作者一致' : '作者不一致，将进入人工审核' }}</div>
        <details class="mt-2">
          <summary class="cursor-pointer text-xs font-medium text-primary">查看识别详情</summary>
          <div class="mt-2 flex flex-wrap gap-1.5">
            <Badge
              v-for="[label, confidence] in confidenceRows"
              :key="label"
              :variant="confidence === 'high' ? 'verified' : 'trust'"
            >
              {{ label }} {{ formatConfidence(confidence) }}
            </Badge>
          </div>
        </details>
      </div>
    </div>
  </div>
  <PublishSectionCard
    v-else
    :index="1"
    title="导入 linux.do 原帖（可选）"
    description="已有原帖可粘贴链接自动回填；没有原帖也可以手动填写并发布。"
  >
    <label class="text-sm font-medium" for="linuxdo-topic-url">linux.do 原帖链接 <span class="text-xs text-muted-foreground">可选</span></label>
    <div class="mt-2 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
      <Input
        id="linuxdo-topic-url"
        :model-value="topicUrl"
        placeholder="https://linux.do/t/topic/123456"
        @update:model-value="value => emit('update:topicUrl', String(value))"
      />
      <Button class="sm:min-w-32" :disabled="parsePending" @click="emit('parse')">
        <RefreshCw :class="['h-4 w-4', parsePending ? 'animate-spin' : '']" />
        {{ parseButtonLabel }}
      </Button>
    </div>
    <p class="mt-2 text-xs text-muted-foreground">仅提取发布所需的结构化信息，不在本站复制帖子全文。没有原帖也可以跳过此步。</p>
    <p v-if="!topicUrl.trim()" class="mt-2 rounded-md border border-info/20 bg-info/10 px-3 py-2 text-xs leading-5 text-info">
      没有原帖？你可以先手动填写车源，发布后复制发帖文案到 linux.do。
    </p>
    <p v-if="error" class="mt-2 text-xs text-destructive">{{ error }}</p>

    <div v-if="parsedTopic" class="mt-3 grid gap-3 rounded-lg border border-success/25 bg-success/10 p-3 md:grid-cols-[auto_1fr] md:items-center">
      <span class="grid h-7 w-7 place-items-center rounded-full bg-success text-xs font-bold text-success-foreground"><Check class="h-4 w-4" /></span>
      <div class="min-w-0">
        <div class="text-sm font-semibold">原帖读取成功 · 作者 {{ parsedTopic.authorUsername }}</div>
        <div class="mt-1 text-xs leading-5 text-muted-foreground">
          已识别 {{ parsedTopic.detected.productText || '产品待确认' }}、{{ parsedTopic.detected.regionText || '地区待确认' }}、¥{{ parsedTopic.detected.monthlyPriceCny ?? '-' }}/月、总名额 {{ parsedTopic.detected.totalSeats ?? '-' }}、剩余名额 {{ parsedTopic.detected.availableSeats ?? '-' }}
        </div>
        <div class="mt-1 text-xs text-muted-foreground">最后更新 {{ parsedTopic.updatedAt }} · {{ parsedTopic.authorMatchesBoundUser ? '作者一致' : '作者不一致，将进入人工审核' }}</div>
        <details class="mt-2">
          <summary class="cursor-pointer text-xs font-medium text-primary">查看识别详情</summary>
          <div class="mt-2 flex flex-wrap gap-1.5">
            <Badge
              v-for="[label, confidence] in confidenceRows"
              :key="label"
              :variant="confidence === 'high' ? 'verified' : 'trust'"
            >
              {{ label }} {{ formatConfidence(confidence) }}
            </Badge>
          </div>
        </details>
      </div>
    </div>
  </PublishSectionCard>
</template>
