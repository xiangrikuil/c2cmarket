<script setup lang="ts">
import { Textarea } from '@/components/ui/textarea'
import type { CarpoolPublishForm } from './types'
import PublishSectionCard from './PublishSectionCard.vue'

defineProps<{
  form: CarpoolPublishForm
  errors: Partial<Record<string, string>>
}>()

const templates = [
  '付款周期按自然月结算',
  '价格锁定至本周期结束',
  '人数变化后下期重新确认',
  '中转方式：VPS 转发 / 家宽转发 / 成员邀请',
  '家宽地区：仅填写国家或地区，不填写具体 IP',
  'Sub2API 托管管理：支持 / 不支持，具体方式站外确认',
  'Web 端使用：支持 / 不支持',
  '服务中断按车主承诺处理',
  '禁止违反上游规则的用途',
  '售后通常当天响应',
]

function insertTemplate(form: CarpoolPublishForm, value: string) {
  if (form.rulesNote.includes(value)) return
  form.rulesNote = [form.rulesNote.trim(), value].filter(Boolean).join('；')
}
</script>

<template>
  <PublishSectionCard
    :index="7"
    title="规则说明与买家须知"
    description="写清付款周期、退款规则、名额变化、中转或托管边界、Web 端支持、禁止用途和车主承诺响应；倍率和每月额度已在基础信息中结构化填写。"
  >
    <Textarea
      v-model="form.rulesNote"
      class="min-h-32"
      maxlength="1200"
      placeholder="建议说明：付款周期、价格锁定、退款规则、名额变化；中转方式（VPS 转发 / 家宽转发 / 成员邀请）；家宽地区（只写国家或地区，不写具体 IP）；是否支持 Sub2API 托管管理（仅站外确认，平台不收集凭据）；是否可用 Web 端；车主承诺响应。倍率和每月额度请使用基础信息中的结构化字段，不要填写账号密码、管理员凭据、session token、refresh token、API Key、付款二维码或银行卡号。"
    />
    <div class="mt-2 flex items-center justify-between gap-3">
      <p v-if="errors.rulesNote" class="text-xs text-destructive">{{ errors.rulesNote }}</p>
      <p class="ml-auto text-xs text-muted-foreground">已输入 {{ form.rulesNote.length }} / 1200 字</p>
    </div>
    <details class="mt-3 rounded-md border border-border bg-muted/30 p-3">
      <summary class="cursor-pointer text-sm font-medium">常用说明模板</summary>
      <div class="mt-3 flex flex-wrap gap-2">
        <button
          v-for="template in templates"
          :key="template"
          type="button"
          class="rounded-full border border-border bg-background px-3 py-1 text-xs hover:bg-muted"
          @click="insertTemplate(form, template)"
        >
          + {{ template }}
        </button>
      </div>
    </details>
  </PublishSectionCard>
</template>
