<script setup lang="ts">
import {
  ArrowRight,
  CarFront,
  Code2,
  Heart,
  MessageCircle,
  Scale,
  ShoppingBag,
  TriangleAlert,
} from 'lucide-vue-next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import AccountCompletenessCard from '@/components/personal-center/AccountCompletenessCard.vue'
import PendingActivityPanel from '@/components/personal-center/PendingActivityPanel.vue'
import ProfileOverviewCard from '@/components/personal-center/ProfileOverviewCard.vue'
import PublishedContentSection from '@/components/personal-center/PublishedContentSection.vue'
import type {
  AccountAlert,
  AccountCompleteness,
  PersonalCenterMetric,
  PersonalCenterTask,
  PublishedContentItem,
} from '@/lib/personalCenterDashboard'
import type { UserProfile } from '@/lib/api'

const props = defineProps<{
  profile: UserProfile
  metrics: PersonalCenterMetric[]
  pendingTasks: PersonalCenterTask[]
  publishedItems: PublishedContentItem[]
  completeness: AccountCompleteness | null
  accountAlert: AccountAlert | null
  tasksLoading: boolean
  tasksError: boolean
  tasksUnavailable: boolean
  publishedLoading: boolean
  publishedError: boolean
  publishedUnavailable: boolean
  showFirstTransactionGuide: boolean
  completenessLoading: boolean
  completenessError: boolean
  enabledContactCount: number
  contactsLoading: boolean
  contactsError: boolean
  buyerRideCount: number | null
  relatedApiOrderCount: number | null
}>()

defineEmits<{
  retryTasks: []
  retryPublished: []
  retryCompleteness: []
}>()

function contactsSummary() {
  if (props.contactsLoading) return '正在读取'
  if (props.contactsError) return '暂不可用'
  return props.enabledContactCount > 0 ? `${props.enabledContactCount} 种方式可用` : '未配置'
}
</script>

<template>
  <div class="mx-auto w-full max-w-[1440px] space-y-4">
    <ProfileOverviewCard :profile="profile" :metrics="metrics" />

    <Alert v-if="showFirstTransactionGuide" class="border-primary/25 bg-primary/5 text-foreground">
      <ShoppingBag />
      <AlertTitle>开始第一笔交易</AlertTitle>
      <AlertDescription>
        <p>先看看当前可交易的拼车和 API 服务，找到合适内容后再发起申请或创建订单。</p>
        <div class="mt-3 flex flex-wrap gap-2">
          <Button as-child size="sm">
            <RouterLink to="/carpools"><CarFront class="h-4 w-4" />浏览拼车</RouterLink>
          </Button>
          <Button as-child size="sm" variant="outline">
            <RouterLink to="/api-market"><Code2 class="h-4 w-4" />浏览 API 服务</RouterLink>
          </Button>
        </div>
      </AlertDescription>
    </Alert>

    <div class="grid min-w-0 gap-4 min-[1100px]:grid-cols-[minmax(0,1fr)_320px] min-[1100px]:items-start">
      <main class="min-w-0 space-y-4">
        <PendingActivityPanel
          :tasks="pendingTasks"
          :loading="tasksLoading"
          :has-error="tasksError"
          :unavailable="tasksUnavailable"
          :has-published-content="publishedItems.length > 0"
          @retry="$emit('retryTasks')"
        />

        <section class="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
          <div class="min-w-0">
            <div class="mb-3">
              <h2 class="font-semibold">快速发布</h2>
              <p class="mt-1 text-xs text-muted-foreground">创建新的交易内容</p>
            </div>
            <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-1">
              <Button as-child size="lg" class="h-auto min-h-12 justify-start px-4">
                <RouterLink to="/carpools/new"><CarFront class="h-4 w-4" />发布车源</RouterLink>
              </Button>
              <Button as-child size="lg" variant="outline" class="h-auto min-h-12 justify-start px-4">
                <RouterLink to="/api-market/new"><Code2 class="h-4 w-4" />发布 API 服务</RouterLink>
              </Button>
            </div>
          </div>

          <Card class="gap-0 border-border p-4 shadow-sm">
            <div>
              <h2 class="font-semibold">常用管理</h2>
              <p class="mt-1 text-xs text-muted-foreground">进入交易记录和账户设置</p>
            </div>
            <div class="mt-3 divide-y divide-border">
              <RouterLink to="/my/rides" class="group flex min-h-12 items-center gap-3 py-2 outline-none focus-visible:ring-2 focus-visible:ring-ring">
                <CarFront class="h-4 w-4 text-primary" />
                <span class="min-w-0 flex-1"><strong class="block text-sm">我的上车</strong><small class="text-xs text-muted-foreground">{{ buyerRideCount === null ? '暂不可用' : `${buyerRideCount} 条申请记录` }}</small></span>
                <ArrowRight class="h-4 w-4 text-muted-foreground group-hover:text-foreground" />
              </RouterLink>
              <RouterLink to="/my/api-orders" class="group flex min-h-12 items-center gap-3 py-2 outline-none focus-visible:ring-2 focus-visible:ring-ring">
                <ShoppingBag class="h-4 w-4 text-primary" />
                <span class="min-w-0 flex-1"><strong class="block text-sm">API 订单</strong><small class="text-xs text-muted-foreground">{{ relatedApiOrderCount === null ? '暂不可用' : `${relatedApiOrderCount} 笔相关订单` }}</small></span>
                <ArrowRight class="h-4 w-4 text-muted-foreground group-hover:text-foreground" />
              </RouterLink>
              <RouterLink to="/my/favorites" class="group flex min-h-12 items-center gap-3 py-2 outline-none focus-visible:ring-2 focus-visible:ring-ring">
                <Heart class="h-4 w-4 text-rose-600" />
                <span class="min-w-0 flex-1"><strong class="block text-sm">我的收藏</strong><small class="text-xs text-muted-foreground">查看收藏的市场内容</small></span>
                <ArrowRight class="h-4 w-4 text-muted-foreground group-hover:text-foreground" />
              </RouterLink>
              <RouterLink to="/my/contacts" class="group flex min-h-12 items-center gap-3 py-2 outline-none focus-visible:ring-2 focus-visible:ring-ring">
                <MessageCircle class="h-4 w-4 text-amber-600" />
                <span class="min-w-0 flex-1"><strong class="block text-sm">联系方式</strong><small class="text-xs text-muted-foreground">{{ contactsSummary() }}</small></span>
                <ArrowRight class="h-4 w-4 text-muted-foreground group-hover:text-foreground" />
              </RouterLink>
            </div>
          </Card>
        </section>

        <PublishedContentSection
          :items="publishedItems"
          :loading="publishedLoading"
          :has-error="publishedError"
          :unavailable="publishedUnavailable"
          @retry="$emit('retryPublished')"
        />
      </main>

      <aside class="min-w-0 space-y-3 min-[1100px]:sticky min-[1100px]:top-[72px]">
        <AccountCompletenessCard
          :completeness="completeness"
          :loading="completenessLoading"
          :has-error="completenessError"
          @retry="$emit('retryCompleteness')"
        />

        <Alert v-if="accountAlert" class="border-warning/35 bg-warning/5 text-foreground">
          <TriangleAlert />
          <AlertTitle>{{ accountAlert.title }}</AlertTitle>
          <AlertDescription>
            <p>{{ accountAlert.description }}</p>
            <Button as-child size="sm" variant="outline" class="mt-3">
              <RouterLink :to="accountAlert.to">{{ accountAlert.actionLabel }}</RouterLink>
            </Button>
          </AlertDescription>
        </Alert>

        <Card class="gap-3 border-border p-4 shadow-sm">
          <div class="flex items-start gap-3">
            <Scale class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
            <div>
              <h2 class="text-sm font-semibold">帮助与规则</h2>
              <p class="mt-1 text-xs leading-5 text-muted-foreground">平台记录交易状态与双方确认，不参与垫付或私下担保。</p>
            </div>
          </div>
          <Button as-child size="sm" variant="ghost" class="w-full justify-between">
            <RouterLink to="/announcements/platform-rules-api-service-publish-update">查看平台规则<ArrowRight class="h-4 w-4" /></RouterLink>
          </Button>
        </Card>
      </aside>
    </div>
  </div>
</template>
