<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Plus } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import FilterBar from '@/components/market/FilterBar.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import PageTitle from '@/components/market/PageTitle.vue'
import TablePagination from '@/components/market/TablePagination.vue'
import { usePagination } from '@/composables/usePagination'
import { containsSensitiveContent, firstError, isBlank, isLinuxDoTopicUrl, isPositiveNumber, type FieldErrors } from '@/lib/formValidation'
import { canPublishProductPlan, getProductPlanBySlug, getProductPlanForName, productMatchesPlan, productPlanOptions } from '@/lib/productCategories'
import { useDemands, useSubmitDemandMutation } from '@/queries/useMarketQueries'
import { toast } from 'vue-sonner'

type Field = 'sourceUrl' | 'title' | 'maxPrice' | 'region' | 'note' | 'sensitive'

const customProductValue = 'custom'
const productOptions = productPlanOptions
  .filter(canPublishProductPlan)
  .map(item => ({ value: item.slug, label: item.label, note: item.note }))
const filters = [
  { label: '产品', items: ['全部', ...productOptions.map(item => item.label)], active: '全部' },
  { label: '状态', items: ['全部', '匹配中', '待审核', '已匹配', '已关闭'], active: '全部' },
]

const selected = ref(Object.fromEntries(filters.map(group => [group.label, group.active ?? group.items[0]])))
const { data } = useDemands()
const submitMutation = useSubmitDemandMutation()
const submittedId = ref('')
const errors = reactive<FieldErrors<Field>>({})
const form = reactive({
  sourceUrl: 'https://linux.do/t/topic/234567',
  productSlug: 'chatgpt-business',
  customProduct: '',
  maxPrice: '190',
  region: '美国区',
  ownerPreference: 'personal' as 'personal' | 'only-personal' | 'any',
  note: '希望通过官方 workspace 成员席位加入，优先个人车主，不接受共享主账号或密码。',
})

const selectedProductLabel = computed(() => {
  if (form.productSlug === customProductValue) return form.customProduct.trim()
  return getProductPlanBySlug(form.productSlug)?.label ?? ''
})

const rows = computed(() => (data.value ?? []).filter(row => {
  const productFilter = selected.value['产品']
  return (productFilter === '全部' || productMatchesPlan(row.title.replace(/^求\s*/, ''), getProductPlanForName(productFilter)?.slug ?? productFilter) || row.title.includes(productFilter))
    && (selected.value['状态'] === '全部' || row.status === selected.value['状态'])
}))

const pagination = usePagination(rows)
const demandRows = computed(() => data.value ?? [])
const productDemandRanking = computed(() => {
  const counts = new Map<string, number>()
  for (const row of demandRows.value) {
    const normalizedTitle = row.title.replace(/^求\s*/, '')
    const plan = getProductPlanForName(normalizedTitle)
    const label = plan?.label ?? normalizedTitle.split(/[（(·/]/)[0].trim()
    counts.set(label, (counts.get(label) ?? 0) + 1)
  }
  return [...counts.entries()]
    .map(([label, count]) => ({ label, count }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label))
    .slice(0, 3)
})
const totalDemandCount = computed(() => productDemandRanking.value.reduce((sum, item) => sum + item.count, 0))
const hottestDemandCount = computed(() => productDemandRanking.value[0]?.count ?? 0)
const hotProductCount = computed(() => productDemandRanking.value.length)

const ownerPreferenceLabel = computed(() => {
  if (form.ownerPreference === 'only-personal') return '只看个人车主'
  if (form.ownerPreference === 'any') return '不限'
  return '个人车主优先'
})

function setErrors(next: FieldErrors<Field>) {
  for (const key of Object.keys(errors) as Field[]) delete errors[key]
  Object.assign(errors, next)
}

function validate() {
  const next: FieldErrors<Field> = {}
  if (isBlank(form.sourceUrl)) next.sourceUrl = '请填写 linux.do 求车原帖。'
  else if (!isLinuxDoTopicUrl(form.sourceUrl)) next.sourceUrl = '原帖链接必须是 https://linux.do/t/*。'
  if (isBlank(selectedProductLabel.value)) next.title = '请选择想要的产品，或填写自定义产品。'
  if (!isPositiveNumber(form.maxPrice)) next.maxPrice = '请填写大于 0 的最高月费。'
  if (isBlank(form.region)) next.region = '请填写开通区或地区偏好。'
  if (containsSensitiveContent(Object.values(form).map(String))) next.sensitive = '请移除密码、API Key、token 或付款码等敏感内容。'
  setErrors(next)
  return Object.keys(next).length === 0
}

function submitDemand() {
  if (!validate()) {
    toast.warning(firstError(errors) ?? '请先修正表单。')
    return
  }
  submitMutation.mutate({
    sourceUrl: form.sourceUrl,
    title: selectedProductLabel.value,
    maxPrice: Number(form.maxPrice),
    region: form.region,
    ownerPreference: form.ownerPreference,
    note: form.note,
  }, {
    onSuccess(data) {
      submittedId.value = data.id
      toast.success('求车需求已进入需求大厅和管理台审核视图。')
    },
    onError(error) {
      toast.error(error instanceof Error ? error.message : '提交失败')
    },
  })
}
</script>
<template>
  <div>
    <PageTitle title="找车源 / 需求大厅" description="求车也需要绑定 linux.do 求车原帖。车主可按需求池开车，平台按条件匹配。" />
    <div class="mb-6 grid gap-6 lg:grid-cols-[1fr_0.8fr]">
      <Card class="p-6">
        <h2 class="text-lg font-semibold">发布求车需求</h2>
        <div class="mt-5 grid gap-4 md:grid-cols-2">
          <label class="space-y-2 md:col-span-2">
            <span class="text-sm font-medium">linux.do 求车原帖</span>
            <Input v-model="form.sourceUrl" placeholder="https://linux.do/t/..." />
            <p v-if="errors.sourceUrl" class="text-xs text-destructive">{{ errors.sourceUrl }}</p>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">想要产品</span>
            <Select v-model="form.productSlug">
              <SelectTrigger class="w-full">
                <SelectValue placeholder="选择产品" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="item in productOptions" :key="item.value" :value="item.value">
                  {{ item.label }}
                </SelectItem>
                <SelectItem :value="customProductValue">其他 / 自定义</SelectItem>
              </SelectContent>
            </Select>
            <Input
              v-if="form.productSlug === customProductValue"
              v-model="form.customProduct"
              placeholder="输入产品名称，例如 Notion AI Plus"
            />
            <p v-else class="text-xs text-muted-foreground">{{ getProductPlanBySlug(form.productSlug)?.note }}</p>
            <p v-if="errors.title" class="text-xs text-destructive">{{ errors.title }}</p>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">最高月费（元）</span>
            <Input v-model="form.maxPrice" inputmode="decimal" placeholder="190" />
            <p v-if="errors.maxPrice" class="text-xs text-destructive">{{ errors.maxPrice }}</p>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">开通区</span>
            <Input v-model="form.region" placeholder="美国区 / 不限" />
            <p v-if="errors.region" class="text-xs text-destructive">{{ errors.region }}</p>
          </label>
          <label class="space-y-2">
            <span class="text-sm font-medium">车主偏好</span>
            <Select v-model="form.ownerPreference">
              <SelectTrigger class="w-full"><SelectValue placeholder="选择车主偏好" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="personal">个人车主优先</SelectItem>
                <SelectItem value="only-personal">只看个人车主</SelectItem>
                <SelectItem value="any">不限</SelectItem>
              </SelectContent>
            </Select>
          </label>
          <label class="space-y-2 md:col-span-2">
            <span class="text-sm font-medium">补充说明</span>
            <Textarea v-model="form.note" placeholder="说明可接受地区、预算弹性、联系节奏；不要填写账号密码、API Key 或 token。" />
            <p v-if="errors.note || errors.sensitive" class="text-xs text-destructive">{{ errors.note || errors.sensitive }}</p>
          </label>
        </div>
        <div v-if="submittedId" class="mt-5 rounded-md border border-border bg-accent p-3 text-sm">
          已提交求车需求：
          <RouterLink class="font-medium underline underline-offset-4" :to="`/demands/${submittedId}`">{{ submittedId }}</RouterLink>
        </div>
        <div class="mt-6 flex justify-end">
          <Button :disabled="submitMutation.isPending.value" @click="submitDemand">
            <Plus class="h-4 w-4" />{{ submitMutation.isPending.value ? '提交中' : '提交需求' }}
          </Button>
        </div>
      </Card>
      <Card class="overflow-hidden p-6">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold">需求热度概览</h2>
            <p class="mt-1 text-sm text-muted-foreground">基于当前需求池聚合，帮助车主判断开车方向。</p>
          </div>
          <Badge variant="verified" class="shrink-0">实时数据</Badge>
        </div>

        <div class="mt-5 grid gap-3 sm:grid-cols-3">
          <div class="rounded-xl border border-emerald-200 bg-emerald-50 p-4">
            <div class="flex items-center gap-3">
              <span class="h-8 w-8 rounded-lg bg-emerald-500"></span>
              <div>
                <div class="text-2xl font-semibold text-foreground">{{ totalDemandCount }}</div>
                <div class="text-xs text-muted-foreground">总需求人数</div>
              </div>
            </div>
          </div>
          <div class="rounded-xl border border-sky-200 bg-sky-50 p-4">
            <div class="flex items-center gap-3">
              <span class="h-8 w-8 rounded-lg bg-sky-500"></span>
              <div>
                <div class="text-2xl font-semibold text-foreground">{{ hotProductCount }}</div>
                <div class="text-xs text-muted-foreground">热门产品</div>
              </div>
            </div>
          </div>
          <div class="rounded-xl border border-violet-200 bg-violet-50 p-4">
            <div class="flex items-center gap-3">
              <span class="h-8 w-8 rounded-lg bg-violet-500"></span>
              <div>
                <div class="text-2xl font-semibold text-foreground">{{ hottestDemandCount }}</div>
                <div class="text-xs text-muted-foreground">最高热度</div>
              </div>
            </div>
          </div>
        </div>

        <div class="mt-6">
          <div class="mb-3 flex items-center justify-between text-sm">
            <span class="font-semibold">热门产品排行</span>
            <span class="text-xs text-muted-foreground">需求人数</span>
          </div>
          <div class="space-y-3">
            <div v-for="(item, index) in productDemandRanking" :key="item.label" class="rounded-xl bg-muted/35 p-3">
              <div class="flex items-center gap-3">
                <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-xs font-semibold text-primary">
                  {{ String(index + 1).padStart(2, '0') }}
                </span>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center justify-between gap-3">
                    <span class="truncate text-sm font-semibold">{{ item.label }}</span>
                    <span class="shrink-0 text-sm font-semibold">{{ item.count }} 人</span>
                  </div>
                  <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-background">
                    <div class="h-full rounded-full bg-primary" :style="{ width: `${Math.max(12, Math.round((item.count / Math.max(hottestDemandCount, 1)) * 100))}%` }"></div>
                  </div>
                </div>
              </div>
            </div>
            <div v-if="!productDemandRanking.length" class="rounded-xl border border-dashed border-border p-4 text-sm text-muted-foreground">
              暂无需求热度数据。
            </div>
          </div>
        </div>

        <div class="mt-5 rounded-xl border border-primary/15 bg-primary/5 p-4">
          <div class="font-semibold">让需求更快匹配</div>
          <p class="mt-1 text-sm text-muted-foreground">预算、开通区和加入方式越明确，车主越容易判断是否开车。</p>
          <div class="mt-3 flex flex-wrap gap-2 text-xs">
            <Badge variant="trust">预算明确</Badge>
            <Badge variant="trust">官方席位</Badge>
            <Badge variant="trust">个人车主优先</Badge>
          </div>
        </div>
      </Card>
    </div>
    <FilterBar v-model="selected" :groups="filters" :result-count="rows.length" />
    <div v-if="rows.length === 0" class="rounded-xl border border-border bg-card p-8 text-center text-sm text-muted-foreground">当前筛选条件下暂无求车需求。</div>
    <SoftTable v-else :columns="['需求', '预算', '偏好', '来源', '提交人', '状态', '更新时间']">
      <tr v-for="row in pagination.paginatedRows.value" :key="row.id">
        <td>
          <RouterLink :to="`/demands/${row.id}`" class="font-medium hover:underline">{{ row.title }}</RouterLink>
          <div class="text-xs text-muted-foreground">{{ row.require }}</div>
        </td>
        <td>¥{{ row.maxPrice }}</td>
        <td>{{ 'ownerPreference' in row ? (row.ownerPreference === 'only-personal' ? '只看个人车主' : row.ownerPreference === 'any' ? '不限' : '个人车主优先') : ownerPreferenceLabel }}</td>
        <td><Badge variant="secondary">{{ row.linuxdoPost }}</Badge></td>
        <td>{{ row.poster }} · 信任等级{{ row.trustLevel }}</td>
        <td><Badge variant="secondary">{{ row.status }}</Badge></td>
        <td class="text-muted-foreground">{{ 'updatedAt' in row ? row.updatedAt : '刚刚更新' }}</td>
      </tr>
      <template #footer>
        <TablePagination
          v-model:page="pagination.page.value"
          :page-count="pagination.pageCount.value"
          :total="pagination.total.value"
          :start-item="pagination.startItem.value"
          :end-item="pagination.endItem.value"
        />
      </template>
    </SoftTable>
  </div>
</template>
