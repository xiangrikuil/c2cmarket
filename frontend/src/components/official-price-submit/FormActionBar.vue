<script setup lang="ts">
import { Save, Send, Clock3 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

defineProps<{
  savedAt: string
  canSubmit: boolean
  submitting: boolean
}>()

const emit = defineEmits<{
  saveDraft: []
  submit: []
}>()
</script>

<template>
  <div class="flex flex-col gap-3 border-t border-border bg-card px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
    <div class="flex items-center gap-2 text-xs text-muted-foreground">
      <Clock3 class="h-4 w-4" />
      <span>草稿已自动保存于 {{ savedAt }}</span>
    </div>
    <div class="grid gap-2 sm:flex sm:justify-end">
      <Button class="w-full sm:w-auto" variant="outline" :disabled="submitting" @click="emit('saveDraft')">
        <Save class="h-4 w-4" />保存草稿
      </Button>
      <Button class="w-full sm:w-auto" :disabled="!canSubmit || submitting" @click="emit('submit')">
        <Send class="h-4 w-4" />{{ submitting ? '提交中' : '提交线索' }}
      </Button>
    </div>
  </div>
</template>
