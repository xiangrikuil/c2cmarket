<script setup lang="ts">
import { computed } from 'vue'
import { getApiMerchantAvatarText, getApiMerchantDisplayName, type ApiService } from '@/lib/api'

const props = defineProps<{
  service: Pick<ApiService, 'merchant' | 'merchantIdentityMode' | 'merchantDisplayName' | 'merchantAvatarUrl'>
}>()

const alt = computed(() => `${getApiMerchantDisplayName(props.service)}头像`)
</script>

<template>
  <span class="overflow-hidden">
    <img v-if="service.merchantAvatarUrl" :src="service.merchantAvatarUrl" :alt="alt" class="h-full w-full object-cover" />
    <span v-else>{{ getApiMerchantAvatarText(service) }}</span>
  </span>
</template>
