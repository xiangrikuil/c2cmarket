<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { computed } from 'vue'
import type { CarpoolProductCatalogItem, CarpoolPublishForm, RegionOption } from './types'
import CarpoolProductCombobox from './CarpoolProductCombobox.vue'
import PublishSectionCard from './PublishSectionCard.vue'
import { quotaFieldLabel } from '@/lib/quota'

const props = defineProps<{
  form: CarpoolPublishForm
  catalog: CarpoolProductCatalogItem[]
  regions: RegionOption[]
  errors: Partial<Record<string, string>>
}>()

const selectedProduct = computed(() => props.catalog.find(item => item.id === props.form.productId) ?? null)
const quotaLabel = computed(() => quotaFieldLabel(selectedProduct.value))
const quotaUnit = computed(() => selectedProduct.value?.quotaUnit || 'USD')
</script>

<template>
  <PublishSectionCard
    :index="2"
    title="基础信息"
    description="选择要发布的订阅产品，并补充地区、月费、倍率和每月额度。"
  >
    <div class="grid gap-4 md:grid-cols-2">
      <div class="space-y-2">
        <div class="flex items-center gap-2 text-sm font-medium">
          产品 <span class="text-xs text-primary">必填</span>
        </div>
        <CarpoolProductCombobox
          v-model="form.productId"
          :custom-product-name="form.customProductName"
          :catalog="catalog"
          @update:custom-product-name="value => form.customProductName = value"
        />
        <p v-if="errors.product" class="text-xs text-destructive">{{ errors.product }}</p>
      </div>

      <label class="space-y-2">
        <span class="block text-sm font-medium">开通区 <span class="text-xs text-primary">必填</span></span>
        <Select v-model="form.regionCode">
          <SelectTrigger class="w-full bg-background"><SelectValue placeholder="选择开通区" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="region in regions" :key="region.code" :value="region.code">{{ region.displayName }}</SelectItem>
          </SelectContent>
        </Select>
        <p v-if="errors.region" class="text-xs text-destructive">{{ errors.region }}</p>
      </label>

      <label class="space-y-2">
        <span class="block text-sm font-medium">月费 <span class="text-xs text-primary">必填</span></span>
        <Input
          :model-value="form.monthlyPriceCny ?? ''"
          type="number"
          min="0"
          placeholder="68"
          @update:model-value="value => form.monthlyPriceCny = value === '' ? null : Number(value)"
        />
        <p v-if="errors.monthlyPriceCny" class="text-xs text-destructive">{{ errors.monthlyPriceCny }}</p>
        <p class="text-xs text-muted-foreground">默认按人民币 / 月展示。</p>
      </label>

      <label class="space-y-2">
        <span class="block text-sm font-medium">计费周期</span>
        <Input model-value="月付" readonly />
        <p class="text-xs text-muted-foreground">拼车列表默认使用月费比较。</p>
      </label>

      <label class="space-y-2">
        <span class="block text-sm font-medium">倍率 <span class="text-xs text-primary">必填</span></span>
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

      <div class="space-y-2">
        <span class="block text-sm font-medium">{{ quotaLabel }} <span class="text-xs text-primary">必填</span></span>
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
        <p class="text-xs text-muted-foreground">每人每月额度由套餐目录配置单位；平台不展示总额度，也不做资源池拆分。</p>
      </div>
    </div>
  </PublishSectionCard>
</template>
