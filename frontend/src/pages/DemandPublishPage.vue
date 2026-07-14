<script setup lang="ts">
import { computed, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Check, Circle, FilePenLine, SearchCheck, Send } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import PageTitle from '@/components/market/PageTitle.vue'
import { containsSensitiveContent, firstError, isBlank, isLinuxDoTopicUrl, isPositiveNumber, type FieldErrors } from '@/lib/formValidation'
import { useCarpoolProductCatalog, useSubmitDemandMutation } from '@/queries/useMarketQueries'

type Field = 'sourceUrl' | 'title' | 'maxPrice' | 'region' | 'sensitive'

const router = useRouter()
const { data: productCatalog, isLoading, error } = useCarpoolProductCatalog()
const submitMutation = useSubmitDemandMutation()
const errors = reactive<FieldErrors<Field>>({})
const form = reactive({
  sourceUrl: '',
  productId: '',
  maxPrice: '',
  region: '',
  ownerPreference: 'personal' as 'personal' | 'only-personal' | 'any',
  note: '',
})

const productOptions = computed(() => (productCatalog.value ?? [])
  .filter(item => item.active && item.publishPolicy === 'allowed')
  .map(item => ({ value: item.id, label: item.displayName, note: item.policyNote || item.description })))
const selectedProduct = computed(() => productOptions.value.find(item => item.value === form.productId) ?? null)
const publishChecklist = computed(() => [
  { label: '已填写有效的 linux.do 原帖', done: isLinuxDoTopicUrl(form.sourceUrl) },
  { label: '已选择可发布套餐', done: Boolean(selectedProduct.value) },
  { label: '预算与地区已补充', done: isPositiveNumber(form.maxPrice) && !isBlank(form.region) },
])

function setErrors(next: FieldErrors<Field>) {
  for (const key of Object.keys(errors) as Field[]) delete errors[key]
  Object.assign(errors, next)
}

function validate() {
  const next: FieldErrors<Field> = {}
  if (isBlank(form.sourceUrl)) next.sourceUrl = '请填写 linux.do 求车原帖。'
  else if (!isLinuxDoTopicUrl(form.sourceUrl)) next.sourceUrl = '原帖链接必须是 https://linux.do/t/*。'
  if (!selectedProduct.value) next.title = '请选择管理员维护的套餐。'
  if (!isPositiveNumber(form.maxPrice)) next.maxPrice = '请填写大于 0 的最高月费。'
  if (isBlank(form.region)) next.region = '请填写地区偏好。'
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
    title: selectedProduct.value!.label,
    maxPrice: Number(form.maxPrice),
    region: form.region,
    ownerPreference: form.ownerPreference,
    note: form.note,
  }, {
    onSuccess(record) {
      toast.success('求车需求已发布。')
      router.push(`/demands/${record.id}`)
    },
    onError(reason) {
      toast.error(reason instanceof Error ? reason.message : '提交失败')
    },
  })
}
</script>

<template>
  <div class="demand-publish-page space-y-5">
    <div class="demand-publish-heading rounded-xl border px-5 py-4"><PageTitle title="发布求车" description="说明套餐、预算和地区偏好，让车主使用已有车源回应；不要填写账号或付款凭据。">
      <template #action><RouterLink to="/demands"><Button variant="outline"><ArrowLeft class="h-4 w-4" />返回大厅</Button></RouterLink></template>
    </PageTitle></div>
    <section class="demand-publish-steps" aria-label="发布求车步骤">
      <div class="is-active"><span>1</span><div><strong>填写原帖与套餐</strong><small>绑定真实需求来源</small></div></div>
      <i />
      <div :class="{ 'is-active': selectedProduct }"><span>2</span><div><strong>设置预算偏好</strong><small>帮助车主判断匹配度</small></div></div>
      <i />
      <div :class="{ 'is-active': publishChecklist.every(item => item.done) }"><span>3</span><div><strong>预览并发布</strong><small>确认公开展示内容</small></div></div>
    </section>
    <div class="grid gap-5 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-start">
      <Card class="demand-publish-form p-6">
        <div class="grid gap-5 md:grid-cols-2">
          <label class="space-y-2 md:col-span-2"><span class="text-sm font-medium">linux.do 求车原帖</span><Input v-model="form.sourceUrl" placeholder="https://linux.do/t/..." /><p v-if="errors.sourceUrl" class="text-xs text-destructive">{{ errors.sourceUrl }}</p></label>
          <label class="space-y-2"><span class="text-sm font-medium">想要的套餐</span><Select v-model="form.productId"><SelectTrigger class="w-full"><SelectValue placeholder="选择套餐" /></SelectTrigger><SelectContent><SelectItem v-for="item in productOptions" :key="item.value" :value="item.value">{{ item.label }}</SelectItem></SelectContent></Select><p v-if="isLoading" class="text-xs text-muted-foreground">正在读取套餐目录…</p><p v-else-if="error" class="text-xs text-destructive">套餐目录读取失败。</p><p v-else class="text-xs text-muted-foreground">{{ selectedProduct?.note || '选择平台当前允许发布的套餐。' }}</p><p v-if="errors.title" class="text-xs text-destructive">{{ errors.title }}</p></label>
          <label class="space-y-2"><span class="text-sm font-medium">最高月费（元）</span><Input v-model="form.maxPrice" inputmode="decimal" placeholder="例如 190" /><p v-if="errors.maxPrice" class="text-xs text-destructive">{{ errors.maxPrice }}</p></label>
          <label class="space-y-2"><span class="text-sm font-medium">地区偏好</span><Input v-model="form.region" placeholder="美国区 / 不限" /><p v-if="errors.region" class="text-xs text-destructive">{{ errors.region }}</p></label>
          <label class="space-y-2"><span class="text-sm font-medium">车主偏好</span><Select v-model="form.ownerPreference"><SelectTrigger class="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="personal">个人车主优先</SelectItem><SelectItem value="only-personal">只看个人车主</SelectItem><SelectItem value="any">不限</SelectItem></SelectContent></Select></label>
          <label class="space-y-2 md:col-span-2"><span class="text-sm font-medium">补充说明</span><Textarea v-model="form.note" placeholder="说明期望时间、加入方式和预算弹性。" /><p v-if="errors.sensitive" class="text-xs text-destructive">{{ errors.sensitive }}</p></label>
        </div>
        <div class="mt-6 flex justify-end"><Button :disabled="submitMutation.isPending.value" @click="submitDemand"><Send class="h-4 w-4" />{{ submitMutation.isPending.value ? '发布中…' : '发布需求' }}</Button></div>
      </Card>
      <Card class="demand-publish-preview p-5 lg:sticky lg:top-16">
        <div class="flex items-center gap-2 text-xs font-medium text-muted-foreground"><FilePenLine class="h-4 w-4 text-orange-600" />公开页预览</div>
        <h2 class="mt-3 text-lg font-semibold">{{ selectedProduct ? `求 ${selectedProduct.label}` : '选择套餐后显示标题' }}</h2>
        <div class="mt-4 text-3xl font-semibold">{{ form.maxPrice ? `¥${form.maxPrice}` : '预算待填写' }}<span class="text-sm font-normal text-muted-foreground"> / 月以内</span></div>
        <dl class="mt-5 grid gap-3 text-sm"><div class="flex justify-between gap-3"><dt class="text-muted-foreground">地区</dt><dd>{{ form.region || '待填写' }}</dd></div><div class="flex justify-between gap-3"><dt class="text-muted-foreground">回应方式</dt><dd>使用已有车源回应</dd></div></dl>
        <p class="mt-5 border-t border-border pt-4 text-xs leading-5 text-muted-foreground">需求公开页不会展示联系方式、订单号或任何账号凭据。</p>
        <div class="mt-5 rounded-xl border border-orange-100 bg-orange-50/70 p-3">
          <div class="flex items-center gap-2 text-sm font-semibold"><SearchCheck class="h-4 w-4 text-orange-600" />发布前检查</div>
          <ul class="mt-3 space-y-2 text-xs">
            <li v-for="item in publishChecklist" :key="item.label" class="flex items-center gap-2" :class="item.done ? 'text-emerald-700' : 'text-muted-foreground'">
              <Check v-if="item.done" class="h-3.5 w-3.5" />
              <Circle v-else class="h-3.5 w-3.5" />
              {{ item.label }}
            </li>
          </ul>
        </div>
      </Card>
    </div>
  </div>
</template>
