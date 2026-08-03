<script setup lang="ts">
import { computed } from 'vue'
import {
  Bot,
  CalendarClock,
  CheckCircle2,
  ClipboardList,
  Clock3,
  CreditCard,
  Gauge,
  KeyRound,
  Network,
  PackageOpen,
  ShoppingCart,
  TimerReset,
  WalletCards,
  Zap,
} from 'lucide-vue-next'
import ApiFreeServiceCard from '@/components/api-market/ApiFreeServiceCard.vue'
import type { ApiFreeServiceCardData } from '@/components/api-market/apiFreeServiceCard'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { getApiTTFTBandLabel } from '@/lib/api'
import { getProductCategoryIconSrc } from '@/lib/productCategoryIcon'
import type { ApiServicePublishForm, CatalogById, SellingMode } from './types'
import { accountPoolLabel, apiQuotaBoundaryNotice, distributionLabels, enabledPaymentOptions, formatMultiplier, generatedTitle, paymentMethodLabels, providerCategoryLabels, selectedCatalogItems, simplifiedApiQuotaRules, warrantyLabel } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  catalogById: CatalogById
  completeness: Array<{ label: string, status: 'done' | 'pending' | 'conflict' }>
  risks: string[]
  quotaForMinimumPurchase: string
  sellingMode: SellingMode
  previewOnly?: boolean
}>()

const isLimitedQuotaMode = computed(() => props.sellingMode === 'limited')
const providerIconSrc = computed(() => getProductCategoryIconSrc(props.form.providerCategory, new Map()))
const title = computed(() => isLimitedQuotaMode.value
  ? `${providerCategoryLabels[props.form.providerCategory]} API · 限时额度包`
  : generatedTitle(props.form, props.catalogById))
const merchantDisplayName = computed(() => props.form.merchantIdentityMode === 'store_alias' ? props.form.merchantDisplayName.trim() || '待设置商家展示名' : '公开个人身份')
const selectedModels = computed(() => selectedCatalogItems(props.form, props.catalogById))
const previewPackage = computed(() => props.form.billingMode === 'fixed_package' ? props.form.packages.find(item => item.enabled) ?? null : null)
const isFreeQuotaMode = computed(() => !isLimitedQuotaMode.value && props.form.billingMode !== 'fixed_package')
const quotaExpiresAtLabel = computed(() => props.form.quotaExpiresAt ? props.form.quotaExpiresAt.replace('T', ' ') : '待填写')
const paymentSummary = computed(() => {
  const labels = enabledPaymentOptions(props.form).map(option => paymentMethodLabels[option.paymentMethod])
  return labels.length ? `${labels.join(' / ')} · 固定 ${props.form.paymentWindowMinutes} 分钟` : '待配置'
})
const paymentMethods = computed(() => enabledPaymentOptions(props.form).map(option => option.paymentMethod))
const pendingItems = computed(() => props.completeness.filter(item => item.status === 'pending'))
const conflictItems = computed(() => props.completeness.filter(item => item.status === 'conflict'))
const completedItems = computed(() => props.completeness.filter(item => item.status === 'done'))
const completionPercent = computed(() => props.completeness.length
  ? Math.round((completedItems.value.length / props.completeness.length) * 100)
  : 0)
const checkMessage = computed(() => {
  if (conflictItems.value.length) return `需处理：${conflictItems.value.map(item => item.label).join('、')}`
  if (pendingItems.value.length) return `还差：${pendingItems.value.map(item => item.label).join('、')}`
  return isLimitedQuotaMode.value ? '基础服务已完成，可以进入额度包设置' : '必填项已完成，可以发布'
})
const previewRows = computed(() => {
  const rows = [
    {
      label: '支持模型',
      value: selectedModels.value.map(item => item.displayName).join(' / ') || '待选择',
      icon: Bot,
    },
  ]
  if (isLimitedQuotaMode.value) {
    rows.push(
      { label: '服务倍率', value: props.form.distributionSystem === 'sub2api' ? '1.00x' : formatMultiplier(props.form.defaultMultiplier), icon: Gauge },
      { label: '开售方式', value: '全天 / 定时 · 绝对失效', icon: CalendarClock },
    )
  } else {
    rows.push(
      { label: `¥${simplifiedApiQuotaRules.minimumPurchaseCny} 约可购`, value: props.quotaForMinimumPurchase, icon: PackageOpen },
      { label: '有效至', value: quotaExpiresAtLabel.value, icon: Clock3 },
      { label: '服务倍率', value: props.form.distributionSystem === 'sub2api' ? '1.00x' : formatMultiplier(props.form.defaultMultiplier), icon: Gauge },
    )
  }
  rows.push(
	{ label: '首字响应 / 最大并发', value: `${getApiTTFTBandLabel(props.form.declaredTtftBand)} / ${props.form.declaredMaxConcurrency} · 商户自报，平台未测速`, icon: Zap },
	{ label: '号池', value: accountPoolLabel(props.form), icon: Network },
    { label: '收款方式', value: paymentSummary.value, icon: CreditCard },
    { label: '接入类型', value: distributionLabels[props.form.distributionSystem], icon: Network },
  )
  return rows
})
const freeServiceCard = computed<ApiFreeServiceCardData>(() => ({
  title: title.value,
  delivery: distributionLabels[props.form.distributionSystem],
  models: selectedModels.value.map(item => item.displayName),
  category: props.form.providerCategory,
  categoryLabel: providerCategoryLabels[props.form.providerCategory],
  iconSrc: providerIconSrc.value,
  cnyPerUsdAllowance: props.form.cnyPerUsdCredit ?? 0,
  minimumPurchaseCny: props.form.minimumPurchaseCny ?? 0,
  availableUsdAllowance: props.form.availableCreditUsd ?? 0,
  maximumPurchaseCny: props.form.maximumPurchaseCny ?? 0,
  multiplier: props.form.distributionSystem === 'sub2api' ? '1.00x' : formatMultiplier(props.form.defaultMultiplier),
  ttftLabel: getApiTTFTBandLabel(props.form.declaredTtftBand),
  declaredMaxConcurrency: props.form.declaredMaxConcurrency || '—',
  paymentWindowMinutes: props.form.paymentWindowMinutes,
  merchantName: props.form.merchantDisplayName.trim() || merchantDisplayName.value,
  merchantType: props.form.merchantIdentityMode === 'store_alias' ? '商户' : '个人卖家',
  expiresAt: quotaExpiresAtLabel.value,
	accountPoolLabel: accountPoolLabel(props.form),
	merchantRefundCommitment: props.form.warranty.mode === 'merchant_full_refund',
	merchantBadges: [],
}))

const buyerFlow = [
  { label: '选择额度包', icon: ShoppingCart },
  { label: '创建订单', icon: ClipboardList },
  { label: '站外付款', icon: WalletCards },
  { label: '确认收款', icon: CheckCircle2 },
  { label: '交付凭证', icon: KeyRound },
]
</script>

<template>
  <div class="min-w-0 space-y-2">
    <ApiFreeServiceCard v-if="isFreeQuotaMode" :card="freeServiceCard" preview />

    <template v-else>
      <div
        class="rounded-md border px-3 py-1.5 text-[11px] leading-5"
        :class="conflictItems.length ? 'border-destructive/25 bg-destructive/5 text-destructive' : pendingItems.length ? 'border-warning/25 bg-warning/10 text-warning' : 'border-success/20 bg-success/5 text-success'"
      >
        {{ checkMessage }}
      </div>

      <div class="rounded-md border border-border bg-card px-3 py-2.5">
        <div class="flex items-center justify-between gap-3 text-xs">
          <span class="font-medium">发布完整度</span>
          <strong>{{ completionPercent }}%</strong>
        </div>
        <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-muted">
          <div class="h-full rounded-full bg-primary transition-[width]" :style="{ width: `${completionPercent}%` }" />
        </div>
      </div>

      <Card class="api-publish-preview-card overflow-hidden p-0 shadow-sm" :class="isLimitedQuotaMode || previewPackage ? 'is-limited' : 'is-free'">
      <div
        class="border-b p-3"
        :class="isLimitedQuotaMode || previewPackage ? 'border-orange-200 bg-orange-50/70' : 'border-emerald-200 bg-emerald-50/70'"
      >
        <div class="flex items-center justify-between gap-3">
          <div class="flex items-center gap-2 text-xs font-medium" :class="isLimitedQuotaMode || previewPackage ? 'text-orange-800' : 'text-emerald-800'">
            <TimerReset v-if="isLimitedQuotaMode || previewPackage" class="h-4 w-4" />
            <PackageOpen v-else class="h-4 w-4" />
            买家看到的内容
          </div>
          <Badge :variant="isLimitedQuotaMode ? 'status' : previewPackage ? 'model' : 'trust'">
            {{ isLimitedQuotaMode ? '限时额度包' : previewPackage ? '固定额度包' : '自由额度' }}
          </Badge>
        </div>
        <div class="mt-2 flex items-center gap-2">
          <span v-if="providerIconSrc" class="grid h-8 w-8 shrink-0 place-items-center rounded-md border border-border bg-white">
            <img :src="providerIconSrc" alt="" class="h-5 w-5 object-contain" />
          </span>
          <h2 class="min-w-0 text-base font-semibold leading-snug">{{ title }}</h2>
        </div>
        <div class="mt-1 text-xs font-medium">{{ merchantDisplayName }}</div>
        <div class="mt-2 flex flex-wrap gap-1.5">
          <Badge variant="trust">信任等级3</Badge>
          <Badge variant="verified">已绑定 linux.do</Badge>
          <Badge v-if="form.merchantIdentityMode === 'store_alias'" variant="secondary">不公开社区用户名</Badge>
        </div>
      </div>

      <template v-if="previewPackage">
        <div class="grid gap-2 px-3 py-3 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
          <div class="rounded-md border border-orange-200 bg-orange-50/55 p-2.5">
            <div class="text-xs text-orange-800">套餐价格</div>
            <div class="mt-1 text-lg font-semibold text-orange-950">¥{{ previewPackage.priceCny }}</div>
          </div>
          <div class="rounded-md border border-border bg-muted/35 p-2.5">
            <div class="text-xs text-muted-foreground">面板额度（USD）</div>
            <div class="mt-1 text-lg font-semibold">${{ previewPackage.panelAllowance }}</div>
          </div>
        </div>

        <dl class="api-publish-preview-list px-3 pb-3 text-xs">
          <div><dt>套餐有效期</dt><dd>{{ previewPackage.durationDays }} 天，交付后开始</dd></div>
          <div><dt>套餐库存</dt><dd>{{ previewPackage.stockTotal }} 份</dd></div>
          <div><dt>套餐模型</dt><dd>{{ previewPackage.modelCatalogIds.map(id => catalogById.get(id)?.displayName ?? id).join(' / ') || '待选择' }}</dd></div>
          <div><dt>收款方式</dt><dd>{{ paymentSummary }}</dd></div>
		  <div><dt>接入类型</dt><dd>{{ distributionLabels[form.distributionSystem] }}</dd></div>
		  <div><dt>号池</dt><dd>{{ accountPoolLabel(form) }}</dd></div>
		  <div><dt>最大并发</dt><dd>{{ form.declaredMaxConcurrency }}</dd></div>
		  <div><dt>退款承诺</dt><dd>{{ warrantyLabel(form.warranty) }}</dd></div>
        </dl>
      </template>

      <div v-else-if="isLimitedQuotaMode" class="border-b border-orange-100 px-3 py-2.5">
        <div class="flex gap-2 rounded-md bg-orange-50 p-2.5 text-orange-950">
          <PackageOpen class="mt-0.5 h-4 w-4 shrink-0 text-orange-600" />
          <div>
            <div class="text-xs font-semibold">额度包价格与库存待设置</div>
            <p class="mt-0.5 text-[11px] leading-4 text-orange-900/70">额度、总价、库存、绝对失效时间与最长 10 分钟交付；倍率沿用基础服务。</p>
          </div>
        </div>
      </div>

      <template v-else>
        <div class="grid gap-2 px-3 py-3 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
          <div class="rounded-md border border-emerald-200 bg-emerald-50/55 p-2.5">
            <div class="text-xs text-emerald-800">美元额度售价</div>
            <div class="mt-1 text-lg font-semibold text-emerald-950">¥{{ form.cnyPerUsdCredit ?? 0 }} / $1</div>
          </div>
          <div class="rounded-md border border-border bg-muted/35 p-2.5">
            <div class="text-xs text-muted-foreground">可售额度</div>
            <div class="mt-1 text-lg font-semibold">${{ form.availableCreditUsd ?? 0 }}</div>
          </div>
        </div>

        <dl class="api-publish-preview-list px-3 pb-3 text-xs">
          <div v-for="row in previewRows" :key="row.label">
            <dt><component :is="row.icon" class="h-3.5 w-3.5" />{{ row.label }}</dt>
            <dd v-if="row.label === '收款方式'" class="inline-flex items-center justify-end gap-1.5">
              <span v-if="paymentMethods.length" class="inline-flex -space-x-1">
                <ApiPaymentMethodIcon v-for="method in paymentMethods" :key="method" :method="method" size="sm" />
              </span>
              <span>{{ row.value }}</span>
            </dd>
            <dd v-else>{{ row.value }}</dd>
          </div>
        </dl>
      </template>

      <div v-if="previewOnly" class="border-t border-border px-3 py-2.5">
        <div class="text-[11px] font-medium text-muted-foreground">服务说明</div>
        <p class="mt-1 whitespace-pre-line text-xs leading-5" :class="previewOnly ? '' : 'line-clamp-2'">{{ form.merchantNote || '待填写服务说明' }}</p>
      </div>

      <div class="border-t border-border bg-muted/35 px-3 py-2 text-[11px] leading-5 text-muted-foreground">
        {{ previewOnly ? apiQuotaBoundaryNotice : '平台记录订单，不代收款；不保存 API Key。' }}
      </div>
      </Card>

      <section v-if="previewOnly" class="rounded-md border border-primary/20 bg-primary/5 p-3" aria-labelledby="buyer-flow-title">
      <div class="flex items-center gap-2">
        <Gauge class="h-4 w-4 text-primary" />
        <h3 id="buyer-flow-title" class="text-xs font-semibold">买家购买流程</h3>
      </div>
      <ol class="mt-2 grid grid-cols-5 gap-1">
        <li v-for="step in buyerFlow" :key="step.label" class="flex min-w-0 flex-col items-center gap-1 text-center text-[10px] leading-4">
          <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md border border-primary/20 bg-background text-primary">
            <component :is="step.icon" class="h-3.5 w-3.5" />
          </span>
          <span class="font-medium">{{ step.label }}</span>
        </li>
      </ol>
      <p class="mt-2 border-t border-primary/15 pt-2 text-[11px] leading-5 text-muted-foreground">卖家确认收款后交付；平台记录订单，不代收款。</p>
      </section>

      <div v-if="previewOnly && risks.length" class="space-y-1.5">
        <div v-for="risk in risks" :key="risk" class="rounded-md border border-amber-200 bg-amber-50 px-3 py-1.5 text-[11px] leading-5 text-amber-800">
          {{ risk }}
        </div>
      </div>
    </template>
  </div>
</template>
