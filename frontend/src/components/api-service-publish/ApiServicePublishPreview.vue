<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import type { ApiServicePublishForm, CatalogById } from './types'
import { billingLabels, deliveryLabels, distributionLabels, formatMultiplier, generatedTitle, providerCategoryLabels, selectedCatalogItems, sub2ApiPricingPolicy, usageLabels, warrantyLabel } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  catalogById: CatalogById
  completeness: Array<{ label: string, status: 'done' | 'pending' | 'conflict' }>
  risks: string[]
  quotaForMinimumPurchase: string
}>()

const title = computed(() => generatedTitle(props.form, props.catalogById))
const merchantDisplayName = computed(() => props.form.merchantIdentityMode === 'store_alias' ? props.form.merchantDisplayName.trim() || '待填写商家展示名' : '公开个人身份')
const selectedModels = computed(() => selectedCatalogItems(props.form, props.catalogById))
const supportsImage = computed(() => props.form.imageCapability.enabled && (props.form.imageCapability.supportsTextToImage || props.form.imageCapability.supportsImageToImage))
const imageCapabilityText = computed(() => {
  const values: string[] = []
  if (props.form.imageCapability.supportsTextToImage) values.push('文生图')
  if (props.form.imageCapability.supportsImageToImage) values.push('图生图')
  return values.join(' / ') || '待配置'
})
const completenessPercent = computed(() => {
  if (!props.completeness.length) return 0
  return Math.round((props.completeness.filter(item => item.status === 'done').length / props.completeness.length) * 100)
})
</script>

<template>
  <aside class="min-w-0 space-y-3 lg:sticky lg:top-16">
    <Card class="api-publish-preview-card overflow-hidden shadow-sm">
      <div class="p-4">
        <div class="text-xs text-muted-foreground">发布预览</div>
        <h2 class="mt-2 text-lg font-semibold leading-snug">{{ title }}</h2>
        <div class="mt-2 text-sm font-medium">{{ merchantDisplayName }}</div>
        <p v-if="form.shortDescription" class="mt-1 text-sm text-muted-foreground">{{ form.shortDescription }}</p>
        <div class="mt-3 flex flex-wrap gap-1.5">
          <Badge variant="identity">个人商户</Badge>
          <Badge variant="trust">信任等级3</Badge>
          <Badge variant="verified">已绑定 linux.do</Badge>
          <Badge v-if="form.merchantIdentityMode === 'store_alias'" variant="secondary">不公开社区用户名</Badge>
          <Badge v-if="supportsImage" variant="verified">支持生图</Badge>
          <Badge v-else variant="model">不支持生图</Badge>
        </div>
      </div>

      <dl class="api-publish-preview-list px-4 pb-4 text-sm">
        <div><dt>展示身份</dt><dd>{{ form.merchantIdentityMode === 'store_alias' ? '商家展示名' : '公开个人身份' }}</dd></div>
        <div><dt>分发系统</dt><dd>{{ distributionLabels[form.distributionSystem] }}</dd></div>
        <div><dt>模型大类</dt><dd>{{ providerCategoryLabels[form.providerCategory] }}</dd></div>
        <div><dt>模型</dt><dd>{{ selectedModels.map(item => item.displayName).join(' / ') || '待选择' }}</dd></div>
        <div><dt>计费</dt><dd>{{ billingLabels[form.billingMode] }}</dd></div>
        <template v-if="form.distributionSystem === 'sub2api'">
          <div><dt>美元额度售价</dt><dd>¥{{ form.cnyPerUsdCredit ?? 0 }} / $1</dd></div>
          <div><dt>文本倍率</dt><dd>{{ formatMultiplier(sub2ApiPricingPolicy.textModelMultiplier) }} 固定</dd></div>
          <div v-if="supportsImage"><dt>生图</dt><dd>{{ imageCapabilityText }}</dd></div>
          <div v-if="supportsImage"><dt>生图倍率</dt><dd>{{ formatMultiplier(sub2ApiPricingPolicy.imageMultiplier) }} 固定</dd></div>
          <div><dt>最低意向上限</dt><dd>{{ quotaForMinimumPurchase }}</dd></div>
        </template>
        <template v-else>
          <div><dt>默认倍率</dt><dd>{{ formatMultiplier(form.defaultMultiplier) }}</dd></div>
        </template>
        <div><dt>接入方式</dt><dd>{{ form.deliveryModes.map(mode => deliveryLabels[mode]).join(' / ') || '待选择' }}</dd></div>
        <div><dt>用量</dt><dd>{{ usageLabels[form.usageVisibility] }}</dd></div>
        <div><dt>有效期</dt><dd>{{ form.validity.mode === 'permanent' ? '永久' : `站外确认后 ${form.validity.days ?? 0} 天` }}</dd></div>
        <div><dt>商户承诺</dt><dd>{{ warrantyLabel(form.warranty) }} · 平台不担保、不代赔</dd></div>
        <div><dt>最低意向</dt><dd>{{ form.billingMode === 'fixed_package' ? '按套餐' : `¥${form.minimumPurchaseCny ?? 0} 起` }}</dd></div>
      </dl>

    </Card>

    <Card class="api-publish-card !gap-3 !p-4">
      <div class="flex items-center justify-between">
        <h2 class="text-base font-semibold">发布完整度</h2>
        <span class="text-xs text-muted-foreground">{{ completenessPercent }}%</span>
      </div>
      <div class="mt-3 h-2 overflow-hidden rounded-full bg-muted">
        <div class="h-full rounded-full bg-primary" :style="{ width: `${completenessPercent}%` }" />
      </div>
      <div class="mt-3 space-y-2">
        <div v-for="item in completeness" :key="item.label" class="flex items-center gap-2 text-sm">
          <span
            class="grid h-5 w-5 place-items-center rounded-full text-[11px] font-semibold"
            :class="item.status === 'done' ? 'bg-emerald-50 text-emerald-700' : item.status === 'conflict' ? 'bg-amber-50 text-amber-700' : 'bg-muted text-muted-foreground'"
          >
            {{ item.status === 'done' ? '✓' : item.status === 'conflict' ? '!' : '·' }}
          </span>
          <span>{{ item.label }}</span>
        </div>
      </div>
    </Card>

    <div v-if="risks.length" class="space-y-2">
      <div v-for="risk in risks" :key="risk" class="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
        {{ risk }}
      </div>
    </div>
  </aside>
</template>
