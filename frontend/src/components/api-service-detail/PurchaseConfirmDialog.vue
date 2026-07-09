<script setup lang="ts">
import { Button } from '@/components/ui/button'
import type { ApiService } from '@/lib/api'
import { formatCredit, formatCny, formatMultiplier } from './utils'

defineProps<{
  open: boolean
  service: ApiService
  amount: number
  submitting: boolean
}>()

const emit = defineEmits<{
  close: []
  confirm: []
}>()
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 grid place-items-center bg-foreground/30 p-4">
      <div class="w-full max-w-md rounded-xl border border-border bg-card shadow-lg">
        <div class="border-b border-border p-4">
          <h2 class="text-lg font-semibold">核对购买意向规则</h2>
          <p class="mt-1 text-sm text-muted-foreground">提交后将立即展示商户选择的联系方式，同时商户可以查看你在本次意向中选择的联系方式。</p>
        </div>
        <dl class="space-y-3 p-4 text-sm">
          <div class="flex justify-between gap-4">
            <dt class="text-muted-foreground">意向金额</dt>
            <dd class="font-semibold">{{ formatCny(amount) }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-muted-foreground">意向额度上限</dt>
            <dd class="font-semibold">{{ formatCredit(Math.round(amount * service.creditPerCny)) }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-muted-foreground">锁定倍率</dt>
            <dd class="font-semibold">{{ formatMultiplier(service.defaultMultiplier) }}</dd>
          </div>
          <div class="flex justify-between gap-4">
            <dt class="text-muted-foreground">有效期</dt>
            <dd class="font-semibold">{{ service.expiresAt }}</dd>
          </div>
        </dl>
        <div class="mx-4 mb-4 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-xs leading-5 text-destructive">
          双方后续自行站外沟通，平台不处理支付、不托管凭据，也不验证服务可用性。站外只允许买家专属、可撤销的子账号或子 Key；不得在平台填写、粘贴或上传主账号、主 Key、API Key、密码、token、Session、Cookie 或面板登录凭据。
        </div>
        <div class="flex justify-end gap-2 border-t border-border p-4">
          <Button variant="outline" :disabled="submitting" @click="emit('close')">取消</Button>
          <Button :disabled="submitting" @click="emit('confirm')">
            {{ submitting ? '提交中...' : '提交购买意向并查看联系方式' }}
          </Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
