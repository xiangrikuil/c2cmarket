<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { Clock3, Send } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import LocalTime from '@/components/market/LocalTime.vue'
import StarRatingDisplay from '@/components/review/StarRatingDisplay.vue'
import StarRatingInput from '@/components/review/StarRatingInput.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Dialog, DialogDescription, DialogFooter, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import type { ReviewCenterRow, SubmitReviewPayload } from '@/lib/api'
import { useSubmitReviewMutation } from '@/queries/useMarketQueries'

const props = defineProps<{ open: boolean, row: ReviewCenterRow | null }>()
const emit = defineEmits<{ 'update:open': [value: boolean], saved: [row: ReviewCenterRow] }>()
const mutation = useSubmitReviewMutation()
const form = reactive({ rating: null as number | null, tags: [] as string[], note: '' })
const canSubmit = computed(() => Boolean(props.row?.canCreate || props.row?.canEdit))
const contentVisible = computed(() => props.row?.rating !== null)
const formValid = computed(() => form.rating !== null && (form.tags.length > 0 || form.note.trim().length > 0))

watch(() => [props.open, props.row?.id] as const, ([open]) => {
  if (!open || !props.row) return
  form.rating = props.row.rating
  form.tags = [...props.row.tags]
  form.note = props.row.note ?? ''
}, { immediate: true })

function setOpen(value: boolean) {
  if (!mutation.isPending.value) emit('update:open', value)
}

function toggleTag(code: string, checked: boolean | 'indeterminate') {
  if (checked === true) {
    if (form.tags.includes(code)) return
    if (form.tags.length >= 5) return toast.warning('最多选择 5 个标签。')
    form.tags.push(code)
  } else {
    form.tags = form.tags.filter(item => item !== code)
  }
}

function tagLabel(value: string) {
  return props.row?.allowedTags.find(tag => tag.code === value)?.label ?? value
}

function submit() {
  if (!props.row || !canSubmit.value || !formValid.value || form.rating === null) return
  const payload: SubmitReviewPayload = {
    transactionType: props.row.transactionType,
    transactionId: props.row.transactionId,
    operation: props.row.canEdit ? 'edit' : 'create',
    rating: form.rating,
    tags: [...form.tags],
    note: form.note.trim(),
  }
  mutation.mutate(payload, {
    onSuccess(saved) {
      toast.success(saved.visibility === 'published' ? '双方评价已公开，公开后不可修改。' : '评价已提交，待公开；截止前可以修改。')
      emit('saved', saved)
      emit('update:open', false)
    },
    onError(error) {
      toast.error(error instanceof Error ? error.message : '评价提交失败，请稍后重试。')
    },
  })
}
</script>

<template>
  <Dialog :open="open" @update:open="setOpen">
    <DialogScrollContent class="my-2 max-h-[calc(100dvh-1rem)] w-[calc(100vw-1rem)] max-w-2xl overflow-y-auto p-5 sm:my-8 sm:max-h-[calc(100dvh-4rem)] sm:p-6">
      <DialogHeader v-if="row">
        <div class="flex flex-wrap items-center gap-2 pr-8">
          <DialogTitle>{{ row.canEdit ? '修改评价' : row.canCreate ? '评价交易' : '评价详情' }}</DialogTitle>
          <Badge variant="outline">{{ row.transactionType === 'api_order' ? 'API 订单' : '拼车' }}</Badge>
        </div>
        <DialogDescription>{{ row.target }} · 对方 {{ row.counterparty }}</DialogDescription>
      </DialogHeader>

      <template v-if="row">
        <div class="grid gap-1 border-y border-border py-3 text-sm sm:grid-cols-2">
          <span>交易完成：<LocalTime :value="row.completedAt" /></span>
          <span>评价截止：<LocalTime :value="row.reviewDeadlineAt" /></span>
        </div>

        <div v-if="canSubmit" class="space-y-5">
          <fieldset>
            <legend class="text-sm font-medium">评分</legend>
            <StarRatingInput v-model="form.rating" class="mt-2" />
            <p class="mt-1 text-xs text-muted-foreground">{{ form.rating ? `${form.rating} 分` : '请选择评分' }}</p>
          </fieldset>

          <fieldset class="space-y-2">
            <legend class="text-sm font-medium">体验标签（最多 5 个）</legend>
            <div class="grid gap-2 sm:grid-cols-2">
              <label v-for="tag in row.allowedTags" :key="tag.code" class="flex min-h-10 items-center gap-2 rounded-md border border-border px-3 py-2 text-sm">
                <Checkbox :model-value="form.tags.includes(tag.code)" @update:model-value="checked => toggleTag(tag.code, checked)" />
                <span>{{ tag.label }}</span>
              </label>
            </div>
          </fieldset>

          <label class="block space-y-2">
            <span class="text-sm font-medium">评价说明（可选）</span>
            <Textarea v-model="form.note" rows="4" maxlength="600" placeholder="也可以不写说明，至少选择一个标签即可。" />
            <span class="block text-right text-xs text-muted-foreground">{{ form.note.length }}/600</span>
          </label>

          <p class="flex items-start gap-2 text-xs leading-5 text-muted-foreground">
            <Clock3 class="mt-0.5 h-4 w-4 shrink-0" />
            评价公开前对对方不可见。双方都提交后立即公开；如只有一方提交，将在评价截止后公开。公开后不可修改。
          </p>
        </div>

        <div v-else-if="contentVisible && row.rating !== null" class="space-y-4">
          <StarRatingDisplay :rating="row.rating" />
          <div v-if="row.tags.length" class="flex flex-wrap gap-1.5"><Badge v-for="tag in row.tags" :key="tag" variant="secondary">{{ tagLabel(tag) }}</Badge></div>
          <p v-if="row.note" class="whitespace-pre-line text-sm leading-6">{{ row.note }}</p>
          <p v-else class="text-sm text-muted-foreground">该评价未填写文字说明。</p>
        </div>

        <p v-else class="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">
          {{ row.status === 'expired' ? '评价已截止。' : row.visibility === 'removed' ? '评价已移除。' : '评价尚未公开。' }}
        </p>

        <DialogFooter v-if="canSubmit">
          <Button variant="outline" :disabled="mutation.isPending.value" @click="setOpen(false)">取消</Button>
          <Button :disabled="!formValid || mutation.isPending.value" @click="submit">
            <Send class="h-4 w-4" />{{ mutation.isPending.value ? '提交中' : row.canEdit ? '保存修改' : '提交评价' }}
          </Button>
        </DialogFooter>
      </template>
    </DialogScrollContent>
  </Dialog>
</template>
