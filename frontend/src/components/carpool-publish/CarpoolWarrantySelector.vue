<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import type { CarpoolPublishForm, CarpoolWarrantyMode } from './types'
import PublishSectionCard from './PublishSectionCard.vue'

defineProps<{
  form: CarpoolPublishForm
  errors: Partial<Record<string, string>>
}>()

const options: Array<{ value: CarpoolWarrantyMode, title: string, description: string }> = [
  { value: 'no_warranty', title: '不作补偿承诺', description: '上游异常或封禁时车主不承诺补偿。' },
  { value: 'remaining_days_compensation', title: '车主按剩余天数补偿', description: '服务中断后，车主承诺按未使用天数退款或补时。' },
  { value: 'fixed_days_warranty', title: '固定天数车主承诺', description: '在约定期限内由车主提供补偿或替换。' },
]
</script>

<template>
  <PublishSectionCard
    :index="5"
    title="车主承诺与售后"
    description="使用结构化规则表达车主承诺口径；平台不担保、不代赔。"
  >
    <div class="grid gap-3 md:grid-cols-3">
      <button
        v-for="option in options"
        :key="option.value"
        type="button"
        class="min-h-24 rounded-lg border p-3 text-left transition"
        :class="form.warranty.mode === option.value ? 'border-primary bg-primary/10 ring-1 ring-primary' : 'border-border bg-background hover:bg-muted'"
        @click="form.warranty.mode = option.value"
      >
        <span class="block text-sm font-semibold">{{ option.title }}</span>
        <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
      </button>
    </div>

    <div v-if="form.warranty.mode !== 'no_warranty'" class="mt-4 grid gap-3 md:grid-cols-2">
      <label v-if="form.warranty.mode === 'fixed_days_warranty'" class="space-y-2">
        <span class="text-sm font-medium">承诺天数</span>
        <Input
          :model-value="form.warranty.fixedWarrantyDays ?? ''"
          type="number"
          min="1"
          placeholder="7"
          @update:model-value="value => form.warranty.fixedWarrantyDays = value === '' ? null : Number(value)"
        />
      </label>
      <label class="space-y-2" :class="form.warranty.mode === 'remaining_days_compensation' ? 'md:col-span-2' : ''">
        <span class="text-sm font-medium">补偿方式</span>
        <Input
          :model-value="form.warranty.compensationMethod ?? ''"
          placeholder="车主承诺按不可用天数补时或退还对应周期费用"
          @update:model-value="value => form.warranty.compensationMethod = String(value)"
        />
      </label>
      <label class="space-y-2 md:col-span-2">
        <span class="text-sm font-medium">不适用情形</span>
        <Textarea
          :model-value="form.warranty.exclusions ?? ''"
          class="min-h-20"
          placeholder="例如滥用、违反上游规则、个人网络环境异常等。"
          @update:model-value="value => form.warranty.exclusions = String(value)"
        />
      </label>
    </div>
    <p v-if="errors.warranty" class="mt-2 text-xs text-destructive">{{ errors.warranty }}</p>
  </PublishSectionCard>
</template>
