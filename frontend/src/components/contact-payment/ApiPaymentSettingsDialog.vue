<script setup lang="ts">
import { ref, watch } from 'vue'
import ApiPaymentSettingsEditor from '@/components/contact-payment/ApiPaymentSettingsEditor.vue'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import type { ApiPaymentAccountSettings } from '@/lib/apiPaymentSettings'

const props = defineProps<{
  open: boolean
  settings: ApiPaymentAccountSettings
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
  saved: [settings: ApiPaymentAccountSettings]
}>()

const editorDirty = ref(false)
const discardConfirmationOpen = ref(false)
const editorSession = ref(0)

watch(
  () => props.open,
  (open, wasOpen) => {
    if (!open || wasOpen) return
    editorDirty.value = false
    discardConfirmationOpen.value = false
    editorSession.value += 1
  },
)

function requestClose() {
  if (!editorDirty.value) {
    emit('update:open', false)
    return
  }
  discardConfirmationOpen.value = true
}

function handleDialogOpenChange(open: boolean) {
  if (open) {
    emit('update:open', true)
    return
  }
  requestClose()
}

function discardAndClose() {
  discardConfirmationOpen.value = false
  editorDirty.value = false
  emit('update:open', false)
}

function handleSaved(settings: ApiPaymentAccountSettings) {
  editorDirty.value = false
  emit('saved', settings)
  emit('update:open', false)
}
</script>

<template>
  <Dialog :open="open" @update:open="handleDialogOpenChange">
    <DialogContent class="grid max-h-[calc(100dvh-1rem)] grid-rows-[auto_minmax(0,1fr)] gap-0 overflow-hidden p-0 sm:max-w-3xl">
      <DialogHeader class="border-b border-border px-4 py-4 pr-12 text-left sm:px-5">
        <DialogTitle>API 收款设置</DialogTitle>
        <DialogDescription>
          修改账户级收款资料并保存，当前发布内容会保留在本页。
        </DialogDescription>
      </DialogHeader>
      <div class="min-h-0 overflow-y-auto p-4 sm:p-5">
        <ApiPaymentSettingsEditor
          v-if="open"
          :key="editorSession"
          :settings="settings"
          layout="dialog"
          @cancel="requestClose"
          @dirty-change="editorDirty = $event"
          @saved="handleSaved"
        />
      </div>
    </DialogContent>
  </Dialog>

  <Dialog
    :open="discardConfirmationOpen"
    @update:open="open => { discardConfirmationOpen = open }"
  >
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>放弃未保存的修改？</DialogTitle>
        <DialogDescription>
          关闭后，本次修改不会影响已保存的收款设置，也不会改变当前发布表单。
        </DialogDescription>
      </DialogHeader>
      <DialogFooter>
        <Button variant="outline" @click="discardConfirmationOpen = false">继续修改</Button>
        <Button variant="destructive" @click="discardAndClose">放弃修改</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
