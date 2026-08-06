<script setup lang="ts">
import { Blocks } from 'lucide-vue-next'
import type { AcceptableValue } from 'reka-ui'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { getProductCategoryIconSrc } from '@/lib/productCategoryIcon'
import type { ApiProviderCategory } from './types'

defineProps<{
  modelValue: ApiProviderCategory
  selectedCount: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: ApiProviderCategory]
}>()

function selectCategory(value: AcceptableValue) {
  if (value === 'gpt' || value === 'claude' || value === 'other') emit('update:modelValue', value)
}

const fallbackCategoryIcons = new Map<string, string>()
const options: Array<{ value: ApiProviderCategory, title: string, description: string, detail: string, iconSrc: string | null }> = [
  {
    value: 'gpt',
    title: 'GPT',
    description: '可选择多个 GPT / OpenAI 模型。',
    detail: '适合出售 OpenAI 兼容额度。',
    iconSrc: getProductCategoryIconSrc('gpt', fallbackCategoryIcons),
  },
  {
    value: 'claude',
    title: 'Claude',
    description: '可选择多个 Claude 模型。',
    detail: '适合出售 Anthropic 兼容额度。',
    iconSrc: getProductCategoryIconSrc('claude', fallbackCategoryIcons),
  },
  {
    value: 'other',
    title: '其他',
    description: '适用于 Gemini、其他代理模型或人工审核目录。',
    detail: '接入细节在备注中说明。',
    iconSrc: getProductCategoryIconSrc('other', fallbackCategoryIcons),
  },
]
</script>

<template>
  <section class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="flex items-start gap-2">
          <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-indigo-50 text-indigo-600">
            <Blocks class="h-4 w-4" />
          </span>
          <div>
            <h2>模型范围</h2>
            <p>GPT 与 Claude 需分开发；切换大类会校正不兼容模型。</p>
          </div>
        </div>
        <Badge variant="model">已选 {{ selectedCount }} 个模型</Badge>
      </div>
    </div>

    <div class="api-publish-card-body">
      <RadioGroup
        :model-value="modelValue"
        class="api-publish-provider-segment"
        aria-label="模型大类"
        @update:model-value="selectCategory"
      >
        <Label
          v-for="option in options"
          :key="option.value"
          :for="`provider-category-${option.value}`"
          class="api-publish-segment-button inline-flex cursor-pointer items-center justify-center gap-2 font-normal"
          :class="{ 'is-active': modelValue === option.value }"
        >
          <RadioGroupItem :id="`provider-category-${option.value}`" :value="option.value" />
          <img v-if="option.iconSrc" :src="option.iconSrc" alt="" class="h-4 w-4 object-contain" />
          <Blocks v-else class="h-4 w-4" />
          <span>{{ option.title }}</span>
        </Label>
      </RadioGroup>
    </div>
  </section>
</template>
