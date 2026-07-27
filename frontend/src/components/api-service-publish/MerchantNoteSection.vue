<script setup lang="ts">
import { Gauge } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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
      <div class="flex items-start gap-2">
        <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-amber-50 text-amber-600">
          <Gauge class="h-4 w-4" />
        </span>
        <div>
          <h2>服务体验与说明</h2>
          <p>填写商户自报体验、用量核对与售后口径。</p>
        </div>
      </div>
    </div>

    <div class="api-publish-card-body space-y-3">
      <div class="grid gap-3 md:grid-cols-3">
        <label class="space-y-2">
          <span class="text-sm font-medium">首字响应区间</span>
          <Select v-model="form.declaredTtftBand">
            <SelectTrigger
              :aria-invalid="Boolean(errors.performance)"
              :aria-describedby="errors.performance ? 'api-publish-performance-error' : undefined"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="under_1s">&lt;1 秒</SelectItem>
              <SelectItem value="1_to_3s">1-3 秒</SelectItem>
              <SelectItem value="3_to_5s">3-5 秒</SelectItem>
              <SelectItem value="5_to_10s">5-10 秒</SelectItem>
              <SelectItem value="over_10s">&gt;10 秒</SelectItem>
            </SelectContent>
          </Select>
        </label>
        <label class="space-y-2">
          <span class="text-sm font-medium">建议并发</span>
          <Input
            v-model.number="form.recommendedConcurrency"
            :aria-invalid="Boolean(errors.performance)"
            :aria-describedby="errors.performance ? 'api-publish-performance-error' : undefined"
            type="number"
            min="1"
            max="100000"
          />
        </label>
        <label class="space-y-2">
          <span class="text-sm font-medium">最近确认时间</span>
          <Input
            v-model="form.performanceConfirmedAt"
            :aria-invalid="Boolean(errors.performance)"
            :aria-describedby="errors.performance ? 'api-publish-performance-error' : undefined"
            type="datetime-local"
          />
        </label>
      </div>
      <p v-if="errors.performance" id="api-publish-performance-error" class="text-xs text-destructive">{{ errors.performance }}</p>
      <p class="text-xs text-muted-foreground">商户自报，平台未测速。额度包将冻结购买时的声明。</p>

      <div class="rounded-md border border-border bg-muted/45 px-3 py-2 text-xs leading-5 text-muted-foreground">
        不要填写 API Key、token、密码、Session、Cookie、付款码或面板凭据；买家创建订单后，双方站外确认 API 细节。
      </div>

      <div class="space-y-2">
        <Textarea
          v-model="form.merchantNote"
          :aria-invalid="Boolean(errors.merchantNote)"
          :aria-describedby="errors.merchantNote ? 'api-publish-merchant-note-error' : undefined"
          class="min-h-28"
          maxlength="800"
          placeholder="请说明用量核对、限速规则、可用时间和售后口径。"
        />
        <div class="flex items-center justify-between gap-3">
          <p v-if="errors.merchantNote" id="api-publish-merchant-note-error" class="text-xs text-destructive">{{ errors.merchantNote }}</p>
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
