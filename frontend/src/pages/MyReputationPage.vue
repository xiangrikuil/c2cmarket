<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Activity, BadgeCheck, Info, ShieldAlert } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import ReputationMetricsGrid from '@/components/reputation/ReputationMetricsGrid.vue'
import ReputationProgressList from '@/components/reputation/ReputationProgressList.vue'
import ReputationSummaryCard from '@/components/reputation/ReputationSummaryCard.vue'
import WorkspaceSectionTabs from '@/components/workspace/WorkspaceSectionTabs.vue'
import {
  reputationRoleLabel,
  reputationScopeLabel,
  snapshotToSummary,
} from '@/lib/reputationPresentation'
import { useMyReputationQuery } from '@/queries/useReputationQueries'
import { usePromotionRewardPublicConfig } from '@/queries/usePromotionRewardQueries'
import type { ReputationRole, ReputationScope } from '@/types/reputation'

const role = ref<ReputationRole>('buyer')
const scope = ref<ReputationScope>('overall')
const promotionNoticeConsumed = ref(false)
const route = useRoute()
const router = useRouter()
const { data, isLoading, error, refetch } = useMyReputationQuery()
const promotionConfigQuery = usePromotionRewardPublicConfig()
const promotionTabEnabled = computed(() => promotionConfigQuery.isSuccess.value
  ? promotionConfigQuery.data.value?.programEnabled === true
  : undefined)

const snapshot = computed(() => data.value?.items.find(item =>
  item.role === role.value && item.scope === scope.value,
) ?? null)
const summary = computed(() => snapshot.value ? snapshotToSummary(snapshot.value) : null)
const activeRestrictions = computed(() => data.value?.activeRestrictions ?? [])

watch(() => route.query.notice, notice => {
  if (notice !== 'promotion-disabled' || promotionNoticeConsumed.value) return
  promotionNoticeConsumed.value = true
  if (import.meta.client) toast.info('当前推广活动未开放，已返回信誉成长。')
  const query = { ...route.query }
  delete query.notice
  router.replace({ query })
}, { immediate: true })

function restrictionImpactLabel(actionCode: string) {
  const labels: Record<string, string> = {
    api_service_publish: 'API 服务新接单、发布与恢复',
    api_order_create: '创建 API 订单',
    carpool_publish: '发布订阅拼车',
    carpool_apply: '申请加入拼车',
    carpool_accept: '接受拼车申请',
    contact_view: '查看联系方式',
    review_submit: '提交评价',
    all: '全部受限动作',
  }
  return labels[actionCode] ?? actionCode
}
</script>

<template>
  <div class="space-y-6">
    <PageTitle
      title="信誉与权益"
      description="查看买家和卖家在不同交易范围内的可验证履约事实、风险状态与成长进度。"
    />

    <WorkspaceSectionTabs section="reputation-rights" :promotion-enabled="promotionTabEnabled" />

    <section v-if="activeRestrictions.length" class="border-y border-destructive/30 py-5" aria-labelledby="active-restrictions-title">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <ShieldAlert class="h-5 w-5 text-destructive" />
          <h2 id="active-restrictions-title" class="text-lg font-semibold">当前有效限制</h2>
        </div>
        <Badge variant="destructive">{{ activeRestrictions.length }} 项</Badge>
      </div>
      <div class="mt-4 space-y-4">
        <article v-for="item in activeRestrictions" :key="`${item.reasonCode}:${item.startsAt}`" class="border-l-2 border-destructive pl-4">
          <div class="flex flex-wrap items-start justify-between gap-2">
            <div>
              <h3 class="text-sm font-semibold">{{ restrictionImpactLabel(item.actionCode) }}</h3>
              <p class="mt-1 text-sm leading-6 text-muted-foreground">{{ item.publicReason }}</p>
            </div>
            <Badge variant="outline">{{ item.roleScope === 'seller' ? '卖家范围' : item.roleScope === 'buyer' ? '买家范围' : '全部角色' }}</Badge>
          </div>
          <p v-if="item.actionCode === 'api_service_publish' && item.reasonCode === 'api_order_remedy_overdue'" class="mt-3 text-sm font-medium">
            当前暂停 API 服务新接单、发布和恢复。已成立订单仍可继续付款、交付、完成、售后和纠纷处理。
          </p>
          <div class="mt-3 flex flex-wrap gap-x-5 gap-y-1 text-xs text-muted-foreground">
            <span>生效：<LocalTime :value="item.startsAt" /></span>
            <span>截止：<LocalTime v-if="item.endsAt" :value="item.endsAt" /><template v-else>长期有效</template></span>
          </div>
        </article>
      </div>
    </section>

    <div class="flex flex-col gap-3 border-b pb-5 lg:flex-row lg:items-center lg:justify-between">
      <Tabs v-model="role">
        <TabsList aria-label="选择信誉角色">
          <TabsTrigger value="buyer">买家信誉</TabsTrigger>
          <TabsTrigger value="seller">卖家信誉</TabsTrigger>
        </TabsList>
      </Tabs>
      <Tabs v-model="scope">
        <TabsList aria-label="选择信誉范围">
          <TabsTrigger value="overall">综合</TabsTrigger>
          <TabsTrigger value="carpool">订阅拼车</TabsTrigger>
          <TabsTrigger value="api">API 服务</TabsTrigger>
        </TabsList>
      </Tabs>
    </div>

    <div v-if="isLoading" class="grid gap-4 lg:grid-cols-2">
      <SkeletonBlock :lines="5" />
      <SkeletonBlock :lines="5" />
    </div>
    <ErrorState
      v-else-if="error"
      title="信誉数据加载失败"
      description="当前无法读取真实信誉快照，请稍后重试。"
      @retry="refetch()"
    />
    <EmptyState
      v-else-if="!snapshot"
      title="暂无信誉快照"
      description="该角色与范围暂时没有可用信誉数据。"
    />

    <template v-else>
      <ReputationSummaryCard :summary="summary" />

      <section class="border-b pb-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 class="text-lg font-semibold">原始指标</h2>
            <p class="mt-1 text-sm text-muted-foreground">
              {{ reputationRoleLabel(snapshot.role) }} · {{ reputationScopeLabel(snapshot.scope) }}
            </p>
          </div>
          <Badge variant="outline" class="gap-1">
            <Activity class="h-3.5 w-3.5" />
            {{ snapshot.ruleVersion }}
          </Badge>
        </div>
        <div class="mt-5">
          <ReputationMetricsGrid :metrics="snapshot.metrics" />
        </div>
      </section>

      <section class="border-b pb-6">
        <div class="mb-4 flex items-center gap-2">
          <BadgeCheck class="h-5 w-5 text-primary" />
          <h2 class="text-lg font-semibold">成长与证据</h2>
        </div>
        <ReputationProgressList :items="snapshot.progress" />
      </section>

      <Alert>
        <Info />
        <AlertTitle>快照信息</AlertTitle>
        <AlertDescription class="flex flex-wrap gap-x-5 gap-y-1">
          <span>计算时间：<LocalTime :value="snapshot.calculatedAt" /></span>
          <span>规则版本：{{ snapshot.ruleVersion }}</span>
          <span v-if="snapshot.sourceDataUpdatedAt">事实更新时间：<LocalTime :value="snapshot.sourceDataUpdatedAt" /></span>
        </AlertDescription>
      </Alert>
    </template>
  </div>
</template>
