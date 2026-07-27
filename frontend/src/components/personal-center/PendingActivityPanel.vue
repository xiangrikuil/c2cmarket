<script setup lang="ts">
import { ArrowRight, Inbox, RefreshCw } from 'lucide-vue-next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import type { PersonalCenterTask } from '@/lib/personalCenterDashboard'

defineProps<{
  tasks: PersonalCenterTask[]
  loading: boolean
  hasError: boolean
  unavailable: boolean
  hasPublishedContent: boolean
}>()

defineEmits<{ retry: [] }>()
</script>

<template>
  <Card class="gap-0 border-border p-4 shadow-sm sm:p-5">
    <header class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="font-semibold">待办与最近动态</h2>
        <p class="mt-1 text-xs text-muted-foreground">优先展示当前需要你处理的交易状态</p>
      </div>
      <Button as-child size="sm" variant="ghost">
        <RouterLink to="/my/notifications">查看全部<ArrowRight class="h-4 w-4" /></RouterLink>
      </Button>
    </header>

    <Alert v-if="hasError && !unavailable" class="mt-4 border-warning/30 bg-warning/5 text-foreground">
      <RefreshCw />
      <AlertTitle>部分待办暂时无法加载</AlertTitle>
      <AlertDescription class="flex flex-wrap items-center justify-between gap-2">
        <span>已成功加载的事项仍可处理。</span>
        <Button size="sm" variant="outline" @click="$emit('retry')">重新加载</Button>
      </AlertDescription>
    </Alert>

    <ErrorState
      v-if="unavailable"
      class="mt-4 min-h-[132px]"
      title="待办暂时无法加载"
      description="当前无法确认是否存在需要处理的交易，请稍后重试。"
      @retry="$emit('retry')"
    />

    <div v-else-if="loading && tasks.length === 0" class="mt-4 space-y-2" aria-busy="true" aria-label="正在加载待办">
      <div v-for="index in 3" :key="index" class="h-[68px] animate-pulse rounded-lg bg-muted" />
    </div>

    <div v-else-if="tasks.length" class="mt-3 divide-y divide-border">
      <RouterLink
        v-for="task in tasks.slice(0, 6)"
        :key="task.key"
        :to="task.to"
        class="group grid min-h-[72px] grid-cols-[minmax(0,1fr)_auto] items-center gap-3 py-3 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      >
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">{{ task.typeLabel }}</Badge>
            <span class="text-xs text-muted-foreground">{{ task.status }}</span>
          </div>
          <div class="mt-1 truncate text-sm font-medium">{{ task.title }}</div>
          <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
            <span class="font-medium text-primary">{{ task.nextAction }}</span>
            <span class="text-muted-foreground"><LocalTime :value="task.updatedAt" /></span>
          </div>
        </div>
        <ArrowRight class="h-4 w-4 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
      </RouterLink>
    </div>

    <div v-else class="mt-4 grid min-h-[132px] place-items-center rounded-lg border border-dashed border-border bg-muted/20 px-4 py-5 text-center">
      <div class="max-w-md">
        <Inbox class="mx-auto h-6 w-6 text-muted-foreground" />
        <h3 class="mt-2 text-sm font-semibold">当前没有需要处理的交易</h3>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">
          {{ hasPublishedContent ? '新的申请、付款确认和订单状态会显示在这里。' : '发布车源或 API 服务后，相关申请和订单动态会显示在这里。' }}
        </p>
        <div class="mt-3 flex flex-wrap justify-center gap-2">
          <Button v-if="hasPublishedContent" as-child size="sm" variant="outline"><RouterLink to="/carpools">去市场看看</RouterLink></Button>
          <template v-else>
            <Button as-child size="sm"><RouterLink to="/carpools/new">发布车源</RouterLink></Button>
            <Button as-child size="sm" variant="outline"><RouterLink to="/api-market/new">发布 API 服务</RouterLink></Button>
          </template>
        </div>
      </div>
    </div>
  </Card>
</template>
