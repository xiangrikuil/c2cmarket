<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { ApiQuotaUsagePolicyInput, ApiWritableQuotaLimitMode } from '@/types/apiQuota'

const props = defineProps<{
  modelValue: ApiQuotaUsagePolicyInput
  error?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ApiQuotaUsagePolicyInput]
}>()

type Period = keyof ApiQuotaUsagePolicyInput
const periods: Period[] = ['fiveHour', 'daily']

function updateMode(period: Period, value: unknown) {
  const mode: ApiWritableQuotaLimitMode = value === 'limited' ? 'limited' : 'unlimited'
  emit('update:modelValue', {
    ...props.modelValue,
    [period]: mode === 'limited'
      ? { mode, amountUsd: props.modelValue[period].amountUsd ?? '' }
      : { mode },
  })
}

function updateAmount(period: Period, value: string | number) {
  emit('update:modelValue', {
    ...props.modelValue,
    [period]: { mode: 'limited', amountUsd: String(value) },
  })
}
</script>

<template>
  <fieldset class="space-y-3">
    <legend class="text-sm font-semibold">额度使用规则</legend>
    <div class="grid gap-3 sm:grid-cols-2">
      <div v-for="period in periods" :key="period" class="space-y-2">
        <label class="text-xs font-medium" :for="`quota-${period}-mode`">{{ period === 'fiveHour' ? '5h 限额' : '每日限额' }}</label>
        <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-2">
          <Select :model-value="modelValue[period].mode" @update:model-value="value => updateMode(period, value)">
            <SelectTrigger :id="`quota-${period}-mode`"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="limited">金额上限</SelectItem>
              <SelectItem value="unlimited">不限</SelectItem>
            </SelectContent>
          </Select>
          <div v-if="modelValue[period].mode === 'limited'" class="flex overflow-hidden rounded-md border border-input bg-background">
            <span class="grid w-8 shrink-0 place-items-center border-r border-border text-sm text-muted-foreground">$</span>
            <Input
              :model-value="modelValue[period].amountUsd ?? ''"
              class="border-0 shadow-none focus-visible:ring-0"
              inputmode="decimal"
              placeholder="50"
              @update:model-value="value => updateAmount(period, value)"
            />
          </div>
          <div v-else class="flex h-10 items-center rounded-md border border-border bg-muted/35 px-3 text-xs text-muted-foreground">明确不限</div>
        </div>
      </div>
    </div>
    <p v-if="error" class="text-xs text-destructive" role="alert">{{ error }}</p>
    <p v-else class="text-xs leading-5 text-muted-foreground">金额为模型倍率计费后的美元额度；每份买家凭据独立适用，每日按 UTC+8 自然日重置。</p>
  </fieldset>
</template>
