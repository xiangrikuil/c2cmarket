<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Search } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import PageTitle from '@/components/market/PageTitle.vue'
import { useSearchMarket } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const keyword = computed(() => String(route.query.q ?? '').trim())
const draft = ref(keyword.value)
const { data, isFetching } = useSearchMarket(keyword)

watch(keyword, value => {
  draft.value = value
})

const groupedResults = computed(() => {
  const groups = new Map<string, NonNullable<typeof data.value>>()
  for (const item of data.value ?? []) {
    groups.set(item.type, [...(groups.get(item.type) ?? []), item])
  }
  return Array.from(groups.entries()).map(([type, rows]) => ({ type, rows }))
})

function runSearch() {
  const q = draft.value.trim()
  router.push(q ? { path: '/search', query: { q } } : { path: '/search' })
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="全局搜索" description="聚合官网公开价格、车源、求车、API 服务和公开用户；真实模式下只展示当前公开可见的业务结果。" />

    <Card class="p-5">
      <div class="flex gap-2">
        <Input v-model="draft" placeholder="搜索产品、地区、车主、商户或求车关键词" @keyup.enter="runSearch" />
        <Button @click="runSearch"><Search class="h-4 w-4" />搜索</Button>
      </div>
    </Card>

    <div v-if="!keyword" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">输入关键词后展示聚合结果。</div>
    <div v-else-if="isFetching" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在搜索「{{ keyword }}」...</div>
    <div v-else-if="(data ?? []).length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">没有找到与「{{ keyword }}」相关的结果。</div>

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
