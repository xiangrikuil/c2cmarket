<script setup lang="ts">
import { Box } from 'lucide-vue-next'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import SubmitSectionHeader from './SubmitSectionHeader.vue'
import OfficialPriceCatalogCombobox from './OfficialPriceCatalogCombobox.vue'
import type { CarpoolProductCatalogItem } from '@/components/carpool-publish/types'
import type { OfficialPriceSubmitErrors, OfficialPriceSubmitForm } from './types'

defineProps<{
  form: OfficialPriceSubmitForm
  errors: OfficialPriceSubmitErrors
  catalog: CarpoolProductCatalogItem[]
  regionOptions: string[]
  channelOptions: string[]
}>()

defineEmits<{
  'select-product': [value: string]
  'select-plan': [plan: CarpoolProductCatalogItem]
  'select-custom-plan': [value: string]
}>()
</script>

<template>
  <section>
    <SubmitSectionHeader title="基础信息" hint="用于识别具体产品与适用区域" :icon="Box" />
    <div class="grid gap-4 md:grid-cols-2">
      <label class="space-y-2">
        <span class="text-sm font-medium">产品 <span class="text-destructive">*</span></span>
        <OfficialPriceCatalogCombobox
          mode="product"
          :model-value="form.product"
          :product-text="form.product"
          :product-plan-id="form.productPlanId"
          :catalog="catalog"
          @select-product="$emit('select-product', $event)"
          @select-plan="$emit('select-plan', $event)"
          @select-custom-plan="$emit('select-custom-plan', $event)"
        />
        <p v-if="errors.product" class="text-xs text-destructive">{{ errors.product }}</p>
      </label>
      <label class="space-y-2">
        <span class="text-sm font-medium">套餐 <span class="text-destructive">*</span></span>
        <OfficialPriceCatalogCombobox
          mode="plan"
          :model-value="form.plan"
          :product-text="form.product"
          :product-plan-id="form.productPlanId"
          :catalog="catalog"
          @select-product="$emit('select-product', $event)"
          @select-plan="$emit('select-plan', $event)"
          @select-custom-plan="$emit('select-custom-plan', $event)"
        />
        <p v-if="errors.plan" class="text-xs text-destructive">{{ errors.plan }}</p>
      </label>
      <label class="space-y-2">
        <span class="text-sm font-medium">国家 / 地区 <span class="text-destructive">*</span></span>
        <Select v-model="form.region">
          <SelectTrigger class="w-full"><SelectValue placeholder="选择国家或地区" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="item in regionOptions" :key="item" :value="item">{{ item }}</SelectItem>
          </SelectContent>
        </Select>
        <p v-if="errors.region" class="text-xs text-destructive">{{ errors.region }}</p>
      </label>
      <label class="space-y-2">
        <span class="text-sm font-medium">渠道 <span class="text-destructive">*</span></span>
        <Select v-model="form.channel">
          <SelectTrigger class="w-full"><SelectValue placeholder="选择渠道" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="item in channelOptions" :key="item" :value="item">{{ item }}</SelectItem>
          </SelectContent>
        </Select>
        <p v-if="errors.channel" class="text-xs text-destructive">{{ errors.channel }}</p>
      </label>
    </div>
  </section>
</template>
