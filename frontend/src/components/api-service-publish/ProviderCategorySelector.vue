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
    detail: '适合出售 OpenAI 兼容额度。',
  },
  {
    value: 'claude',
    title: 'Claude',
    description: '可选择多个 Claude 模型。',
    detail: '适合出售 Anthropic 兼容额度。',
  },
  {
    value: 'other',
    title: '其他',
    description: '适用于 Gemini、其他代理模型或人工审核目录。',
    detail: '接入细节在备注中说明。',
  },
]
</script>

<template>
  <section class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2>2. 出售模型</h2>
          <p class="mt-1 text-xs text-muted-foreground">选择出售的模型大类和具体模型；GPT 与 Claude 需要分开发布。</p>
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
        当前已选择 {{ selectedCount }} 个模型。切换模型大类会清空不兼容模型。
      </div>
    </div>
  </section>
</template>
