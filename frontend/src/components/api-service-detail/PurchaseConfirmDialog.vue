<script setup lang="ts">
import { ref, watch } from 'vue'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import type { ApiService, ApiServicePackage } from '@/lib/api'
import { estimateUsdAllowance, formatCredit, formatCny, formatMultiplier } from './utils'

const props = defineProps<{
  open: boolean
  service: ApiService
  amount: number
  selectedPackage: ApiServicePackage | null
  submitting: boolean
}>()

const emit = defineEmits<{
  close: []
  confirm: []
}>()

const acknowledged = ref(false)

watch(() => props.open, open => {
  if (open) acknowledged.value = false
})

function confirm() {
  if (!acknowledged.value) return
  emit('confirm')
}

function updateOpen(open: boolean) {
  if (!open && !props.submitting) emit('close')
}
</script>

<template>
  <Dialog :open="open" @update:open="updateOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>确认创建订单</DialogTitle>
        <DialogDescription>核对金额、额度和有效期后创建订单，并在倒计时内完成站外付款。</DialogDescription>
      </DialogHeader>
      <dl class="space-y-3 text-sm">
        <div class="flex justify-between gap-4">
          <dt class="text-muted-foreground">支付金额</dt>
          <dd class="font-semibold">{{ formatCny(amount) }}</dd>
        </div>
        <div v-if="selectedPackage" class="flex justify-between gap-4">
          <dt class="text-muted-foreground">限时流量包</dt>
          <dd class="text-right font-semibold">{{ selectedPackage.name }} · {{ selectedPackage.durationDays }} 天</dd>
        </div>
        <div v-if="!selectedPackage" class="flex justify-between gap-4">
          <dt class="text-muted-foreground">锁定美元额度</dt>
          <dd class="font-semibold">{{ formatCredit(estimateUsdAllowance(String(amount), service)) }}</dd>
        </div>
        <div v-else class="flex justify-between gap-4">
          <dt class="text-muted-foreground">面板额度</dt>
          <dd class="font-semibold">{{ selectedPackage.panelAllowance }}</dd>
        </div>
        <div v-if="!selectedPackage" class="flex justify-between gap-4">
          <dt class="text-muted-foreground">锁定倍率</dt>
          <dd class="font-semibold">{{ formatMultiplier(service.defaultMultiplier) }}</dd>
        </div>
        <div v-if="!selectedPackage" class="flex justify-between gap-4">
          <dt class="text-muted-foreground">API 额度有效期</dt>
          <dd class="font-semibold">{{ service.expiresAt }}</dd>
        </div>
        <div v-else class="flex justify-between gap-4">
          <dt class="text-muted-foreground">有效期起点</dt>
          <dd class="text-right font-semibold">商户提交交付后开始</dd>
        </div>
      </dl>
      <label class="flex items-start gap-2 rounded-md border border-border bg-muted/40 p-3 text-sm leading-5">
        <Checkbox v-model="acknowledged" class="mt-0.5" />
        <span>我已核对订单金额与{{ selectedPackage ? '套餐模型、倍率和库存' : '额度' }}；创建后将锁定下单信息并启动付款倒计时。付款由我与商户直接完成，平台不代收或托管资金。</span>
      </label>
      <DialogFooter>
        <Button variant="outline" :disabled="submitting" @click="emit('close')">取消</Button>
        <Button :disabled="submitting || !acknowledged" @click="confirm">
          {{ submitting ? '创建中…' : '确认创建订单' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
