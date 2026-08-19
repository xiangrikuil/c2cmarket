<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { ExternalLink, MessagesSquare } from 'lucide-vue-next'
import type { UserContactMethod } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import type { ApiServicePublishForm } from './types'

defineProps<{
  form: ApiServicePublishForm
  contacts: UserContactMethod[]
  loading: boolean
  error?: string
}>()

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
            <p>成交时锁定当前微信，仅向订单参与方展示。</p>
          </div>
        </div>
        <Button size="sm" variant="outline" as-child>
          <RouterLink to="/my/contacts">管理联系方式 <ExternalLink class="h-3.5 w-3.5" /></RouterLink>
        </Button>
      </div>
    </div>
    <div class="api-publish-card-body space-y-2">
      <div v-if="loading" class="rounded-md border border-border px-3 py-3 text-sm text-muted-foreground">正在读取联系方式...</div>
      <div
        v-for="contact in contacts"
        v-else
        :key="contact.id"
        class="flex items-start gap-3 rounded-md border border-border px-3 py-3"
      >
        <span class="min-w-0 flex-1">
          <span class="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm font-medium">
            <span>{{ contact.label || '微信' }}</span>
            <span class="text-xs font-normal text-muted-foreground">已配置</span>
          </span>
          <span class="mt-0.5 block break-all text-xs text-muted-foreground">{{ contact.maskedValue }}</span>
        </span>
      </div>
      <div v-if="!loading && !contacts.length" class="rounded-md border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
        暂未配置微信，发布 API 服务前请先到个人中心填写。
      </div>
      <p v-if="error" class="text-xs text-destructive" role="alert">{{ error }}</p>
      <p v-else class="text-xs leading-5 text-muted-foreground">微信自动用于 API 交易联系；linux.do 仅作为身份与信任信息展示。</p>
    </div>
  </Card>
</template>
