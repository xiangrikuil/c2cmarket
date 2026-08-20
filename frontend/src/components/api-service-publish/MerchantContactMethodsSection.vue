<script setup lang="ts">
import { MessagesSquare } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import TransactionContactSelector from '@/components/contact-payment/TransactionContactSelector.vue'
import type { ApiServicePublishForm } from './types'

defineProps<{
  form: ApiServicePublishForm
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
            <p>为这项服务产生的新交易选择一个联系方式。</p>
          </div>
        </div>
      </div>
    </div>
    <div class="api-publish-card-body">
      <TransactionContactSelector
        v-model="form.ownerContactMethodId"
        title="服务交易联系方式"
        description="发布或更新时锁定当前版本；linux.do 只作为身份信息。"
        :error="error"
      />
    </div>
  </Card>
</template>
