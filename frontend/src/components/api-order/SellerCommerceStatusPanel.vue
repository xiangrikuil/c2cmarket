<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowRight, CircleCheck, Clock3, RefreshCw, ShieldAlert } from 'lucide-vue-next'
import LocalTime from '@/components/market/LocalTime.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { SellerCommerceStatus } from '@/lib/api'

const props = withDefaults(defineProps<{
  status?: SellerCommerceStatus | null
  targetServiceId?: string
  loading?: boolean
  error?: boolean
}>(), {
  status: null,
  targetServiceId: '',
  loading: false,
  error: false,
})

defineEmits<{ retry: [] }>()

const serviceLimited = computed(() => Boolean(
  props.status?.level === 'service_limited'
  && (!props.targetServiceId || props.status.affectedServiceIds.includes(props.targetServiceId)),
))
const blocked = computed(() => props.status?.level === 'account_limited' || serviceLimited.value)
const relevantDisputes = computed(() => {
  const disputes = props.status?.disputes ?? []
  if (props.status?.level === 'account_limited' || !props.targetServiceId) return disputes
  return disputes.filter(item => item.apiServiceId === props.targetServiceId)
})
const waitingBuyerCount = computed(() => (props.status?.disputes ?? []).filter(item => item.nextActor === 'counterparty').length)
const sellerActionCount = computed(() => (props.status?.disputes ?? []).filter(item => ['respondent', 'responsible_party'].includes(item.nextActor)).length)
const title = computed(() => {
  if (props.loading) return '正在确认经营状态'
  if (props.error) return '经营状态暂时无法确认'
  if (props.status?.level === 'account_limited') return '全部服务暂停新接单'
  if (serviceLimited.value) return props.targetServiceId ? '当前服务暂停新接单' : '部分服务暂停新接单'
  return '经营正常'
})
const description = computed(() => {
  if (props.loading) return '检查完成前不会提交开启接单操作。'
  if (props.error) return '请重新检查；编辑和保存草稿仍然可用。'
  if (props.status?.level === 'account_limited') return `存在 ${props.status.activeBuyerCount} 位不同买家的活动纠纷或履行逾期。已成立订单仍可继续履约。`
  if (serviceLimited.value) return props.targetServiceId
    ? '当前服务存在多位买家的活动纠纷或卖家响应逾期，其他服务不受影响。'
    : `有 ${props.status?.affectedServiceIds.length ?? 0} 个服务因多位买家的活动纠纷或卖家响应逾期暂停新接单，其他服务不受影响。`
  if ((props.status?.activeDisputeCount ?? 0) > 0) return '活动纠纷只冻结对应订单；未达到风险阈值，不影响其他服务接单。'
  return '当前没有影响新接单的活动纠纷。'
})
</script>

<template>
  <section class="border-y border-border bg-muted/25 px-4 py-3" aria-live="polite">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="flex min-w-0 gap-3">
        <div class="mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-md bg-background">
          <CircleCheck v-if="!loading && !error && !blocked" class="h-4 w-4 text-success" />
          <Clock3 v-else-if="loading" class="h-4 w-4 text-muted-foreground" />
          <ShieldAlert v-else class="h-4 w-4 text-destructive" />
        </div>
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-sm font-semibold">经营状态：{{ title }}</h2>
            <Badge v-if="status" variant="secondary">活动纠纷 {{ status.activeDisputeCount }}</Badge>
            <Badge v-if="status" variant="secondary">阻断经营 {{ status.blockingDisputeCount }}</Badge>
          </div>
          <p class="mt-1 text-sm text-muted-foreground">{{ description }}</p>
          <p v-if="status?.activeDisputeCount" class="mt-1 text-xs text-muted-foreground">
            待你处理 {{ sellerActionCount }} 笔 · 等待买家确认 {{ waitingBuyerCount }} 笔
          </p>
        </div>
      </div>
      <Button v-if="error" type="button" size="icon" variant="outline" title="重新检查经营状态" @click="$emit('retry')">
        <RefreshCw class="h-4 w-4" />
      </Button>
    </div>

    <div v-if="blocked && relevantDisputes.length" class="mt-3 divide-y divide-border border-t border-border">
      <div v-for="item in relevantDisputes.slice(0, 3)" :key="item.disputeId" class="flex flex-col gap-2 py-2 text-sm sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <p class="truncate font-medium">{{ item.orderNo }} · {{ item.serviceTitle }}</p>
          <p class="text-xs text-muted-foreground">
            <span v-if="item.dueAt">当前阶段截止 <LocalTime :value="item.dueAt" /></span>
            <span v-else>等待当前责任人处理</span>
          </p>
        </div>
        <RouterLink :to="`/my/disputes/${item.disputeId}`" class="inline-flex shrink-0 items-center gap-1 text-sm font-medium text-primary hover:underline">
          查看案件 <ArrowRight class="h-4 w-4" />
        </RouterLink>
      </div>
    </div>
  </section>
</template>
