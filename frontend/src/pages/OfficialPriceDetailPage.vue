<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { CalendarClock, CreditCard, ExternalLink, Globe2, Info, Package, ReceiptText, Tag } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import EmptyState from '@/components/market/EmptyState.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { useOfficialPrice, useOfficialPrices } from '@/queries/useMarketQueries'
import { getProductCategory } from '@/lib/productCategories'
import { getProductCategoryIconSrc } from '@/lib/productCategoryIcon'

const route = useRoute()
const id = computed(() => String(route.params.id ?? ''))
const { data: price, isLoading } = useOfficialPrice(id)
const { data: allPrices } = useOfficialPrices()
const builtInProductIcons = new Map<string, string>()
const productIconSrc = computed(() => price.value ? getProductCategoryIconSrc(getProductCategory(price.value.product), builtInProductIcons) : null)
const relatedPrices = computed(() => price.value ? (allPrices.value ?? []).filter(item => item.id !== price.value!.id && item.product === price.value!.product).slice(0, 3) : [])

function openSource(url: string) {
  if (/^https?:\/\//.test(url)) window.open(url, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <SkeletonBlock v-if="isLoading" :lines="7" />
  <EmptyState v-else-if="!price" title="未找到价格记录" description="该记录不存在、已被移除或尚未公开。"><template #action><RouterLink to="/official-prices"><Button variant="outline">返回官网价格</Button></RouterLink></template></EmptyState>
  <div v-else class="official-price-detail-page space-y-5">
    <header class="official-price-detail-reference-head"><span class="official-price-detail-icon"><img v-if="productIconSrc" :src="productIconSrc" alt="" /><Package v-else class="h-5 w-5" /></span><div><div class="flex flex-wrap items-center gap-2"><h1>{{ price.product }} {{ price.plan }} 官网价格</h1><Badge class="bg-violet-100 text-violet-700">官方信息</Badge></div><p>官网公开价格详情，用于理解地区和渠道差异，不代表平台市场成交价。</p></div></header>
    <div class="official-price-detail-reference-layout">
      <main class="min-w-0 space-y-4">
        <Card class="official-price-detail-metrics p-0"><dl><div><Tag /><dt>官网公开价</dt><dd>{{ price.originalPrice }}</dd><small>{{ price.cny === null ? '待换算' : `约合 ¥${price.cny}` }}</small></div><div><CreditCard /><dt>价格类型</dt><dd>官网公开价</dd><small>{{ price.plan }}</small></div><div><Globe2 /><dt>适用地区</dt><dd>{{ price.region }}</dd><small>{{ price.channel }}</small></div><div><ReceiptText /><dt>税费口径</dt><dd>以结算页为准</dd><small>地区政策可能变化</small></div><div><CalendarClock /><dt>最后更新</dt><dd>{{ price.updatedAt }}</dd><small>{{ price.status }}</small></div></dl></Card>

        <nav class="official-price-detail-tabs" aria-label="价格详情内容"><span>套餐说明</span><span class="is-active">地区与渠道</span><span>税费说明</span><span>付款方式参考</span><span>购买建议</span><span>风险提示</span></nav>

        <Card class="official-price-detail-comparison p-5"><h2>地区与渠道信息</h2><p>以下为当前官网价格记录中的已维护信息，其他地区请以对应官网结算页为准。</p><div class="mt-4 overflow-x-auto"><table><thead><tr><th>地区</th><th>渠道</th><th>官网公开价</th><th>折合人民币</th><th>开通方式</th></tr></thead><tbody><tr><td>{{ price.region }}</td><td>{{ price.channel }}</td><td>{{ price.originalPrice }}</td><td>{{ price.cny === null ? '待换算' : `¥${price.cny}` }}</td><td>{{ price.openingMethod }}</td></tr></tbody></table></div><div class="mt-4 flex gap-3 text-xs leading-5 text-muted-foreground"><Info class="mt-0.5 h-4 w-4 shrink-0 text-warning" /><p>汇率、税费、地区资格和渠道政策都可能改变实际支付金额。</p></div></Card>

        <Card class="p-5"><h2 class="font-semibold">价格使用说明</h2><ul class="mt-3 space-y-2 text-sm leading-6 text-muted-foreground"><li>• 此价格仅供参考，不等于市场交易价格，也不等于本站车源或 API 服务成交价。</li><li>• 购买资格、税费与实际可用功能均以产品官网展示为准。</li><li>• 记录来源和更新时间可在右侧查看。</li></ul></Card>
      </main>

      <Card class="official-price-detail-source p-5 lg:sticky lg:top-16"><div class="flex items-center justify-between"><h2 class="font-semibold">当前官网价格</h2><Badge class="bg-violet-100 text-violet-700">官方</Badge></div><div class="mt-4 text-3xl font-semibold text-primary">{{ price.originalPrice }}</div><div class="mt-1 text-sm text-muted-foreground">{{ price.cny === null ? '人民币价格待换算' : `约合 ¥${price.cny}` }}</div><dl class="mt-5 grid gap-3 text-sm"><div class="flex justify-between gap-3"><dt class="text-muted-foreground">地区</dt><dd>{{ price.region }}</dd></div><div class="flex justify-between gap-3"><dt class="text-muted-foreground">购买渠道</dt><dd>{{ price.channel }}</dd></div><div class="flex justify-between gap-3"><dt class="text-muted-foreground">最后更新</dt><dd>{{ price.updatedAt }}</dd></div><div><dt class="text-muted-foreground">信息来源</dt><dd class="mt-1 break-all">{{ price.source }}</dd></div></dl><div class="mt-4 rounded-lg border border-primary/15 bg-primary/5 p-3 text-xs leading-5 text-muted-foreground">本页面价格信息来自公开来源，仅供参考，实际以结算页面为准。</div><Button class="mt-4 w-full" :disabled="!/^https?:\/\//.test(price.source)" @click="openSource(price.source)"><ExternalLink class="h-4 w-4" />打开原始来源</Button><div v-if="relatedPrices.length" class="mt-5 border-t border-border pt-4"><h3 class="text-sm font-semibold">相关记录</h3><RouterLink v-for="item in relatedPrices" :key="item.id" :to="`/official-prices/${item.id}`" class="mt-2 flex justify-between gap-3 text-xs"><span class="truncate">{{ item.plan }} · {{ item.region }}</span><span>{{ item.originalPrice }}</span></RouterLink></div></Card>
    </div>
  </div>
</template>
