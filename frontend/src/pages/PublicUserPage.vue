<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { AlertTriangle, ExternalLink, Flag, ShoppingBag, UsersRound } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import SoftTable from '@/components/market/SoftTable.vue'
import StatusTabs from '@/components/market/StatusTabs.vue'
import { getApiDeliveryModeLabel } from '@/lib/api'
import { getPricingDisplay, getRemainingSeats } from '@/lib/pricing'
import { useCreatePublicUserReportMutation, usePublicUserProfileQuery } from '@/queries/useMarketQueries'

const route = useRoute()
const router = useRouter()
const username = computed(() => String(route.params.username ?? ''))
const { data, isLoading } = usePublicUserProfileQuery(username)
const reportMutation = useCreatePublicUserReportMutation()
const activeTab = ref('概览')
const profile = computed(() => data.value?.profile)

const completedTotal = computed(() => {
  if (!profile.value?.privacy.showCompletionStats) return null
  return (profile.value.stats.completedCarpoolsLast30Days ?? 0) + (profile.value.stats.completedApiOrdersLast30Days ?? 0)
})

const completionLabel = computed(() => {
  if (completedTotal.value === null) return '已隐藏'
  return completedTotal.value < 5 ? '记录较少' : `${completedTotal.value} 单`
})

function serviceSummary(service: { deliveryModes: Array<'api_key_endpoint' | 'sub2api_panel_account'>, usageVisibility: string, warranty: string }) {
  const access = service.deliveryModes.map(getApiDeliveryModeLabel).join(' / ')
  const visibility = service.usageVisibility === 'panel_realtime' ? '面板实时可见' : service.usageVisibility === 'merchant_readonly' ? '商户只读确认' : '不展示用量'
  const warranty = service.warranty.includes('24') || service.warranty.includes('补') || service.warranty.includes('承诺') ? '商户承诺' : '售后协商'
  return `${access} · ${visibility} · ${warranty}`
}

function reportPublicProfile() {
  if (!profile.value || reportMutation.isPending.value) return
  const description = window.prompt('请填写脱敏举报说明，不要包含联系方式、密码、API Key、token、session、cookie 或恢复码。')
  if (!description) return
  reportMutation.mutate({
    username: profile.value.username,
    reasonCode: 'other',
    title: '公开主页举报',
    description,
  }, {
    onSuccess: () => toast.success('举报已提交。'),
    onError: error => toast.error(error instanceof Error ? error.message : '提交失败'),
  })
}
</script>

<template>
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载用户主页...</div>
  <div v-else-if="!data || !profile" class="rounded-xl border border-border bg-card p-8">
    <h1 class="text-xl font-semibold">未找到用户</h1>
    <p class="mt-2 text-sm text-muted-foreground">该公开主页不存在或尚未同步到当前 mock 数据。</p>
    <Button class="mt-5" variant="outline" @click="router.push('/api-market')">返回 API 集市</Button>
  </div>
  <div v-else class="space-y-4">
    <Card class="p-5">
      <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
        <div class="flex min-w-0 gap-4">
          <div class="grid h-14 w-14 shrink-0 place-items-center overflow-hidden rounded-full bg-primary text-lg font-semibold text-primary-foreground">
            <img v-if="profile.avatarUrl" :src="profile.avatarUrl" alt="公开头像" class="h-full w-full object-cover" />
            <span v-else>{{ profile.avatarText }}</span>
          </div>
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="text-2xl font-semibold">{{ profile.displayName }}</h1>
              <Badge v-if="profile.trustLevel !== null" variant="trust">信任等级{{ profile.trustLevel }}</Badge>
              <Badge v-for="badge in profile.badges" :key="badge.id" variant="secondary">{{ badge.label }}</Badge>
            </div>
            <div class="mt-2 text-sm text-muted-foreground">
              @{{ profile.username }}
              <span v-if="profile.linuxDoUsername"> · linux.do @{{ profile.linuxDoUsername }}</span>
              <span v-if="profile.createdAt"> · 加入 {{ profile.createdAt }}</span>
              <span v-if="profile.lastActiveAt"> · 最近活跃 {{ profile.lastActiveAt }}</span>
            </div>
            <p v-if="profile.bio" class="mt-2 max-w-3xl text-sm text-muted-foreground">{{ profile.bio }}</p>
            <div class="mt-2 flex flex-wrap gap-2">
              <Button size="sm" variant="outline" @click="toast('将打开 linux.do 主页；当前 mock 不跳转外部站点。')"><ExternalLink class="h-4 w-4" />查看 linux.do 主页</Button>
              <Button v-if="profile.privacy.allowPublicProfileReport" size="sm" variant="outline" :disabled="reportMutation.isPending.value" @click="reportPublicProfile"><Flag class="h-4 w-4" />举报</Button>
            </div>
          </div>
        </div>
        <div v-if="profile.stats.unresolvedDisputeCount" class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
          <AlertTriangle class="mr-1 inline h-4 w-4" />存在未解决纠纷，相关服务由系统控制接单状态。
        </div>
      </div>
    </Card>

    <div class="grid gap-3 md:grid-cols-4">
      <Card class="p-4"><div class="text-xs text-muted-foreground">近30天完成</div><div class="mt-1 text-2xl font-semibold">{{ completionLabel }}</div><div v-if="completedTotal !== null && completedTotal < 5" class="text-xs text-muted-foreground">记录较少，不作为负面信号</div></Card>
      <Card class="p-4"><div class="text-xs text-muted-foreground">响应中位时间</div><div class="mt-1 text-2xl font-semibold">{{ profile.stats.responseMedianMinutes === null ? '已隐藏' : `${profile.stats.responseMedianMinutes} 分钟` }}</div></Card>
      <Card class="p-4"><div class="text-xs text-muted-foreground">责任取消</div><div class="mt-1 text-2xl font-semibold">{{ profile.stats.buyerResponsibilityCancellationCount + profile.stats.sellerResponsibilityCancellationCount }}</div></Card>
      <Card class="p-4"><div class="text-xs text-muted-foreground">未解决纠纷</div><div class="mt-1 text-2xl font-semibold">{{ profile.stats.unresolvedDisputeCount }}</div></Card>
    </div>

    <Card class="p-4 text-sm text-muted-foreground">
      公开主页只展示公开资料、铭牌、脱敏统计和公开业务记录，不展示任何联系方式、登录或设备信息、意向敏感详情。
    </Card>

    <StatusTabs v-model="activeTab" :items="['概览', '在售服务', '完成记录', '交易评价', '纠纷记录']" />

    <div v-if="activeTab === '概览'" class="grid gap-4 lg:grid-cols-2">
      <Card class="p-5">
        <h2 class="font-semibold">在售拼车车源</h2>
        <div class="mt-4 space-y-3">
          <div v-for="carpool in data.carpools" :key="carpool.id" class="flex items-center justify-between gap-3 border-b border-border pb-3 text-sm">
            <div><div class="font-medium">{{ carpool.product }}</div><div class="text-xs text-muted-foreground">{{ carpool.region }} · 剩余 {{ getRemainingSeats(carpool) }} 位</div></div>
            <RouterLink :to="`/carpools/${carpool.id}`"><Button size="sm" variant="outline"><UsersRound class="h-4 w-4" />查看</Button></RouterLink>
          </div>
          <p v-if="data.carpools.length === 0" class="text-sm text-muted-foreground">暂无公开在售拼车车源。</p>
        </div>
      </Card>
      <Card class="p-5">
        <h2 class="font-semibold">在售 API 服务</h2>
        <div class="mt-4 space-y-3">
          <div v-for="service in data.services" :key="service.id" class="flex items-center justify-between gap-3 border-b border-border pb-3 text-sm">
            <div><div class="font-medium">{{ service.title }}</div><div class="text-xs text-muted-foreground">{{ serviceSummary(service) }}</div></div>
            <RouterLink :to="`/api-market/${service.id}`"><Button size="sm"><ShoppingBag class="h-4 w-4" />查看</Button></RouterLink>
          </div>
          <p v-if="data.services.length === 0" class="text-sm text-muted-foreground">暂无公开在售 API 服务。</p>
        </div>
      </Card>
    </div>

    <SoftTable v-else-if="activeTab === '在售服务'" :columns="['类型', '服务', '价格 / 余额', '状态', '操作']">
      <tr v-for="carpool in data.carpools" :key="`carpool-${carpool.id}`">
        <td>拼车</td>
        <td><div class="font-medium">{{ carpool.product }}</div><div class="text-xs text-muted-foreground">{{ carpool.region }} · {{ carpool.ownerType }}</div></td>
        <td>{{ getPricingDisplay(carpool).primaryLabel }} ¥{{ getPricingDisplay(carpool).primaryPrice }}</td>
        <td><Badge>{{ carpool.status }}</Badge></td>
        <td><RouterLink :to="`/carpools/${carpool.id}`"><Button size="sm" variant="outline">查看</Button></RouterLink></td>
      </tr>
      <tr v-for="service in data.services" :key="`api-${service.id}`">
        <td>API</td>
        <td><div class="font-medium">{{ service.title }}</div><div class="text-xs text-muted-foreground">{{ serviceSummary(service) }}</div></td>
        <td>余额 ${{ service.balance }}</td>
        <td><Badge :variant="service.online ? 'verified' : 'secondary'">{{ service.online ? '在线' : '离线' }}</Badge></td>
        <td><RouterLink :to="`/api-market/${service.id}`"><Button size="sm">查看</Button></RouterLink></td>
      </tr>
    </SoftTable>

    <SoftTable v-else-if="activeTab === '完成记录'" :columns="['日期', '服务类型', '接入方式', '金额范围', '完成状态']">
      <tr v-for="record in data.completions" :key="record.id">
        <td>{{ record.date }}</td>
        <td>{{ record.serviceType }}</td>
        <td>{{ getApiDeliveryModeLabel(record.deliveryMode) }}</td>
        <td>{{ record.amountRange }}</td>
        <td><Badge variant="verified">{{ record.status }}</Badge></td>
      </tr>
      <tr v-if="data.completions.length === 0"><td colspan="5" class="py-8 text-center text-sm text-muted-foreground">暂无平台确认完成记录。</td></tr>
    </SoftTable>

    <SoftTable v-else-if="activeTab === '交易评价'" :columns="['日期', '服务类型', '标签', '评价']">
      <tr v-for="review in data.reviews" :key="review.id">
        <td><div>{{ review.date }}</div><Badge v-if="review.verified" class="mt-1" variant="verified">已验证交易</Badge></td>
        <td>{{ review.serviceType }}</td>
        <td><div class="flex flex-wrap gap-1"><Badge v-for="tag in review.tags.slice(0, 3)" :key="tag" variant="capability">{{ tag }}</Badge></div></td>
        <td class="text-muted-foreground">{{ review.note }}</td>
      </tr>
      <tr v-if="data.reviews.length === 0"><td colspan="4" class="py-8 text-center text-sm text-muted-foreground">暂无交易评价。</td></tr>
    </SoftTable>

    <SoftTable v-else :columns="['纠纷类型', '处理结果', '处理时间', '状态']">
      <tr>
        <td colspan="4" class="bg-muted/40 text-sm text-muted-foreground">
          未解决 {{ profile.stats.unresolvedDisputeCount }} · 近90天已处理 {{ profile.stats.resolvedDisputeCountLast90Days === null ? '已隐藏' : profile.stats.resolvedDisputeCountLast90Days }}。公开信息已脱敏，不展示截图、联系方式或双方敏感信息。
        </td>
      </tr>
      <tr v-for="dispute in data.disputes" :key="dispute.id">
        <td>{{ dispute.type }}</td>
        <td>{{ dispute.result }}</td>
        <td>{{ dispute.handledAt }}</td>
        <td><Badge :variant="dispute.unresolved ? 'destructive' : 'secondary'">{{ dispute.unresolved ? '未解决' : '已处理' }}</Badge></td>
      </tr>
    </SoftTable>
  </div>
</template>
