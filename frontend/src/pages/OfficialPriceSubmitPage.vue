<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { Card } from '@/components/ui/card'
import BasicInfoSection from '@/components/official-price-submit/BasicInfoSection.vue'
import PriceSourceSection from '@/components/official-price-submit/PriceSourceSection.vue'
import AdditionalInfoSection from '@/components/official-price-submit/AdditionalInfoSection.vue'
import FormActionBar from '@/components/official-price-submit/FormActionBar.vue'
import ListingPreview from '@/components/official-price-submit/ListingPreview.vue'
import SubmissionCompleteness from '@/components/official-price-submit/SubmissionCompleteness.vue'
import SubmissionSteps from '@/components/official-price-submit/SubmissionSteps.vue'
import type { CarpoolProductCatalogItem } from '@/components/carpool-publish/types'
import type {
  CompletenessItem,
  OfficialPriceSubmitErrors,
  OfficialPriceSubmitField,
  OfficialPriceSubmitForm,
  SourceLinkState,
  SubmitterPreview,
} from '@/components/official-price-submit/types'
import { submitOfficialPriceLead } from '@/lib/api'
import { shouldUseRealBackend } from '@/lib/backendClient'
import {
  containsSensitiveContent,
  firstError,
  isBlank,
  isHttpUrl,
  isLinuxDoTopicUrl,
} from '@/lib/formValidation'
import { useCarpoolProductCatalog, useMyProfileQuery } from '@/queries/useMarketQueries'
import { toast } from 'vue-sonner'

const regionOptions = ['菲律宾区', '土耳其区', '香港区', '美国区', '日本区', '新加坡区', '其他']
const channelOptions = ['Web', 'iOS', 'Android', 'Apple Store', 'Google Play', '官网', '其他']
const currencyOptions = ['PHP', 'TRY', 'HKD', 'USD', 'JPY', 'SGD', 'CNY']
const openingMethodOptions = ['Apple Store / 虚拟卡 / 本地卡', 'Apple Store', '虚拟卡', '本地卡', 'Google Play', '礼品卡', '其他']

const form = reactive<OfficialPriceSubmitForm>({
  productPlanId: '',
  product: 'ChatGPT',
  plan: 'Pro',
  region: '菲律宾区',
  channel: 'Web',
  originalPriceCurrency: 'PHP',
  originalPriceAmount: '7,990',
  originalPrice: 'PHP 7,990',
  openingMethod: 'Apple Store / 虚拟卡 / 本地卡',
  sourceUrl: 'https://linux.do/t/topic/234567',
  note: '',
})

const errors = reactive<OfficialPriceSubmitErrors>({})
const submittedId = ref('')
const isSubmitting = ref(false)
const savedAt = ref(currentTimeLabel())
const queryClient = useQueryClient()
const { data: profile } = useMyProfileQuery()
const { data: productCatalog } = useCarpoolProductCatalog()
const catalog = computed(() => productCatalog.value ?? [])

watch(
  () => [form.originalPriceCurrency, form.originalPriceAmount] as const,
  ([currency, amount]) => {
    form.originalPrice = `${currency} ${amount}`.trim()
  },
  { immediate: true },
)

watch(
  form,
  () => {
    savedAt.value = currentTimeLabel()
  },
  { deep: true },
)

const sourceLinkState = computed<SourceLinkState>(() => {
  if (isBlank(form.sourceUrl)) return 'idle'
  return isLinuxDoTopicUrl(form.sourceUrl) || isHttpUrl(form.sourceUrl) ? 'success' : 'error'
})

const sourceHost = computed(() => {
  if (sourceLinkState.value !== 'success') return ''
  try {
    return new URL(form.sourceUrl).hostname
  } catch {
    return ''
  }
})

const previewTitle = computed(() => [form.product, form.plan].filter(value => !isBlank(value)).join(' '))
const formattedPrice = computed(() => {
  const amount = normalizedAmount(form.originalPriceAmount)
  if (!form.originalPriceCurrency || !amount) return ''
  return `${form.originalPriceCurrency} ${amount}`
})
const methodTags = computed(() => form.openingMethod.split('/').map(item => item.trim()).filter(Boolean).slice(0, 3))
const submitterPreview = computed<SubmitterPreview>(() => {
  const displayName = profile.value?.displayName || profile.value?.username || '当前用户'
  return {
    name: displayName,
    trustLevel: profile.value?.linuxDoBinding.trustLevel ?? null,
    verified: Boolean(profile.value?.linuxDoBinding.bound),
    avatarText: (displayName.trim()[0] ?? '用').toUpperCase(),
  }
})

const basicInfoComplete = computed(() => [form.product, form.plan, form.region, form.channel].every(value => !isBlank(value)))
const priceSourceComplete = computed(() => !isBlank(form.originalPriceAmount) && !isBlank(form.openingMethod))
const sourceValid = computed(() => sourceLinkState.value === 'success')
const noteComplete = computed(() => !isBlank(form.note))
const completeness = computed(() => {
  let score = 0
  if (basicInfoComplete.value) score += 40
  if (priceSourceComplete.value) score += 25
  if (sourceValid.value) score += 25
  if (noteComplete.value) score += 10
  return score
})
const completenessItems = computed<CompletenessItem[]>(() => [
  { label: '基础信息完整', status: basicInfoComplete.value ? 'done' : 'pending', hint: basicInfoComplete.value ? '已完成' : '待补充' },
  { label: '来源链接有效', status: sourceValid.value ? 'done' : sourceLinkState.value === 'error' ? 'warning' : 'pending', hint: sourceValid.value ? '已通过' : sourceLinkState.value === 'error' ? '需修正' : '待填写' },
  { label: '补充说明完善', status: noteComplete.value ? 'done' : 'warning', hint: noteComplete.value ? '已填写' : '建议完善' },
])
const canSubmit = computed(() => basicInfoComplete.value && priceSourceComplete.value && sourceValid.value && !isSubmitting.value)

const productLabels: Record<CarpoolProductCatalogItem['categoryCode'], string> = {
  gpt: 'ChatGPT',
  claude: 'Claude',
  cursor: 'Cursor',
  gemini: 'Gemini',
  perplexity: 'Perplexity',
  other: '其他',
}

function currentTimeLabel() {
  return new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date())
}

function normalizedAmount(value: string) {
  return value.trim().replace(/\s+/g, '')
}

function hasValidAmount(value: string) {
  const amount = normalizedAmount(value)
  return /^\d{1,3}(,\d{3})*(\.\d{1,2})?$|^\d+(\.\d{1,2})?$/.test(amount)
}

function productLabel(plan: CarpoolProductCatalogItem) {
  return productLabels[plan.categoryCode]
}

function planLabel(plan: CarpoolProductCatalogItem) {
  const product = productLabel(plan)
  return plan.displayName.replace(new RegExp(`^${escapeRegExp(product)}\\s*`, 'i'), '').trim() || plan.displayName
}

function stripProductPrefix(value: string, product: string) {
  const normalizedProduct = product.trim()
  if (!normalizedProduct) return value.trim()
  return value.replace(new RegExp(`^${escapeRegExp(normalizedProduct)}\\s*`, 'i'), '').trim() || value.trim()
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function normalizedText(value: string) {
  return value.trim().toLowerCase().replace(/\s+/g, ' ')
}

function findPlanByTexts(plans: CarpoolProductCatalogItem[], product: string, plan: string) {
  const productText = normalizedText(product)
  const planText = normalizedText(plan)
  return plans.find(item => {
    if (normalizedText(productLabel(item)) !== productText) return false
    const labels = [planLabel(item), item.displayName, item.slug].map(normalizedText)
    return labels.includes(planText) || labels.some(label => label.endsWith(` ${planText}`))
  }) ?? null
}

function selectProduct(value: string) {
  form.product = value
  const currentPlan = catalog.value.find(item => item.id === form.productPlanId)
  if (!currentPlan || productLabel(currentPlan) !== value) {
    form.productPlanId = ''
  }
}

function selectCatalogPlan(plan: CarpoolProductCatalogItem) {
  form.productPlanId = plan.id
  form.product = productLabel(plan)
  form.plan = planLabel(plan)
}

function selectCustomPlan(value: string) {
  form.productPlanId = ''
  form.plan = stripProductPrefix(value, form.product)
}

watch(
  catalog,
  plans => {
    if (form.productPlanId || !plans.length) return
    const match = findPlanByTexts(plans, form.product, form.plan)
    if (match) selectCatalogPlan(match)
  },
  { immediate: true },
)

function setErrors(next: OfficialPriceSubmitErrors) {
  for (const key of Object.keys(errors) as OfficialPriceSubmitField[]) delete errors[key]
  Object.assign(errors, next)
}

function validate() {
  const next: OfficialPriceSubmitErrors = {}
  if (isBlank(form.product)) next.product = '请选择产品。'
  if (isBlank(form.plan)) next.plan = '请选择套餐。'
  if (isBlank(form.region)) next.region = '请选择国家或地区。'
  if (isBlank(form.channel)) next.channel = '请选择渠道。'
  if (isBlank(form.originalPriceAmount)) next.originalPrice = '请填写原币价格。'
  else if (!hasValidAmount(form.originalPriceAmount)) next.originalPrice = '金额格式不正确。'
  if (isBlank(form.openingMethod)) next.openingMethod = '请选择开通方式。'
  if (isBlank(form.sourceUrl)) next.sourceUrl = '请填写 linux.do 低价帖或来源链接。'
  else if (sourceLinkState.value !== 'success') next.sourceUrl = '来源链接格式不合法。'
  if (containsSensitiveContent(Object.values(form))) next.note = '请移除密码、API Key、token、Sub2API 密钥或完整付款码内容。'
  setErrors(next)
  return Object.keys(next).length === 0
}

async function saveDraft() {
  savedAt.value = currentTimeLabel()
  toast('低价线索草稿已保存在本地 mock 状态。')
}

async function submit() {
  if (!validate()) {
    toast.warning(firstError(errors) ?? '请先修正表单。')
    return
  }
  isSubmitting.value = true
  try {
    const result = await submitOfficialPriceLead({
      productPlanId: form.productPlanId,
      product: form.product,
      plan: form.plan,
      region: form.region,
      channel: form.channel,
      originalPrice: form.originalPrice,
      originalPriceCurrency: form.originalPriceCurrency,
      originalPriceAmount: form.originalPriceAmount,
      openingMethod: form.openingMethod,
      sourceUrl: form.sourceUrl,
      note: form.note,
    })
    submittedId.value = String(result.id)
    await queryClient.invalidateQueries({ queryKey: ['official-prices'] })
    await queryClient.invalidateQueries({ queryKey: ['home-market'] })
    await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
    await queryClient.invalidateQueries({ queryKey: ['notifications'] })
    toast.success(shouldUseRealBackend() ? '低价线索已提交到真实后端审核队列。' : '低价线索已进入待验证队列，当前为前端本地反馈。')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '提交失败。')
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="space-y-5">
    <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
      <div>
        <h1 class="text-3xl font-semibold tracking-tight">提交低价线索</h1>
        <p class="mt-2 text-muted-foreground">分享可验证的低价信息，审核后将成为平台行情参考。</p>
      </div>
      <SubmissionSteps />
    </div>

    <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_330px] xl:items-start">
      <Card class="overflow-hidden p-0 shadow-sm">
        <div class="space-y-5 p-5">
          <BasicInfoSection
            :form="form"
            :errors="errors"
            :catalog="catalog"
            :region-options="regionOptions"
            :channel-options="channelOptions"
            @select-product="selectProduct"
            @select-plan="selectCatalogPlan"
            @select-custom-plan="selectCustomPlan"
          />
          <PriceSourceSection
            :form="form"
            :errors="errors"
            :currency-options="currencyOptions"
            :opening-method-options="openingMethodOptions"
            :source-link-state="sourceLinkState"
            :source-host="sourceHost"
          />
          <AdditionalInfoSection :form="form" :errors="errors" />
          <div v-if="submittedId" class="rounded-md border border-border bg-accent p-3 text-sm">
            {{ shouldUseRealBackend() ? '已提交到真实后端审核队列' : '已创建本地 mock 线索' }}：{{ submittedId }}
          </div>
        </div>
        <FormActionBar :saved-at="savedAt" :can-submit="canSubmit" :submitting="isSubmitting" @save-draft="saveDraft" @submit="submit" />
      </Card>

      <aside class="space-y-4 xl:sticky xl:top-16">
        <ListingPreview
          :form="form"
          :title="previewTitle"
          :formatted-price="formattedPrice"
          :method-tags="methodTags"
          :source-link-state="sourceLinkState"
          :source-host="sourceHost"
          :submitter="submitterPreview"
        />
        <SubmissionCompleteness :percent="completeness" :items="completenessItems" />
      </aside>
    </div>
  </div>
</template>
