<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Search } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import PageTitle from '@/components/market/PageTitle.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import { trackAnalytics } from '@/lib/analytics'
import { useSearchMarket } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const keyword = computed(() => String(route.query.q ?? '').trim())
const draft = ref(keyword.value)
const pendingSearchKeyword = ref<string | null>(null)
const { data, isFetching } = useSearchMarket(keyword)

watch(keyword, value => {
  draft.value = value
})

watch([data, isFetching], () => {
  flushPendingSearchTrack()
})

const groupedResults = computed(() => {
  const groups = new Map<string, NonNullable<typeof data.value>>()
  for (const item of data.value ?? []) {
    groups.set(item.type, [...(groups.get(item.type) ?? []), item])
  }
  return Array.from(groups.entries()).map(([type, rows]) => ({ type, rows }))
})

function flushPendingSearchTrack() {
  if (pendingSearchKeyword.value === null || isFetching.value || keyword.value !== pendingSearchKeyword.value) return
  trackAnalytics('search_submit', {
    source_route: analyticsSourceRoute(),
    has_query: Boolean(keyword.value),
    result_count: data.value?.length ?? 0,
    filters_count: 0,
  })
  pendingSearchKeyword.value = null
}

function runSearch() {
  const q = draft.value.trim()
  pendingSearchKeyword.value = q
  void router.push(q ? { path: '/search', query: { q } } : { path: '/search' }).finally(() => {
    flushPendingSearchTrack()
  })
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="全局搜索" description="聚合官网公开价格、车源、求车、API 服务和公开用户；只展示当前公开可见的业务结果。" />

    <Card class="p-5">
      <div class="flex gap-2">
        <Input v-model="draft" placeholder="搜索产品、地区、车主、商户或求车关键词" @keyup.enter="runSearch" />
        <Button @click="runSearch"><Search class="h-4 w-4" />搜索</Button>
      </div>
    </Card>

    <EmptyState v-if="!keyword" title="搜索整个平台" description="可以搜索产品、地区、车主、商户、求车需求或公开用户。">
      <template #action><div class="flex flex-wrap justify-center gap-2"><RouterLink to="/carpools"><Button variant="outline">热门拼车</Button></RouterLink><RouterLink to="/api-market"><Button variant="outline">API 市场</Button></RouterLink></div></template>
    </EmptyState>
    <SkeletonTable v-else-if="isFetching" :rows="4" :columns="3" />
    <EmptyState v-else-if="(data ?? []).length === 0" title="没有匹配结果" :description="`没有找到与「${keyword}」相关的公开对象，可以换一个关键词或直接浏览市场。`">
      <template #action><div class="flex flex-wrap justify-center gap-2"><RouterLink to="/carpools"><Button variant="outline">查看拼车</Button></RouterLink><RouterLink to="/api-market"><Button variant="outline">浏览 API 服务</Button></RouterLink></div></template>
    </EmptyState>

    <div v-else class="space-y-5">
      <Card v-for="group in groupedResults" :key="group.type" class="p-0">
        <div class="flex items-center justify-between border-b border-border px-5 py-4">
          <div>
            <h2 class="font-semibold">{{ group.type }}</h2>
            <p v-if="group.type === '商户'" class="mt-1 text-xs text-muted-foreground">商户结果来自公开商户资料；店铺别名服务不会反查真实用户。</p>
            <p v-else-if="group.type === '用户'" class="mt-1 text-xs text-muted-foreground">用户结果来自公开个人主页，不代表该用户公开了 API 商户身份。</p>
            <p v-else-if="group.type === '官方价格'" class="mt-1 text-xs text-muted-foreground">价格结果按产品、地区、渠道或提交人命中。</p>
          </div>
          <Badge variant="secondary">{{ group.rows.length }}</Badge>
        </div>
        <div class="divide-y divide-border">
          <RouterLink
            v-for="item in group.rows"
            :key="item.id"
            :to="item.to"
            class="block p-5 transition hover:bg-accent"
          >
            <div class="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
              <div>
                <div class="font-medium">{{ item.title }}</div>
                <div class="mt-1 text-sm text-muted-foreground">{{ item.subtitle }}</div>
              </div>
              <Badge>{{ item.badge }}</Badge>
            </div>
          </RouterLink>
        </div>
      </Card>
    </div>
  </div>
</template>
