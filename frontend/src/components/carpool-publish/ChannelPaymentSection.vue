<script setup lang="ts">
import type { CarpoolPublishForm, OpeningChannelOption, PaymentMethodCode, PaymentMethodOption, PublishFieldState } from './types'
import type { AcceptableValue } from 'reka-ui'
import PublishSectionCard from './PublishSectionCard.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

const props = defineProps<{
  form: CarpoolPublishForm
  openingChannels: OpeningChannelOption[]
  paymentMethods: PaymentMethodOption[]
  errors: Partial<Record<string, string>>
  fieldStates?: Partial<Record<string, PublishFieldState>>
  highlightedKey?: string
}>()

function selectPayment(code: PaymentMethodCode) {
  props.form.paymentMethodCodes = [code]
}

function adminAccountSelectValue() {
  if (props.form.providesAdminAccount === null) return ''
  return props.form.providesAdminAccount ? 'true' : 'false'
}

function setAdminAccount(value: AcceptableValue) {
  if (value === 'true') props.form.providesAdminAccount = true
  else if (value === 'false') props.form.providesAdminAccount = false
  else props.form.providesAdminAccount = null
}

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
        <p v-else-if="fieldState('openingChannel') === 'pendingRequired'" class="text-xs text-warning">请选择买家实际开通渠道。</p>
      </div>

      <div id="carpool-task-paymentMethods" class="space-y-2" :class="fieldShellClass('paymentMethods')">
        <div class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>付款方式 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('paymentMethods')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('paymentMethods')">{{ stateLabel('paymentMethods') }}</span>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="method in paymentMethods"
            :key="method.code"
            type="button"
            :aria-pressed="form.paymentMethodCodes.includes(method.code)"
            class="rounded-md border px-3 py-2 text-sm font-medium transition"
            :class="form.paymentMethodCodes.includes(method.code) ? 'border-primary bg-primary/10 text-primary' : 'border-border bg-background hover:bg-muted'"
            @click="selectPayment(method.code)"
          >
            {{ method.displayName }}
          </button>
        </div>
        <p v-if="errors.paymentMethodCodes" class="text-xs text-destructive">{{ errors.paymentMethodCodes }}</p>
        <p v-else-if="fieldState('paymentMethods') === 'pendingRequired'" class="text-xs text-warning">请选择一种站外付款方式。</p>
        <p v-else class="text-xs text-muted-foreground">车源只保留一种付款方式，买家按该方式站外确认。</p>
      </div>
    </div>

    <div id="carpool-task-distribution" class="mt-5 space-y-3" :class="fieldShellClass('distribution')">
      <div class="flex items-center justify-between gap-2 text-sm font-medium">
        <span>分发方式与管理员账号 <span class="text-xs text-primary">必填</span></span>
        <span v-if="stateLabel('distribution')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('distribution')">{{ stateLabel('distribution') }}</span>
      </div>
      <div class="grid gap-3 md:grid-cols-2">
        <label class="space-y-2 text-sm">
          <span class="font-medium">分发方式</span>
          <Select v-model="form.distributionMethod">
            <SelectTrigger class="w-full bg-background">
              <SelectValue placeholder="选择分发方式" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="sub2api">Sub2API</SelectItem>
              <SelectItem value="other">其他</SelectItem>
            </SelectContent>
          </Select>
        </label>
        <label class="space-y-2 text-sm">
          <span class="font-medium">管理员账号</span>
          <Select :model-value="adminAccountSelectValue()" @update:model-value="setAdminAccount">
            <SelectTrigger class="w-full bg-background">
              <SelectValue placeholder="选择是否提供" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="true">提供管理员账号</SelectItem>
              <SelectItem value="false">不提供管理员账号</SelectItem>
            </SelectContent>
          </Select>
        </label>
      </div>
      <label v-if="form.distributionMethod === 'other'" class="block space-y-2 text-sm">
        <span class="font-medium">其他分发说明</span>
        <Textarea v-model="form.distributionMethodNote" class="min-h-20 bg-background" placeholder="说明站外分发方式，不填写账号、密码、面板地址、API Key、Session、Cookie 或 token。" />
      </label>
      <p v-if="errors.distribution" class="text-xs text-destructive">{{ errors.distribution }}</p>
      <p v-else class="text-xs text-muted-foreground">这里只展示公开信号；具体权限和使用细节请站外确认，平台不保存任何凭据。</p>
    </div>
  </PublishSectionCard>
</template>
