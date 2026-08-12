<script setup lang="ts">
import { CreditCard, Mail, MessageCircle } from 'lucide-vue-next'
import ApiPaymentMethodIcon from '@/components/api-payment/ApiPaymentMethodIcon.vue'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  apiPaymentMethodLabels,
  type ApiPaymentOption,
} from '@/lib/apiPaymentSettings'

defineProps<{
  open: boolean
  displayName: string
  avatarText: string
  avatarUrl?: string | null
  savedWechat: string
  savedEmail: string
  savedPaymentOptions: ApiPaymentOption[]
  showPayment?: boolean
  hasUnsavedChanges: boolean
}>()

const emit = defineEmits<{
  'update:open': [open: boolean]
}>()
</script>

<template>
  <Dialog :open="open" @update:open="value => emit('update:open', value)">
    <DialogContent class="buyer-preview-dialog sm:max-w-xl">
      <DialogHeader>
        <DialogTitle>买家看到的信息</DialogTitle>
        <DialogDescription>
          以下为当前已保存资料；不会出现在公开主页。
        </DialogDescription>
      </DialogHeader>

      <p v-if="hasUnsavedChanges" class="rounded-md border border-warning/25 bg-warning/10 px-3 py-2 text-xs leading-5 text-warning">
        未保存更改不在此预览中
      </p>

      <div class="buyer-preview-profile">
        <span class="grid h-11 w-11 shrink-0 place-items-center overflow-hidden rounded-full bg-primary text-sm font-semibold text-primary-foreground">
          <img v-if="avatarUrl" :src="avatarUrl" alt="" class="h-full w-full object-cover" />
          <span v-else>{{ avatarText }}</span>
        </span>
        <div class="min-w-0">
          <strong class="block truncate text-sm">{{ displayName }}</strong>
          <span class="mt-1 block text-xs text-muted-foreground">交易所需资料</span>
        </div>
      </div>

      <div class="grid gap-3 sm:grid-cols-2">
        <section class="buyer-preview-section">
          <div class="flex items-center gap-2">
            <MessageCircle class="h-4 w-4 text-primary" />
            <h3 class="text-sm font-semibold">微信</h3>
          </div>
          <p class="mt-2 break-all text-sm text-muted-foreground">{{ savedWechat || '未配置' }}</p>
          <Badge class="mt-3" variant="outline">有效联系窗口</Badge>
        </section>

        <section class="buyer-preview-section">
          <div class="flex items-center gap-2">
            <Mail class="h-4 w-4 text-info" />
            <h3 class="text-sm font-semibold">邮箱</h3>
          </div>
          <p class="mt-2 break-all text-sm text-muted-foreground">{{ savedEmail || '未配置' }}</p>
          <Badge class="mt-3" variant="outline">有效联系窗口</Badge>
        </section>
      </div>

      <section v-if="showPayment" class="buyer-preview-section">
        <div class="flex items-center gap-2">
          <CreditCard class="h-4 w-4 text-success" />
          <h3 class="text-sm font-semibold">API 收款方式</h3>
        </div>
        <p v-if="!savedPaymentOptions.length" class="mt-2 text-sm text-muted-foreground">未配置</p>
        <div v-else class="mt-3 grid gap-3 sm:grid-cols-2">
          <div v-for="option in savedPaymentOptions" :key="option.paymentMethod" class="flex items-center gap-3">
            <img
              v-if="option.paymentQrCodeDataUrl"
              :src="option.paymentQrCodeDataUrl"
              :alt="`${apiPaymentMethodLabels[option.paymentMethod]}收款码`"
              class="h-12 w-12 shrink-0 rounded-md border border-border object-contain"
            />
            <ApiPaymentMethodIcon v-else :method="option.paymentMethod" size="md" />
            <div class="min-w-0">
              <strong class="block text-sm">{{ apiPaymentMethodLabels[option.paymentMethod] }}</strong>
              <span class="mt-1 block text-xs text-muted-foreground">已保存收款码</span>
            </div>
          </div>
        </div>
        <Badge class="mt-3" variant="outline">API 订单</Badge>
      </section>

      <p class="text-xs leading-5 text-muted-foreground">
        {{ showPayment ? '联系方式只在有效联系窗口中展示；收款方式只在买家创建 API 订单后展示。' : '联系方式只在有效联系窗口中展示。' }}
      </p>
    </DialogContent>
  </Dialog>
</template>
