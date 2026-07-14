<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getMerchantApiOrders, getMyApiOrders } from '@/lib/api'

const route = useRoute()
const router = useRouter()
const message = ref('正在定位对应的 API 订单…')

onMounted(async () => {
  const intentId = String(route.params.id ?? '')
  const [buyerResult, merchantResult] = await Promise.allSettled([
    getMyApiOrders(),
    getMerchantApiOrders(),
  ])
  const buyerOrder = buyerResult.status === 'fulfilled'
    ? buyerResult.value.find(item => item.apiPurchaseIntentId === intentId)
    : undefined
  if (buyerOrder) {
    await router.replace(`/my/api-orders/${buyerOrder.id}`)
    return
  }
  const merchantOrder = merchantResult.status === 'fulfilled'
    ? merchantResult.value.find(item => item.apiPurchaseIntentId === intentId)
    : undefined
  if (merchantOrder) {
    await router.replace(`/merchant/api-orders/${merchantOrder.id}`)
    return
  }
  message.value = '该历史记录尚未生成可查看的订单，已返回订单列表。'
  await router.replace('/my/api-orders')
})
</script>

<template>
  <div class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">
    {{ message }}
  </div>
</template>
