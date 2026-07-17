<script setup lang="ts">
import { computed } from 'vue'
import { Award, Zap } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { getApiMerchantBadges } from '@/lib/apiMerchantBadges'
import type { ApiService } from '@/lib/api'

const props = defineProps<{
  service: Pick<ApiService,
    | 'completed30d'
    | 'publiclyOrderable'
    | 'recommendationResponseMedianMinutes'
    | 'responseMedianMinutes'
    | 'trustLevel'
    | 'unresolvedDisputes'
  >
}>()

const badges = computed(() => getApiMerchantBadges(props.service))
</script>

<template>
  <Badge
    v-for="badge in badges"
    :key="badge.kind"
    variant="outline"
    :title="badge.description"
    class="api-merchant-achievement-badge"
    :class="`api-merchant-achievement-badge--${badge.kind}`"
  >
    <Award v-if="badge.kind === 'quality'" />
    <Zap v-else />
    {{ badge.label }}
  </Badge>
</template>
