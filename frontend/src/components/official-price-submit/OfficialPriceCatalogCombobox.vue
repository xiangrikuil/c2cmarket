<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, ChevronsUpDown, Plus, Search } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import type { CarpoolProductCatalogItem } from '@/components/carpool-publish/types'

type Mode = 'product' | 'plan'

const props = defineProps<{
  mode: Mode
  modelValue: string
  productText: string
  productPlanId: string
  catalog: CarpoolProductCatalogItem[]
}>()

const emit = defineEmits<{
  'select-product': [value: string]
  'select-plan': [plan: CarpoolProductCatalogItem]
  'select-custom-plan': [value: string]
}>()

const open = ref(false)
const query = ref('')

const categoryOrder: CarpoolProductCatalogItem['categoryCode'][] = ['gpt', 'claude', 'cursor', 'gemini', 'perplexity', 'other']
const categoryLabels: Record<CarpoolProductCatalogItem['categoryCode'], string> = {
  gpt: 'ChatGPT',
  claude: 'Claude',
  cursor: 'Cursor',
  gemini: 'Gemini',
  perplexity: 'Perplexity',
  other: '其他',
}

const selectedCatalogPlan = computed(() => props.catalog.find(item => item.id === props.productPlanId) ?? null)
const normalizedQuery = computed(() => query.value.trim().toLowerCase())
const productOptions = computed(() => {
  const seen = new Set<string>()
  return categoryOrder
    .filter(category => props.catalog.some(item => item.categoryCode === category))
    .map(category => categoryLabels[category])
    .filter(label => {
      if (seen.has(label)) return false
      seen.add(label)
      if (!normalizedQuery.value) return true
      return label.toLowerCase().includes(normalizedQuery.value)
    })
})
const planOptions = computed(() => {
  const productText = props.productText.trim().toLowerCase()
  const selectedCategory = categoryOrder.find(category => categoryLabels[category].toLowerCase() === productText)
  const filtered = props.catalog.filter(item => {
    const productMatches = selectedCategory ? item.categoryCode === selectedCategory : true
    if (!productMatches) return false
    if (!normalizedQuery.value) return true
    return [item.displayName, item.slug, item.providerCode, item.categoryCode]
      .some(value => value.toLowerCase().includes(normalizedQuery.value))
  })
  return filtered.sort((a, b) => a.sortOrder - b.sortOrder)
})
const selectedLabel = computed(() => {
  if (props.mode === 'plan' && selectedCatalogPlan.value) return selectedCatalogPlan.value.displayName
  return props.modelValue.trim() || (props.mode === 'product' ? '选择或输入产品' : '选择或输入套餐')
})
const canCreateCustom = computed(() => {
  const value = query.value.trim()
  if (!value) return false
  if (props.mode === 'product') return !productOptions.value.some(item => item.toLowerCase() === value.toLowerCase())
  return !planOptions.value.some(item => item.displayName.toLowerCase() === value.toLowerCase())
})

function selectProduct(value: string) {
  emit('select-product', value)
  query.value = ''
  open.value = false
}

function selectPlan(item: CarpoolProductCatalogItem) {
  emit('select-plan', item)
  query.value = ''
  open.value = false
}

function selectCustom(value: string) {
  const normalized = value.trim()
  if (!normalized) return
  if (props.mode === 'product') emit('select-product', normalized)
  else emit('select-custom-plan', normalized)
  query.value = ''
  open.value = false
}
</script>

<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button variant="outline" class="h-10 w-full justify-between bg-background px-3 font-normal">
        <span class="truncate">{{ selectedLabel }}</span>
        <ChevronsUpDown class="h-4 w-4 text-muted-foreground" />
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-[min(460px,calc(100vw-32px))] p-2" align="start">
      <div class="relative">
        <Search class="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input v-model="query" class="pl-8" :placeholder="mode === 'product' ? '搜索或输入产品' : '搜索或输入套餐'" />
      </div>

      <div class="mt-2 max-h-72 overflow-y-auto pr-1">
        <template v-if="mode === 'product'">
          <button
            v-for="item in productOptions"
            :key="item"
            type="button"
            class="flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-accent"
            @click="selectProduct(item)"
          >
            <Check :class="['h-4 w-4 shrink-0', item === modelValue ? 'text-primary opacity-100' : 'opacity-0']" />
            <span class="truncate font-medium">{{ item }}</span>
          </button>
        </template>

        <template v-else>
          <button
            v-for="item in planOptions"
            :key="item.id"
            type="button"
            class="flex w-full items-start gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-accent"
            @click="selectPlan(item)"
          >
            <Check :class="['mt-0.5 h-4 w-4 shrink-0', item.id === productPlanId ? 'text-primary opacity-100' : 'opacity-0']" />
            <span class="min-w-0">
              <span class="block truncate font-medium">{{ item.displayName }}</span>
              <span class="mt-0.5 block truncate text-xs text-muted-foreground">{{ item.slug }}</span>
            </span>
          </button>
        </template>

        <button
          v-if="canCreateCustom"
          type="button"
          class="mt-1 flex w-full items-center gap-2 rounded-md px-2 py-2 text-left text-sm hover:bg-accent"
          @click="selectCustom(query)"
        >
          <Plus class="h-4 w-4 shrink-0 text-primary" />
          <span class="min-w-0">
            <span class="block truncate font-medium">使用“{{ query.trim() }}”</span>
            <span class="mt-0.5 block truncate text-xs text-muted-foreground">作为自定义{{ mode === 'product' ? '产品' : '套餐' }}提交</span>
          </span>
        </button>

        <div
          v-if="((mode === 'product' && !productOptions.length) || (mode === 'plan' && !planOptions.length)) && !canCreateCustom"
          class="px-2 py-6 text-center text-sm text-muted-foreground"
        >
          没有匹配项
        </div>
      </div>
    </PopoverContent>
  </Popover>
</template>
