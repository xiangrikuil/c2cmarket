<script setup lang="ts">
import type { CarpoolPublishForm, OpeningChannelOption, PaymentMethodCode, PaymentMethodOption } from './types'
import PublishSectionCard from './PublishSectionCard.vue'

const props = defineProps<{
  form: CarpoolPublishForm
  openingChannels: OpeningChannelOption[]
  paymentMethods: PaymentMethodOption[]
  errors: Partial<Record<string, string>>
}>()

function togglePayment(code: PaymentMethodCode) {
  if (props.form.paymentMethodCodes.includes(code)) {
    props.form.paymentMethodCodes = props.form.paymentMethodCodes.filter(item => item !== code)
  } else {
    props.form.paymentMethodCodes = [...props.form.paymentMethodCodes, code]
  }
}
</script>

<template>
  <PublishSectionCard
    :index="4"
    title="开通渠道与支付方式"
    description="开通渠道和支付方式分开维护，Google Play 属于渠道，Google Pay 属于支付方式。"
  >
    <div class="grid gap-5 md:grid-cols-2">
      <div class="space-y-2">
        <div class="text-sm font-medium">开通渠道 <span class="text-xs text-primary">必填</span></div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="channel in openingChannels"
            :key="channel.code"
            type="button"
            class="rounded-md border px-3 py-2 text-sm font-medium transition"
            :class="form.openingChannelCode === channel.code ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
            @click="form.openingChannelCode = channel.code"
          >
            {{ channel.displayName }}
          </button>
        </div>
        <p v-if="errors.openingChannelCode" class="text-xs text-destructive">{{ errors.openingChannelCode }}</p>
      </div>

      <div class="space-y-2">
        <div class="text-sm font-medium">支付方式 <span class="text-xs text-primary">至少一项</span></div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="method in paymentMethods"
            :key="method.code"
            type="button"
            class="rounded-md border px-3 py-2 text-sm font-medium transition"
            :class="form.paymentMethodCodes.includes(method.code) ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
            @click="togglePayment(method.code)"
          >
            {{ method.displayName }}
          </button>
        </div>
        <p v-if="errors.paymentMethodCodes" class="text-xs text-destructive">{{ errors.paymentMethodCodes }}</p>
      </div>
    </div>
  </PublishSectionCard>
</template>
