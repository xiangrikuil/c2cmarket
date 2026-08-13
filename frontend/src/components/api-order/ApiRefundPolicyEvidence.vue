<script setup lang="ts">
import { computed, ref } from 'vue'
import { FileText } from 'lucide-vue-next'
import type { ApiServiceCommercialSnapshot } from '@/lib/api'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  apiMerchantRefundPolicyApplicability,
  apiMerchantRefundPolicyExclusions,
  apiMerchantRefundPolicyVersion,
} from '@/lib/apiOrderDispute'

const props = defineProps<{
  snapshot: ApiServiceCommercialSnapshot
}>()

const open = ref(false)
const isKnownPolicy = computed(() => props.snapshot.merchantRefundPolicyVersion === apiMerchantRefundPolicyVersion)

function versionLabel(version: string | null | undefined) {
  if (version === apiMerchantRefundPolicyVersion) return 'API 商户退款规则 v1'
  return version?.trim() || '历史订单未冻结规则版本'
}
</script>

<template>
  <div class="space-y-1.5">
    <div class="flex flex-wrap items-center gap-2">
      <span class="font-medium">{{ versionLabel(snapshot.merchantRefundPolicyVersion) }}</span>
      <Badge variant="secondary">下单时已锁定</Badge>
    </div>
    <Button size="sm" variant="outline" @click="open = true"><FileText class="h-3.5 w-3.5" />查看规则证据</Button>
    <Dialog v-model:open="open">
      <DialogContent class="max-h-[88dvh] overflow-y-auto sm:max-w-[560px]">
        <DialogHeader>
          <DialogTitle>{{ versionLabel(snapshot.merchantRefundPolicyVersion) }}</DialogTitle>
          <DialogDescription>以下内容来自下单时冻结的订单快照，后续服务资料修改不会覆盖。</DialogDescription>
        </DialogHeader>
        <dl class="space-y-4 text-sm">
          <div>
            <dt class="font-medium">商户售后承诺</dt>
            <dd class="mt-1 whitespace-pre-wrap leading-6 text-muted-foreground">{{ snapshot.warranty || '历史订单未冻结售后承诺。' }}</dd>
          </div>
          <template v-if="isKnownPolicy">
            <div>
              <dt class="font-medium">适用范围</dt>
              <dd class="mt-1 leading-6 text-muted-foreground">{{ snapshot.merchantRefundCommitment ? apiMerchantRefundPolicyApplicability : '本订单未选择商户全额退款承诺，具体问题按冻结订单说明由双方协商。' }}</dd>
            </div>
            <div>
              <dt class="font-medium">不适用情形</dt>
              <dd class="mt-1 leading-6 text-muted-foreground">{{ snapshot.merchantRefundCommitment ? apiMerchantRefundPolicyExclusions : '本订单没有额外全额退款承诺，不生成新的退款适用条件。' }}</dd>
            </div>
          </template>
          <div>
            <dt class="font-medium">平台交易边界</dt>
            <dd class="mt-1 whitespace-pre-wrap leading-6 text-muted-foreground">{{ snapshot.refundPolicy || '历史订单未冻结平台交易边界。' }}</dd>
          </div>
        </dl>
        <DialogFooter><Button variant="outline" @click="open = false">关闭</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
