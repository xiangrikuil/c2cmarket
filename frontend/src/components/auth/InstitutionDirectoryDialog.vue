<script setup lang="ts">
import { Building2, Mail } from 'lucide-vue-next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

defineProps<{
  open: boolean
  institutions: Array<{ domain: string, institutionName: string }>
}>()

const emit = defineEmits<{
  (event: 'update:open', value: boolean): void
}>()
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent
      class="bottom-0 top-auto max-h-[78dvh] translate-y-0 gap-0 overflow-hidden rounded-b-none p-0 sm:bottom-auto sm:top-1/2 sm:max-h-[min(640px,calc(100dvh-3rem))] sm:max-w-lg sm:-translate-y-1/2 sm:rounded-lg"
    >
      <DialogHeader class="border-b border-border px-5 py-5 text-left sm:px-6">
        <DialogTitle>已开放学校邮箱域名</DialogTitle>
        <DialogDescription>学校邮箱域名需要与下列条目完全一致。</DialogDescription>
      </DialogHeader>
      <div class="overflow-y-auto px-5 py-3 sm:px-6" tabindex="0">
        <ul class="divide-y divide-border" aria-label="已开放学校邮箱域名">
          <li v-for="institution in institutions" :key="institution.domain" class="flex items-start gap-3 py-3.5">
            <span class="grid h-9 w-9 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground">
              <Building2 class="h-4 w-4" />
            </span>
            <span class="min-w-0">
              <span class="block text-sm font-medium text-foreground">{{ institution.institutionName }}</span>
              <span class="mt-0.5 flex items-center gap-1.5 break-all text-xs text-muted-foreground">
                <Mail class="h-3.5 w-3.5 shrink-0" />@{{ institution.domain }}
              </span>
            </span>
          </li>
        </ul>
      </div>
    </DialogContent>
  </Dialog>
</template>
