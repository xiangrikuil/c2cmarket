<script setup lang="ts">
import { Boxes, CircleDollarSign, Clock3, PackageOpen, TimerReset } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { LIMITED_API_QUOTA_OFFERS_ENABLED } from '@/lib/featureFlags'
import { sellingModeLabels, type SellingMode } from './types'

const emit = defineEmits<{
  select: [value: SellingMode]
}>()

type SellingModeOption = {
  value: SellingMode
  title: string
  description: string
  facts: string[]
  icon: typeof CircleDollarSign
  iconClass: string
}

const allModes: SellingModeOption[] = [
  {
    value: 'free',
    title: sellingModeLabels.free,
    description: '买家自定金额，按你的美元额度售价换算。',
    facts: ['持续接单', '买家自定金额', '额度可累计'],
    icon: CircleDollarSign,
    iconClass: 'bg-emerald-600 text-white',
  },
  {
    value: 'package',
    title: sellingModeLabels.package,
    description: '预设价格、面板额度、有效期和库存，买家按包购买。',
    facts: ['固定规格', '多档套餐', '交付后计时'],
    icon: PackageOpen,
    iconClass: 'bg-blue-600 text-white',
  },
  {
    value: 'limited',
    title: sellingModeLabels.limited,
    description: '设置绝对失效时间，并按全天或指定场次放量。',
    facts: ['绝对失效', '定时放量', '最长 10 分钟交付'],
    icon: TimerReset,
    iconClass: 'bg-orange-500 text-white',
  },
]
const modes = allModes.filter(mode => mode.value !== 'limited' || LIMITED_API_QUOTA_OFFERS_ENABLED)
</script>

<template>
  <section aria-labelledby="selling-mode-title" class="mx-auto w-full max-w-5xl">
    <div class="mb-5 text-center">
      <div class="mx-auto grid h-10 w-10 place-items-center rounded-md bg-primary/10 text-primary">
        <Boxes class="h-5 w-5" />
      </div>
      <h2 id="selling-mode-title" class="mt-3 text-lg font-semibold">选择销售模式</h2>
      <p class="mt-1 text-sm text-muted-foreground">{{ modes.length }} 种模式独立发布，选择后再填写对应配置。</p>
    </div>

    <div class="grid gap-3" :class="modes.length === 2 ? 'md:grid-cols-2' : 'md:grid-cols-3'">
      <Card
        v-for="mode in modes"
        :key="mode.value"
        class="gap-0 rounded-lg p-4 shadow-sm transition-colors hover:border-primary/40"
      >
        <div class="flex min-w-0 items-start gap-3">
          <span class="grid h-9 w-9 shrink-0 place-items-center rounded-md" :class="mode.iconClass">
            <component :is="mode.icon" class="h-4 w-4" />
          </span>
          <div class="min-w-0">
            <h3 class="text-sm font-semibold">{{ mode.title }}</h3>
            <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ mode.description }}</p>
          </div>
        </div>

        <div class="mt-4 flex flex-wrap gap-1.5">
          <span
            v-for="fact in mode.facts"
            :key="fact"
            class="rounded-md bg-muted px-2 py-1 text-[11px] text-muted-foreground"
          >
            {{ fact }}
          </span>
        </div>

        <div class="mt-5 md:mt-auto md:pt-5">
          <Button class="w-full" :data-publish-mode="mode.value" @click="emit('select', mode.value)">
            <Clock3 v-if="mode.value === 'limited'" class="h-4 w-4" />
            <PackageOpen v-else-if="mode.value === 'package'" class="h-4 w-4" />
            <CircleDollarSign v-else class="h-4 w-4" />
            选择{{ mode.title }}
          </Button>
        </div>
      </Card>
    </div>
  </section>
</template>
