<script setup lang="ts">
import { computed } from 'vue'
import { CheckCircle2, CircleDashed, LockKeyhole, MinusCircle } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  isPassiveReputationEvidence,
  reputationProgressTarget,
  reputationProgressValue,
} from '@/lib/reputationPresentation'
import type { ReputationProgressItem } from '@/types/reputation'

const props = defineProps<{ items: ReputationProgressItem[] }>()
const activeItems = computed(() => props.items.filter(item => !isPassiveReputationEvidence(item)))
const passiveItems = computed(() => props.items.filter(isPassiveReputationEvidence))

function statusLabel(item: ReputationProgressItem) {
  if (item.status === 'met') return '已满足'
  if (item.status === 'blocked') return '受风险状态影响'
  if (item.status === 'unavailable') return '被动证据'
  return '继续积累'
}
</script>

<template>
  <div class="space-y-5">
    <section>
      <h3 class="text-sm font-semibold">可主动改善</h3>
      <div class="mt-2 divide-y rounded-lg border">
        <div v-for="item in activeItems" :key="item.code" class="flex min-h-16 items-center gap-3 px-3 py-2.5">
          <CheckCircle2 v-if="item.status === 'met'" class="h-4 w-4 shrink-0 text-emerald-600" />
          <LockKeyhole v-else-if="item.status === 'blocked'" class="h-4 w-4 shrink-0 text-destructive" />
          <CircleDashed v-else class="h-4 w-4 shrink-0 text-muted-foreground" />
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium">{{ item.label }}</span>
              <Badge variant="outline">{{ statusLabel(item) }}</Badge>
            </div>
            <p class="mt-1 text-xs text-muted-foreground">
              当前 {{ reputationProgressValue(item) }}
              <template v-if="reputationProgressTarget(item)"> · 规则参考 {{ reputationProgressTarget(item) }}</template>
            </p>
          </div>
          <Button v-if="item.actionUrl && item.actionLabel" as-child size="sm" variant="outline">
            <RouterLink :to="item.actionUrl">{{ item.actionLabel }}</RouterLink>
          </Button>
        </div>
      </div>
    </section>

    <section>
      <h3 class="text-sm font-semibold">被动证据</h3>
      <div class="mt-2 divide-y rounded-lg border">
        <div v-for="item in passiveItems" :key="item.code" class="flex min-h-14 items-center gap-3 px-3 py-2.5">
          <MinusCircle class="h-4 w-4 shrink-0 text-muted-foreground" />
          <div class="min-w-0 flex-1">
            <div class="font-medium">{{ item.label }}</div>
            <p class="mt-1 text-xs text-muted-foreground">{{ reputationProgressValue(item) }} · 由平台内已完成交易自然形成</p>
          </div>
          <Badge variant="secondary">{{ statusLabel(item) }}</Badge>
        </div>
      </div>
    </section>
  </div>
</template>
