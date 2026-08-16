<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { ImageOff } from 'lucide-vue-next'
import type { DisputeEvidenceReference } from '@/lib/disputeEvidenceBackend'
import { disputeEvidenceKindLabels, disputeEvidenceUsageLabels, fetchDisputeEvidenceContent } from '@/lib/disputeEvidenceBackend'

const props = defineProps<{
  items: DisputeEvidenceReference[]
}>()

const objectURLs = ref<Record<string, string>>({})
const failed = ref<Set<string>>(new Set())

function revoke(id: string) {
  const url = objectURLs.value[id]
  if (url) URL.revokeObjectURL(url)
  const next = { ...objectURLs.value }
  delete next[id]
  objectURLs.value = next
}

async function load(item: DisputeEvidenceReference) {
  if (objectURLs.value[item.id] || failed.value.has(item.id)) return
  try {
    const blob = await fetchDisputeEvidenceContent(item.contentPath)
    objectURLs.value = { ...objectURLs.value, [item.id]: URL.createObjectURL(blob) }
  } catch {
    failed.value = new Set([...failed.value, item.id])
  }
}

watch(() => props.items, (items) => {
  const current = new Set(items.map(item => item.id))
  Object.keys(objectURLs.value).forEach((id) => {
    if (!current.has(id)) revoke(id)
  })
  void Promise.all(items.map(load))
}, { immediate: true, deep: true })

onBeforeUnmount(() => Object.keys(objectURLs.value).forEach(revoke))
</script>

<template>
  <div v-if="items.length" class="grid grid-cols-2 gap-3 sm:grid-cols-3">
    <figure v-for="item in items" :key="item.id" class="overflow-hidden border border-border bg-muted/30">
      <a v-if="objectURLs[item.id]" :href="objectURLs[item.id]" target="_blank" rel="noreferrer" class="block aspect-square overflow-hidden bg-muted">
        <img :src="objectURLs[item.id]" :alt="disputeEvidenceKindLabels[item.kind]" class="h-full w-full object-cover">
      </a>
      <div v-else class="grid aspect-square place-items-center px-3 text-center text-xs text-muted-foreground">
        <ImageOff v-if="failed.has(item.id)" class="h-5 w-5" />
        <span v-else>读取中...</span>
      </div>
      <figcaption class="space-y-0.5 border-t border-border px-2 py-2 text-xs">
        <div class="font-medium">{{ disputeEvidenceKindLabels[item.kind] }}</div>
        <div class="text-muted-foreground">{{ disputeEvidenceUsageLabels[item.usage] }}</div>
      </figcaption>
    </figure>
  </div>
</template>
