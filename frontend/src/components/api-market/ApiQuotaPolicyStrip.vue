<script setup lang="ts">
import { computed } from 'vue'
import { apiQuotaUsageLimitLabel } from '@/lib/apiQuotaPolicy'
import type { ApiQuotaUsagePolicy, ApiQuotaUsagePolicyInput } from '@/types/apiQuota'

const props = withDefaults(defineProps<{
  policy: ApiQuotaUsagePolicy | ApiQuotaUsagePolicyInput
  expiryValue: string
  expiryLabel?: string
}>(), {
  expiryLabel: '统一到期',
})

const ariaLabel = computed(() => [
  `5h 限额 ${apiQuotaUsageLimitLabel(props.policy.fiveHour)}`,
  `每日限额 ${apiQuotaUsageLimitLabel(props.policy.daily)}`,
  `${props.expiryLabel} ${props.expiryValue}`,
].join('，'))
</script>

<template>
  <dl class="api-quota-policy-strip" :aria-label="ariaLabel">
    <div title="卖家声明的倍率计费后美元额度；每份买家交付凭据独立适用。">
      <dt>5h 限额</dt>
      <dd>{{ apiQuotaUsageLimitLabel(policy.fiveHour) }}</dd>
    </div>
    <div title="卖家声明的倍率计费后美元额度；每份买家交付凭据独立适用，并按 UTC+8 自然日重置。">
      <dt>每日限额</dt>
      <dd>{{ apiQuotaUsageLimitLabel(policy.daily) }}</dd>
    </div>
    <div :title="expiryValue">
      <dt>{{ expiryLabel }}</dt>
      <dd>{{ expiryValue }}</dd>
    </div>
  </dl>
</template>

<style scoped>
.api-quota-policy-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  min-height: 3.25rem;
  border-block: 1px solid var(--border);
  background: #f8fafc;
}

.api-quota-policy-strip > div {
  min-width: 0;
  padding: 0.4375rem 0.5rem;
  text-align: center;
}

.api-quota-policy-strip > div + div {
  border-left: 1px solid var(--border);
}

.api-quota-policy-strip dt {
  color: var(--muted-foreground);
  font-size: 0.625rem;
  line-height: 0.875rem;
}

.api-quota-policy-strip dd {
  min-height: 1.25rem;
  margin-top: 0.125rem;
  overflow-wrap: anywhere;
  color: var(--foreground);
  font-size: 0.71875rem;
  font-weight: 650;
  line-height: 0.8125rem;
  white-space: normal;
}
</style>
