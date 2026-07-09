<script setup lang="ts">
import { Badge } from '@/components/ui/badge'
import type { ApiProviderCategory } from './types'

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
          <h2>2. 模型范围</h2>
          <p class="mt-1 text-xs text-muted-foreground">GPT 与 Claude 需分开发；切换大类会自动校正不兼容模型。</p>
        </div>
        <Badge variant="model">已选 {{ selectedCount }} 个模型</Badge>
      </div>
    </div>

    <div class="api-publish-card-body">
        <div class="api-publish-provider-segment" role="group" aria-label="模型大类">
        <button
          v-for="option in options"
          :key="option.value"
          type="button"
          class="api-publish-segment-button"
          :class="{ 'is-active': modelValue === option.value }"
          @click="emit('update:modelValue', option.value)"
        >
          {{ option.title }}
        </button>
      </div>
    </div>
  </section>
</template>
