<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

type WorkspaceSection = 'reputation-rights' | 'support-center'

const props = defineProps<{
  section: WorkspaceSection
  promotionEnabled?: boolean
}>()

const route = useRoute()
const configurations = {
  'reputation-rights': {
    label: '信誉与权益',
    items: [
      { label: '信誉成长', to: '/my/reputation' },
      { label: '推广权益', to: '/my/promotion-benefits' },
    ],
  },
  'support-center': {
    label: '支持中心',
    items: [
      { label: '举报与申诉', to: '/my/reports' },
      { label: '问题反馈', to: '/my/feedback' },
    ],
  },
} as const

const configuration = computed(() => configurations[props.section])
const visibleItems = computed(() => configuration.value.items.filter(item => (
  item.to !== '/my/promotion-benefits' || props.promotionEnabled !== false
)))
const activeValue = computed(() => visibleItems.value.find(item => (
  route.path === item.to || route.path.startsWith(`${item.to}/`)
))?.to ?? visibleItems.value[0].to)
</script>

<template>
  <div class="max-w-full overflow-x-auto" data-workspace-section-tabs>
    <Tabs :model-value="activeValue" class="w-max min-w-full">
      <TabsList class="w-max min-w-full justify-start" :aria-label="configuration.label">
        <TabsTrigger
          v-for="item in visibleItems"
          :key="item.to"
          :value="item.to"
          as-child
          class="flex-none px-4"
        >
          <RouterLink :to="item.to">{{ item.label }}</RouterLink>
        </TabsTrigger>
      </TabsList>
    </Tabs>
  </div>
</template>
