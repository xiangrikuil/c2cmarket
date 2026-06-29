<script setup lang="ts">
import { Badge } from '@/components/ui/badge'

defineProps<{
  badges?: string[]
  trust?: number
  ownerType?: string
  class?: string
}>()
</script>

<template>
  <div :class="['flex flex-wrap gap-1', $props.class]">
    <Badge v-if="ownerType" variant="secondary">{{ ownerType }}</Badge>
    <Badge v-if="trust" variant="secondary">信任等级{{ trust }}</Badge>
    <Badge v-for="badge in badges" :key="badge" variant="secondary">{{ badge }}</Badge>
  </div>
</template>
