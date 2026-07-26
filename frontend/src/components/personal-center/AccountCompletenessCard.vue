<script setup lang="ts">
import { Check, Circle, RefreshCw } from 'lucide-vue-next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import type { AccountCompleteness } from '@/lib/personalCenterDashboard'

defineProps<{
  completeness: AccountCompleteness | null
  loading: boolean
  hasError: boolean
}>()

defineEmits<{ retry: [] }>()
</script>

<template>
  <Card class="gap-0 border-border p-4 shadow-sm">
    <div class="flex items-center justify-between gap-3">
      <h2 class="font-semibold">账户完整度</h2>
      <strong v-if="completeness && !loading && !hasError" class="text-primary">{{ completeness.percentage }}%</strong>
      <span v-else class="h-5 w-10 animate-pulse rounded bg-muted" aria-label="正在加载" />
    </div>

    <template v-if="completeness && !loading && !hasError">
      <div
        class="mt-3 h-2 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-label="账户完整度"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-valuenow="completeness.percentage"
      >
        <div class="h-full rounded-full bg-primary transition-[width]" :style="{ width: `${completeness.percentage}%` }" />
      </div>

      <div class="mt-4 space-y-1">
        <RouterLink
          v-for="task in completeness.tasks"
          :key="task.id"
          :to="task.to"
          class="group flex min-h-10 items-start gap-2 rounded-md px-1.5 py-2 outline-none hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring"
        >
          <span
            class="mt-0.5 grid h-4 w-4 shrink-0 place-items-center rounded-full"
            :class="task.completed ? 'bg-success text-white' : 'text-muted-foreground'"
          >
            <Check v-if="task.completed" class="h-3 w-3" />
            <Circle v-else class="h-4 w-4" />
          </span>
          <span class="min-w-0">
            <strong class="block text-xs font-medium">{{ task.label }}</strong>
            <small class="mt-0.5 block text-xs leading-4 text-muted-foreground">{{ task.description }}</small>
          </span>
        </RouterLink>
      </div>

      <Button v-if="completeness.missingCount" as-child class="mt-3 w-full" variant="outline">
        <RouterLink :to="completeness.nextTo">继续完善</RouterLink>
      </Button>
      <p v-else class="mt-3 text-xs leading-5 text-muted-foreground">当前账户资料与必要恢复方式已完成。</p>
    </template>

    <div v-else-if="loading" class="mt-4 space-y-2" aria-busy="true" aria-label="正在加载账户完整度">
      <div class="h-2 animate-pulse rounded bg-muted" />
      <div v-for="index in 4" :key="index" class="h-10 animate-pulse rounded bg-muted" />
    </div>

    <Alert v-else class="mt-4 border-warning/30 bg-warning/5 text-foreground">
      <RefreshCw />
      <AlertTitle>账户完整度暂不可用</AlertTitle>
      <AlertDescription>
        <p>部分账户状态读取失败，暂不显示可能不准确的百分比。</p>
        <Button size="sm" variant="outline" class="mt-2" @click="$emit('retry')">重新加载</Button>
      </AlertDescription>
    </Alert>
  </Card>
</template>
