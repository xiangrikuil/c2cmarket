<script setup lang="ts">
import type { CarpoolPublishForm, OpeningChannelOption, PaymentMethodOption, PublishFieldState } from './types'
import PublishSectionCard from './PublishSectionCard.vue'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const props = defineProps<{
  form: CarpoolPublishForm
  openingChannels: OpeningChannelOption[]
  paymentMethods: PaymentMethodOption[]
  errors: Partial<Record<string, string>>
  fieldStates?: Partial<Record<string, PublishFieldState>>
  highlightedKey?: string
}>()

function fieldState(key: string): PublishFieldState {
  return props.fieldStates?.[key] ?? 'idle'
}

function fieldShellClass(key: string) {
  const state = fieldState(key)
  return [
    'rounded-lg border p-3 transition-colors',
    state === 'error' ? 'border-destructive/45 bg-destructive/5' : '',
    state === 'pendingRequired' ? 'border-warning/40 bg-warning/5' : '',
    state === 'complete' ? 'border-border bg-background' : '',
    state === 'idle' ? 'border-transparent bg-transparent p-0' : '',
    props.highlightedKey === key ? 'ring-2 ring-primary/60 ring-offset-2 ring-offset-background' : '',
  ]
}

function stateLabel(key: string) {
  const state = fieldState(key)
  if (state === 'error') return '需要处理'
  if (state === 'pendingRequired') return '待填写'
  if (state === 'complete') return '已完成'
  return ''
}

function stateLabelClass(key: string) {
  const state = fieldState(key)
  if (state === 'error') return 'bg-destructive/10 text-destructive'
  if (state === 'pendingRequired') return 'bg-warning/10 text-warning'
  if (state === 'complete') return 'bg-success/10 text-success'
  return 'bg-muted text-muted-foreground'
}
</script>

<template>
  <PublishSectionCard
    :index="3"
    title="开通渠道与付款方式"
    description="开通渠道和付款方式分开维护，Google Play 属于渠道，Google Pay 属于付款方式。"
  >
    <div class="grid gap-5 md:grid-cols-2">
      <div id="carpool-task-openingChannel" class="space-y-2" :class="fieldShellClass('openingChannel')">
        <div class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>开通渠道 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('openingChannel')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('openingChannel')">{{ stateLabel('openingChannel') }}</span>
        </div>
        <Select v-model="form.openingChannelCode">
          <SelectTrigger class="w-full bg-background"><SelectValue placeholder="选择开通渠道" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="channel in openingChannels" :key="channel.code" :value="channel.code">{{ channel.displayName }}</SelectItem>
          </SelectContent>
        </Select>
        <Input v-if="form.openingChannelCode === 'other'" v-model="form.customOpeningChannel" maxlength="64" placeholder="填写其他开通渠道" />
        <p v-if="errors.openingChannelCode" class="text-xs text-destructive">{{ errors.openingChannelCode }}</p>
        <p v-else-if="fieldState('openingChannel') === 'pendingRequired'" class="text-xs text-warning">请选择买家实际开通渠道。</p>
      </div>

      <div id="carpool-task-paymentMethods" class="space-y-2" :class="fieldShellClass('paymentMethods')">
        <div class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>付款方式 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('paymentMethods')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('paymentMethods')">{{ stateLabel('paymentMethods') }}</span>
        </div>
        <Select v-model="form.paymentMethodCode">
          <SelectTrigger class="w-full bg-background"><SelectValue placeholder="选择付款方式" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="method in paymentMethods" :key="method.code" :value="method.code">{{ method.displayName }}</SelectItem>
          </SelectContent>
        </Select>
        <Input v-if="form.paymentMethodCode === 'other'" v-model="form.customPaymentMethod" maxlength="64" placeholder="填写其他付款方式" />
        <p v-if="errors.paymentMethodCode" class="text-xs text-destructive">{{ errors.paymentMethodCode }}</p>
        <p v-else-if="fieldState('paymentMethods') === 'pendingRequired'" class="text-xs text-warning">请选择一种站外付款方式。</p>
        <p v-else class="text-xs text-muted-foreground">车源只保留一种付款方式，买家按该方式站外确认。</p>
      </div>
    </div>

  </PublishSectionCard>
</template>
