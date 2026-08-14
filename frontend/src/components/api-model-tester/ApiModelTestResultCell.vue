<script setup lang="ts">
import { LoaderCircle } from 'lucide-vue-next'
import StatusBadge from '@/components/market/StatusBadge.vue'
import { apiModelTesterProtocolPresentation } from '@/lib/apiModelTesterPresentation'
import type { ApiModelTesterProtocolResult, ApiModelTesterRowState } from '@/types/apiModelTester'

const props = defineProps<{
  row?: ApiModelTesterRowState
  result?: ApiModelTesterProtocolResult
}>()
</script>

<template>
  <div class="flex min-h-10 flex-col justify-center gap-1">
    <div v-if="props.row?.state === 'pending'" class="flex items-center gap-2 text-xs text-muted-foreground">
      <LoaderCircle class="h-4 w-4 animate-spin" />调用中
    </div>
    <StatusBadge v-else-if="result" :status="result.errorCode || 'succeeded'" :label="apiModelTesterProtocolPresentation(result).label" :tone="apiModelTesterProtocolPresentation(result).tone" />
    <StatusBadge v-else-if="props.row?.state === 'cancelled'" status="cancelled" label="已取消" tone="neutral" />
    <StatusBadge v-else-if="props.row?.state === 'failed'" status="failed" label="请求失败" tone="risk" />
    <span v-else class="text-xs text-muted-foreground">未测试</span>
    <span v-if="result" class="text-[11px] text-muted-foreground">{{ result.durationMs }} ms<span v-if="result.httpStatusClass"> · HTTP {{ result.httpStatusClass }}xx</span></span>
    <span v-else-if="props.row?.message" class="line-clamp-2 text-[11px] leading-4 text-muted-foreground" :title="props.row.message">{{ props.row.message }}</span>
  </div>
</template>
