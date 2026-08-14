<script setup lang="ts">
import { ref, watch } from 'vue'
import { LoaderCircle, RefreshCw } from 'lucide-vue-next'
import { useIntersectionObserver } from '@vueuse/core'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  hasMore: boolean
  loading: boolean
  error?: boolean
}>()

const emit = defineEmits<{
  load: []
  retry: []
}>()

const target = ref<HTMLElement | null>(null)
const visible = ref(false)

useIntersectionObserver(
  target,
  ([entry]) => {
    visible.value = Boolean(entry?.isIntersecting)
  },
  { rootMargin: '400px 0px' },
)

watch(
  () => [visible.value, props.hasMore, props.loading, props.error] as const,
  ([isVisible, hasMore, loading, error]) => {
    if (isVisible && hasMore && !loading && !error) emit('load')
  },
  { immediate: true },
)
</script>

<template>
  <div ref="target" class="flex min-h-14 items-center justify-center py-2 text-xs text-muted-foreground" role="status" :aria-busy="loading">
    <span v-if="loading" class="inline-flex items-center gap-2"><LoaderCircle class="h-4 w-4 animate-spin" />正在加载更多</span>
    <Button v-else-if="error" size="sm" variant="outline" @click="emit('retry')"><RefreshCw class="h-4 w-4" />重试加载</Button>
    <span v-else-if="!hasMore">已加载全部</span>
    <span v-else>继续向下滚动</span>
  </div>
</template>
