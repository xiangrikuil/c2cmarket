<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { ExternalLink, MessagesSquare } from 'lucide-vue-next'
import type { UserContactMethod } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import type { ApiServicePublishForm } from './types'

const props = defineProps<{
  form: ApiServicePublishForm
  contacts: UserContactMethod[]
  loading: boolean
  error?: string
}>()

const contactTypeLabels = {
  linuxdo: 'linux.do',
  wechat: '微信',
  email: '邮箱',
  telegram: 'Telegram',
  other: '其他',
} as const

function toggleContact(id: string, checked: boolean) {
  props.form.ownerContactMethodIds = checked
    ? [...new Set([...props.form.ownerContactMethodIds, id])]
    : props.form.ownerContactMethodIds.filter(item => item !== id)
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="flex items-start gap-2">
          <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-emerald-50 text-emerald-700">
            <MessagesSquare class="h-4 w-4" />
          </span>
          <div>
            <h2>订单联系方式</h2>
            <p>成交时锁定所选联系方式，仅向订单参与方展示。</p>
          </div>
        </div>
        <Button size="sm" variant="outline" as-child>
          <RouterLink to="/my/contacts">管理联系方式 <ExternalLink class="h-3.5 w-3.5" /></RouterLink>
        </Button>
      </div>
    </div>
    <div class="api-publish-card-body space-y-2">
      <div v-if="loading" class="rounded-md border border-border px-3 py-3 text-sm text-muted-foreground">正在读取联系方式...</div>
      <label
        v-for="contact in contacts"
        v-else
        :key="contact.id"
        class="flex items-start gap-3 rounded-md border border-border px-3 py-3 hover:bg-muted/35"
      >
        <Checkbox
          :model-value="form.ownerContactMethodIds.includes(contact.id)"
          class="mt-0.5"
          @update:model-value="value => toggleContact(contact.id, Boolean(value))"
        />
        <span class="min-w-0 flex-1">
          <span class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-medium">
            <span>{{ contact.label || contactTypeLabels[contact.type] }}</span>
            <span class="text-xs font-normal text-muted-foreground">{{ contactTypeLabels[contact.type] }}</span>
          </span>
          <span class="mt-0.5 block break-all text-xs text-muted-foreground">{{ contact.maskedValue }}</span>
        </span>
      </label>
      <div v-if="!loading && !contacts.length" class="rounded-md border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
        暂无可用于 API 订单的联系方式，请先确认当前账号已绑定 linux.do，或在个人中心添加微信。
      </div>
      <p v-if="error" class="text-xs text-destructive" role="alert">{{ error }}</p>
      <p v-else class="text-xs leading-5 text-muted-foreground">linux.do 来自当前账号绑定且只显示一项；建议同时选择微信和 linux.do，方便买家联系。</p>
    </div>
  </Card>
</template>
