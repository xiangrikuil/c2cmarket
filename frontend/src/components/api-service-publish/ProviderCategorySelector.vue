<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import type { ApiProviderCategory } from './types'
import { providerCategoryLabels } from './utils'

defineProps<{
  modelValue: ApiProviderCategory
  selectedCount: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ApiProviderCategory]
}>()

const options: Array<{ value: ApiProviderCategory, title: string, description: string, detail: string }> = [
  {
    value: 'gpt',
    title: 'GPT',
    description: '可选择多个 GPT / OpenAI 模型。',
    detail: 'Sub2API 模式下可配置文生图和图生图能力。',
  },
  {
    value: 'claude',
    title: 'Claude',
    description: '可选择多个 Claude 模型。',
    detail: '不显示 GPT 图像生成配置。',
  },
  {
    value: 'other',
    title: '其他',
    description: '适用于 Gemini、其他代理模型或人工审核目录。',
    detail: '进入审核时需要说明模型来源和用量查看方式。',
  },
]
</script>

<template>
  <section class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2>2. 模型大类</h2>
          <p class="mt-1 text-xs text-muted-foreground">每条服务只能选择一个模型大类；GPT 与 Claude 必须分开发布。</p>
        </div>
        <Badge variant="model">{{ providerCategoryLabels[modelValue] }}</Badge>
      </div>
    </div>

    <div class="api-publish-card-body">
      <div class="api-publish-provider-grid">
        <button
          v-for="option in options"
          :key="option.value"
          type="button"
          class="api-publish-option-card"
          :class="{ 'is-active': modelValue === option.value }"
          @click="emit('update:modelValue', option.value)"
        >
          <span class="block text-sm font-semibold">{{ option.title }}</span>
          <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ option.description }}</span>
          <span class="mt-2 block text-[11px] leading-5 text-muted-foreground">{{ option.detail }}</span>
        </button>
      </div>

      <div class="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
        GPT 与 Claude 必须分开发布，不能同时存在于同一服务中。切换模型大类会清空不兼容模型；当前已选择 {{ selectedCount }} 个模型。
      </div>
    </div>
  </section>
</template>
