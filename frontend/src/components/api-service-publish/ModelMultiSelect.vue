<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import type { ModelCatalogItem } from '@/lib/api'
import type { ApiProviderCategory, ApiServicePublishForm } from './types'
import { selectedModelIdSet, summarizeSelectedModelNames } from './modelSelection'
import { modelProviderCategory, providerCategoryLabels, providerLabel } from './utils'

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
const expanded = ref(false)
const selectedIds = computed(() => selectedModelIdSet(props.form.selectedModels))
const selectedModelNames = computed(() => props.form.selectedModels
  .filter(item => item.enabled)
  .map(item => props.catalog.find(model => model.id === item.modelId)?.displayName ?? item.modelId)
  .filter(Boolean))
const selectedSummary = computed(() => summarizeSelectedModelNames(selectedModelNames.value))
const filteredModels = computed(() => {
  const normalized = keyword.value.trim().toLowerCase()
  return props.catalog
    .filter(item => {
      const matched = !normalized || [item.id, item.displayName, item.name, providerLabel(item.provider)].some(value => value.toLowerCase().includes(normalized))
      return matched && modelProviderCategory(item.provider) === props.providerCategory
    })
    .sort((a, b) => Number(selectedIds.value.has(b.id)) - Number(selectedIds.value.has(a.id)) || a.displayName.localeCompare(b.displayName))
})
const collapsedLimit = 12
const visibleModels = computed(() => expanded.value || keyword.value.trim() ? filteredModels.value : filteredModels.value.slice(0, collapsedLimit))
const hiddenCount = computed(() => Math.max(filteredModels.value.length - visibleModels.value.length, 0))
const listClass = computed(() => expanded.value ? 'max-h-64 overflow-y-auto' : '')
</script>

<template>
  <div class="rounded-lg border border-border bg-background">
    <div class="space-y-2 border-b border-border p-3">
      <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
        <Input v-model="keyword" :placeholder="`搜索 ${providerCategoryLabels[providerCategory]} 模型`" />
        <Badge variant="model">{{ selectedModelNames.length }} 个模型</Badge>
      </div>
      <div class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-muted/45 px-3 py-2">
        <span class="text-xs font-medium" :class="selectedModelNames.length ? 'text-foreground' : 'text-muted-foreground'">{{ selectedSummary }}</span>
      </div>
      <p v-if="errors.selectedModels" class="mt-2 text-xs text-destructive">{{ errors.selectedModels }}</p>
    </div>
    <div class="p-3">
      <div v-if="!filteredModels.length" class="text-sm text-muted-foreground">当前模型大类没有可添加的模型。</div>
      <div v-else class="api-publish-model-chip-list" :class="listClass">
        <button
          v-for="model in visibleModels"
          :key="model.id"
          type="button"
          class="api-publish-model-chip"
          :class="{ 'is-active': selectedIds.has(model.id) }"
          :aria-pressed="selectedIds.has(model.id)"
          :title="`${model.displayName} · ${model.name}`"
          @click="emit('toggleModel', model.id)"
        >
          <Check v-if="selectedIds.has(model.id)" class="h-3.5 w-3.5" />
          <span>{{ model.displayName }}</span>
        </button>
      </div>
      <button
        v-if="hiddenCount || expanded"
        type="button"
        class="mt-3 rounded-full border border-border px-3 py-1 text-xs font-medium text-muted-foreground hover:bg-muted"
        @click="expanded = !expanded"
      >
        {{ expanded ? '收起' : `展开全部，更多 ${hiddenCount} 个` }}
      </button>
    </div>
  </div>
</template>
