<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { ModelCatalogItem } from '@/lib/api'
import type { ApiServicePublishForm, CatalogById } from './types'
import { capabilityLabel, formatActualPrice, formatPrice } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  catalogById: CatalogById
}>()

const emit = defineEmits<{
  removeModel: [id: string]
}>()

const selectedRows = computed(() => props.form.selectedModels
  .filter(item => item.enabled)
  .map(item => ({ model: props.catalogById.get(item.modelId) }))
  .filter((row): row is { model: ModelCatalogItem } => Boolean(row.model)))
</script>

<template>
  <div class="overflow-x-auto rounded-lg border border-border">
    <table class="api-publish-model-table w-full text-sm">
      <thead class="bg-muted/60 text-xs text-muted-foreground">
        <tr class="border-b border-border">
          <th class="px-3 py-2 text-left font-medium">模型</th>
          <th class="px-3 py-2 text-left font-medium">能力</th>
          <th class="px-3 py-2 text-left font-medium">官方价（输入 / 缓存 / 输出）</th>
          <th class="px-3 py-2 text-left font-medium">实际价格预览</th>
          <th class="px-3 py-2 text-right font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!selectedRows.length">
          <td colspan="5" class="px-3 py-5 text-center text-muted-foreground">请先从模型目录添加至少一个模型。</td>
        </tr>
        <tr v-for="row in selectedRows" :key="row.model.id" class="border-b border-border last:border-b-0">
          <td class="px-3 py-3">
            <div class="font-semibold">{{ row.model.displayName }}</div>
            <div class="text-xs text-muted-foreground">{{ row.model.provider }}</div>
          </td>
          <td class="px-3 py-3">
            <div class="flex flex-wrap gap-1">
              <Badge v-for="capability in row.model.capabilities" :key="capability" :variant="capability.includes('image') ? 'verified' : 'model'">
                {{ capabilityLabel(capability) }}
              </Badge>
            </div>
          </td>
          <td class="px-3 py-3 text-xs text-muted-foreground">
            {{ formatPrice(row.model.officialInputPricePerMillion) }} /
            {{ formatPrice(row.model.officialCachedInputPricePerMillion) }} /
            {{ formatPrice(row.model.officialOutputPricePerMillion) }}
          </td>
          <td class="px-3 py-3 text-xs font-semibold">
            {{ formatActualPrice(row.model.officialInputPricePerMillion, form.defaultMultiplier) }} /
            {{ formatActualPrice(row.model.officialCachedInputPricePerMillion, form.defaultMultiplier) }} /
            {{ formatActualPrice(row.model.officialOutputPricePerMillion, form.defaultMultiplier) }}
          </td>
          <td class="px-3 py-3 text-right">
            <Button size="sm" variant="ghost" class="h-auto px-2 text-sm text-muted-foreground hover:text-destructive" @click="emit('removeModel', row.model.id)">移除</Button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
