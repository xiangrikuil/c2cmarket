<script setup lang="ts">
import { computed } from 'vue'
import { Card } from '@/components/ui/card'
import { getSupportedModelPriceRows, type ApiService } from '@/lib/api'
import { formatMultiplier, formatPricePerMillion } from './utils'

const props = defineProps<{
  service: ApiService
}>()

const supportedRows = computed(() => getSupportedModelPriceRows(props.service))
</script>

<template>
  <Card class="gap-0 overflow-hidden py-0 shadow-sm">
    <div class="flex items-start justify-between gap-4 border-b border-border px-4 py-3">
      <div>
        <h2 class="text-base font-semibold">模型价格明细</h2>
        <p class="mt-1 text-xs text-muted-foreground">仅展示该服务声明支持的模型；官方价格由平台价格库维护，实际价格按当前服务倍率折算。</p>
      </div>
      <div class="shrink-0 text-right text-xs text-muted-foreground">
        <div>价格版本 {{ service.officialPricingVersion }}</div>
        <div class="mt-1">最后更新 {{ service.officialPricingUpdatedAt }}</div>
      </div>
    </div>

    <div v-if="supportedRows.length" class="overflow-x-auto">
      <table class="w-full min-w-[980px] text-sm">
        <thead class="bg-muted/50 text-xs text-muted-foreground">
          <tr class="border-b border-border">
            <th class="px-4 py-3 text-left font-medium">模型</th>
            <th class="px-3 py-3 text-right font-medium">官方输入价格</th>
            <th class="px-3 py-3 text-right font-medium">官方缓存输入价格</th>
            <th class="px-3 py-3 text-right font-medium">官方输出价格</th>
            <th class="px-3 py-3 text-right font-medium">商户倍率</th>
            <th class="px-3 py-3 text-right font-medium">实际输入价格</th>
            <th class="px-3 py-3 text-right font-medium">实际缓存价格</th>
            <th class="px-4 py-3 text-right font-medium">实际输出价格</th>
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
            <td class="px-3 py-4 text-right font-semibold">{{ formatPricePerMillion(row.actualInputPricePerMillion, '¥') }}</td>
            <td class="px-3 py-4 text-right font-semibold">{{ formatPricePerMillion(row.actualCachedInputPricePerMillion, '¥') }}</td>
            <td class="px-4 py-4 text-right font-semibold">{{ formatPricePerMillion(row.actualOutputPricePerMillion, '¥') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="px-4 py-8 text-sm text-muted-foreground">
      当前服务尚未配置可展示的支持模型价格，提交意向前请先联系商户补充确认。
    </div>

    <div class="border-t border-border px-4 py-3 text-xs text-muted-foreground">
      价格单位：每 100 万 Tokens。实际扣费以提交意向时保存的价格和倍率快照为参考，最终由双方站外确认。
    </div>
  </Card>
</template>
