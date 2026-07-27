<script setup lang="ts">
import { computed } from 'vue'
import { Badge, type BadgeVariants } from '@/components/ui/badge'
import type { SourceAuthorResourceSummary } from '@/data/mock'
import {
  sourceAuthorVerificationLabel,
  sourceAuthorVerificationStatus,
} from '@/lib/sourceAuthorVerification'

const props = defineProps<{
  verification?: SourceAuthorResourceSummary
}>()

const label = computed(() => sourceAuthorVerificationLabel(props.verification))
const variant = computed<BadgeVariants['variant']>(() => {
  const status = sourceAuthorVerificationStatus(props.verification)
  if (status === 'verified') return 'verified'
  if (status === 'mismatch') return 'destructive'
  if (status === 'pending') return 'trust'
  return 'secondary'
})
</script>

<template>
  <Badge :variant="variant">{{ label }}</Badge>
</template>
