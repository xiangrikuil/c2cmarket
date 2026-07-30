<script setup lang="ts">
import { computed } from 'vue'
import { CreditCard, Settings2 } from 'lucide-vue-next'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  apiPaymentMethodLabels,
  apiPaymentSettingsMissingReason,
  apiPaymentSettingsSummary,
  enabledApiPaymentOptions,
  isApiPaymentAccountSettingsComplete,
  type ApiPaymentAccountSettings,
} from '@/lib/apiPaymentSettings'
import type { ApiServicePublishForm } from './types'

const props = defineProps<{
  form: ApiServicePublishForm
  settings: ApiPaymentAccountSettings
  loading: boolean
}>()

const emit = defineEmits<{
  edit: []
}>()

const enabledOptions = computed(() => enabledApiPaymentOptions(props.settings))
const complete = computed(() => isApiPaymentAccountSettingsComplete(props.settings))
const missingReason = computed(() => apiPaymentSettingsMissingReason(props.settings))
const summary = computed(() => apiPaymentSettingsSummary(props.settings))
const enabledOption = computed(() => enabledOptions.value[0] ?? null)
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="flex items-start gap-2">
          <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-cyan-50 text-cyan-600">
            <CreditCard class="h-4 w-4" />
          </span>
          <div>
            <h2>收款与接单</h2>
            <p>使用账户级收款设置，发布时复制为服务快照。</p>
          </div>
        </div>
        <Badge :variant="complete ? 'verified' : 'secondary'">{{ complete ? '已配置' : '待配置' }}</Badge>
      </div>
    </div>

    <div class="api-publish-card-body space-y-3">
      <div
        class="flex gap-2 rounded-md border px-3 py-2 text-xs leading-5"
        :class="complete ? 'border-success/20 bg-success/5 text-success' : 'border-warning/25 bg-warning/10 text-warning'"
      >
        <CreditCard class="mt-0.5 h-4 w-4 shrink-0" />
        <div class="min-w-0">
          <div class="font-medium">{{ loading ? '正在读取 API 收款设置...' : summary }}</div>
          <p class="mt-0.5 leading-5">
            {{ complete ? '发布后买家可按该服务快照创建订单；之后修改个人中心不会静默改变已发布服务。' : missingReason }}
          </p>
        </div>
      </div>

      <div class="flex flex-col gap-2 rounded-md border border-border bg-muted/35 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between">
        <div v-if="enabledOption" class="flex min-w-0 items-center gap-2.5">
          <ApiPaymentMethodIcon :method="enabledOption.paymentMethod" size="md" />
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-sm font-semibold">{{ apiPaymentMethodLabels[enabledOption.paymentMethod] }}</span>
              <Badge variant="verified">当前收款方式</Badge>
            </div>
            <p class="mt-0.5 truncate text-xs text-muted-foreground">
              {{ enabledOption.paymentInstructions.trim() || '已上传收款码，买家创建订单后可见' }}
            </p>
          </div>
        </div>
        <span v-else class="text-sm text-muted-foreground">尚未选择收款方式</span>
        <span class="shrink-0 text-xs text-muted-foreground">固定 {{ form.paymentWindowMinutes }} 分钟确认</span>
      </div>

      <div class="flex flex-col gap-2 rounded-md border border-border bg-muted/50 px-3 py-2 text-[11px] leading-5 text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
        <span>平台不托管支付；收款信息只在订单后用于站外确认。</span>
        <Button
          class="shrink-0"
          size="sm"
          variant="outline"
          :disabled="loading"
          @click="emit('edit')"
        >
          <Settings2 class="h-3.5 w-3.5" />{{ complete ? '修改收款方式' : '设置收款方式' }}
        </Button>
      </div>
    </div>
  </Card>
</template>
