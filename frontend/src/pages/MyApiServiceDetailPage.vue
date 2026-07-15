<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, ExternalLink, PackageSearch } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import CompactStats from '@/components/market/CompactStats.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ShortId from '@/components/market/ShortId.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import {
  getApiMerchantDisplayName,
  getApiMerchantVisibilityLabel,
  type ApiService,
} from '@/lib/api'
import {
  useMyApiService,
  usePauseApiServiceMutation,
  usePublishApiServiceMutation,
  useResumeApiServiceMutation,
} from '@/queries/useMarketQueries'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const { data: service, isLoading, error } = useMyApiService(id)
const publishMutation = usePublishApiServiceMutation()
const pauseMutation = usePauseApiServiceMutation()
const resumeMutation = useResumeApiServiceMutation()
const actionPending = computed(() => publishMutation.isPending.value || pauseMutation.isPending.value || resumeMutation.isPending.value)
const errorMessage = computed(() => error.value instanceof Error ? error.value.message : '无法读取这条 API 服务，请确认当前账号是发布者。')
const serviceStats = computed(() => service.value ? [
  { label: '可售美元额度', value: `$${service.value.balance}`, hint: '扣除已冻结订单后的可售口径' },
  { label: '美元额度售价', value: `¥${(1 / service.value.creditPerCny).toFixed(2)} / $1` },
  { label: '最低订单金额', value: `¥${service.value.minimumPurchaseCny}` },
  { label: '今日订单', value: `${service.value.todayOrderCount} / ${service.value.dailyOrderLimit}` },
] : [])

function statusLabel(item: ApiService) {
  if (item.online) return '在线接单'
  if (item.state === 'reviewing') return '审核中'
  if (item.state === 'paused') return '已暂停'
  return '离线'
}

function statusVariant(item: ApiService) {
  if (item.online) return 'default'
  if (item.state === 'reviewing' || item.state === 'paused') return 'secondary'
  return 'outline'
}

function publishService() {
  if (!service.value || actionPending.value) return
  publishMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已上线。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '上线失败。'),
  })
}

function pauseService() {
  if (!service.value || actionPending.value) return
  pauseMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已暂停。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '暂停失败。'),
  })
}

function resumeService() {
  if (!service.value || actionPending.value) return
  resumeMutation.mutate(service.value.id, {
    onSuccess: () => toast.success('API 服务已恢复上线。'),
    onError: actionError => toast.error(actionError instanceof Error ? actionError.message : '恢复失败。'),
  })
}
</script>

<template>
  <SkeletonBlock v-if="isLoading" :lines="8" />

  <EmptyState v-else-if="!service" title="无法打开服务管理页" :description="errorMessage"><template #action><RouterLink to="/my/api-services"><Button variant="outline">返回我的 API 服务</Button></RouterLink></template></EmptyState>

  <div v-else class="space-y-4">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div>
        <RouterLink to="/my/api-services" class="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
          <ArrowLeft class="h-4 w-4" />
          我的 API 服务
        </RouterLink>
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <h1 class="text-2xl font-semibold md:text-3xl">{{ service.title }}</h1>
          <Badge :variant="statusVariant(service)">{{ statusLabel(service) }}</Badge>
        </div>
        <p class="mt-2 text-sm text-muted-foreground">
          {{ getApiMerchantDisplayName(service) }} · {{ getApiMerchantVisibilityLabel(service) }} · 服务编号 <ShortId :value="service.id" prefix="API-SVC" copyable />
        </p>
      </div>

      <div class="flex flex-wrap gap-2">
        <RouterLink v-if="service.publiclyOrderable" :to="`/api-market/${service.id}?preview=owner`">
          <Button variant="outline" class="gap-2"><ExternalLink class="h-4 w-4" />以买家视角预览</Button>
        </RouterLink>
        <Button v-else variant="outline" disabled>当前不可公开预览</Button>
        <RouterLink to="/merchant/api-orders">
          <Button variant="outline" class="gap-2"><PackageSearch class="h-4 w-4" />查看 API 订单</Button>
        </RouterLink>
        <Button v-if="service.state === 'offline'" :disabled="actionPending" @click="publishService">上线服务</Button>
        <Button v-if="service.online" :disabled="actionPending" variant="destructive" @click="pauseService">暂停接单</Button>
        <Button v-if="service.state === 'paused'" :disabled="actionPending" @click="resumeService">恢复接单</Button>
      </div>
    </div>

    <CompactStats :items="serviceStats" />

    <Card class="border-primary/20 bg-primary/5 p-4 text-sm text-muted-foreground">价格、额度、模型和付款规则修改只影响新订单；已有订单继续使用创建时冻结的服务、金额、额度和联系方式快照。</Card>

    <div class="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader><CardTitle>服务配置</CardTitle></CardHeader>
        <CardContent class="space-y-3 text-sm">
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">接入类型</span><span class="text-right font-medium">{{ service.delivery }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">支持模型</span><span class="text-right font-medium">{{ service.models.join(' / ') }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">服务倍率</span><span class="text-right font-medium">{{ service.rate }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">额度有效期</span><span class="text-right font-medium">{{ service.expiresAt }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">收款方式</span><span class="text-right font-medium">{{ service.acceptedPaymentMethods?.join(' / ') || '待配置' }}</span></div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader><CardTitle>经营状态</CardTitle></CardHeader>
        <CardContent class="space-y-3 text-sm">
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">公开状态</span><span class="text-right font-medium">{{ service.publiclyOrderable ? '买家可下单' : '不在公开接单' }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">最近状态变更</span><span class="text-right font-medium">{{ service.lastOnlineConfirmedAt }}</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">响应时限</span><span class="text-right font-medium">{{ service.expectedResponseMinutes }} 分钟</span></div>
          <div class="flex justify-between gap-4"><span class="text-muted-foreground">未解决纠纷</span><span class="text-right font-medium">{{ service.unresolvedDisputes }}</span></div>
          <p v-if="service.warning" class="rounded-md bg-muted p-3 text-muted-foreground">{{ service.warning }}</p>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
