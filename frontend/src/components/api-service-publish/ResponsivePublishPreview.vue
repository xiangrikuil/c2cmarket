<script setup lang="ts">
import { useMediaQuery } from '@vueuse/core'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

withDefaults(defineProps<{
  title?: string
  description?: string
}>(), {
  title: '发布预览',
  description: '根据当前已填写内容生成。',
})

const open = defineModel<boolean>('open', { default: false })
const desktopPreview = useMediaQuery('(min-width: 1241px)')
</script>

<template>
  <aside v-if="desktopPreview" class="api-publish-responsive-preview min-w-0">
    <slot />
  </aside>

  <Dialog v-else v-model:open="open">
    <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription>{{ description }}</DialogDescription>
      </DialogHeader>
      <slot />
    </DialogContent>
  </Dialog>
</template>
