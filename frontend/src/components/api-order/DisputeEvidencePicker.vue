<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { ImagePlus, RefreshCw, Trash2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { disputeEvidenceKindLabels, uploadDisputeEvidence, type DisputeEvidenceAsset, type DisputeEvidenceKind, type DisputeEvidenceReference } from '@/lib/disputeEvidenceBackend'

const props = withDefaults(defineProps<{
  orderId: string
  visibility?: DisputeEvidenceReference['visibility']
}>(), {
  visibility: 'participants_admin',
})
const model = defineModel<DisputeEvidenceAsset[]>({ default: () => [] })
const input = ref<HTMLInputElement | null>(null)
const kind = ref<DisputeEvidenceKind>('other_redacted_fact')
const uploading = ref(false)
const progress = ref(0)
const failedFiles = ref<File[]>([])
const redactionConfirmed = ref(false)
const previews = new Map<string, string>()

const remaining = computed(() => Math.max(0, 3 - model.value.length))
const visibilityNotice = computed(() => {
  if (props.visibility === 'submitter_admin') return '提交后，仅材料提交者和管理员可见，纠纷另一方不可见。'
  if (props.visibility === 'appellant_admin') return '提交后，仅申诉人和管理员可见，纠纷另一方不可见。'
  return '提交后，纠纷双方和管理员可见。'
})

function chooseFiles() {
  if (!redactionConfirmed.value) {
    toast.warning('请先确认图片已遮挡敏感内容和二维码。')
    return
  }
  input.value?.click()
}

async function selected(event: Event) {
  const target = event.target as HTMLInputElement
  const files = Array.from(target.files ?? []).slice(0, remaining.value)
  target.value = ''
  if (!files.length) return
  await upload(files)
}

async function upload(files: File[]) {
  uploading.value = true
  progress.value = 0
  failedFiles.value = []
  try {
    const assets = await uploadDisputeEvidence(props.orderId, kind.value, files, redactionConfirmed.value, value => { progress.value = value })
    assets.forEach((asset, index) => previews.set(asset.id, URL.createObjectURL(files[index])))
    model.value = [...model.value, ...assets].slice(0, 3)
    redactionConfirmed.value = false
  } catch (error) {
    failedFiles.value = files
    toast.error(error instanceof Error ? error.message : '图片上传失败。')
  } finally {
    uploading.value = false
  }
}

function remove(asset: DisputeEvidenceAsset) {
  const preview = previews.get(asset.id)
  if (preview) URL.revokeObjectURL(preview)
  previews.delete(asset.id)
  model.value = model.value.filter(item => item.id !== asset.id)
}

function reset() {
  for (const preview of previews.values()) URL.revokeObjectURL(preview)
  previews.clear()
  model.value = []
  failedFiles.value = []
  progress.value = 0
  redactionConfirmed.value = false
}

defineExpose({ reset })
onBeforeUnmount(() => {
  for (const preview of previews.values()) URL.revokeObjectURL(preview)
})
</script>

<template>
  <div class="space-y-3 border-t border-border pt-3">
    <label class="flex items-start gap-2 text-xs leading-5 text-muted-foreground">
      <Checkbox v-model="redactionConfirmed" class="mt-0.5" :disabled="uploading" />
      <span>我已遮挡 API Key、密码、Token、Cookie、完整账号信息及所有二维码。</span>
    </label>
    <div class="flex flex-wrap items-center gap-2">
      <Select v-model="kind" :disabled="uploading">
        <SelectTrigger class="w-[160px]"><SelectValue /></SelectTrigger>
        <SelectContent>
          <SelectItem v-for="(label, value) in disputeEvidenceKindLabels" :key="value" :value="value">{{ label }}</SelectItem>
        </SelectContent>
      </Select>
      <Button type="button" variant="outline" size="sm" :disabled="uploading || remaining === 0 || !redactionConfirmed" @click="chooseFiles">
        <ImagePlus class="h-4 w-4" />添加图片
      </Button>
      <input ref="input" class="hidden" type="file" accept="image/jpeg,image/png,image/webp" multiple @change="selected">
      <span class="text-xs text-muted-foreground">{{ model.length }} / 3</span>
    </div>
    <div v-if="uploading" class="h-1.5 overflow-hidden bg-muted" role="progressbar" :aria-valuenow="progress">
      <div class="h-full bg-primary transition-[width]" :style="{ width: `${progress}%` }" />
    </div>
    <div v-if="model.length" class="grid grid-cols-3 gap-2">
      <div v-for="asset in model" :key="asset.id" class="relative aspect-square overflow-hidden border border-border bg-muted">
        <img v-if="previews.get(asset.id)" :src="previews.get(asset.id)" class="h-full w-full object-cover" alt="待提交证据预览">
        <div v-else class="grid h-full place-items-center px-2 text-center text-xs text-muted-foreground">{{ disputeEvidenceKindLabels[asset.kind] }}</div>
        <Button type="button" size="icon" variant="secondary" class="absolute right-1 top-1 h-7 w-7" title="移除图片" @click="remove(asset)"><Trash2 class="h-3.5 w-3.5" /></Button>
      </div>
    </div>
    <Button v-if="failedFiles.length" type="button" variant="outline" size="sm" :disabled="uploading || !redactionConfirmed" @click="upload(failedFiles)"><RefreshCw class="h-4 w-4" />重试上传</Button>
    <p class="text-xs leading-5 text-muted-foreground">图片仅作为辅助材料，不代表平台确认付款、退款或履约。请先遮盖 API Key、密码、Token、Cookie、完整账号及二维码；{{ visibilityNotice }}</p>
  </div>
</template>
