<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Scale } from 'lucide-vue-next'
import ApiOrderDisputePanel from '@/components/api-order/ApiOrderDisputePanel.vue'
import { Button } from '@/components/ui/button'

const route = useRoute()
const router = useRouter()
const disputeId = computed(() => String(route.params.id ?? ''))
const orderId = computed(() => String(route.query.orderId ?? ''))
const merchantSource = computed(() => route.query.from === 'merchant')
const orderPath = computed(() => orderId.value
  ? `${merchantSource.value ? '/merchant/api-orders' : '/my/api-orders'}/${encodeURIComponent(orderId.value)}`
  : merchantSource.value ? '/merchant/api-orders' : '/my/api-orders')
</script>

<template>
  <main class="mx-auto w-full max-w-5xl px-4 py-6 sm:px-6 lg:px-8">
    <Button variant="ghost" size="sm" class="-ml-3 mb-4" @click="router.push(orderPath)">
      <ArrowLeft class="h-4 w-4" />返回订单
    </Button>
    <header class="flex flex-wrap items-start justify-between gap-4 border-b border-border pb-5">
      <div>
        <h1 class="flex items-center gap-2 text-xl font-semibold"><Scale class="h-5 w-5 text-warning" />订单纠纷处理</h1>
        <p class="mt-1 text-sm text-muted-foreground">案件记录、平台处理和履行反馈</p>
      </div>
    </header>
    <ApiOrderDisputePanel :dispute-id="disputeId" />
  </main>
</template>
