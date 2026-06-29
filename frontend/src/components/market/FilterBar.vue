<script setup lang="ts">
import { computed } from 'vue'
import { X } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { toast } from 'vue-sonner'

const props = defineProps<{
  modelValue: Record<string, string>
  groups: Array<{
    label: string
    items: string[]
    active?: string
    kind?: 'select' | 'segmented'
    placeholder?: string
  }>
  resultCount?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, string>]
}>()

function selectFilter(group: string, item: string) {
  emit('update:modelValue', { ...props.modelValue, [group]: item })
  if (item !== defaultValue(group)) {
    toast(`已筛选：${group} = ${item}`)
  }
}

function defaultValue(group: string) {
  return props.groups.find(item => item.label === group)?.active ?? props.groups.find(item => item.label === group)?.items[0] ?? '全部'
}

function resetFilter(group: string) {
  selectFilter(group, defaultValue(group))
}

function clearAll() {
  emit('update:modelValue', Object.fromEntries(props.groups.map(group => [group.label, group.active ?? group.items[0]])))
}

const chips = computed(() => props.groups
  .filter(group => props.modelValue[group.label] && props.modelValue[group.label] !== defaultValue(group.label))
  .map(group => ({ label: group.label, value: props.modelValue[group.label] })))

function isSegmented(group: { label: string, items: string[], kind?: 'select' | 'segmented' }) {
  return group.kind === 'segmented' || (group.label === '状态' && group.items.length <= 4)
}
</script>

<template>
  <div class="c2c-filterbar mb-4 rounded-lg border border-border bg-card px-3 py-2">
    <div class="grid gap-2 lg:flex lg:items-center">
      <div
        v-for="group in groups"
        :key="group.label"
        class="min-w-0"
        :class="isSegmented(group) ? 'lg:shrink-0' : 'lg:w-[160px]'"
      >
        <div v-if="isSegmented(group)" class="grid grid-cols-3 rounded-md border border-border bg-background p-1">
          <Button
            v-for="item in group.items.slice(0, 3)"
            :key="item"
            class="h-7 px-2 text-xs"
            size="sm"
            :variant="item === modelValue[group.label] ? 'default' : 'ghost'"
            @click="selectFilter(group.label, item)"
          >
            {{ item }}
          </Button>
        </div>
        <label v-else class="grid gap-1">
          <span class="text-[11px] font-medium leading-none text-muted-foreground">{{ group.label }}</span>
          <Select
            :model-value="modelValue[group.label]"
            @update:model-value="value => selectFilter(group.label, String(value))"
          >
            <SelectTrigger class="h-8 w-full bg-background text-xs">
              <SelectValue :placeholder="group.placeholder ?? `全部${group.label}`" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="item in group.items" :key="item" :value="item">
                {{ item }}
              </SelectItem>
            </SelectContent>
          </Select>
        </label>
      </div>
      <div v-if="resultCount !== undefined" class="ml-auto hidden shrink-0 text-xs text-muted-foreground lg:block">
        共 {{ resultCount }} 条记录
      </div>
    </div>

    <div v-if="chips.length" class="mt-2 flex items-center gap-2 border-t border-border pt-2">
      <span class="shrink-0 text-xs text-muted-foreground">已选</span>
      <div class="flex min-w-0 flex-1 gap-1.5 overflow-x-auto">
        <Badge
          v-for="chip in chips"
          :key="`${chip.label}-${chip.value}`"
          variant="trust"
          class="cursor-pointer gap-1"
          @click="resetFilter(chip.label)"
        >
          {{ chip.value }}
          <X class="h-3 w-3" />
        </Badge>
      </div>
      <Button class="h-7 shrink-0 px-2 text-xs" variant="ghost" size="sm" @click="clearAll">清除全部</Button>
      <span v-if="resultCount !== undefined" class="shrink-0 text-xs text-muted-foreground lg:hidden">共 {{ resultCount }} 条</span>
    </div>
  </div>
</template>
