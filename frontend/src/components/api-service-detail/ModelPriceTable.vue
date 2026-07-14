<script setup lang="ts">
import { computed } from 'vue'
import { getSupportedModelPriceRows, type ApiService } from '@/lib/api'
import { formatBeijingDateTime, formatMultiplier, formatPricePerMillion } from './utils'

const props = defineProps<{
  service: ApiService
}>()

const supportedRows = computed(() => getSupportedModelPriceRows(props.service))
</script>

<template>
  <div>
    <div class="flex justify-end px-4 pb-3 text-xs text-muted-foreground">
      <div class="shrink-0 text-right text-xs text-muted-foreground">
        <div>价格版本 {{ service.officialPricingVersion }}</div>
        <div class="mt-1">最后更新 {{ formatBeijingDateTime(service.officialPricingUpdatedAt) }}</div>
      </div>
    </div>

    <div v-if="supportedRows.length" class="overflow-x-auto">
      <table class="w-full min-w-[860px] text-sm">
        <thead class="bg-muted/50 text-xs text-muted-foreground">
          <tr class="border-b border-border">
            <th class="px-4 py-3 text-left font-medium">模型</th>
            <th class="px-3 py-3 text-right font-medium">官方输入价格</th>
            <th class="px-3 py-3 text-right font-medium">官方缓存输入价格</th>
            <th class="px-3 py-3 text-right font-medium">官方输出价格</th>
            <th class="px-3 py-3 text-right font-medium">商户倍率</th>
            <th class="px-4 py-3 text-right font-medium">实际价格</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in supportedRows" :key="row.modelId" class="border-b border-border last:border-b-0">
            <td class="px-4 py-4">
              <div class="font-semibold">{{ row.modelName }}</div>
              <div class="mt-1 text-xs text-muted-foreground">{{ row.provider }} · 示例价格</div>
            </td>
            <td class="px-3 py-4 text-right text-muted-foreground">{{ formatPricePerMillion(row.officialInputPricePerMillion, '$') }}</td>
            <td class="px-3 py-4 text-right text-muted-foreground">{{ formatPricePerMillion(row.officialCachedInputPricePerMillion, '$') }}</td>
            <td class="px-3 py-4 text-right text-muted-foreground">{{ formatPricePerMillion(row.officialOutputPricePerMillion, '$') }}</td>
            <td class="px-3 py-4 text-right font-semibold text-primary">{{ formatMultiplier(row.merchantMultiplier) }}</td>
            <td class="px-4 py-4 text-right font-semibold">
              <div>输入 {{ formatPricePerMillion(row.actualInputPricePerMillion, '¥') }}</div>
              <div class="mt-1 text-xs text-muted-foreground">缓存 {{ formatPricePerMillion(row.actualCachedInputPricePerMillion, '¥') }} · 输出 {{ formatPricePerMillion(row.actualOutputPricePerMillion, '¥') }}</div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="px-4 py-8 text-sm text-muted-foreground">
      当前服务尚未配置可展示的支持模型价格，创建订单前请先联系商户补充确认。
    </div>

    <div class="border-t border-border px-4 py-3 text-xs text-muted-foreground">
      价格单位：每 100 万 Tokens。创建订单时保存当前价格与倍率快照。
    </div>
  </div>
</template>
