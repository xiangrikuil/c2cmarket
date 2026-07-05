<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { Eye, Send, ShieldCheck } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiServicePublishPreview from '@/components/api-service-publish/ApiServicePublishPreview.vue'
import MerchantNoteSection from '@/components/api-service-publish/MerchantNoteSection.vue'
import ModelMultiSelect from '@/components/api-service-publish/ModelMultiSelect.vue'
import PaymentSettingsSection from '@/components/api-service-publish/PaymentSettingsSection.vue'
import PriceInventorySection from '@/components/api-service-publish/PriceInventorySection.vue'
import ProviderCategorySelector from '@/components/api-service-publish/ProviderCategorySelector.vue'
import type { ApiProviderCategory, ApiServicePublishForm } from '@/components/api-service-publish/types'
import { toggleSelectedModel } from '@/components/api-service-publish/modelSelection'
import { apiServiceDetailPath } from '@/components/api-service-publish/publishAssistant'
import {
  apiQuotaBoundaryNotice,
  applySimplifiedApiQuotaDefaults,
  createDefaultPaymentOptions,
  defaultPaymentWindowMinutes,
  enabledPaymentOptions,
  formatUsdQuotaForCny,
  generatedTitle,
  merchantNoteTemplate,
  modelProviderCategory,
  paymentMethodLabels,
  providerCategoryLabels,
  selectedCatalogItems,
  sub2ApiPricingPolicy,
} from '@/components/api-service-publish/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { containsSensitiveContent, firstError, type FieldErrors } from '@/lib/formValidation'
import { submitApiService } from '@/lib/api'
import { useModelCatalog } from '@/queries/useMarketQueries'

type Field =
  | 'merchantIdentity'
  | 'merchantDisplayName'
  | 'providerCategory'
  | 'cnyPerUsdCredit'
  | 'selectedModels'
  | 'availableCreditUsd'
  | 'paymentWindowMinutes'
  | 'paymentOptions'
  | 'merchantNote'
  | 'sensitive'

const { data: modelCatalog, isLoading: catalogLoading } = useModelCatalog()
const queryClient = useQueryClient()
const submittedId = ref('')
const errors = reactive<FieldErrors<Field>>({})
const pendingProviderCategory = ref<ApiProviderCategory | null>(null)

const form = reactive<ApiServicePublishForm>({
  merchantIdentityMode: 'store_alias',
  merchantDisplayName: '小葵 API',
  distributionSystem: 'sub2api',
  distributionSystemNote: '',
  providerCategory: 'gpt',
  billingMode: 'metered_credit',
  deliveryModes: ['api_key_endpoint'],
  shortDescription: '建议首次小额测试',
  cnyPerUsdCredit: 0.8,
  manualBillingNote: '',
  defaultMultiplier: sub2ApiPricingPolicy.textModelMultiplier,
  selectedModels: [
    { modelId: 'gpt-5-mini', multiplierOverride: null, enabled: true },
  ],
  imageCapability: {
    enabled: false,
    supportsTextToImage: false,
    supportsImageToImage: false,
    pricingMode: 'same_multiplier',
    customMultiplier: null,
    note: '',
  },
  availableCreditUsd: 500,
  minimumPurchaseCny: 20,
  maximumPurchaseCny: 300,
  paymentWindowMinutes: defaultPaymentWindowMinutes,
  paymentOptions: createDefaultPaymentOptions(),
  packages: [],
  validity: {
    mode: 'days',
    days: 30,
    startsAt: 'delivered_at',
  },
  usageVisibility: 'merchant_confirmed',
  warranty: {
    mode: 'no_warranty',
    warrantyDays: null,
    coverage: null,
    compensation: null,
    exclusions: null,
    refundNote: null,
  },
  merchantNote: merchantNoteTemplate,
})

const catalog = computed(() => modelCatalog.value ?? [])
const filteredCatalog = computed(() => catalog.value.filter(item => modelProviderCategory(item.provider) === form.providerCategory))
const catalogById = computed(() => new Map(catalog.value.map(item => [item.id, item])))
const selectedModels = computed(() => selectedCatalogItems(form, catalogById.value))
const incompatibleSelectedModels = computed(() => selectedModels.value.filter(item => modelProviderCategory(item.provider) !== form.providerCategory))
const missingSelectedModels = computed(() => form.selectedModels.filter(item => item.enabled && !catalogById.value.has(item.modelId)))
const pendingProviderCategoryLabel = computed(() => pendingProviderCategory.value ? providerCategoryLabels[pendingProviderCategory.value] : '')
const quotaForMinimumPurchase = computed(() => formatUsdQuotaForCny(form.cnyPerUsdCredit, form.minimumPurchaseCny ?? 0))
const submittedServicePath = computed(() => apiServiceDetailPath(submittedId.value))
const enabledPayments = computed(() => enabledPaymentOptions(form))
const paymentWindowValid = computed(() => form.paymentWindowMinutes >= 3 && form.paymentWindowMinutes <= 15)
const paymentSettingsComplete = computed(() => enabledPayments.value.length > 0 && enabledPayments.value.every(option => option.paymentInstructions.trim()) && paymentWindowValid.value)

function syncHiddenPublishFields() {
  applySimplifiedApiQuotaDefaults(form)
}

syncHiddenPublishFields()

watch([catalog, () => form.providerCategory], () => {
  if (!catalog.value.length) return
  const compatibleSelected = form.selectedModels.filter(item => {
    const model = catalogById.value.get(item.modelId)
    return item.enabled && model && modelProviderCategory(model.provider) === form.providerCategory
  })
  if (compatibleSelected.length) {
    form.selectedModels = compatibleSelected
    return
  }
  const firstModel = filteredCatalog.value[0]
  form.selectedModels = firstModel
    ? [{ modelId: firstModel.id, multiplierOverride: null, enabled: true }]
    : []
}, { immediate: true })

function setErrors(next: FieldErrors<Field>) {
  for (const key of Object.keys(errors) as Field[]) delete errors[key]
  Object.assign(errors, next)
}

function hasContactLikeText(value: string) {
  return /@|微信|VX|vx|telegram|tg|邮箱|email|https?:\/\/|linux\.do|\.com|\.cn|[0-9]{6,}/i.test(value)
}

function hasMisleadingMerchantName(value: string) {
  return /官方|担保|兜底|认证|跑路|实名/i.test(value)
}

function validate(requireComplete: boolean) {
  syncHiddenPublishFields()
  const next: FieldErrors<Field> = {}
  const merchantDisplayName = form.merchantDisplayName.trim()
  if (!['public_profile', 'store_alias'].includes(form.merchantIdentityMode)) next.merchantIdentity = '请选择对外展示身份。'
  if (form.merchantIdentityMode === 'store_alias') {
    if (!merchantDisplayName) next.merchantDisplayName = '请填写商家展示名。'
    else if (merchantDisplayName.length < 2 || merchantDisplayName.length > 20) next.merchantDisplayName = '商家展示名必须为 2-20 个字符。'
    else if (hasContactLikeText(merchantDisplayName)) next.merchantDisplayName = '商家展示名不能包含联系方式、链接或 linux.do 用户名。'
    else if (hasMisleadingMerchantName(merchantDisplayName)) next.merchantDisplayName = '商家展示名不能包含官方、担保、兜底等误导词。'
  }
  if (!form.providerCategory) next.providerCategory = '请选择模型大类。'
  if (!form.cnyPerUsdCredit || form.cnyPerUsdCredit < sub2ApiPricingPolicy.minimumCnyPerUsdCredit || form.cnyPerUsdCredit > sub2ApiPricingPolicy.maximumCnyPerUsdCredit) {
    next.cnyPerUsdCredit = '每 $1 美元额度售价必须大于 0。'
  }
  if (!form.availableCreditUsd || form.availableCreditUsd <= 0) next.availableCreditUsd = '可售美元额度必须大于 0。'
  if (!form.selectedModels.some(item => item.enabled)) next.selectedModels = '至少选择一个模型。'
  if (missingSelectedModels.value.length) next.selectedModels = '已选模型不在当前后端模型目录中，请重新选择。'
  if (incompatibleSelectedModels.value.length) next.selectedModels = '已选模型必须全部属于当前模型大类。'
  if (!paymentWindowValid.value) next.paymentWindowMinutes = '买家确认付款窗口必须在 3 到 15 分钟之间。'
  if (!enabledPayments.value.length) {
    next.paymentOptions = '请至少启用一种收款方式。'
  } else {
    const missingInstruction = enabledPayments.value.find(option => !option.paymentInstructions.trim())
    if (missingInstruction) next.paymentOptions = `请填写${paymentMethodLabels[missingInstruction.paymentMethod]}收款说明。`
  }
  if (!form.merchantNote.trim()) next.merchantNote = '请填写备注信息。'
  if (form.merchantNote.length > 800) next.merchantNote = '备注信息最多 800 字。'
  if (containsSensitiveContent([
    form.merchantDisplayName,
    form.merchantNote,
    ...form.paymentOptions.map(option => option.paymentInstructions),
  ])) next.sensitive = '请移除 API Key、Sub2API key、endpoint 密钥、token、Session、Cookie、密码、付款码或面板凭据。'

  if (!requireComplete) {
    delete next.merchantIdentity
    if (form.merchantIdentityMode === 'public_profile' || form.merchantDisplayName.trim()) delete next.merchantDisplayName
    delete next.providerCategory
    delete next.selectedModels
    delete next.availableCreditUsd
    delete next.cnyPerUsdCredit
    delete next.paymentWindowMinutes
    delete next.paymentOptions
    delete next.merchantNote
  }

  setErrors(next)
  return Object.keys(next).length === 0
}

const completeness = computed(() => {
  const conflict = (label: string) => ({ label, status: 'conflict' as const })
  const done = (label: string) => ({ label, status: 'done' as const })
  const pending = (label: string) => ({ label, status: 'pending' as const })
  return [
    form.merchantIdentityMode === 'public_profile' || form.merchantDisplayName.trim() ? done('展示身份') : pending('展示身份'),
    form.cnyPerUsdCredit && form.cnyPerUsdCredit > 0 ? done('额度售价') : pending('额度售价'),
    form.availableCreditUsd && form.availableCreditUsd > 0 ? done('可售额度') : pending('可售额度'),
    paymentSettingsComplete.value ? done('收款方式') : pending('收款方式'),
    form.providerCategory ? done('模型大类') : pending('模型大类'),
    incompatibleSelectedModels.value.length ? conflict('具体模型') : form.selectedModels.some(item => item.enabled) ? done('具体模型') : pending('具体模型'),
    form.merchantNote.trim() ? done('备注信息') : pending('备注信息'),
  ]
})

const risks = computed(() => {
  const rows: string[] = []
  rows.push('接入细节和用量核对由双方站外确认，平台不保存凭据，也不提供实时校验。')
  if (incompatibleSelectedModels.value.length) rows.push('当前存在不属于所选模型大类的模型，需清空后才能提交。')
  return rows
})

const canSubmit = computed(() => completeness.value.every(item => item.status === 'done'))
const publishBlockReason = computed(() => {
  if (canSubmit.value) return ''
  const pendingItem = completeness.value.find(item => item.status !== 'done')
  if (pendingItem?.label === '收款方式') {
    if (!paymentWindowValid.value) return '买家确认付款窗口必须在 3 到 15 分钟之间。'
    if (!enabledPayments.value.length) return '先启用并填写至少一种收款方式，发布后才会进入公开服务列表。'
    return '请填写已启用收款方式的站外收款说明。'
  }
  if (pendingItem) return `请先补全：${pendingItem.label}。`
  return '请先补全发布配置。'
})

const publishMutation = useMutation({
  mutationFn: () => {
    syncHiddenPublishFields()
    return submitApiService({
      ...form,
      generatedTitle: generatedTitle(form, catalogById.value),
      status: 'reviewing',
    })
  },
  async onSuccess(result) {
    submittedId.value = String(result.id)
    await invalidateApiServicePublishQueries()
    toast.success('API 服务已发布并开启接单，已进入公开服务列表。')
  },
  onError(error) {
    toast.error(error instanceof Error ? error.message : 'API 服务发布失败，请检查配置后重试。')
  },
})

async function invalidateApiServicePublishQueries() {
  await queryClient.invalidateQueries({ queryKey: ['api-services'] })
  await queryClient.invalidateQueries({ queryKey: ['api-market'] })
  await queryClient.invalidateQueries({ queryKey: ['home-market'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['notifications'] })
}

function setStoreAliasVisibility(event: Event) {
  form.merchantIdentityMode = event.target instanceof HTMLInputElement && event.target.checked ? 'store_alias' : 'public_profile'
}

function toggleModel(id: string) {
  const model = catalogById.value.get(id)
  if (!model || modelProviderCategory(model.provider) !== form.providerCategory) return
  form.selectedModels = toggleSelectedModel(form.selectedModels, id)
}

function publishService() {
  syncHiddenPublishFields()
  if (!validate(true)) {
    toast.warning(firstError(errors) ?? '请先补全发布配置。')
    return
  }
  publishMutation.mutate()
}

function preview() {
  toast(`预览标题：${generatedTitle(form, catalogById.value)}`)
}

function selectedModelsCompatibleWith(category: ApiProviderCategory) {
  return selectedModels.value.filter(item => modelProviderCategory(item.provider) !== category)
}

function requestProviderCategory(value: ApiProviderCategory) {
  if (value === form.providerCategory) return
  if (selectedModelsCompatibleWith(value).length) {
    pendingProviderCategory.value = value
    return
  }
  applyProviderCategory(value)
}

function applyProviderCategory(value: ApiProviderCategory) {
  form.providerCategory = value
  form.selectedModels = form.selectedModels.filter(item => {
    const model = catalogById.value.get(item.modelId)
    return model ? modelProviderCategory(model.provider) === value : false
  })
  pendingProviderCategory.value = null
}

function cancelProviderCategoryChange() {
  pendingProviderCategory.value = null
}

function confirmProviderCategoryChange() {
  if (!pendingProviderCategory.value) return
  applyProviderCategory(pendingProviderCategory.value)
}
</script>

<template>
  <div class="api-publish-page space-y-4 pb-20 md:pb-0">
    <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
      <div>
        <h1 class="text-2xl font-semibold md:text-3xl">发布 API 额度</h1>
        <p class="mt-2 max-w-3xl text-sm text-muted-foreground">快速发布可售额度；买家提交意向后，双方站外确认接入细节。</p>
      </div>
      <div class="hidden gap-2 md:grid lg:flex">
        <Button variant="outline" @click="preview"><Eye class="h-4 w-4" />预览</Button>
      </div>
    </div>

    <div class="flex gap-3 rounded-lg border border-primary/20 bg-primary/5 px-4 py-3 text-sm leading-6 text-foreground">
      <ShieldCheck class="mt-0.5 h-4 w-4 shrink-0 text-primary" />
      <span>{{ apiQuotaBoundaryNotice }}</span>
    </div>

    <div v-if="errors.sensitive" class="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
      {{ errors.sensitive }}
    </div>
    <div v-if="submittedId" class="flex flex-col gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800 md:flex-row md:items-center md:justify-between">
      <span>API 服务已发布：{{ submittedId }}。可以打开详情检查前台展示效果。</span>
      <RouterLink v-if="submittedServicePath" :to="submittedServicePath">
        <Button size="sm">查看服务详情</Button>
      </RouterLink>
    </div>

    <div class="api-publish-layout grid min-w-0 gap-4 lg:items-start">
      <section class="min-w-0 space-y-3">
        <PriceInventorySection :form="form" :errors="errors" />

        <PaymentSettingsSection :form="form" :errors="errors" />

        <ProviderCategorySelector
          :model-value="form.providerCategory"
          :selected-count="selectedModels.length"
          @update:model-value="requestProviderCategory"
        />

        <Card class="api-publish-card">
          <div class="api-publish-card-header">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2>选择具体模型</h2>
                <p>搜索并勾选要出售的模型；所选模型按实际消耗额度计算。</p>
              </div>
              <Badge variant="model">{{ selectedModels.length }} 个模型</Badge>
            </div>
          </div>
          <div class="api-publish-card-body">
            <div v-if="incompatibleSelectedModels.length" class="mb-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
              当前存在不属于所选模型大类的模型，请切换模型大类并确认清空，或手动移除冲突模型。
            </div>
            <div v-if="catalogLoading" class="rounded-lg border border-border bg-background p-4 text-sm text-muted-foreground">正在加载平台模型目录...</div>
            <ModelMultiSelect v-else :form="form" :provider-category="form.providerCategory" :catalog="filteredCatalog" :errors="errors" @toggle-model="toggleModel" />
          </div>
        </Card>

        <MerchantNoteSection :form="form" :errors="errors" />
      </section>

      <ApiServicePublishPreview
        :form="form"
        :catalog-by-id="catalogById"
        :completeness="completeness"
        :risks="risks"
        :quota-for-minimum-purchase="quotaForMinimumPurchase"
        :submitted-id="submittedId"
      />
    </div>

    <div class="sticky bottom-0 z-30 border-t border-border bg-background/95 p-3 shadow-lg backdrop-blur md:static md:rounded-xl md:border md:bg-card md:p-4 md:shadow-sm">
      <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
        <div class="space-y-3">
          <div class="font-semibold">展示身份</div>
          <p class="text-sm text-muted-foreground">默认不公开社区身份，仅展示商家展示名；买家提交意向后再站外确认接入细节。</p>
          <label class="flex items-start gap-2 text-sm">
            <input
              type="checkbox"
              class="mt-0.5 h-4 w-4 accent-primary"
              :checked="form.merchantIdentityMode === 'store_alias'"
              @change="setStoreAliasVisibility"
            />
            <span>
              不公开社区身份，仅展示商家展示名
              <span class="mt-0.5 block text-xs text-muted-foreground">买家仍可看到已绑定 linux.do、信任等级、交易评价与纠纷记录。</span>
            </span>
          </label>
          <label v-if="form.merchantIdentityMode === 'store_alias'" class="block max-w-md space-y-1">
            <span class="text-xs font-medium text-muted-foreground">商家展示名</span>
            <Input v-model="form.merchantDisplayName" maxlength="20" placeholder="例如：小葵 API" />
            <p v-if="errors.merchantDisplayName" class="text-xs text-destructive">{{ errors.merchantDisplayName }}</p>
            <p v-else class="text-xs text-muted-foreground">2-20 个字符；不能包含联系方式、链接、背书承诺或 linux.do 用户名。</p>
          </label>
        </div>
        <div class="grid gap-2 md:flex md:shrink-0">
          <Button :disabled="publishMutation.isPending.value || !canSubmit" @click="publishService"><Send class="h-4 w-4" />{{ publishMutation.isPending.value ? '发布中' : !paymentSettingsComplete ? '先配置收款方式' : '发布 API 额度' }}</Button>
          <p v-if="publishBlockReason" class="max-w-xs text-xs leading-5 text-warning md:text-right">{{ publishBlockReason }}</p>
        </div>
      </div>
    </div>

    <div
      v-if="pendingProviderCategory"
      class="fixed inset-0 z-40 grid place-items-center bg-background/80 p-4 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-labelledby="provider-category-confirm-title"
      @click.self="cancelProviderCategoryChange"
    >
      <Card class="w-full max-w-md p-5 shadow-lg">
        <h2 id="provider-category-confirm-title" class="text-base font-semibold">切换模型大类</h2>
        <p class="mt-2 text-sm leading-6 text-muted-foreground">
          切换到 {{ pendingProviderCategoryLabel }} 会清空当前不兼容的模型选择。GPT 与 Claude 必须分开发布，不能同时存在于同一服务中。
        </p>
        <div class="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button variant="outline" @click="cancelProviderCategoryChange">取消</Button>
          <Button @click="confirmProviderCategoryChange">确认切换并清空</Button>
        </div>
      </Card>
    </div>
  </div>
</template>
