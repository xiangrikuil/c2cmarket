<script setup lang="ts">
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import type { ApiServicePublishForm, WarrantyMode } from './types'

defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const options: Array<{ value: WarrantyMode, title: string, description: string }> = [
  { value: 'no_warranty', title: '不作承诺', description: '上游封禁、停服或不可用时，商户不承诺补偿。' },
  { value: 'upstream_refund_only', title: '上游退款跟随', description: '仅在上游实际退款后，商户按规则处理买家剩余额度。' },
  { value: 'merchant_warranty', title: '商户承诺', description: '配置承诺天数、适用范围和补偿方式；平台不担保、不代赔。' },
]

const templates = [
  '建议首次提交 ¥20 意向测试',
  '站外确认后按所选接入方式使用',
  '高峰期响应可能变慢',
  '部分模型可能临时维护',
  '禁止滥用或高并发压测',
  '平台不担保、不代赔',
  '图像生成按平台只读价格展示',
]

function insertTemplate(form: ApiServicePublishForm, value: string) {
  if (form.merchantNote.includes(value)) return
  form.merchantNote = [form.merchantNote.trim(), value].filter(Boolean).join('；')
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>7. 商户承诺与买家须知</h2>
      <p>Sub2API 标准额度至少选择一项商户承诺，买家须知用于前台展示提交意向前注意事项；平台不担保、不代赔。</p>
    </div>

    <div class="api-publish-card-body space-y-4">
      <div class="grid gap-3 md:grid-cols-3">
        <button
          v-for="option in options"
          :key="option.value"
          type="button"
          class="api-publish-warranty-option"
          :class="{ 'is-active': form.warranty.mode === option.value }"
          @click="form.warranty.mode = option.value"
        >
          <span class="block text-sm font-semibold">{{ option.title }}</span>
          <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
        </button>
      </div>
      <p v-if="errors.warranty && form.warranty.mode === 'no_warranty'" class="text-xs text-destructive">{{ errors.warranty }}</p>

      <div v-if="form.warranty.mode === 'upstream_refund_only'" class="space-y-2">
        <label class="text-sm font-medium">退款处理说明</label>
        <Textarea
          :model-value="form.warranty.refundNote ?? ''"
          class="min-h-20"
          placeholder="说明上游退款后的处理口径。"
          @update:model-value="value => form.warranty.refundNote = String(value)"
        />
        <p v-if="errors.warranty" class="text-xs text-destructive">{{ errors.warranty }}</p>
      </div>

      <div v-if="form.warranty.mode === 'merchant_warranty'" class="grid gap-3 md:grid-cols-2">
        <label class="space-y-2">
          <span class="text-sm font-medium">承诺天数</span>
          <Input :model-value="form.warranty.warrantyDays ?? ''" placeholder="7" @update:model-value="value => form.warranty.warrantyDays = Number(value)" />
        </label>
        <label class="space-y-2">
          <span class="text-sm font-medium">适用范围</span>
          <Input
            :model-value="form.warranty.coverage ?? ''"
            placeholder="接口不可用、余额异常等"
            @update:model-value="value => form.warranty.coverage = String(value)"
          />
        </label>
        <label class="space-y-2">
          <span class="text-sm font-medium">补偿方式</span>
          <Input
            :model-value="form.warranty.compensation ?? ''"
            placeholder="商户承诺按不可用时长补偿额度"
            @update:model-value="value => form.warranty.compensation = String(value)"
          />
        </label>
        <label class="space-y-2">
          <span class="text-sm font-medium">不适用情形</span>
          <Input
            :model-value="form.warranty.exclusions ?? ''"
            placeholder="滥用、高并发压测、上游策略变动等"
            @update:model-value="value => form.warranty.exclusions = String(value)"
          />
        </label>
        <p v-if="errors.warranty" class="text-xs text-destructive md:col-span-2">{{ errors.warranty }}</p>
      </div>

      <div class="space-y-2">
        <label class="text-sm font-medium">买家须知</label>
        <Textarea
          v-model="form.merchantNote"
          class="min-h-28"
          maxlength="800"
          placeholder="请填写提交意向前需要确认的事项。"
        />
        <div class="flex items-center justify-between gap-3">
          <p v-if="errors.merchantNote" class="text-xs text-destructive">{{ errors.merchantNote }}</p>
          <p class="ml-auto text-xs text-muted-foreground">已输入 {{ form.merchantNote.length }} / 800 字</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="template in templates"
            :key="template"
            type="button"
            class="rounded-full border border-border bg-background px-3 py-1 text-xs hover:bg-muted"
            @click="insertTemplate(form, template)"
          >
            + {{ template }}
          </button>
        </div>
      </div>
    </div>
  </Card>
</template>
