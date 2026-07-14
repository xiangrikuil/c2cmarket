<script setup lang="ts">
import { ref } from 'vue'
import { Card } from '@/components/ui/card'
import type { ApiService } from '@/lib/api'
import ModelPriceTable from './ModelPriceTable.vue'

defineProps<{
  service: ApiService
}>()

const activeTab = ref<'prices' | 'service' | 'guide'>('prices')

const tabs = [
  { value: 'prices', label: '模型价格' },
  { value: 'service', label: '服务说明' },
  { value: 'guide', label: '购买须知' },
] as const
</script>

<template>
  <Card class="gap-0 overflow-hidden py-0 shadow-sm">
    <div class="flex gap-6 border-b border-border px-5" role="tablist" aria-label="API 服务详情">
      <button
        v-for="tab in tabs"
        :key="tab.value"
        type="button"
        role="tab"
        class="relative py-4 text-sm font-medium transition-colors"
        :class="activeTab === tab.value ? 'text-primary' : 'text-muted-foreground hover:text-foreground'"
        :aria-selected="activeTab === tab.value"
        @click="activeTab = tab.value"
      >
        {{ tab.label }}
        <span v-if="activeTab === tab.value" class="absolute inset-x-0 bottom-0 h-0.5 bg-primary" />
      </button>
    </div>

    <div v-if="activeTab === 'prices'" class="py-4">
      <ModelPriceTable :service="service" />
    </div>
    <div v-else-if="activeTab === 'service'" class="p-6 text-sm leading-7">
      <p>{{ service.merchantNote }}</p>
      <dl class="mt-5 grid gap-4 border-t border-border pt-5 sm:grid-cols-2">
        <div><dt class="text-muted-foreground">接入类型</dt><dd class="mt-1 font-semibold">{{ service.delivery }}</dd></div>
        <div><dt class="text-muted-foreground">API 额度有效期</dt><dd class="mt-1 font-semibold">{{ service.expiresAt }}</dd></div>
      </dl>
    </div>
    <div v-else class="p-6 text-sm leading-7 text-muted-foreground">
      <p>创建订单前请核对金额、模型价格和商户倍率；创建时会保存当前服务信息快照。</p>
      <p class="mt-2">提交成功后可在订单详情查看商户收款资料和后续进度。</p>
    </div>
  </Card>
</template>
