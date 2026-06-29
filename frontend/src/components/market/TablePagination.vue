<script setup lang="ts">
import { computed } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  page: number
  pageCount: number
  total: number
  startItem: number
  endItem: number
}>()

const emit = defineEmits<{
  'update:page': [page: number]
}>()

const visiblePages = computed(() => {
  const pages = new Set<number>([1, props.pageCount, props.page - 1, props.page, props.page + 1])
  return [...pages]
    .filter(page => page >= 1 && page <= props.pageCount)
    .sort((a, b) => a - b)
})

function setPage(page: number) {
  emit('update:page', page)
}
</script>

<template>
  <div class="flex flex-col gap-2 border-t border-border bg-card px-4 py-3 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
    <div>
      共 {{ total }} 条
      <span v-if="total > 0">，第 {{ startItem }}-{{ endItem }} 条</span>
    </div>
    <div class="flex flex-wrap items-center gap-1">
      <Button size="sm" variant="outline" :disabled="page <= 1" aria-label="上一页" @click="setPage(page - 1)">
        <ChevronLeft class="h-4 w-4" />
        上一页
      </Button>
      <Button
        v-for="item in visiblePages"
        :key="item"
        size="sm"
        :variant="item === page ? 'default' : 'outline'"
        :aria-current="item === page ? 'page' : undefined"
        @click="setPage(item)"
      >
        {{ item }}
      </Button>
      <Button size="sm" variant="outline" :disabled="page >= pageCount" aria-label="下一页" @click="setPage(page + 1)">
        下一页
        <ChevronRight class="h-4 w-4" />
      </Button>
    </div>
  </div>
</template>
