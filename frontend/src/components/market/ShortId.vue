<script setup lang="ts">
import { computed } from 'vue'
import { Copy } from 'lucide-vue-next'
import { shortId } from '@/lib/presentation'

const props = withDefaults(defineProps<{ value: string, prefix?: string, copyable?: boolean }>(), { prefix: '', copyable: false })
const display = computed(() => shortId(props.value, props.prefix))

async function copy() {
  await navigator.clipboard?.writeText(props.value)
}
</script>

<template>
  <span class="inline-flex items-center gap-1 font-mono text-xs" :title="value">
    <span>{{ display }}</span>
    <button v-if="copyable" type="button" class="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground" :aria-label="`复制完整编号 ${display}`" @click="copy">
      <Copy class="h-3.5 w-3.5" />
    </button>
  </span>
</template>
