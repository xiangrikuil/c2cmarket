<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import { useCloseDemandMutation, useDemand } from '@/queries/useMarketQueries'
import { toast } from 'vue-sonner'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const { data: demand, isLoading } = useDemand(id)
const closeMutation = useCloseDemandMutation()

const ownerPreferenceLabel = computed(() => {
  if (!demand.value) return '-'
  if (demand.value.ownerPreference === 'only-personal' || demand.value.ownerPreference === 'only_personal') return '只看个人车主'
  if (demand.value.ownerPreference === 'any') return '不限'
  return '个人车主优先'
})

function toggleClosed() {
  if (!demand.value) return
  closeMutation.mutate(demand.value.id, {
    onSuccess(data) {
      toast.success(data.status === '已关闭' ? '求车需求已关闭。' : '求车需求已重新打开。')
    },
    onError(error) {
      toast.error(error instanceof Error ? error.message : '操作失败')
    },
  })
}
</script>

<template>
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载求车需求...</div>
  <div v-else-if="!demand" class="rounded-xl border border-border bg-card p-8">
    <h1 class="text-xl font-semibold">未找到求车需求</h1>
    <p class="mt-2 text-sm text-muted-foreground">该需求 ID 不存在，可能已关闭或尚未通过审核。</p>
    <RouterLink to="/demands"><Button class="mt-5" variant="outline">返回需求大厅</Button></RouterLink>
  </div>
  <div v-else class="space-y-5">
    <PageTitle :title="demand.title" description="求车需求只展示匹配所需上下文，不展示完整联系方式；后续联系仍需在 linux.do 原帖或站外确认。" />

    <div class="grid gap-4 md:grid-cols-4">
      <Card class="p-5">
        <div class="text-sm text-muted-foreground">最高月费</div>
        <div class="mt-2 text-3xl font-semibold">¥{{ demand.maxPrice }}</div>
        <div class="text-xs text-muted-foreground">提交人预算</div>
      </Card>
      <Card class="p-5">
        <div class="text-sm text-muted-foreground">开通区</div>
        <div class="mt-2 text-2xl font-semibold">{{ demand.region }}</div>
        <div class="text-xs text-muted-foreground">按提交人偏好展示</div>
      </Card>
      <Card class="p-5">
        <div class="text-sm text-muted-foreground">车主偏好</div>
        <div class="mt-2 text-2xl font-semibold">{{ ownerPreferenceLabel }}</div>
        <div class="text-xs text-muted-foreground">仅用于匹配排序</div>
      </Card>
      <Card class="p-5">
        <div class="text-sm text-muted-foreground">当前状态</div>
        <div class="mt-2"><Badge>{{ demand.status }}</Badge></div>
        <div class="mt-2 text-xs text-muted-foreground">{{ demand.updatedAt }}</div>
      </Card>
    </div>

    <div class="grid gap-5 lg:grid-cols-[1fr_360px]">
      <Card class="p-5">
        <h2 class="text-lg font-semibold">需求说明</h2>
        <dl class="mt-5 grid gap-4 text-sm">
          <div class="grid gap-1 border-b border-border pb-3 sm:grid-cols-[140px_1fr]">
            <dt class="text-muted-foreground">提交人</dt>
            <dd>{{ demand.poster }} · 信任等级{{ demand.trustLevel }}</dd>
          </div>
          <div class="grid gap-1 border-b border-border pb-3 sm:grid-cols-[140px_1fr]">
            <dt class="text-muted-foreground">linux.do 原帖</dt>
            <dd><a class="underline underline-offset-4" :href="demand.sourceUrl" target="_blank" rel="noreferrer">{{ demand.linuxdoPost }}</a></dd>
          </div>
          <div class="grid gap-1 border-b border-border pb-3 sm:grid-cols-[140px_1fr]">
            <dt class="text-muted-foreground">匹配条件</dt>
            <dd>{{ demand.require }}</dd>
          </div>
          <div class="grid gap-1 sm:grid-cols-[140px_1fr]">
            <dt class="text-muted-foreground">补充说明</dt>
            <dd>{{ demand.note || '暂无补充说明。' }}</dd>
          </div>
        </dl>
      </Card>

      <Card class="p-5">
        <h2 class="text-lg font-semibold">提交人操作</h2>
        <p class="mt-2 text-sm text-muted-foreground">提交人可以关闭或重新提交审核。关闭后仍保留记录，方便管理台和通知中心追踪。</p>
        <Button class="mt-5 w-full" :variant="demand.status === '已关闭' ? 'default' : 'outline'" :disabled="closeMutation.isPending.value" @click="toggleClosed">
          {{ demand.status === '已关闭' ? '重新打开需求' : '关闭需求' }}
        </Button>
        <RouterLink to="/admin/demands">
          <Button class="mt-3 w-full" variant="outline">查看管理台审核视图</Button>
        </RouterLink>
      </Card>
    </div>
  </div>
</template>
