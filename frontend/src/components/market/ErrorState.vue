<script setup lang="ts">
defineProps<{ title?: string, description?: string }>()
defineEmits<{ retry: [] }>()
</script>

<template>
  <div class="grid min-h-48 place-items-center rounded-xl border border-risk/25 bg-risk/5 p-8 text-center" role="alert">
    <div class="max-w-md"><h2 class="font-semibold text-risk">{{ title ?? '加载失败' }}</h2><p class="mt-2 text-sm text-muted-foreground">{{ description ?? '数据暂时不可用，请稍后重试。' }}</p><button type="button" class="mt-5 rounded-md border border-border bg-background px-3 py-2 text-sm font-medium" @click="$emit('retry')">重新加载</button></div>
  </div>
</template>
