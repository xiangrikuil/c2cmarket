<script setup lang="ts">
import { ImageUp, Trash2 } from 'lucide-vue-next'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Textarea } from '@/components/ui/textarea'
import {
  apiPaymentMethodLabels,
  apiPaymentMethods,
  apiPaymentMethodRequiresQrCode,
  isApiPaymentOptionComplete,
  type ApiPaymentOption,
} from '@/lib/apiPaymentSettings'

const props = defineProps<{
  option: ApiPaymentOption
  dirty: boolean
  disabled: boolean
}>()

const emit = defineEmits<{
  'update:enabled': [enabled: boolean]
  'update:instructions': [instructions: string]
  upload: [event: Event]
  'request-remove-qr': []
}>()

const methodHint = apiPaymentMethods.find(item => item.value === props.option.paymentMethod)?.hint ?? ''
</script>

<template>
  <Card class="contact-payment-option-card p-4" :class="{ 'border-primary/30': option.enabled }">
    <div v-if="!option.enabled" class="flex min-h-10 items-center gap-3">
      <ApiPaymentMethodIcon :method="option.paymentMethod" size="md" />
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-2">
          <h3 class="text-sm font-semibold">{{ apiPaymentMethodLabels[option.paymentMethod] }}</h3>
          <Badge variant="secondary">未启用</Badge>
          <Badge v-if="dirty" variant="secondary">有未保存更改</Badge>
        </div>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ methodHint }}</p>
      </div>
      <Button
        type="button"
        size="sm"
        variant="outline"
        :disabled="disabled"
        @click="emit('update:enabled', true)"
      >
        启用配置
      </Button>
    </div>

    <div v-else class="space-y-4">
      <div class="flex items-start gap-3">
        <Checkbox
          :model-value="option.enabled"
          class="mt-2"
          :disabled="disabled"
          :aria-label="`停用${apiPaymentMethodLabels[option.paymentMethod]}`"
          @update:model-value="value => emit('update:enabled', Boolean(value))"
        />
        <ApiPaymentMethodIcon :method="option.paymentMethod" size="md" />
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="text-sm font-semibold">{{ apiPaymentMethodLabels[option.paymentMethod] }}</h3>
            <Badge :variant="isApiPaymentOptionComplete(option) ? 'verified' : 'secondary'">
              {{ isApiPaymentOptionComplete(option) ? '已就绪' : '待补全' }}
            </Badge>
            <Badge v-if="dirty" variant="secondary">有未保存更改</Badge>
          </div>
          <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ methodHint }}</p>
        </div>
      </div>

      <div v-if="apiPaymentMethodRequiresQrCode(option.paymentMethod)" class="contact-payment-qr-editor">
        <div class="grid h-20 w-20 shrink-0 place-items-center overflow-hidden rounded-md border border-border bg-muted/40">
          <img
            v-if="option.paymentQrCodeDataUrl"
            :src="option.paymentQrCodeDataUrl"
            :alt="`${apiPaymentMethodLabels[option.paymentMethod]}收款码`"
            class="h-full w-full object-contain"
          />
          <ImageUp v-else class="h-5 w-5 text-muted-foreground" />
        </div>
        <div class="min-w-0 flex-1 space-y-2">
          <div class="flex flex-wrap gap-2">
            <input
              :id="`api-payment-qr-${option.paymentMethod}`"
              class="sr-only"
              type="file"
              accept="image/png,image/jpeg,image/webp"
              :disabled="disabled"
              @change="emit('upload', $event)"
            />
            <label
              :for="`api-payment-qr-${option.paymentMethod}`"
              class="contact-payment-upload-button"
              :class="{ 'pointer-events-none opacity-50': disabled }"
            >
              <ImageUp class="h-4 w-4" />
              {{ option.paymentQrCodeDataUrl ? '替换收款码' : '上传收款码' }}
            </label>
            <Button
              v-if="option.paymentQrCodeDataUrl"
              type="button"
              size="icon"
              variant="outline"
              :disabled="disabled"
              :aria-label="`删除${apiPaymentMethodLabels[option.paymentMethod]}收款码`"
              title="删除收款码"
              @click="emit('request-remove-qr')"
            >
              <Trash2 class="h-4 w-4" />
            </Button>
          </div>
          <p class="text-xs leading-5 text-muted-foreground">支持 PNG、JPG、WebP，最多 512KB。</p>
        </div>
      </div>

      <label class="block space-y-2">
        <span class="text-sm font-medium">付款说明（选填）</span>
        <Textarea
          :model-value="option.paymentInstructions"
          class="min-h-16 text-sm"
          maxlength="160"
          :disabled="disabled"
          placeholder="填写收款码备注、核对口径或站外确认节奏。"
          @update:model-value="value => emit('update:instructions', String(value))"
        />
      </label>
    </div>
  </Card>
</template>
