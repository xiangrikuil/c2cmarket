<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowRight, ExternalLink, FileSearch, ListChecks, MessageSquareText, SearchCheck } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { useDemand } from '@/queries/useMarketQueries'
import { markMissingQueryAsNotFoundOnServer, prefetchQueriesOnServer } from '@/queries/prefetchQueriesOnServer'
import { useEntitySeo } from '@/composables/useEntitySeo'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const demandQuery = useDemand(id)
const { data: demand, isLoading } = demandQuery
prefetchQueriesOnServer(demandQuery)
markMissingQueryAsNotFoundOnServer(demandQuery, () => Boolean(demand.value))
const ownerPreferenceLabel = computed(() => {
  if (!demand.value) return '—'
  if (demand.value.ownerPreference === 'only-personal' || demand.value.ownerPreference === 'only_personal') return '只看个人车主'
  if (demand.value.ownerPreference === 'any') return '不限车主类型'
  return '个人车主优先'
})
useEntitySeo({
  indexable: computed(() => Boolean(demand.value)),
  title: computed(() => demand.value ? `${demand.value.title}｜求车需求｜C2CMarket` : '求车需求详情｜C2CMarket'),
  description: computed(() => demand.value ? `${demand.value.region}求车需求，预算上限 ¥${demand.value.maxPrice}/月，${ownerPreferenceLabel.value}。` : '查看公开求车需求详情。'),
  schema: computed(() => demand.value ? {
    '@type': 'Demand',
    name: demand.value.title,
    description: demand.value.note || demand.value.require,
    areaServed: demand.value.region,
  } : null),
})
</script>

<template>
  <SkeletonBlock v-if="isLoading" :lines="7" />
  <EmptyState v-else-if="!demand" title="未找到求车需求" description="该需求不存在、已下架或链接有误。"><template #action><RouterLink to="/demands"><Button variant="outline">返回求车大厅</Button></RouterLink></template></EmptyState>
  <div v-else class="demand-detail-page space-y-5">
    <div class="demand-detail-heading rounded-xl border px-5 py-4">
      <div class="flex items-start gap-4">
        <span class="demand-detail-title-icon"><FileSearch class="h-5 w-5" /></span>
        <PageTitle :title="demand.title" description="公开需求只展示匹配所需信息；联系方式和后续确认仍在 linux.do 原帖或双方认可的站外渠道进行。" />
      </div>
    </div>
    <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-start">
      <div class="space-y-5">
        <Card class="demand-detail-summary p-6">
          <div class="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
            <div><div class="text-sm text-muted-foreground">预算上限</div><div class="mt-2 text-4xl font-semibold">¥{{ demand.maxPrice }}<span class="text-base font-normal text-muted-foreground"> / 月</span></div></div>
            <Badge variant="secondary">{{ demand.status }}</Badge>
          </div>
          <dl class="mt-7 grid gap-4 text-sm sm:grid-cols-2">
            <div class="border-t border-border pt-3"><dt class="text-muted-foreground">地区偏好</dt><dd class="mt-1 font-medium">{{ demand.region }}</dd></div>
            <div class="border-t border-border pt-3"><dt class="text-muted-foreground">车主偏好</dt><dd class="mt-1 font-medium">{{ ownerPreferenceLabel }}</dd></div>
            <div class="border-t border-border pt-3"><dt class="text-muted-foreground">发布者</dt><dd class="mt-1 font-medium">{{ demand.poster }} · 信任等级 {{ demand.trustLevel }}</dd></div>
            <div class="border-t border-border pt-3"><dt class="text-muted-foreground">最近更新</dt><dd class="mt-1 font-medium">{{ demand.updatedAt }}</dd></div>
          </dl>
        </Card>
        <Card class="p-6"><h2 class="font-semibold">需求说明</h2><p class="mt-3 text-sm leading-7">{{ demand.note || demand.require || '发布者暂未补充更多说明。' }}</p><a class="mt-5 inline-flex items-center gap-2 text-sm text-primary hover:underline" :href="demand.sourceUrl" target="_blank" rel="noreferrer">查看 linux.do 原帖 <ExternalLink class="h-4 w-4" /></a></Card>
      </div>
      <Card class="demand-detail-action p-5 lg:sticky lg:top-16">
        <h2 class="font-semibold">回应这条需求</h2>
        <p class="mt-2 text-sm leading-6 text-muted-foreground">选择你已经发布的车源，在原有车源和申请流程中继续沟通，不创建另一套孤立的撮合记录。</p>
        <ol class="demand-detail-response-steps mt-4">
          <li><span><SearchCheck class="h-4 w-4" /></span><div><strong>选择现有车源</strong><small>套餐、地区和席位应匹配</small></div></li>
          <li><span><MessageSquareText class="h-4 w-4" /></span><div><strong>提交回应</strong><small>对方查看车源后决定申请</small></div></li>
          <li><span><ListChecks class="h-4 w-4" /></span><div><strong>进入上车流程</strong><small>席位和状态仍以车源为准</small></div></li>
        </ol>
        <RouterLink :to="{ path: '/my/carpools', query: { respondTo: demand.id } }"><Button class="mt-5 w-full" :disabled="demand.status !== '匹配中'">使用我的车源回应 <ArrowRight class="h-4 w-4" /></Button></RouterLink>
        <RouterLink :to="{ path: '/carpools/new', query: { demand: demand.id } }"><Button class="mt-3 w-full" variant="outline">发布新车源</Button></RouterLink>
        <p v-if="demand.status !== '匹配中'" class="mt-3 text-xs text-muted-foreground">当前需求为“{{ demand.status }}”，暂不接受新回应。</p>
      </Card>
    </div>
  </div>
</template>
