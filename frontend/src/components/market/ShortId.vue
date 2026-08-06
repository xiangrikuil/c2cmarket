<script setup lang="ts">
import { computed } from 'vue'
import { Copy } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { shortId } from '@/lib/presentation'

const props = withDefaults(defineProps<{ value: string, prefix?: string, copyable?: boolean, full?: boolean }>(), { prefix: '', copyable: false, full: false })
const display = computed(() => props.full ? props.value : shortId(props.value, props.prefix))

async function copy() {
  await navigator.clipboard?.writeText(props.value)
}
</script>

<template>
  <span class="inline-flex items-center gap-1 font-mono text-xs" :title="value">
    <span>{{ display }}</span>
    <Button v-if="copyable" type="button" size="icon" variant="ghost" class="h-6 w-6 text-muted-foreground" :aria-label="`复制完整编号 ${display}`" @click="copy">
      <Copy class="h-3.5 w-3.5" />
    </Button>
  </span>
</template>
