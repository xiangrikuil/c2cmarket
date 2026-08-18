<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, ChevronDown, ChevronRight, Minus } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Collapsible, CollapsibleContent } from '@/components/ui/collapsible'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { groupApiPackageFilterModels, type ApiPackageProviderGroup } from '@/lib/apiPackageModelFilter'
import type { ApiPackageFilterModel } from '@/lib/api'

const props = withDefaults(defineProps<{
  modelValue: string[]
  options: ApiPackageFilterModel[]
  inline?: boolean
}>(), {
  inline: false,
})

const emit = defineEmits<{ 'update:modelValue': [value: string[]] }>()
const groups = computed(() => groupApiPackageFilterModels(props.options))
const selectedIds = computed(() => new Set(props.modelValue))
const openGroups = ref<Record<string, boolean>>({})
const summary = computed(() => {
  if (props.modelValue.length === 0) return '全部模型'
  if (props.modelValue.length === 1) return props.options.find(model => model.id === props.modelValue[0])?.modelKey ?? '已选 1 个模型'
  return `已选 ${props.modelValue.length} 个模型`
})

const emitSelection = (ids: Set<string>) => {
  emit('update:modelValue', props.options.filter(model => ids.has(model.id)).map(model => model.id))
}

const groupState = (group: ApiPackageProviderGroup): boolean | 'indeterminate' => {
  const selectedCount = group.models.filter(model => selectedIds.value.has(model.id)).length
  if (selectedCount === 0) return false
  if (selectedCount === group.models.length) return true
  return 'indeterminate'
}

const toggleGroup = (group: ApiPackageProviderGroup) => {
  const next = new Set(selectedIds.value)
  if (groupState(group) === true) group.models.forEach(model => next.delete(model.id))
  else group.models.forEach(model => next.add(model.id))
  emitSelection(next)
}

const toggleModel = (id: string, checked: boolean | 'indeterminate') => {
  const next = new Set(selectedIds.value)
  if (checked === true) next.add(id)
  else next.delete(id)
  emitSelection(next)
}

const toggleOpen = (key: string) => {
  openGroups.value = { ...openGroups.value, [key]: !openGroups.value[key] }
}
</script>

<template>
  <div v-if="inline" class="rounded-md border border-border bg-background p-2" data-testid="api-package-model-filter-inline">
    <div class="flex items-center justify-between px-2 py-1.5">
      <span class="text-sm font-medium">模型</span>
      <span class="text-xs text-muted-foreground">{{ summary }}</span>
    </div>
    <div class="mt-1 divide-y divide-border">
      <Collapsible v-for="group in groups" :key="group.key" :open="Boolean(openGroups[group.key])">
        <div class="flex min-h-10 items-center gap-2 px-2">
          <Checkbox
            :id="`package-model-group-inline-${group.key}`"
            :model-value="groupState(group)"
            :aria-label="`${group.label} 全选`"
            @update:model-value="toggleGroup(group)"
          >
            <Minus v-if="groupState(group) === 'indeterminate'" class="h-3.5 w-3.5" />
            <Check v-else class="h-3.5 w-3.5" />
          </Checkbox>
          <label :for="`package-model-group-inline-${group.key}`" class="min-w-0 flex-1 cursor-pointer text-sm font-medium">
            {{ group.label }} <span class="text-xs font-normal text-muted-foreground">{{ group.models.length }}</span>
          </label>
          <Button type="button" size="icon" variant="ghost" class="h-8 w-8" :aria-label="`${openGroups[group.key] ? '收起' : '展开'} ${group.label}`" @click="toggleOpen(group.key)">
            <ChevronDown v-if="openGroups[group.key]" class="h-4 w-4" />
            <ChevronRight v-else class="h-4 w-4" />
          </Button>
        </div>
        <CollapsibleContent :force-mount="Boolean(openGroups[group.key])">
          <label v-for="model in group.models" :key="model.id" class="flex min-h-9 cursor-pointer items-center gap-2 px-8 py-1.5 text-sm hover:bg-muted/60">
            <Checkbox :model-value="selectedIds.has(model.id)" @update:model-value="checked => toggleModel(model.id, checked)" />
            <span class="min-w-0 flex-1 break-all font-mono text-xs">{{ model.modelKey }}</span>
            <Check v-if="selectedIds.has(model.id)" class="h-3.5 w-3.5 text-primary" />
          </label>
        </CollapsibleContent>
      </Collapsible>
    </div>
  </div>

  <Popover v-else>
    <PopoverTrigger as-child>
      <Button type="button" variant="outline" class="h-11 w-full justify-between gap-2 px-3 font-normal lg:h-9" aria-label="筛选短期流量包模型">
        <span class="truncate">{{ summary }}</span>
        <ChevronDown class="h-4 w-4 shrink-0 opacity-60" />
      </Button>
    </PopoverTrigger>
    <PopoverContent align="start" class="w-[min(22rem,var(--reka-popover-content-available-width))] p-2">
      <ApiPackageModelFilter :model-value="modelValue" :options="options" inline @update:model-value="value => emit('update:modelValue', value)" />
    </PopoverContent>
  </Popover>
</template>
