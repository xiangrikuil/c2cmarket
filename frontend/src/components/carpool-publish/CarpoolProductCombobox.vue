<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, ChevronsUpDown, Search } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { CarpoolProductCatalogItem } from './types'
import { productDisplayName, providerLabels } from './utils'

const props = defineProps<{
  modelValue: string
  customProductName: string | null
  catalog: CarpoolProductCatalogItem[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'update:customProductName': [value: string | null]
}>()

const open = ref(false)
const query = ref('')

const catalogById = computed(() => new Map(props.catalog.map(item => [item.id, item])))
const selectedLabel = computed(() => productDisplayName({
  linuxDoTopicUrl: '',
  parsedTopicId: null,
  productId: props.modelValue,
  customProductName: props.customProductName,
  regionCode: '',
  monthlyPriceCny: null,
  serviceMultiplier: 1,
  monthlyQuotaAmount: null,
  totalSeats: 1,
  occupiedSeats: 0,
  openingChannelCode: '',
  paymentMethodCodes: [],
  accessArrangementMode: 'provider_member_invitation',
  accessArrangementNote: '',
  riskAcknowledged: false,
  policyVersion: null,
  riskNoticeCode: null,
  warranty: { mode: 'no_warranty', fixedWarrantyDays: null, compensationMethod: null, exclusions: null },
  rulesNote: '',
}, catalogById.value))

const groupedOptions = computed(() => {
  const normalized = query.value.trim().toLowerCase()
  const filtered = props.catalog.filter(item => {
    if (item.publishPolicy !== 'allowed') return false
    if (!normalized) return true
    return item.displayName.toLowerCase().includes(normalized) || item.slug.toLowerCase().includes(normalized)
  })

  return (['openai', 'anthropic', 'other'] as const).map(provider => ({
    provider,
    items: filtered.filter(item => item.providerCode === provider),
  })).filter(group => group.items.length)
})

function selectProduct(item: CarpoolProductCatalogItem) {
  emit('update:modelValue', item.id)
  if (!item.allowCustomVariant) emit('update:customProductName', null)
  open.value = false
}
</script>

<template>
  <div class="space-y-2">
    <Popover v-model:open="open">
      <PopoverTrigger as-child>
        <Button variant="outline" class="h-9 w-full justify-between bg-background px-3 font-normal">
          <span class="truncate">{{ modelValue ? selectedLabel : '选择产品' }}</span>
          <ChevronsUpDown class="h-4 w-4 text-muted-foreground" />
        </Button>
      </PopoverTrigger>
      <PopoverContent class="w-[min(420px,calc(100vw-32px))] p-2" align="start">
        <div class="relative">
          <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
          <Input v-model="query" class="pl-8" placeholder="搜索产品目录" />
        </div>
        <div class="mt-2 max-h-72 overflow-y-auto pr-1">
          <div v-for="group in groupedOptions" :key="group.provider" class="py-1">
            <div class="px-2 py-1 text-[11px] font-medium text-muted-foreground">{{ providerLabels[group.provider] }}</div>
            <button
              v-for="item in group.items"
              :key="item.id"
              type="button"
              class="flex w-full items-start gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-accent"
              @click="selectProduct(item)"
            >
              <Check :class="['mt-0.5 h-4 w-4 shrink-0', item.id === modelValue ? 'text-primary opacity-100' : 'opacity-0']" />
              <span class="min-w-0">
                <span class="block truncate font-medium">{{ item.displayName }}</span>
              </span>
            </button>
          </div>
          <div v-if="!groupedOptions.length" class="px-2 py-6 text-center text-sm text-muted-foreground">没有匹配的产品目录</div>
        </div>
      </PopoverContent>
    </Popover>

    <Input
      v-if="catalogById.get(modelValue)?.allowCustomVariant"
      :model-value="customProductName ?? ''"
      placeholder="自定义产品名称"
      @update:model-value="value => emit('update:customProductName', String(value))"
    />
  </div>
</template>
