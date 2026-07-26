<script setup lang="ts">
import { apiPaymentMethodIconSrc, type ApiPaymentMethod } from '@/lib/apiPaymentSettings'

withDefaults(defineProps<{
  method: ApiPaymentMethod
  size?: 'sm' | 'md' | 'lg'
}>(), {
  size: 'md',
})

const sizeClasses = {
  sm: 'h-5 w-5 rounded-[5px]',
  md: 'h-8 w-8 rounded-md',
  lg: 'h-10 w-10 rounded-lg',
}
</script>

<template>
  <span
    aria-hidden="true"
    class="inline-grid shrink-0 place-items-center border"
    :class="[
      sizeClasses[size],
      method === 'wechat'
        ? 'border-[#07C160]/25 bg-[#07C160]/10'
        : 'border-[#1677FF]/25 bg-[#1677FF]/10',
    ]"
  >
    <img :src="apiPaymentMethodIconSrc[method]" alt="" class="h-[62%] w-[62%] object-contain" />
  </span>
</template>
