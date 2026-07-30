<script setup lang="ts">
import { computed } from 'vue'
import { ArrowDown, ArrowUp, Check, Plus, Trash2 } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { createDefaultApiServicePackage } from './packages'
import type { ApiServicePackage, ApiServicePublishForm, CatalogById } from './types'

const props = defineProps<{
  form: ApiServicePublishForm
  catalogById: CatalogById
  errors: Partial<Record<string, string>>
}>()

const selectedModels = computed(() => props.form.selectedModels
  .filter(item => item.enabled)
  .map(item => ({ id: item.modelId, name: props.catalogById.get(item.modelId)?.displayName ?? item.modelId })))

const addPackage = () => props.form.packages.push(createDefaultApiServicePackage(selectedModels.value.map(model => model.id)))
const removePackage = (index: number) => props.form.packages.splice(index, 1)
const movePackage = (index: number, offset: number) => {
  const target = index + offset
  if (target < 0 || target >= props.form.packages.length) return
  const [item] = props.form.packages.splice(index, 1)
  props.form.packages.splice(target, 0, item)
}

const togglePackageModel = (item: ApiServicePackage, modelId: string) => {
  item.modelCatalogIds = item.modelCatalogIds.includes(modelId)
    ? item.modelCatalogIds.filter(id => id !== modelId)
    : [...item.modelCatalogIds, modelId]
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>固定额度包</h2>
      <p>设置固定价格、面板额度、库存和 1 / 3 / 7 / 30 天有效期。</p>
    </div>

    <div class="api-publish-card-body space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div class="text-sm font-semibold">套餐列表</div>
          <p class="mt-1 text-xs text-muted-foreground">套餐有效期从商家提交交付时开始计算。</p>
        </div>
        <Button size="sm" variant="outline" @click="addPackage"><Plus class="h-4 w-4" />添加套餐</Button>
      </div>

      <p v-if="errors.packages" class="text-xs text-destructive">{{ errors.packages }}</p>
      <div class="space-y-3">
        <div v-for="(item, index) in form.packages" :key="item.id" class="rounded-lg border border-border bg-background p-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="flex items-center gap-2">
              <Badge variant="secondary">套餐 {{ index + 1 }}</Badge>
              <label class="flex items-center gap-2 text-xs text-muted-foreground">
                <Checkbox :model-value="item.enabled" @update:model-value="value => item.enabled = Boolean(value)" />启用
              </label>
            </div>
            <div class="flex items-center gap-1">
              <Button size="icon" variant="ghost" title="上移套餐" :disabled="index === 0" @click="movePackage(index, -1)"><ArrowUp class="h-4 w-4" /></Button>
              <Button size="icon" variant="ghost" title="下移套餐" :disabled="index === form.packages.length - 1" @click="movePackage(index, 1)"><ArrowDown class="h-4 w-4" /></Button>
              <Button size="icon" variant="ghost" title="删除套餐" :disabled="form.packages.length === 1" @click="removePackage(index)"><Trash2 class="h-4 w-4" /></Button>
            </div>
          </div>

          <div class="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <label class="space-y-1.5 md:col-span-2 xl:col-span-1"><span class="text-xs font-medium">套餐名称</span><Input v-model="item.name" /></label>
            <label class="space-y-1.5"><span class="text-xs font-medium">价格（元）</span><Input :model-value="item.priceCny" min="0.01" step="0.01" type="number" @update:model-value="value => item.priceCny = Number(value)" /></label>
            <label class="space-y-1.5"><span class="text-xs font-medium">面板额度</span><Input :model-value="item.panelAllowance" min="0.000001" step="0.01" type="number" @update:model-value="value => item.panelAllowance = Number(value)" /></label>
            <label class="space-y-1.5"><span class="text-xs font-medium">有效期</span><select v-model.number="item.durationDays" class="h-9 w-full rounded-md border border-input bg-background px-3 text-sm"><option :value="1">1 天</option><option :value="3">3 天</option><option :value="7">7 天</option><option :value="30">30 天</option></select></label>
            <label class="space-y-1.5"><span class="text-xs font-medium">总库存</span><Input :model-value="item.stockTotal" min="0" step="1" type="number" @update:model-value="value => item.stockTotal = Number(value)" /></label>
          </div>

          <label class="mt-3 block space-y-1.5"><span class="text-xs font-medium">套餐说明</span><Input v-model="item.description" /></label>
          <div class="mt-3 space-y-2">
            <div class="text-xs font-medium">支持模型</div>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="model in selectedModels"
                :key="model.id"
                type="button"
                class="api-publish-model-chip"
                :class="{ 'is-active': item.modelCatalogIds.includes(model.id) }"
                :aria-pressed="item.modelCatalogIds.includes(model.id)"
                @click="togglePackageModel(item, model.id)"
              >
                <Check v-if="item.modelCatalogIds.includes(model.id)" class="h-3.5 w-3.5" />{{ model.name }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Card>
</template>
