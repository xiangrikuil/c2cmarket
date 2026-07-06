<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { CreditCard, ExternalLink } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  apiPaymentMethodLabels,
  apiPaymentMethodRequiresQrCode,
  apiPaymentSettingsMissingReason,
  apiPaymentSettingsSummary,
  enabledApiPaymentOptions,
  isApiPaymentAccountSettingsComplete,
  isApiPaymentOptionComplete,
  type ApiPaymentOption,
  type ApiPaymentAccountSettings,
} from '@/lib/apiPaymentSettings'
import type { ApiServicePublishForm } from './types'

const props = defineProps<{
  form: ApiServicePublishForm
  settings: ApiPaymentAccountSettings
  loading: boolean
}>()

const enabledOptions = computed(() => enabledApiPaymentOptions(props.settings))
const complete = computed(() => isApiPaymentAccountSettingsComplete(props.settings))
const missingReason = computed(() => apiPaymentSettingsMissingReason(props.settings))
const summary = computed(() => apiPaymentSettingsSummary(props.settings))

function optionStatus(option: ApiPaymentOption) {
  if (!option.enabled) return '未启用'
  if (isApiPaymentOptionComplete(option)) return '已就绪'
  return apiPaymentMethodRequiresQrCode(option.paymentMethod) ? '缺收款码' : '缺说明'
}

function optionSummary(option: ApiPaymentOption) {
  if (!option.enabled) return '未启用'
  if (apiPaymentMethodRequiresQrCode(option.paymentMethod)) {
    if (!option.paymentQrCodeDataUrl) return '未上传收款码'
    return option.paymentInstructions.trim() || '已上传收款码，买家提交意向后可见'
  }
  return option.paymentInstructions.trim() || '未填写站外确认说明'
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2>2. 收款与接单</h2>
          <p>使用我的中心里的 API 收款设置；发布时会复制为本服务的接单快照。</p>
        </div>
        <Badge :variant="complete ? 'verified' : 'secondary'">{{ complete ? '已配置' : '待配置' }}</Badge>
      </div>
    </div>

    <div class="api-publish-card-body space-y-3">
      <div
        class="flex gap-3 rounded-md border px-3 py-3 text-sm leading-6"
        :class="complete ? 'border-success/20 bg-success/5 text-success' : 'border-warning/25 bg-warning/10 text-warning'"
      >
        <CreditCard class="mt-1 h-4 w-4 shrink-0" />
        <div class="min-w-0">
          <div class="font-medium">{{ loading ? '正在读取 API 收款设置...' : summary }}</div>
          <p class="mt-1 text-xs leading-5">
            {{ complete ? '发布后买家可按该服务快照提交意向；之后修改我的中心不会静默改变已发布服务。' : missingReason }}
          </p>
        </div>
      </div>

      <div class="grid gap-2 sm:grid-cols-3">
        <div v-for="option in settings.paymentOptions" :key="option.paymentMethod" class="rounded-md border border-border bg-muted/35 p-3">
          <div class="flex items-center justify-between gap-2">
            <span class="text-sm font-semibold">{{ apiPaymentMethodLabels[option.paymentMethod] }}</span>
            <Badge :variant="option.enabled && isApiPaymentOptionComplete(option) ? 'verified' : 'secondary'">
              {{ optionStatus(option) }}
            </Badge>
          </div>
          <p class="mt-2 line-clamp-2 text-xs leading-5 text-muted-foreground">
            {{ optionSummary(option) }}
          </p>
        </div>
      </div>

      <div class="flex flex-col gap-2 rounded-md border border-border bg-muted/50 px-3 py-2 text-xs leading-5 text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
        <span>平台不托管支付；收款码只在买家提交意向后用于站外确认，不保存付款码、API Key、token、账号密码或面板凭据。</span>
        <RouterLink to="/my/contacts" class="shrink-0">
          <Button size="sm" variant="outline">
            去我的中心修改 <ExternalLink class="h-3.5 w-3.5" />
          </Button>
        </RouterLink>
      </div>

      <p v-if="enabledOptions.length" class="text-xs text-muted-foreground">
        本次发布将快照 {{ enabledOptions.length }} 种收款方式，买家确认付款窗口固定 {{ form.paymentWindowMinutes }} 分钟。
      </p>
    </div>
  </Card>
</template>
