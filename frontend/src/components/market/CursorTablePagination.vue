<script setup lang="ts">
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

defineProps<{
  page: number
  itemCount: number
  hasNextPage: boolean
  loading?: boolean
}>()

const emit = defineEmits<{
  previous: []
  next: []
}>()
</script>

<template>
  <div class="flex flex-col gap-2 border-t border-border bg-card px-4 py-3 text-xs text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
    <div>第 {{ page }} 页，本页 {{ itemCount }} 条</div>
    <div class="flex items-center gap-1">
      <Button size="sm" variant="outline" :disabled="loading || page <= 1" aria-label="上一页" @click="emit('previous')">
        <ChevronLeft class="h-4 w-4" />
        上一页
      </Button>
      <Button size="sm" variant="outline" :disabled="loading || !hasNextPage" aria-label="下一页" @click="emit('next')">
        下一页
        <ChevronRight class="h-4 w-4" />
      </Button>
    </div>
  </div>
</template>
