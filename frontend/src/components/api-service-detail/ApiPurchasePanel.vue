<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { Flag, Heart, Share2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import ApiMerchantBadges from '@/components/api-market/ApiMerchantBadges.vue'
import SourceAuthorVerificationBadge from '@/components/market/SourceAuthorVerificationBadge.vue'
import ReputationInlineSummary from '@/components/reputation/ReputationInlineSummary.vue'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import ApiMerchantAvatar from '@/components/api-market/ApiMerchantAvatar.vue'
import { getApiMerchantDisplayName, getApiMerchantProfileUrl, type ApiService } from '@/lib/api'
import { apiPaymentMethodLabels } from '@/lib/apiPaymentSettings'
import PurchaseAmountSelector from './PurchaseAmountSelector.vue'
import PurchaseConfirmDialog from './PurchaseConfirmDialog.vue'
import { compareDecimal } from '@/lib/decimal'
import { isApiServiceTailOrder } from '@/lib/apiServicePricingPresentation'
import { apiServiceAvailableUsdAllowance, estimateUsdAllowance, formatCredit } from './utils'

const props = defineProps<{
  service: ApiService
  amount: number
  selectedPackageId: string
  submitting: boolean
  favorited: boolean
}>()

const emit = defineEmits<{
  'update:amount': [value: number]
  'update:selectedPackageId': [value: string]
  toggleFavorite: []
  confirm: []
}>()

const confirmOpen = ref(false)
const merchantUrl = computed(() => getApiMerchantProfileUrl(props.service))
const fixedPackageMode = computed(() => props.service.billingMode === 'fixed_package')
const tailOrder = computed(() => isApiServiceTailOrder(props.service))
const estimatedCredit = computed(() => tailOrder.value
  ? apiServiceAvailableUsdAllowance(props.service)
  : estimateUsdAllowance(String(props.amount), props.service))
const availablePackages = computed(() => (props.service.packages ?? []).filter(item => item.enabled && item.stockAvailable > 0))
const selectedPackage = computed(() => availablePackages.value.find(item => item.id === props.selectedPackageId) ?? null)
const acceptedPaymentMethods = computed(() => props.service.acceptedPaymentMethods ?? [])
const completedOrderLabel = computed(() => props.service.sellerReputation ? `${props.service.sellerReputation.completedCount} 单` : '暂无数据')
const showSourceAuthorVerification = computed(() => {
  const status = props.service.sourceAuthorVerification?.status
  return status === 'verified' || status === 'mismatch'
})

const amountError = computed(() => {
  const decimalPattern = /^\d+(\.\d{1,2})?$/
  if (fixedPackageMode.value) {
    if (!selectedPackage.value) return '请选择有库存的短期流量包。'
    if (props.amount !== selectedPackage.value.priceCny) return '订单金额必须与套餐固定价格一致。'
    return ''
  }
  if (!Number.isFinite(props.amount) || props.amount <= 0) return '请输入有效金额。'
  if (!decimalPattern.test(String(props.amount))) return '自定义金额最多保留两位小数。'
  if (tailOrder.value && compareDecimal(String(props.amount), String(props.service.maxBuy)) !== 0) return `尾单金额固定为 ¥${props.service.maxBuy}，需一次买完。`
  if (!tailOrder.value && props.amount < props.service.minimumPurchaseCny) return `最低订单金额为 ¥${props.service.minimumPurchaseCny}。`
  if (props.amount > props.service.maxBuy) return `单笔最高订单金额为 ¥${props.service.maxBuy}。`
  if (compareDecimal(estimatedCredit.value, apiServiceAvailableUsdAllowance(props.service)) > 0) return '超过商户当前可售美元额度。'
  return ''
})

const canSubmit = computed(() => !amountError.value && props.service.publiclyOrderable && !props.submitting)

function openConfirm() {
  if (!canSubmit.value) return
  confirmOpen.value = true
}

function confirm() {
  emit('confirm')
}

async function shareService() {
  await navigator.clipboard.writeText(window.location.href)
  toast.success('服务链接已复制。')
}
</script>

<template>
  <Card class="api-service-purchase-panel min-w-0 gap-0 overflow-hidden py-0 shadow-sm lg:sticky lg:top-16">
    <div class="p-5">
      <div class="flex items-start justify-between gap-3">
        <component :is="merchantUrl ? RouterLink : 'div'" :to="merchantUrl || undefined" class="flex min-w-0 items-center gap-3">
          <ApiMerchantAvatar :service="service" class="grid h-11 w-11 shrink-0 place-items-center rounded-full bg-primary text-sm font-semibold text-primary-foreground" />
          <span class="min-w-0">
            <span class="block truncate font-semibold">{{ getApiMerchantDisplayName(service) }}</span>
            <span class="mt-1 flex flex-wrap items-center gap-1.5">
              <ApiMerchantBadges :service="service" />
              <Badge v-for="identity in service.communityIdentities" :key="identity.code" variant="outline">{{ identity.name }}</Badge>
              <template v-if="showSourceAuthorVerification">
                <SourceAuthorVerificationBadge :verification="service.sourceAuthorVerification" />
              </template>
            </span>
          </span>
        </component>
      </div>
      <div class="mt-4 border-t border-border pt-4">
        <ReputationInlineSummary :summary="service.sellerReputation" :compact="service.sellerReputation?.state === 'active'" />
      </div>
      <dl class="mt-4 grid grid-cols-2 gap-3 border-t border-border pt-4 text-xs">
        <div><dt class="text-muted-foreground">可验证完成</dt><dd class="mt-1 font-semibold">{{ completedOrderLabel }}</dd></div>
        <div><dt class="text-muted-foreground">商户声明最大并发</dt><dd class="mt-1 font-semibold">{{ service.declaredMaxConcurrency ?? '未声明' }}</dd></div>
      </dl>
    </div>

    <div class="space-y-4 border-t border-border p-5">
      <div v-if="fixedPackageMode" class="space-y-2">
        <div class="text-sm font-semibold">选择短期流量包</div>
        <Select :model-value="selectedPackageId" @update:model-value="value => emit('update:selectedPackageId', String(value))">
          <SelectTrigger class="w-full"><SelectValue placeholder="请选择套餐" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="item in availablePackages" :key="item.id" :value="item.id">{{ item.name }} · {{ item.durationDays }} 天 · ¥{{ item.priceCny }}</SelectItem>
          </SelectContent>
        </Select>
        <p v-if="amountError" class="text-xs text-destructive">{{ amountError }}</p>
      </div>
      <div v-else class="space-y-2">
        <div class="text-sm font-semibold">支付金额</div>
        <PurchaseAmountSelector :service="service" :model-value="amount" @update:model-value="value => emit('update:amount', value)" />
        <p v-if="amountError" class="text-xs text-destructive">{{ amountError }}</p>
      </div>

      <div class="rounded-lg border border-primary/15 bg-primary/5 p-4">
        <div class="text-xs text-muted-foreground">{{ fixedPackageMode ? '套餐价格' : '预计获得' }}</div>
        <div class="mt-1 text-2xl font-semibold text-primary">{{ fixedPackageMode ? `¥${selectedPackage?.priceCny ?? '—'}` : formatCredit(estimatedCredit) }}</div>
        <div v-if="selectedPackage" class="mt-2 text-xs leading-5 text-muted-foreground">面板额度 {{ selectedPackage.panelAllowance }} · {{ selectedPackage.durationDays }} 天 · 剩余 {{ selectedPackage.stockAvailable }} 份</div>
      </div>

      <div v-if="selectedPackage" class="flex flex-wrap gap-1.5">
        <Badge v-for="model in selectedPackage.models" :key="model.serviceModelId" variant="model">{{ model.modelName }} · {{ model.merchantMultiplier }}x</Badge>
      </div>

      <dl class="grid grid-cols-2 gap-3 text-xs">
        <div><dt class="text-muted-foreground">订单金额</dt><dd class="mt-1 font-medium">{{ selectedPackage ? `¥${selectedPackage.priceCny}` : tailOrder ? `尾单 ¥${service.maxBuy}` : `¥${service.minimumPurchaseCny}–¥${service.maxBuy}` }}</dd></div>
        <div><dt class="text-muted-foreground">付款窗口</dt><dd class="mt-1 font-medium">{{ service.expectedResponseMinutes }} 分钟</dd></div>
      </dl>

      <div v-if="acceptedPaymentMethods.length" class="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-muted/30 px-3 py-2">
        <span class="text-xs text-muted-foreground">支持付款</span>
        <span class="flex flex-wrap items-center justify-end gap-2">
          <span v-for="method in acceptedPaymentMethods" :key="method" class="inline-flex items-center gap-1.5 text-xs font-medium">
            <ApiPaymentMethodIcon :method="method" size="sm" />
            {{ apiPaymentMethodLabels[method] }}
          </span>
        </span>
      </div>

      <Button class="w-full" :disabled="!canSubmit" @click="openConfirm">
        {{ submitting ? '创建中…' : '创建订单并查看付款方式' }}
      </Button>
      <p class="text-xs leading-5 text-muted-foreground">{{ selectedPackage ? '有效期从商户提交交付时开始计算。' : '订单创建后展示下单时锁定的商户收款方式；平台记录状态但不代收、不托管资金。' }}</p>
      <div class="grid grid-cols-3 gap-2">
        <Button variant="outline" size="sm" @click="emit('toggleFavorite')"><Heart class="h-3.5 w-3.5" :class="favorited ? 'fill-current' : ''" />{{ favorited ? '已收藏' : '收藏' }}</Button>
        <Button variant="outline" size="sm" @click="shareService"><Share2 class="h-3.5 w-3.5" />分享</Button>
        <RouterLink :to="{ path: '/my/feedback', query: { target: `api-service:${service.id}` } }"><Button class="w-full" variant="outline" size="sm"><Flag class="h-3.5 w-3.5" />举报</Button></RouterLink>
      </div>
    </div>
  </Card>

  <PurchaseConfirmDialog
    :open="confirmOpen"
    :service="service"
    :amount="amount"
    :selected-package="selectedPackage"
    :submitting="submitting"
    @close="confirmOpen = false"
    @confirm="confirm"
  />
</template>
