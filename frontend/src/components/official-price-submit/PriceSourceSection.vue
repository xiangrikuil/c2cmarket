<script setup lang="ts">
import { CreditCard, Check, CircleAlert } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import SubmitSectionHeader from './SubmitSectionHeader.vue'
import type { OfficialPriceSubmitErrors, OfficialPriceSubmitForm, SourceLinkState } from './types'

defineProps<{
  form: OfficialPriceSubmitForm
  errors: OfficialPriceSubmitErrors
  currencyOptions: string[]
  openingMethodOptions: string[]
  sourceLinkState: SourceLinkState
  sourceHost: string
}>()
</script>

<template>
  <section class="border-t border-border pt-5">
    <SubmitSectionHeader title="价格与来源" hint="建议填写含税价格，并提供可访问来源" :icon="CreditCard" />
    <div class="grid gap-4 md:grid-cols-2">
      <label class="space-y-2">
        <span class="text-sm font-medium">原币价格 <span class="text-destructive">*</span></span>
        <div class="flex overflow-hidden rounded-md border border-input bg-background focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50">
          <Select v-model="form.originalPriceCurrency">
            <SelectTrigger class="h-10 w-24 border-0 shadow-none focus:ring-0">
              <SelectValue placeholder="币种" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="item in currencyOptions" :key="item" :value="item">{{ item }}</SelectItem>
            </SelectContent>
          </Select>
          <Input
            v-model="form.originalPriceAmount"
            class="h-10 border-0 border-l border-border shadow-none focus-visible:ring-0"
            inputmode="decimal"
            placeholder="7,990"
          />
        </div>
        <p v-if="errors.originalPrice" class="text-xs text-destructive">{{ errors.originalPrice }}</p>
      </label>
      <label class="space-y-2">
        <span class="text-sm font-medium">开通方式 <span class="text-destructive">*</span></span>
        <Select v-model="form.openingMethod">
          <SelectTrigger class="w-full"><SelectValue placeholder="选择开通方式" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="item in openingMethodOptions" :key="item" :value="item">{{ item }}</SelectItem>
          </SelectContent>
        </Select>
        <p v-if="errors.openingMethod" class="text-xs text-destructive">{{ errors.openingMethod }}</p>
      </label>
      <label class="space-y-2 md:col-span-2">
        <span class="text-sm font-medium">linux.do 低价帖 / 来源链接 <span class="text-destructive">*</span></span>
        <div class="relative">
          <Input v-model="form.sourceUrl" class="pr-32" placeholder="https://linux.do/t/..." />
          <span
            v-if="sourceLinkState !== 'idle'"
            class="absolute right-2 top-1/2 inline-flex -translate-y-1/2 items-center gap-1 rounded-full px-2 py-1 text-xs font-medium"
            :class="sourceLinkState === 'success' ? 'bg-success/10 text-success' : 'bg-destructive/10 text-destructive'"
          >
            <Check v-if="sourceLinkState === 'success'" class="h-3 w-3" />
            <CircleAlert v-else class="h-3 w-3" />
            {{ sourceLinkState === 'success' ? '链接格式有效' : '格式不合法' }}
          </span>
        </div>
        <p v-if="errors.sourceUrl" class="text-xs text-destructive">{{ errors.sourceUrl }}</p>
        <p v-else class="text-xs text-muted-foreground">
          系统将自动识别来源域名、标题与发布时间。<span v-if="sourceHost">当前来源：{{ sourceHost }}</span>
        </p>
      </label>
    </div>
  </section>
</template>
