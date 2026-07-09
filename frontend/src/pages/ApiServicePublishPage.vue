<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { ExternalLink, Eye, Send, ShieldCheck } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AccountPaymentSummarySection from '@/components/api-service-publish/AccountPaymentSummarySection.vue'
import ApiAccessSourceSection from '@/components/api-service-publish/ApiAccessSourceSection.vue'
import ApiServicePublishPreview from '@/components/api-service-publish/ApiServicePublishPreview.vue'
import MerchantNoteSection from '@/components/api-service-publish/MerchantNoteSection.vue'
import ModelMultiSelect from '@/components/api-service-publish/ModelMultiSelect.vue'
import PriceInventorySection from '@/components/api-service-publish/PriceInventorySection.vue'
import ProviderCategorySelector from '@/components/api-service-publish/ProviderCategorySelector.vue'
import type { ApiProviderCategory, ApiServicePublishForm, DistributionSystem } from '@/components/api-service-publish/types'
import { toggleSelectedModel } from '@/components/api-service-publish/modelSelection'
import { apiPublishAssistantSummary, apiServiceDetailPath } from '@/components/api-service-publish/publishAssistant'
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
  providerCategoryLabels,
  selectedCatalogItems,
  sub2ApiPricingPolicy,
} from '@/components/api-service-publish/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { containsSensitiveContent, firstError, type FieldErrors } from '@/lib/formValidation'
import { submitApiService } from '@/lib/api'
import { trackAnalytics } from '@/lib/analytics'
import { beijingDateTimeInputToISOString, defaultQuotaExpiresAtInput } from '@/lib/apiQuotaExpiration'
import { apiPaymentSettingsMissingReason, cloneApiPaymentAccountSettings, isApiPaymentAccountSettingsComplete, isApiPaymentOptionComplete, isApiPaymentWindowValid } from '@/lib/apiPaymentSettings'
import { useApiPaymentAccountSettingsQuery, useModelCatalog, useMyProfileQuery } from '@/queries/useMarketQueries'

type Field =
  | 'merchantIdentity'
  | 'merchantDisplayName'
  | 'distributionSystem'
  | 'defaultMultiplier'
  | 'providerCategory'
  | 'cnyPerUsdCredit'
  | 'selectedModels'
  | 'availableCreditUsd'
  | 'quotaExpiresAt'
  | 'paymentWindowMinutes'
  | 'paymentOptions'
  | 'merchantNote'
  | 'sensitive'

const { data: modelCatalog, isLoading: catalogLoading } = useModelCatalog()
const { data: accountPaymentSettings, isLoading: paymentSettingsLoading } = useApiPaymentAccountSettingsQuery()
const { data: myProfile, isLoading: profileLoading } = useMyProfileQuery()
const queryClient = useQueryClient()
const route = useRoute()
const analyticsSourceRoute = () => String(route.name ?? 'unknown')
const submittedId = ref('')
const previewOpen = ref(false)
const errors = reactive<FieldErrors<Field>>({})
const pendingProviderCategory = ref<ApiProviderCategory | null>(null)

const form = reactive<ApiServicePublishForm>({
  merchantIdentityMode: 'store_alias',
  merchantDisplayName: '',
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
  quotaExpiresAt: defaultQuotaExpiresAtInput(),
  minimumPurchaseCny: 10,
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
const paymentWindowValid = computed(() => isApiPaymentWindowValid(form.paymentWindowMinutes))
const paymentSettingsComplete = computed(() => isApiPaymentAccountSettingsComplete(form))
const accountPaymentSettingsValue = computed(() => accountPaymentSettings.value ? cloneApiPaymentAccountSettings(accountPaymentSettings.value) : {
  paymentWindowMinutes: defaultPaymentWindowMinutes,
  paymentOptions: createDefaultPaymentOptions(),
  updatedAt: '',
})
const accountPaymentSettingsComplete = computed(() => isApiPaymentAccountSettingsComplete(accountPaymentSettingsValue.value))
const profileDisplayName = computed(() => myProfile.value?.displayName.trim() ?? '')
const profileUsername = computed(() => myProfile.value?.username.trim() ?? '')
const profileMerchantDisplayName = computed(() => profileDisplayName.value || profileUsername.value)
const merchantDisplayNameStatus = computed(() => {
  if (profileLoading.value && !form.merchantDisplayName.trim()) return '正在读取个人资料显示名称...'
  if (form.merchantDisplayName.trim()) return '发布时会快照当前个人资料显示名称；单条 API 额度不单独改名。'
  return '请先到我的中心设置显示名称。'
})

function syncMerchantDisplayNameSnapshot() {
  form.merchantDisplayName = profileMerchantDisplayName.value
}

function syncHiddenPublishFields() {
  syncMerchantDisplayNameSnapshot()
  applySimplifiedApiQuotaDefaults(form)
}

syncHiddenPublishFields()

watch(profileMerchantDisplayName, () => syncMerchantDisplayNameSnapshot(), { immediate: true })

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

watch(accountPaymentSettingsValue, settings => {
  form.paymentWindowMinutes = settings.paymentWindowMinutes
  form.paymentOptions = settings.paymentOptions.map(option => ({ ...option }))
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

function displayNameLength(value: string) {
  return Array.from(value.trim()).length
}

function validate(requireComplete: boolean) {
  syncHiddenPublishFields()
  const next: FieldErrors<Field> = {}
  const merchantDisplayName = form.merchantDisplayName.trim()
  if (!['public_profile', 'store_alias'].includes(form.merchantIdentityMode)) next.merchantIdentity = '请选择对外展示身份。'
  if (form.merchantIdentityMode === 'store_alias') {
    if (!merchantDisplayName) next.merchantDisplayName = profileLoading.value ? '正在读取个人资料显示名称。' : '请先到我的中心设置显示名称。'
    else if (displayNameLength(merchantDisplayName) > 32) next.merchantDisplayName = '商家展示名最多 32 个字符，请到我的中心调整。'
    else if (hasContactLikeText(merchantDisplayName)) next.merchantDisplayName = '商家展示名不能包含联系方式、链接或 linux.do 用户名，请到我的中心调整。'
    else if (hasMisleadingMerchantName(merchantDisplayName)) next.merchantDisplayName = '商家展示名不能包含官方、担保、兜底等误导词，请到我的中心调整。'
  }
  if (!['sub2api', 'other'].includes(form.distributionSystem)) next.distributionSystem = '请选择接入类型。'
  if (form.distributionSystem === 'other' && (!Number.isFinite(form.defaultMultiplier) || form.defaultMultiplier <= 0)) {
    next.defaultMultiplier = '默认服务倍率必须大于 0。'
  }
  if (!form.providerCategory) next.providerCategory = '请选择模型大类。'
  if (!form.cnyPerUsdCredit || form.cnyPerUsdCredit < sub2ApiPricingPolicy.minimumCnyPerUsdCredit || form.cnyPerUsdCredit > sub2ApiPricingPolicy.maximumCnyPerUsdCredit) {
    next.cnyPerUsdCredit = '每 $1 美元额度售价必须大于 0。'
  }
  if (!form.availableCreditUsd || form.availableCreditUsd <= 0) next.availableCreditUsd = '可售美元额度必须大于 0。'
  const quotaExpiresAtISO = beijingDateTimeInputToISOString(form.quotaExpiresAt)
  if (!quotaExpiresAtISO) next.quotaExpiresAt = '请填写有效的额度有效至时间。'
  else if (new Date(quotaExpiresAtISO).getTime() <= Date.now()) next.quotaExpiresAt = '额度有效至时间必须晚于当前时间。'
  if (!form.selectedModels.some(item => item.enabled)) next.selectedModels = '至少选择一个模型。'
  if (missingSelectedModels.value.length) next.selectedModels = '已选模型不在当前后端模型目录中，请重新选择。'
  if (incompatibleSelectedModels.value.length) next.selectedModels = '已选模型必须全部属于当前模型大类。'
  if (!paymentWindowValid.value) next.paymentWindowMinutes = '买家确认付款窗口固定为 10 分钟。'
  if (!enabledPayments.value.length) {
    next.paymentOptions = '请至少启用一种收款方式。'
  } else {
    const missingOption = enabledPayments.value.find(option => !isApiPaymentOptionComplete(option))
    if (missingOption) next.paymentOptions = apiPaymentSettingsMissingReason(form)
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
    delete next.distributionSystem
    if (form.distributionSystem === 'sub2api' || (Number.isFinite(form.defaultMultiplier) && form.defaultMultiplier > 0)) delete next.defaultMultiplier
    delete next.providerCategory
    delete next.selectedModels
    delete next.availableCreditUsd
    delete next.quotaExpiresAt
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
    form.distributionSystem ? done('接入类型') : pending('接入类型'),
    form.distributionSystem === 'sub2api' || (Number.isFinite(form.defaultMultiplier) && form.defaultMultiplier > 0) ? done('服务倍率') : pending('服务倍率'),
    form.cnyPerUsdCredit && form.cnyPerUsdCredit > 0 ? done('额度售价') : pending('额度售价'),
    form.availableCreditUsd && form.availableCreditUsd > 0 ? done('可售额度') : pending('可售额度'),
    beijingDateTimeInputToISOString(form.quotaExpiresAt) ? done('有效时间') : pending('有效时间'),
    accountPaymentSettingsComplete.value && paymentSettingsComplete.value ? done('收款方式') : pending('收款方式'),
    form.providerCategory ? done('模型大类') : pending('模型大类'),
    incompatibleSelectedModels.value.length ? conflict('具体模型') : form.selectedModels.some(item => item.enabled) ? done('具体模型') : pending('具体模型'),
    form.merchantNote.trim() ? done('备注信息') : pending('备注信息'),
  ]
})
const publishAssistant = computed(() => apiPublishAssistantSummary(completeness.value))

const risks = computed(() => {
  const rows: string[] = []
  rows.push('API 细节和用量核对由双方站外确认，平台不保存凭据，也不提供实时校验。')
  if (incompatibleSelectedModels.value.length) rows.push('当前存在不属于所选模型大类的模型，需清空后才能提交。')
  return rows
})

const canSubmit = computed(() => completeness.value.every(item => item.status === 'done'))
const publishBlockReason = computed(() => {
  if (canSubmit.value) return ''
  const pendingItem = completeness.value.find(item => item.status !== 'done')
  if (pendingItem?.label === '收款方式') {
    if (!paymentWindowValid.value) return '买家确认付款窗口固定为 10 分钟。'
    if (!accountPaymentSettingsComplete.value || !enabledPayments.value.length) return '先到我的中心配置 API 收款设置，发布后才会进入公开服务列表。'
    return apiPaymentSettingsMissingReason(form) || '请到我的中心补全已启用收款方式。'
  }
  if (pendingItem?.label === '展示身份') {
    return profileLoading.value ? '正在读取个人资料显示名称。' : '请先到我的中心设置显示名称。'
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
    trackAnalytics('api_service_publish_success', {
      source_route: analyticsSourceRoute(),
      provider_category: form.providerCategory,
      billing_mode: form.billingMode,
      delivery_mode: form.deliveryModes[0],
      minimum_purchase_cny: form.minimumPurchaseCny,
    })
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

function setDistribution(value: DistributionSystem) {
  form.distributionSystem = value
  form.billingMode = 'metered_credit'
  form.usageVisibility = 'merchant_confirmed'
  form.deliveryModes = ['api_key_endpoint']
  if (value === 'sub2api') {
    form.defaultMultiplier = sub2ApiPricingPolicy.textModelMultiplier
    if (!form.distributionSystemNote.trim() || form.distributionSystemNote.includes('其他 API')) {
      form.distributionSystemNote = 'Sub2API 标准美元额度，接入细节由双方站外确认。'
    }
    return
  }
  form.distributionSystemNote = form.distributionSystemNote.trim() || '其他 API 接入，额度与用量由商户站外说明。'
  if (!Number.isFinite(form.defaultMultiplier) || form.defaultMultiplier <= 0) form.defaultMultiplier = 1
}

function setDefaultMultiplier(value: string) {
  form.defaultMultiplier = Number(value)
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
  syncHiddenPublishFields()
  previewOpen.value = true
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
      <div class="flex gap-2">
        <Button variant="outline" @click="preview"><Eye class="h-4 w-4" />预览</Button>
      </div>
    </div>

    <div class="flex gap-3 rounded-lg border border-primary/20 bg-primary/5 px-4 py-3 text-sm leading-6 text-foreground">
      <ShieldCheck class="mt-0.5 h-4 w-4 shrink-0 text-primary" />
      <span>{{ apiQuotaBoundaryNotice }}</span>
    </div>

    <div class="rounded-lg border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <div class="text-sm font-medium">发布必填 {{ publishAssistant.doneCount }} / {{ publishAssistant.totalCount }}</div>
          <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ publishAssistant.topPendingText }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <span class="rounded-full border border-warning/25 bg-warning/10 px-3 py-1 text-xs font-medium text-warning">待补 {{ publishAssistant.pendingCount }}</span>
          <span v-if="publishAssistant.conflictCount" class="rounded-full border border-destructive/25 bg-destructive/10 px-3 py-1 text-xs font-medium text-destructive">冲突 {{ publishAssistant.conflictCount }}</span>
          <span class="rounded-full border border-success/25 bg-success/10 px-3 py-1 text-xs font-medium text-success">已完成 {{ publishAssistant.doneCount }}</span>
        </div>
      </div>
      <div class="mt-3 h-2 overflow-hidden rounded-full bg-muted">
        <div class="h-full rounded-full bg-primary" :style="{ width: `${publishAssistant.progressPercent}%` }" />
      </div>
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

        <AccountPaymentSummarySection
          :form="form"
          :settings="accountPaymentSettingsValue"
          :loading="paymentSettingsLoading"
        />

        <ApiAccessSourceSection :form="form" :errors="errors" @set-distribution="setDistribution" @set-default-multiplier="setDefaultMultiplier" />

        <ProviderCategorySelector
          :model-value="form.providerCategory"
          :selected-count="selectedModels.length"
          @update:model-value="requestProviderCategory"
        />

        <Card class="api-publish-card">
          <div class="api-publish-card-header">
            <div class="flex flex-wrap items-start justify-between gap-3">
              <div>
                <h2>具体模型</h2>
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
          <p class="text-sm text-muted-foreground">默认不公开社区身份，仅展示商家展示名；买家提交意向后再站外确认 API 细节。</p>
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
          <div v-if="form.merchantIdentityMode === 'store_alias'" class="max-w-md rounded-md border border-border bg-muted/35 px-3 py-2">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div class="min-w-0">
                <div class="text-xs font-medium text-muted-foreground">商家展示名</div>
                <div class="mt-1 truncate text-sm font-semibold">{{ form.merchantDisplayName || (profileLoading ? '正在读取个人资料...' : '待设置显示名称') }}</div>
              </div>
              <RouterLink to="/my/profile" class="shrink-0">
                <Button size="sm" variant="outline">
                  去个人资料修改 <ExternalLink class="h-3.5 w-3.5" />
                </Button>
              </RouterLink>
            </div>
            <p v-if="errors.merchantDisplayName" class="text-xs text-destructive">{{ errors.merchantDisplayName }}</p>
            <p v-else class="text-xs leading-5 text-muted-foreground">{{ merchantDisplayNameStatus }}</p>
          </div>
        </div>
        <div class="grid gap-2 md:flex md:shrink-0">
          <Button :disabled="publishMutation.isPending.value || !canSubmit" @click="publishService"><Send class="h-4 w-4" />{{ publishMutation.isPending.value ? '发布中' : !accountPaymentSettingsComplete ? '先配置账号收款' : !paymentSettingsComplete ? '先配置收款方式' : '发布 API 额度' }}</Button>
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

    <Dialog v-model:open="previewOpen">
      <DialogContent class="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>API 额度预览</DialogTitle>
          <DialogDescription>发布前确认买家将看到的核心信息。</DialogDescription>
        </DialogHeader>
        <ApiServicePublishPreview
          :form="form"
          :catalog-by-id="catalogById"
          :completeness="completeness"
          :risks="risks"
          :quota-for-minimum-purchase="quotaForMinimumPurchase"
          :submitted-id="submittedId"
          preview-only
        />
      </DialogContent>
    </Dialog>
  </div>
</template>
