<script setup lang="ts">
import { Card } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import type { ApiServicePublishForm } from './types'
import { merchantNoteQuickInserts } from './utils'

defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const insertSnippet = (form: ApiServicePublishForm, value: string) => {
  if (form.merchantNote.includes(value)) return
  const separator = form.merchantNote.trim().endsWith('。') || form.merchantNote.trim().endsWith('；') ? '\n' : '；'
  form.merchantNote = [form.merchantNote.trim(), value].filter(Boolean).join(separator)
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <h2>3. 备注信息</h2>
      <p>说明接入方式、用量核对、限速规则、可用时间和售后口径。</p>
    </div>

    <div class="api-publish-card-body space-y-3">
      <div class="rounded-md border border-border bg-muted/45 px-3 py-2 text-xs leading-5 text-muted-foreground">
        不要填写 API Key、token、密码、Session、Cookie、付款码或面板凭据；买家提交意向后，双方站外确认接入细节。
      </div>

      <div class="space-y-2">
        <Textarea
          v-model="form.merchantNote"
          class="min-h-40"
          maxlength="800"
          placeholder="请说明接入方式、用量核对、限速规则、可用时间和售后口径。"
        />
        <div class="flex items-center justify-between gap-3">
          <p v-if="errors.merchantNote" class="text-xs text-destructive">{{ errors.merchantNote }}</p>
          <p class="ml-auto text-xs text-muted-foreground">已输入 {{ form.merchantNote.length }} / 800 字</p>
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button
          v-for="snippet in merchantNoteQuickInserts"
          :key="snippet"
          type="button"
          class="rounded-full border border-border bg-background px-3 py-1 text-xs hover:bg-muted"
          @click="insertSnippet(form, snippet)"
        >
          + {{ snippet }}
        </button>
      </div>
    </div>
  </Card>
</template>
