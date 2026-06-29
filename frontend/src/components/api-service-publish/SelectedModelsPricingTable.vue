<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import type { ModelCatalogItem } from '@/lib/api'
import type { ApiServicePublishForm, CatalogById } from './types'
import { capabilityLabel, formatActualPrice, formatMultiplier, formatPrice } from './utils'

const props = defineProps<{
  form: ApiServicePublishForm
  catalogById: CatalogById
  sub2ApiLocked: boolean
}>()

const emit = defineEmits<{
  removeModel: [id: string]
  setMultiplier: [id: string, value: string]
}>()

const selectedRows = computed(() => props.form.selectedModels
  .filter(item => item.enabled)
  .map(item => ({ selection: item, model: props.catalogById.get(item.modelId) }))
  .filter((row): row is { selection: typeof props.form.selectedModels[number], model: ModelCatalogItem } => Boolean(row.model)))

function effectiveMultiplier(row: { multiplierOverride: number | null }) {
  return row.multiplierOverride ?? props.form.defaultMultiplier
}
</script>

<template>
  <div class="overflow-x-auto rounded-lg border border-border">
    <table class="api-publish-model-table w-full text-sm">
      <thead class="bg-muted/60 text-xs text-muted-foreground">
        <tr class="border-b border-border">
          <th class="px-3 py-2 text-left font-medium">模型</th>
          <th class="px-3 py-2 text-left font-medium">能力</th>
          <th class="px-3 py-2 text-left font-medium">官方价（输入 / 缓存 / 输出）</th>
          <th class="px-3 py-2 text-left font-medium">服务倍率</th>
          <th class="px-3 py-2 text-left font-medium">实际价格预览</th>
          <th class="px-3 py-2 text-right font-medium">操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!selectedRows.length">
          <td colspan="6" class="px-3 py-5 text-center text-muted-foreground">请先从模型目录添加至少一个模型。</td>
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
          <td class="px-3 py-3">
            <template v-if="sub2ApiLocked">
              <div class="font-semibold text-primary">1.00x</div>
              <div class="mt-1 text-[11px] text-muted-foreground">平台锁定</div>
            </template>
            <template v-else>
              <Input
                class="h-8 w-24"
                :model-value="row.selection.multiplierOverride === null ? formatMultiplier(form.defaultMultiplier) : formatMultiplier(row.selection.multiplierOverride)"
                @update:model-value="value => emit('setMultiplier', row.model.id, String(value))"
              />
              <div class="mt-1 text-[11px] text-muted-foreground">留空则使用默认倍率</div>
            </template>
          </td>
          <td class="px-3 py-3 text-xs font-semibold">
            {{ formatActualPrice(row.model.officialInputPricePerMillion, sub2ApiLocked ? 1 : effectiveMultiplier(row.selection)) }} /
            {{ formatActualPrice(row.model.officialCachedInputPricePerMillion, sub2ApiLocked ? 1 : effectiveMultiplier(row.selection)) }} /
            {{ formatActualPrice(row.model.officialOutputPricePerMillion, sub2ApiLocked ? 1 : effectiveMultiplier(row.selection)) }}
          </td>
          <td class="px-3 py-3 text-right">
            <button type="button" class="text-sm text-muted-foreground hover:text-destructive" @click="emit('removeModel', row.model.id)">移除</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
