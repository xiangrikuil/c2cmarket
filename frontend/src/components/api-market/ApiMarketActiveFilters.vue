<script setup lang="ts">
import X from 'lucide-vue-next/dist/esm/icons/x.js'
import RotateCcw from 'lucide-vue-next/dist/esm/icons/rotate-ccw.js'
import { Button } from '@/components/ui/button'

defineProps<{
  items: Array<{ key: string, label: string }>
}>()

const emit = defineEmits<{
  remove: [key: string]
  clear: []
}>()
</script>

<template>
  <div v-if="items.length" class="flex flex-wrap items-center gap-2" aria-label="已选筛选条件">
    <span class="text-xs text-muted-foreground">已选</span>
    <Button
      v-for="item in items"
      :key="item.key"
      type="button"
      variant="secondary"
      size="sm"
      class="h-7 max-w-full gap-1 px-2 text-xs font-normal"
      :aria-label="`移除筛选：${item.label}`"
      @click="emit('remove', item.key)"
    >
      <span class="truncate">{{ item.label }}</span>
      <X class="h-3 w-3 shrink-0" />
    </Button>
    <Button type="button" variant="ghost" size="sm" class="h-7 gap-1 px-2 text-xs" @click="emit('clear')">
      <RotateCcw class="h-3 w-3" />全部清除
    </Button>
  </div>
</template>
