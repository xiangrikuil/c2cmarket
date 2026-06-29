<script setup lang="ts">
import { Check, CircleAlert, CircleDashed } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import type { CompletenessItem } from './types'

defineProps<{
  percent: number
  items: CompletenessItem[]
}>()
</script>

<template>
  <Card class="p-4 shadow-sm">
    <div class="flex items-center justify-between gap-3">
      <h2 class="text-base font-semibold">提交完整度</h2>
      <span class="text-sm font-semibold text-primary">{{ percent }}%</span>
    </div>
    <div class="mt-3 h-2 overflow-hidden rounded-full bg-muted">
      <div class="h-full rounded-full bg-primary transition-all" :style="{ width: `${percent}%` }"></div>
    </div>
    <div class="mt-4 space-y-3">
      <div v-for="item in items" :key="item.label" class="flex items-start justify-between gap-3 text-sm">
        <div class="flex min-w-0 items-start gap-2">
          <span
            class="mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full"
            :class="item.status === 'done' ? 'bg-success/10 text-success' : item.status === 'warning' ? 'bg-amber-100 text-amber-700' : 'bg-muted text-muted-foreground'"
          >
            <Check v-if="item.status === 'done'" class="h-3 w-3" />
            <CircleAlert v-else-if="item.status === 'warning'" class="h-3 w-3" />
            <CircleDashed v-else class="h-3 w-3" />
          </span>
          <span>{{ item.label }}</span>
        </div>
        <span class="shrink-0 text-xs text-muted-foreground">{{ item.hint }}</span>
      </div>
    </div>
    <div class="mt-4 border-t border-border pt-3 text-xs leading-5 text-muted-foreground">
      平台边界：仅维护价格情报，不托管支付，不保存第三方账号密码、明文密码、API Key、token 或付款码。
    </div>
  </Card>
</template>
