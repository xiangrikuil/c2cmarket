<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import type { ApiServicePublishForm, CatalogById } from './types'
import { apiServiceDetailPath } from './publishAssistant'
import { apiQuotaBoundaryNotice, distributionLabels, enabledPaymentOptions, formatMultiplier, generatedTitle, paymentMethodLabels, providerCategoryLabels, selectedCatalogItems, simplifiedApiQuotaRules } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  catalogById: CatalogById
  completeness: Array<{ label: string, status: 'done' | 'pending' | 'conflict' }>
  risks: string[]
  quotaForMinimumPurchase: string
  submittedId: string
  previewOnly?: boolean
}>()

const title = computed(() => generatedTitle(props.form, props.catalogById))
const merchantDisplayName = computed(() => props.form.merchantIdentityMode === 'store_alias' ? props.form.merchantDisplayName.trim() || '待设置商家展示名' : '公开个人身份')
const selectedModels = computed(() => selectedCatalogItems(props.form, props.catalogById))
const quotaExpiresAtLabel = computed(() => props.form.quotaExpiresAt ? props.form.quotaExpiresAt.replace('T', ' ') : '待填写')
const paymentSummary = computed(() => {
  const labels = enabledPaymentOptions(props.form).map(option => paymentMethodLabels[option.paymentMethod])
  return labels.length ? `${labels.join(' / ')} · 固定 ${props.form.paymentWindowMinutes} 分钟` : '待配置'
})
const pendingItems = computed(() => props.completeness.filter(item => item.status === 'pending'))
const conflictItems = computed(() => props.completeness.filter(item => item.status === 'conflict'))
const checkMessage = computed(() => {
  if (conflictItems.value.length) return `需处理：${conflictItems.value.map(item => item.label).join('、')}`
  if (pendingItems.value.length) return `还差：${pendingItems.value.map(item => item.label).join('、')}`
  return '必填项已完成，可以发布'
})
const submittedPath = computed(() => apiServiceDetailPath(props.submittedId))
</script>

<template>
  <aside :class="previewOnly ? 'min-w-0 space-y-3' : 'min-w-0 space-y-3 lg:sticky lg:top-16'">
    <div
      class="rounded-lg border px-3 py-2 text-xs leading-5"
      :class="conflictItems.length ? 'border-destructive/25 bg-destructive/5 text-destructive' : pendingItems.length ? 'border-warning/25 bg-warning/10 text-warning' : 'border-success/20 bg-success/5 text-success'"
    >
      {{ checkMessage }}
    </div>

    <Card class="api-publish-preview-card overflow-hidden shadow-sm">
      <div class="p-4">
        <div class="flex items-center justify-between gap-3">
          <div class="text-xs text-muted-foreground">买家预览</div>
          <Badge variant="model">API 额度</Badge>
        </div>
        <h2 class="mt-2 text-lg font-semibold leading-snug">{{ title }}</h2>
        <div class="mt-2 text-sm font-medium">{{ merchantDisplayName }}</div>
        <div class="mt-3 flex flex-wrap gap-1.5">
          <Badge variant="trust">信任等级3</Badge>
          <Badge variant="verified">已绑定 linux.do</Badge>
          <Badge v-if="form.merchantIdentityMode === 'store_alias'" variant="secondary">不公开社区用户名</Badge>
        </div>
      </div>

      <div class="grid gap-2 px-4 pb-4 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
        <div class="rounded-lg border border-border bg-muted/35 p-3">
          <div class="text-xs text-muted-foreground">售价</div>
          <div class="mt-1 text-lg font-semibold">¥{{ form.cnyPerUsdCredit ?? 0 }} / $1</div>
        </div>
        <div class="rounded-lg border border-border bg-muted/35 p-3">
          <div class="text-xs text-muted-foreground">可售额度</div>
          <div class="mt-1 text-lg font-semibold">${{ form.availableCreditUsd ?? 0 }}</div>
        </div>
      </div>

      <dl class="api-publish-preview-list px-4 pb-4 text-sm">
        <div><dt>展示身份</dt><dd>{{ form.merchantIdentityMode === 'store_alias' ? '商家展示名' : '公开个人身份' }}</dd></div>
        <div><dt>模型大类</dt><dd>{{ providerCategoryLabels[form.providerCategory] }}</dd></div>
        <div><dt>模型</dt><dd>{{ selectedModels.map(item => item.displayName).join(' / ') || '待选择' }}</dd></div>
        <div><dt>¥{{ simplifiedApiQuotaRules.minimumPurchaseCny }} 约可购</dt><dd>{{ quotaForMinimumPurchase }}</dd></div>
        <div><dt>有效至</dt><dd>{{ quotaExpiresAtLabel }}</dd></div>
        <div><dt>收款方式</dt><dd>{{ paymentSummary }}</dd></div>
        <div><dt>接入类型</dt><dd>{{ distributionLabels[form.distributionSystem] }}</dd></div>
        <div><dt>服务倍率</dt><dd>{{ form.distributionSystem === 'sub2api' ? '1.00x' : formatMultiplier(form.defaultMultiplier) }}</dd></div>
        <div><dt>用量核对</dt><dd>商户说明，买家自行核对</dd></div>
        <div><dt>平台边界</dt><dd>不担保、不代赔</dd></div>
      </dl>

      <div class="border-t border-border px-4 py-3">
        <div class="text-xs font-medium text-muted-foreground">备注信息</div>
        <p class="mt-2 whitespace-pre-line text-sm leading-6">{{ form.merchantNote || '待填写备注信息' }}</p>
      </div>

      <div class="border-t border-border bg-muted/35 px-4 py-3 text-xs leading-5 text-muted-foreground">
        {{ apiQuotaBoundaryNotice }}
      </div>
    </Card>

    <Card v-if="submittedPath" class="api-publish-card !gap-3 !p-4">
      <div class="text-sm font-semibold text-emerald-800">已发布</div>
      <p class="text-xs leading-5 text-muted-foreground">可以打开服务详情检查前台展示效果。</p>
      <RouterLink :to="submittedPath" class="block">
        <Button size="sm" class="w-full">查看已发布服务</Button>
      </RouterLink>
    </Card>

    <div v-if="risks.length" class="space-y-2">
      <div v-for="risk in risks" :key="risk" class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
        {{ risk }}
      </div>
    </div>
  </aside>
</template>
