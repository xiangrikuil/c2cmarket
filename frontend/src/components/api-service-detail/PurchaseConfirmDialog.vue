<script setup lang="ts">
import { ref, watch } from 'vue'
import { Button } from '@/components/ui/button'
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
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 grid place-items-center bg-foreground/30 p-4">
      <div class="w-full max-w-md rounded-xl border border-border bg-card shadow-lg">
        <div class="border-b border-border p-4">
          <h2 class="text-lg font-semibold">确认创建订单</h2>
        </div>
        <dl class="space-y-3 p-4 text-sm">
          <div class="flex justify-between gap-4">
            <dt class="text-muted-foreground">支付金额</dt>
            <dd class="font-semibold">{{ formatCny(amount) }}</dd>
          </div>
          <div v-if="selectedPackage" class="flex justify-between gap-4">
            <dt class="text-muted-foreground">限时流量包</dt>
            <dd class="text-right font-semibold">{{ selectedPackage.name }} · {{ selectedPackage.durationDays }} 天</dd>
          </div>
          <div v-if="!selectedPackage" class="flex justify-between gap-4">
            <dt class="text-muted-foreground">冻结美元额度</dt>
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
            <dd class="text-right font-semibold">商家提交交付后开始</dd>
          </div>
        </dl>
        <label class="mx-4 mb-4 flex items-start gap-2 rounded-md border border-border bg-muted/40 p-3 text-sm leading-5">
          <input v-model="acknowledged" type="checkbox" class="mt-0.5 h-4 w-4 shrink-0 accent-primary" />
          <span>我已核对订单金额与{{ selectedPackage ? '套餐模型、倍率和库存' : '额度' }}；创建后将冻结快照并启动付款倒计时，付款仍在线下完成。</span>
        </label>
        <div class="flex justify-end gap-2 border-t border-border p-4">
          <Button variant="outline" :disabled="submitting" @click="emit('close')">取消</Button>
          <Button :disabled="submitting || !acknowledged" @click="confirm">
            {{ submitting ? '创建中...' : '确认创建订单' }}
          </Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
