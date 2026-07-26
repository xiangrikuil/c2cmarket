<script setup lang="ts">
import { CheckCircle2, CircleDollarSign, TimerReset } from 'lucide-vue-next'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { SellingMode } from './types'

defineProps<{
  modelValue: SellingMode
}>()

const emit = defineEmits<{
  'update:modelValue': [value: SellingMode]
}>()

function selectMode(value: string | number) {
  if (value === 'free' || value === 'limited') emit('update:modelValue', value)
}
</script>

<template>
  <section aria-labelledby="selling-mode-title" class="space-y-1.5">
    <div class="flex items-center justify-between gap-3">
      <h2 id="selling-mode-title" class="text-sm font-semibold">选择销售方式</h2>
      <span class="text-[11px] text-muted-foreground">两种方式独立发布</span>
    </div>

    <Tabs :model-value="modelValue" @update:model-value="selectMode">
      <TabsList class="grid h-auto w-full grid-cols-1 gap-2 bg-transparent p-0 sm:grid-cols-2">
        <TabsTrigger
          value="free"
          class="group relative h-auto min-h-[82px] w-full items-start justify-start whitespace-normal rounded-md border border-emerald-200 bg-emerald-50/55 p-2.5 text-left shadow-none hover:bg-emerald-50 data-[state=active]:border-emerald-500 data-[state=active]:bg-emerald-50 data-[state=active]:text-emerald-950 data-[state=active]:shadow-[0_0_0_1px_rgb(16_185_129)]"
        >
          <span class="flex w-full items-start gap-2.5">
            <span class="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-emerald-600 text-white">
              <CircleDollarSign class="h-4 w-4" />
            </span>
            <span class="min-w-0 flex-1">
              <span class="flex items-center justify-between gap-3">
                <span class="text-sm font-semibold">自由额度</span>
                <CheckCircle2 v-if="modelValue === 'free'" class="h-4 w-4 shrink-0 text-emerald-600" />
              </span>
              <span class="mt-0.5 block text-[11px] font-normal leading-4 text-emerald-950/75">买家自定金额，按美元额度售价换算。</span>
              <span class="mt-1 flex flex-wrap gap-x-2 text-[10px] font-medium leading-4 text-emerald-800">
                <span>长期持续接单</span>
                <span>按金额购买</span>
                <span>余额可累计</span>
              </span>
            </span>
          </span>
        </TabsTrigger>

        <TabsTrigger
          value="limited"
          class="group relative h-auto min-h-[82px] w-full items-start justify-start whitespace-normal rounded-md border border-orange-200 bg-orange-50/60 p-2.5 text-left shadow-none hover:bg-orange-50 data-[state=active]:border-orange-500 data-[state=active]:bg-orange-50 data-[state=active]:text-orange-950 data-[state=active]:shadow-[0_0_0_1px_rgb(249_115_22)]"
        >
          <span class="flex w-full items-start gap-2.5">
            <span class="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-orange-500 text-white">
              <TimerReset class="h-4 w-4" />
            </span>
            <span class="min-w-0 flex-1">
              <span class="flex items-center justify-between gap-3">
                <span class="text-sm font-semibold">限时额度包</span>
                <CheckCircle2 v-if="modelValue === 'limited'" class="h-4 w-4 shrink-0 text-orange-600" />
              </span>
              <span class="mt-0.5 block text-[11px] font-normal leading-4 text-orange-950/75">固定美元额度、总价和绝对失效时间。</span>
              <span class="mt-1 flex flex-wrap gap-x-2 text-[10px] font-medium leading-4 text-orange-800">
                <span>短期固定规格</span>
                <span>全天或定时放量</span>
                <span>最长 10 分钟交付</span>
              </span>
            </span>
          </span>
        </TabsTrigger>
      </TabsList>
    </Tabs>
  </section>
</template>
