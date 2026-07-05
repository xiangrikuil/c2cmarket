<script setup lang="ts">
import { computed, ref } from 'vue'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import type { ApiDeliveryMode, ApiService } from '@/lib/api'
import PurchaseAmountSelector from './PurchaseAmountSelector.vue'
import PurchaseConfirmDialog from './PurchaseConfirmDialog.vue'
import PurchaseSummary from './PurchaseSummary.vue'

const props = defineProps<{
  service: ApiService
  amount: number
  selectedDeliveryMode: ApiDeliveryMode
  submitting: boolean
}>()

const emit = defineEmits<{
  'update:amount': [value: number]
  'update:selectedDeliveryMode': [value: ApiDeliveryMode]
  confirm: []
}>()

const acknowledged = ref(false)
const confirmOpen = ref(false)

const amountError = computed(() => {
  const decimalPattern = /^\d+(\.\d{1,2})?$/
  if (!Number.isFinite(props.amount) || props.amount <= 0) return '请输入有效金额。'
  if (!decimalPattern.test(String(props.amount))) return '自定义金额最多保留两位小数。'
  if (props.amount < props.service.minimumPurchaseCny) return `最低意向金额为 ¥${props.service.minimumPurchaseCny}。`
  if (props.amount > props.service.maxBuy) return `单笔最高意向金额为 ¥${props.service.maxBuy}。`
  if (props.amount > props.service.balance / props.service.creditPerCny) return '超过商户当前可售美元额度上限。'
  return ''
})

const canSubmit = computed(() => !amountError.value && acknowledged.value && props.service.publiclyOrderable && !props.submitting)

function openConfirm() {
  if (!canSubmit.value) return
  confirmOpen.value = true
}

function confirm() {
  emit('confirm')
}
</script>

<template>
  <Card class="min-w-0 gap-0 overflow-hidden py-0 shadow-sm lg:sticky lg:top-16">
    <div class="flex items-center justify-between border-b border-border px-4 py-3">
      <div>
        <h2 class="text-base font-semibold">提交购买意向</h2>
        <p class="mt-1 text-xs text-muted-foreground">最终金额、接入细节和用量核对由双方站外确认</p>
      </div>
      <span class="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-2 py-1 text-xs text-emerald-700">
        <span class="h-1.5 w-1.5 rounded-full bg-emerald-500" />
        {{ service.publiclyOrderable ? '可提交意向' : '暂不可接单' }}
      </span>
    </div>

    <div class="space-y-4 p-4">
      <div class="space-y-2">
        <div class="text-sm font-semibold">意向金额</div>
        <PurchaseAmountSelector :service="service" :model-value="amount" @update:model-value="value => emit('update:amount', value)" />
        <p v-if="amountError" class="text-xs text-destructive">{{ amountError }}</p>
      </div>

      <PurchaseSummary :service="service" :amount="amount" :selected-delivery-mode="selectedDeliveryMode" />

      <label class="flex items-start gap-2 text-xs text-muted-foreground">
        <input v-model="acknowledged" type="checkbox" class="mt-0.5 h-4 w-4 accent-primary" />
        <span>我已阅读模型价格、接入说明、商户承诺和退款规则，并理解最终由双方站外确认；平台不担保、不代赔。</span>
      </label>

      <Button class="w-full" :disabled="!canSubmit" @click="openConfirm">
        {{ submitting ? '提交中...' : '提交购买意向并查看商户联系方式' }}
      </Button>
      <p class="text-center text-xs text-muted-foreground">提交成功后立即展示商户联系方式；商户也可以查看你选择的联系方式。</p>
      <p class="text-center text-xs font-medium text-destructive">站外仅允许买家专属、可撤销的子账号或子 Key；不得在平台填写、粘贴或上传主账号、主 Key、API Key、密码、token、Session、Cookie 或面板登录凭据。</p>
    </div>
  </Card>

  <PurchaseConfirmDialog
    :open="confirmOpen"
    :service="service"
    :amount="amount"
    :selected-delivery-mode="selectedDeliveryMode"
    :submitting="submitting"
    @close="confirmOpen = false"
    @confirm="confirm"
  />
</template>
