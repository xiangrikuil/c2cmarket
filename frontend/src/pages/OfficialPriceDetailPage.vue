<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { ExternalLink } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import PageTitle from '@/components/market/PageTitle.vue'
import { useOfficialPrice } from '@/queries/useMarketQueries'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const { data: price, isLoading } = useOfficialPrice(id)

function openSource(url: string) {
  if (!/^https?:\/\//.test(url)) return
  window.open(url, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <div v-if="isLoading" class="rounded-xl border border-border bg-card p-8 text-sm text-muted-foreground">正在加载官网公开价格详情...</div>
  <div v-else-if="!price" class="rounded-xl border border-border bg-card p-8">
    <h1 class="text-xl font-semibold">未找到低价线索</h1>
    <p class="mt-2 text-sm text-muted-foreground">该价格记录不存在，可能已被移除或尚未通过审核。</p>
    <RouterLink to="/official-prices"><Button class="mt-5" variant="outline">返回价格线索</Button></RouterLink>
  </div>
  <div v-else>
    <PageTitle
      :title="`${price.product} ${price.plan} 官网公开价格详情`"
      description="展示地区、渠道、原币价格、提交人、linux.do 来源帖和验证状态。线索价格只作行情参考。"
    />
    <div class="grid gap-4 md:grid-cols-4">
      <Card class="p-5"><div class="text-sm text-muted-foreground">当前参考价</div><div class="mt-2 text-3xl font-semibold">¥{{ price.cny ?? '待验证' }}</div><div class="text-xs text-muted-foreground">{{ price.originalPrice }}</div></Card>
      <Card class="p-5"><div class="text-sm text-muted-foreground">地区</div><div class="mt-2 text-3xl font-semibold">{{ price.region }}</div><div class="text-xs text-muted-foreground">{{ price.channel }}</div></Card>
      <Card class="p-5"><div class="text-sm text-muted-foreground">状态</div><div class="mt-2"><Badge :variant="price.status === '已验证' ? 'default' : 'secondary'">{{ price.status }}</Badge></div><div class="mt-2 text-xs text-muted-foreground">{{ price.updatedAt }}更新</div></Card>
      <Card class="p-5"><div class="text-sm text-muted-foreground">提交人</div><div class="mt-2 text-3xl font-semibold">{{ price.submitter }}</div><div class="text-xs text-muted-foreground">信任等级{{ price.submitterTrust }}</div></Card>
    </div>
    <div class="mt-6 grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
      <Card class="p-6">
        <h2 class="text-lg font-semibold">价格记录</h2>
        <div class="mt-5 space-y-4 text-sm">
          <div class="flex justify-between border-b border-border pb-3"><span class="text-muted-foreground">产品</span><span>{{ price.product }} {{ price.plan }}</span></div>
          <div class="flex justify-between border-b border-border pb-3"><span class="text-muted-foreground">开通方式</span><span>{{ price.openingMethod }}</span></div>
          <div class="flex justify-between border-b border-border pb-3"><span class="text-muted-foreground">来源</span><span>{{ price.source }}</span></div>
          <div class="flex justify-between border-b border-border pb-3"><span class="text-muted-foreground">复核</span><span>{{ price.status === '已验证' ? '管理员已确认来源和价格截图' : '等待管理员复核' }}</span></div>
        </div>
      </Card>
      <Card class="p-6">
        <h2 class="text-lg font-semibold">来源与操作</h2>
        <p class="mt-3 text-sm text-muted-foreground">平台只维护价格情报，不承诺该价格可长期复现。需要开通时请回到原帖确认。</p>
        <div class="mt-5 grid gap-3">
          <Button :disabled="!/^https?:\/\//.test(price.source)" @click="openSource(price.source)"><ExternalLink class="h-4 w-4" />打开来源链接</Button>
          <RouterLink to="/official-prices/submit"><Button variant="outline" class="w-full">提交更新线索</Button></RouterLink>
        </div>
      </Card>
    </div>
  </div>
</template>
