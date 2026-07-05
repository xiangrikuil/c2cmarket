<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import type { ModelCatalogItem } from '@/lib/api'
import type { ApiProviderCategory, ApiServicePublishForm } from './types'
import { selectedModelIdSet, summarizeSelectedModelNames } from './modelSelection'
import { capabilityLabel, modelProviderCategory, providerCategoryLabels, providerLabel } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  providerCategory: ApiProviderCategory
  catalog: ModelCatalogItem[]
  errors: Partial<Record<string, string>>
}>()

const emit = defineEmits<{
  toggleModel: [id: string]
}>()

const keyword = ref('')
const selectedIds = computed(() => selectedModelIdSet(props.form.selectedModels))
const selectedModelNames = computed(() => props.form.selectedModels
  .filter(item => item.enabled)
  .map(item => props.catalog.find(model => model.id === item.modelId)?.displayName ?? item.modelId)
  .filter(Boolean))
const selectedSummary = computed(() => summarizeSelectedModelNames(selectedModelNames.value))
const filteredGroups = computed(() => {
  const normalized = keyword.value.trim().toLowerCase()
  const rows = props.catalog.filter(item => {
    const matched = !normalized || [item.displayName, item.name, providerLabel(item.provider)].some(value => value.toLowerCase().includes(normalized))
    return matched && modelProviderCategory(item.provider) === props.providerCategory
  })
  return ['openai', 'anthropic', 'other'].map(provider => ({
    provider,
    label: providerLabel(provider as ModelCatalogItem['provider']),
    rows: rows
      .filter(item => item.provider === provider)
      .sort((a, b) => Number(selectedIds.value.has(b.id)) - Number(selectedIds.value.has(a.id))),
  })).filter(group => group.rows.length)
})
</script>

<template>
  <div class="rounded-lg border border-border bg-background">
    <div class="space-y-2 border-b border-border p-3">
      <Input v-model="keyword" :placeholder="`搜索并添加 ${providerCategoryLabels[providerCategory]} 模型`" />
      <div class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-muted/45 px-3 py-2">
        <span class="text-xs font-medium" :class="selectedModelNames.length ? 'text-foreground' : 'text-muted-foreground'">{{ selectedSummary }}</span>
        <Badge v-if="selectedModelNames.length" variant="model">{{ selectedModelNames.length }} 个模型</Badge>
      </div>
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
            class="api-publish-model-card"
            :class="{ 'is-active': selectedIds.has(model.id) }"
            :aria-pressed="selectedIds.has(model.id)"
            @click="emit('toggleModel', model.id)"
          >
            <div class="flex items-center justify-between gap-3">
              <div>
                <div class="text-sm font-semibold">{{ model.displayName }}</div>
                <div class="mt-1 text-xs text-muted-foreground">{{ model.name }}</div>
              </div>
              <span class="inline-flex items-center gap-1 text-xs font-semibold" :class="selectedIds.has(model.id) ? 'text-primary' : 'text-muted-foreground'">
                <Check v-if="selectedIds.has(model.id)" class="h-3.5 w-3.5" />
                {{ selectedIds.has(model.id) ? '已选择' : '选择' }}
              </span>
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
