<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { formatMonthlyQuota, quotaFieldLabel } from '@/lib/quota'
import type {
  CarpoolProductCatalogItem,
  CarpoolPublishForm,
  CompletenessItem,
  OpeningChannelOption,
  ParsedLinuxDoTopic,
  PaymentMethodOption,
  RegionOption,
} from './types'
import {
  availableSeats,
  requiresSubscriptionRiskAck,
  openingChannelLabels,
  paymentMethodLabels,
  previewTitle,
  warrantyLabel,
} from './utils'

const props = defineProps<{
  form: CarpoolPublishForm
  catalogById: Map<string, CarpoolProductCatalogItem>
  regionsByCode: Map<string, RegionOption>
  openingChannelsByCode: Map<string, OpeningChannelOption>
  paymentMethodsByCode: Map<string, PaymentMethodOption>
  parsedTopic: ParsedLinuxDoTopic | null
  completeness: CompletenessItem[]
  reminders: string[]
  submitPending: boolean
  previewOnly?: boolean
}>()

const emit = defineEmits<{
  saveDraft: []
  submitReview: []
}>()

const remaining = computed(() => availableSeats(props.form))
const completenessPercent = computed(() => {
  if (!props.completeness.length) return 0
  return Math.round((props.completeness.filter(item => item.status === 'done').length / props.completeness.length) * 100)
})
const paymentText = computed(() => {
  const labels = props.form.paymentMethodCodes.map(code => props.paymentMethodsByCode.get(code)?.displayName ?? paymentMethodLabels[code])
  return labels.length ? labels.join(' / ') : '待选择'
})
const openingText = computed(() => {
  const code = props.form.openingChannelCode
  if (!code) return '待选择'
  return props.openingChannelsByCode.get(code)?.displayName ?? openingChannelLabels[code]
})
const selectedProduct = computed(() => props.catalogById.get(props.form.productId) ?? null)
const arrangementLabel = computed(() => {
  if (props.form.accessArrangementMode === 'personal_account_cost_share') return '费用分摊'
  if (props.form.accessArrangementMode === 'provider_member_invitation') return '成员邀请'
  if (props.form.accessArrangementMode === 'owner_managed_access') return '站外托管 / 中转'
  if (props.form.accessArrangementMode === 'other_off_platform') return '站外安排'
  return '需调整'
})
const quotaText = computed(() => formatMonthlyQuota({
  amount: props.form.monthlyQuotaAmount,
  label: selectedProduct.value?.quotaLabel,
  unit: selectedProduct.value?.quotaUnit,
  period: selectedProduct.value?.quotaPeriod,
}, '待确认'))
const quotaLabel = computed(() => quotaFieldLabel(selectedProduct.value))
</script>

<template>
  <aside :class="previewOnly ? 'space-y-4' : 'space-y-4 lg:sticky lg:[top:calc(var(--app-header-height)+16px)]'">
    <Card class="relative overflow-hidden p-5 shadow-sm before:absolute before:inset-x-0 before:top-0 before:h-[3px] before:bg-primary">
      <div class="text-xs text-muted-foreground">车源预览</div>
      <h2 class="mt-2 text-lg font-semibold leading-snug">{{ previewTitle(form, catalogById, regionsByCode) }}</h2>
      <p class="mt-1 text-xs text-muted-foreground">个人车主 · 信任等级3 · {{ parsedTopic ? '原帖已绑定' : '原帖待读取' }}</p>

      <div class="mt-4 flex items-end justify-between gap-3">
        <div class="text-3xl font-bold tracking-tight">¥{{ form.monthlyPriceCny ?? '-' }}<span class="text-sm font-semibold">/月</span></div>
        <div class="text-sm font-semibold text-primary">剩余 {{ remaining }} / {{ form.totalSeats }}</div>
      </div>

        <div class="mt-4 flex flex-wrap gap-1.5">
        <Badge variant="capability">{{ openingText }}</Badge>
        <Badge variant="capability">{{ form.serviceMultiplier ?? '-' }}x</Badge>
        <Badge variant="capability">{{ quotaText }}</Badge>
        <Badge :variant="form.accessArrangementMode === 'not_allowed' ? 'secondary' : 'verified'">
          {{ arrangementLabel }}
        </Badge>
        <Badge v-if="requiresSubscriptionRiskAck(selectedProduct, form)" :variant="form.riskAcknowledged ? 'verified' : 'secondary'">{{ form.riskAcknowledged ? '边界已确认' : '待确认边界' }}</Badge>
        <Badge variant="trust">{{ warrantyLabel(form.warranty) }}</Badge>
        <Badge variant="verified">近期确认</Badge>
      </div>

      <dl class="mt-4 divide-y divide-border text-sm">
        <div class="flex justify-between gap-4 py-2"><dt class="text-muted-foreground">开通区</dt><dd class="font-semibold">{{ regionsByCode.get(form.regionCode)?.displayName || '待选择' }}</dd></div>
        <div class="flex justify-between gap-4 py-2"><dt class="text-muted-foreground">倍率</dt><dd class="font-semibold">{{ form.serviceMultiplier ?? '-' }}x</dd></div>
        <div class="flex justify-between gap-4 py-2"><dt class="text-muted-foreground">{{ quotaLabel }}</dt><dd class="font-semibold">{{ quotaText }}</dd></div>
        <div class="flex justify-between gap-4 py-2"><dt class="text-muted-foreground">支付方式</dt><dd class="text-right font-semibold">{{ paymentText }}</dd></div>
        <div class="flex justify-between gap-4 py-2"><dt class="text-muted-foreground">访问安排</dt><dd class="text-right font-semibold">{{ form.accessArrangementNote || '待填写' }}</dd></div>
        <div class="flex justify-between gap-4 py-2"><dt class="text-muted-foreground">总名额</dt><dd class="font-semibold">{{ form.totalSeats }} 人车 · 已上车 {{ form.occupiedSeats }} 人</dd></div>
        <div class="flex justify-between gap-4 py-2"><dt class="text-muted-foreground">原帖状态</dt><dd class="font-semibold">{{ parsedTopic ? '已读取并绑定' : '待读取' }}</dd></div>
      </dl>
    </Card>

    <Card v-if="!previewOnly" class="p-5 shadow-sm">
      <div class="flex items-center justify-between">
        <h2 class="text-base font-semibold">发布完整度</h2>
        <span class="text-xs text-muted-foreground">{{ completenessPercent }}%</span>
      </div>
      <div class="mt-3 h-2 overflow-hidden rounded-full bg-muted">
        <div class="h-full rounded-full bg-primary" :style="{ width: `${completenessPercent}%` }" />
      </div>
      <div class="mt-4 space-y-2">
        <div v-for="item in completeness" :key="item.label" class="flex items-center gap-2 text-sm">
          <span
            class="grid h-5 w-5 place-items-center rounded-full text-[11px] font-semibold"
            :class="item.status === 'done' ? 'bg-success/10 text-success' : item.status === 'conflict' ? 'bg-warning/10 text-warning' : 'bg-muted text-muted-foreground'"
          >
            {{ item.status === 'done' ? '✓' : item.status === 'conflict' ? '!' : '·' }}
          </span>
          <span>{{ item.label }}</span>
        </div>
      </div>
      <div class="mt-4 hidden gap-2 lg:grid xl:grid-cols-2">
        <Button variant="outline" @click="emit('saveDraft')">保存草稿</Button>
        <Button :disabled="submitPending" @click="emit('submitReview')">{{ submitPending ? '发布中' : '发布车源' }}</Button>
      </div>
    </Card>

    <div v-if="reminders.length" class="space-y-2">
      <div v-for="reminder in reminders" :key="reminder" class="rounded-lg border border-warning/25 bg-warning/10 px-3 py-2 text-xs leading-5 text-warning">
        {{ reminder }}
      </div>
    </div>
  </aside>
</template>
