<script setup lang="ts">
import { computed, ref } from 'vue'
import { ArrowRight, CarFront, Code2, FileSearch, Plus } from 'lucide-vue-next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import type { PublishedContentItem, PublishedContentKind } from '@/lib/personalCenterDashboard'

const props = defineProps<{
  items: PublishedContentItem[]
  loading: boolean
  hasError: boolean
  unavailable: boolean
}>()

defineEmits<{ retry: [] }>()

const activeKind = ref<PublishedContentKind>('all')
const tabs: Array<{ value: PublishedContentKind, label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'carpool', label: '车源' },
  { value: 'api-service', label: 'API 服务' },
  { value: 'demand', label: '求车需求' },
]
const counts = computed(() => ({
  all: props.items.length,
  carpool: props.items.filter(item => item.kind === 'carpool').length,
  'api-service': props.items.filter(item => item.kind === 'api-service').length,
  demand: props.items.filter(item => item.kind === 'demand').length,
}))
const filteredItems = computed(() => props.items
  .filter(item => activeKind.value === 'all' || item.kind === activeKind.value)
  .slice(0, 5))
const emptyAction = computed(() => {
  if (activeKind.value === 'carpool') return { label: '发布车源', to: '/carpools/new' }
  if (activeKind.value === 'api-service') return { label: '发布 API 服务', to: '/api-market/new' }
  if (activeKind.value === 'demand') return { label: '发布求车需求', to: '/demands/new' }
  return { label: '发布车源', to: '/carpools/new' }
})

function kindIcon(kind: PublishedContentItem['kind']) {
  if (kind === 'carpool') return CarFront
  if (kind === 'api-service') return Code2
  return FileSearch
}
</script>

<template>
  <Card class="gap-0 border-border p-4 shadow-sm sm:p-5">
    <header class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="font-semibold">我发布的内容</h2>
        <p class="mt-1 text-xs text-muted-foreground">最近更新的车源、API 服务与求车需求</p>
      </div>
      <div class="flex flex-wrap gap-2 text-xs">
        <RouterLink to="/my/carpools" class="text-primary hover:underline">全部车源</RouterLink>
        <RouterLink to="/my/api-services" class="text-primary hover:underline">全部服务</RouterLink>
        <RouterLink to="/my/demands" class="text-primary hover:underline">全部求车</RouterLink>
      </div>
    </header>

    <Tabs v-model="activeKind" class="mt-4">
      <TabsList class="h-auto max-w-full flex-wrap justify-start">
        <TabsTrigger v-for="tab in tabs" :key="tab.value" :value="tab.value" class="flex-none px-3">
          {{ tab.label }} <span class="text-xs text-muted-foreground">{{ counts[tab.value] }}</span>
        </TabsTrigger>
      </TabsList>
    </Tabs>

    <Alert v-if="hasError && !unavailable" class="mt-3 border-warning/30 bg-warning/5 text-foreground">
      <AlertTitle>部分发布内容暂时无法加载</AlertTitle>
      <AlertDescription class="flex flex-wrap items-center justify-between gap-2">
        <span>当前列表只包含已成功读取的分类。</span>
        <Button size="sm" variant="outline" @click="$emit('retry')">重新加载</Button>
      </AlertDescription>
    </Alert>

    <ErrorState
      v-if="unavailable"
      class="mt-4 min-h-[132px]"
      title="发布内容暂时无法加载"
      description="当前无法确认已发布内容，请稍后重试。"
      @retry="$emit('retry')"
    />

    <div v-else-if="loading && items.length === 0" class="mt-4 space-y-2" aria-busy="true" aria-label="正在加载发布内容">
      <div v-for="index in 3" :key="index" class="h-[86px] animate-pulse rounded-lg bg-muted" />
    </div>

    <div v-else-if="filteredItems.length" class="mt-3 divide-y divide-border">
      <article v-for="item in filteredItems" :key="item.key" class="grid gap-3 py-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
        <div class="flex min-w-0 items-start gap-3">
          <span class="grid h-9 w-9 shrink-0 place-items-center rounded-lg border border-border bg-muted/35 text-muted-foreground">
            <component :is="kindIcon(item.kind)" class="h-4 w-4" />
          </span>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-xs text-muted-foreground">{{ item.kindLabel }}</span>
              <Badge :variant="item.active ? 'verified' : 'secondary'">{{ item.status }}</Badge>
            </div>
            <h3 class="mt-1 truncate text-sm font-medium">{{ item.title }}</h3>
            <p class="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{{ item.summary }}</p>
            <p class="mt-1 text-xs text-muted-foreground"><LocalTime :value="item.updatedAt" /> 更新</p>
          </div>
        </div>
        <Button as-child size="sm" variant="outline" class="w-full md:w-auto">
          <RouterLink :to="item.manageTo" :aria-label="`管理 ${item.title}`">管理<ArrowRight class="h-4 w-4" /></RouterLink>
        </Button>
      </article>
    </div>

    <div v-else class="mt-4 grid min-h-[132px] place-items-center rounded-lg border border-dashed border-border bg-muted/20 px-4 py-5 text-center">
      <div>
        <FileSearch class="mx-auto h-6 w-6 text-muted-foreground" />
        <h3 class="mt-2 text-sm font-semibold">当前分类暂时没有已发布内容</h3>
        <p class="mt-1 text-xs text-muted-foreground">发布后可以在这里查看状态并进入管理。</p>
        <Button as-child size="sm" class="mt-3"><RouterLink :to="emptyAction.to"><Plus class="h-4 w-4" />{{ emptyAction.label }}</RouterLink></Button>
      </div>
    </div>
  </Card>
</template>
