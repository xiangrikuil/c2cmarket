<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { computed } from 'vue'
import type { CarpoolProductCatalogItem, CarpoolPublishForm, PublishFieldState, RegionOption } from './types'
import CarpoolProductCombobox from './CarpoolProductCombobox.vue'
import PublishSectionCard from './PublishSectionCard.vue'
import { quotaFieldLabel } from '@/lib/quota'

const props = defineProps<{
  form: CarpoolPublishForm
  catalog: CarpoolProductCatalogItem[]
  regions: RegionOption[]
  errors: Partial<Record<string, string>>
  fieldStates?: Partial<Record<string, PublishFieldState>>
  highlightedKey?: string
}>()

const selectedProduct = computed(() => props.catalog.find(item => item.id === props.form.productId) ?? null)
const quotaLabel = computed(() => quotaFieldLabel(selectedProduct.value))
const quotaUnit = computed(() => selectedProduct.value?.quotaUnit || 'USD')

function fieldState(key: string): PublishFieldState {
  return props.fieldStates?.[key] ?? 'idle'
}

function fieldShellClass(key: string) {
  const state = fieldState(key)
  return [
    'rounded-lg border p-3 transition-colors',
    state === 'error' ? 'border-destructive/45 bg-destructive/5' : '',
    state === 'pendingRequired' ? 'border-warning/40 bg-warning/5' : '',
    state === 'defaulted' ? 'border-success/35 bg-success/5' : '',
    state === 'complete' ? 'border-border bg-background' : '',
    state === 'idle' ? 'border-transparent bg-transparent p-0' : '',
    props.highlightedKey === key ? 'ring-2 ring-primary/60 ring-offset-2 ring-offset-background' : '',
  ]
}

function stateLabel(key: string) {
  const state = fieldState(key)
  if (state === 'error') return '需要处理'
  if (state === 'pendingRequired') return '待填写'
  if (state === 'defaulted') return '系统默认'
  if (state === 'complete') return '已完成'
  return ''
}

function stateLabelClass(key: string) {
  const state = fieldState(key)
  if (state === 'error') return 'bg-destructive/10 text-destructive'
  if (state === 'pendingRequired') return 'bg-warning/10 text-warning'
  if (state === 'defaulted') return 'bg-success/10 text-success'
  if (state === 'complete') return 'bg-success/10 text-success'
  return 'bg-muted text-muted-foreground'
}
</script>

<template>
  <PublishSectionCard
    :index="1"
    title="基础信息"
    description="选择要发布的订阅产品，并补充地区、月费、倍率和每月额度。"
  >
    <div class="grid gap-4 md:grid-cols-2">
      <div id="carpool-task-product" :class="fieldShellClass('product')">
        <div class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>产品 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('product')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('product')">{{ stateLabel('product') }}</span>
        </div>
        <div class="mt-2">
          <CarpoolProductCombobox
            v-model="form.productId"
            :custom-product-name="form.customProductName"
            :catalog="catalog"
            @update:custom-product-name="value => form.customProductName = value"
          />
        </div>
        <p v-if="errors.product" class="text-xs text-destructive">{{ errors.product }}</p>
        <p v-else-if="fieldState('product') === 'pendingRequired'" class="mt-2 text-xs text-warning">选择套餐目录后，系统会同步访问安排和风险提示。</p>
      </div>

      <label id="carpool-task-region" class="space-y-2" :class="fieldShellClass('region')">
        <span class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>开通区 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('region')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('region')">{{ stateLabel('region') }}</span>
        </span>
        <Select v-model="form.regionCode">
          <SelectTrigger class="w-full bg-background"><SelectValue placeholder="选择开通区" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="region in regions" :key="region.code" :value="region.code">{{ region.displayName }}</SelectItem>
          </SelectContent>
        </Select>
        <p v-if="errors.region" class="text-xs text-destructive">{{ errors.region }}</p>
        <p v-else-if="fieldState('region') === 'pendingRequired'" class="text-xs text-warning">请选择买家实际开通或使用的地区。</p>
      </label>

      <label id="carpool-task-monthlyPrice" class="space-y-2" :class="fieldShellClass('monthlyPrice')">
        <span class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>月费 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('monthlyPrice')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('monthlyPrice')">{{ stateLabel('monthlyPrice') }}</span>
        </span>
        <Input
          :model-value="form.monthlyPriceCny ?? ''"
          type="number"
          min="0"
          placeholder="68"
          @update:model-value="value => form.monthlyPriceCny = value === '' ? null : Number(value)"
        />
        <p v-if="errors.monthlyPriceCny" class="text-xs text-destructive">{{ errors.monthlyPriceCny }}</p>
        <p v-else class="text-xs" :class="fieldState('monthlyPrice') === 'pendingRequired' ? 'text-warning' : 'text-muted-foreground'">默认按人民币 / 月展示。</p>
      </label>

      <label class="space-y-2">
        <span class="block text-sm font-medium">计费周期</span>
        <Input model-value="月付" readonly />
        <p class="text-xs text-muted-foreground">拼车列表默认使用月费比较。</p>
      </label>

      <label id="carpool-task-serviceMultiplier" class="space-y-2" :class="fieldShellClass('serviceMultiplier')">
        <span class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>倍率 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('serviceMultiplier')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('serviceMultiplier')">{{ stateLabel('serviceMultiplier') }}</span>
        </span>
        <Input
          :model-value="form.serviceMultiplier ?? ''"
          type="number"
          min="0.01"
          step="0.01"
          placeholder="1.35"
          @update:model-value="value => form.serviceMultiplier = value === '' ? null : Number(value)"
        />
        <p v-if="errors.serviceMultiplier" class="text-xs text-destructive">{{ errors.serviceMultiplier }}</p>
        <p class="text-xs text-muted-foreground">例如 1.35x，表示车主声明的使用或折算倍率。</p>
      </label>

      <div id="carpool-task-monthlyQuota" class="space-y-2 md:col-span-2" :class="fieldShellClass('monthlyQuota')">
        <span class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>{{ quotaLabel }} <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('monthlyQuota')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('monthlyQuota')">{{ stateLabel('monthlyQuota') }}</span>
        </span>
        <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_80px]">
          <Input
            :model-value="form.monthlyQuotaAmount ?? ''"
            type="number"
            min="0.01"
            step="1"
            placeholder="200"
            @update:model-value="value => form.monthlyQuotaAmount = value === '' ? null : Number(value)"
          />
          <Input :model-value="quotaUnit" readonly />
        </div>
        <p v-if="errors.monthlyQuota" class="text-xs text-destructive">{{ errors.monthlyQuota }}</p>
        <p v-else class="text-xs" :class="fieldState('monthlyQuota') === 'pendingRequired' ? 'text-warning' : 'text-muted-foreground'">每人每月额度由套餐目录配置单位；平台不展示总额度，也不做资源池拆分。</p>
      </div>
    </div>
  </PublishSectionCard>
</template>
