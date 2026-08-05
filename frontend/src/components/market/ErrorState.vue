<script setup lang="ts">
import { Button } from '@/components/ui/button'

defineProps<{ title?: string, description?: string }>()
defineEmits<{ retry: [] }>()
</script>

<template>
  <div class="grid min-h-48 place-items-center rounded-xl border border-risk/25 bg-risk/5 p-8 text-center" role="alert">
    <div class="max-w-md"><h2 class="font-semibold text-risk">{{ title ?? '加载失败' }}</h2><p class="mt-2 text-sm text-muted-foreground">{{ description ?? '数据暂时不可用，请稍后重试。' }}</p><Button type="button" variant="outline" class="mt-5" @click="$emit('retry')">重新加载</Button></div>
  </div>
</template>
