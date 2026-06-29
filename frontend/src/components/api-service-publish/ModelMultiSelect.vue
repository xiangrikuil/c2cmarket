<script setup lang="ts">
import { computed, ref } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import type { ModelCatalogItem } from '@/lib/api'
import type { ApiProviderCategory, ApiServicePublishForm } from './types'
import { capabilityLabel, modelProviderCategory, providerCategoryLabels, providerLabel } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  providerCategory: ApiProviderCategory
  catalog: ModelCatalogItem[]
  errors: Partial<Record<string, string>>
}>()

const emit = defineEmits<{
  addModel: [id: string]
}>()

const keyword = ref('')
const filteredGroups = computed(() => {
  const normalized = keyword.value.trim().toLowerCase()
  const rows = props.catalog.filter(item => {
    const selected = props.form.selectedModels.some(model => model.modelId === item.id && model.enabled)
    const matched = !normalized || [item.displayName, item.name, providerLabel(item.provider)].some(value => value.toLowerCase().includes(normalized))
    return !selected && matched && modelProviderCategory(item.provider) === props.providerCategory
  })
  return ['openai', 'anthropic', 'other'].map(provider => ({
    provider,
    label: providerLabel(provider as ModelCatalogItem['provider']),
    rows: rows.filter(item => item.provider === provider),
  })).filter(group => group.rows.length)
})
</script>

<template>
  <div class="rounded-lg border border-border bg-background">
    <div class="border-b border-border p-3">
      <Input v-model="keyword" :placeholder="`搜索并添加 ${providerCategoryLabels[providerCategory]} 模型`" />
      <p v-if="errors.selectedModels" class="mt-2 text-xs text-destructive">{{ errors.selectedModels }}</p>
    </div>
    <div class="max-h-64 overflow-y-auto p-3">
      <div v-if="!filteredGroups.length" class="text-sm text-muted-foreground">当前模型大类没有可添加的模型。</div>
      <div v-for="group in filteredGroups" :key="group.provider" class="mb-4 last:mb-0">
        <div class="mb-2 text-xs font-semibold text-muted-foreground">{{ group.label }}</div>
        <div class="grid gap-2">
          <button
            v-for="model in group.rows"
            :key="model.id"
            type="button"
            class="rounded-md border border-border bg-card p-3 text-left hover:bg-muted"
            @click="emit('addModel', model.id)"
          >
            <div class="flex items-center justify-between gap-3">
              <div>
                <div class="text-sm font-semibold">{{ model.displayName }}</div>
                <div class="mt-1 text-xs text-muted-foreground">{{ model.name }}</div>
              </div>
              <span class="text-xs text-muted-foreground">添加</span>
            </div>
            <div class="mt-2 flex flex-wrap gap-1">
              <Badge v-for="capability in model.capabilities" :key="capability" :variant="capability.includes('image') ? 'verified' : 'model'">
                {{ capabilityLabel(capability) }}
              </Badge>
            </div>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
