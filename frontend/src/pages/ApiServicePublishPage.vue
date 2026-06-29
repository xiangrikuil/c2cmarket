<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { Eye, Send } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import ApiServicePublishPreview from '@/components/api-service-publish/ApiServicePublishPreview.vue'
import DeliveryModeSection from '@/components/api-service-publish/DeliveryModeSection.vue'
import DistributionBillingSection from '@/components/api-service-publish/DistributionBillingSection.vue'
import ImageCapabilitySection from '@/components/api-service-publish/ImageCapabilitySection.vue'
import ModelMultiSelect from '@/components/api-service-publish/ModelMultiSelect.vue'
import PriceInventorySection from '@/components/api-service-publish/PriceInventorySection.vue'
import ProviderCategorySelector from '@/components/api-service-publish/ProviderCategorySelector.vue'
import SelectedModelsPricingTable from '@/components/api-service-publish/SelectedModelsPricingTable.vue'
import WarrantySection from '@/components/api-service-publish/WarrantySection.vue'
import type { ApiProviderCategory, ApiServicePublishForm, BillingMode, DistributionSystem, PublishDeliveryMode, UsageVisibility } from '@/components/api-service-publish/types'
import { billingLabels, formatUsdQuotaForCny, generatedTitle, modelProviderCategory, providerCategoryLabels, selectedCatalogItems, sub2ApiPricingPolicy } from '@/components/api-service-publish/utils'
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
  | 'distributionSystemNote'
  | 'deliveryModes'
  | 'defaultMultiplier'
  | 'cnyPerUsdCredit'
  | 'selectedModels'
  | 'imageCapability'
  | 'availableCreditUsd'
  | 'manualBillingNote'
  | 'minimumPurchaseCny'
  | 'packages'
  | 'validity'
  | 'usageVisibility'
  | 'warranty'
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
  deliveryModes: ['api_key_endpoint', 'sub2api_panel_account'],
  shortDescription: '面板实时用量，建议首次小额测试',
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
  packages: [
    { id: 'pkg-20', name: '¥20 测试套餐', priceCny: 20, durationDays: 30, description: '小额测试模型可用性和响应速度', inventory: 10 },
    { id: 'pkg-50', name: '¥50 常用套餐', priceCny: 50, durationDays: 30, description: '常用开发测试额度', inventory: 8 },
  ],
  validity: {
    mode: 'days',
    days: 30,
    startsAt: 'delivered_at',
  },
  usageVisibility: 'panel_realtime',
  warranty: {
    mode: 'merchant_warranty',
    warrantyDays: 7,
    coverage: '接口连续不可用、余额异常或模型与发布说明不符。',
    compensation: '按不可用时长或异常额度补偿。',
    exclusions: '滥用、高并发压测、上游策略变更或买家配置错误。',
    refundNote: '',
  },
  merchantNote: '建议首次提交 ¥20 意向测试。站外确认后按所选接入方式使用；高峰期响应可能稍慢；部分模型维护期间可能临时下线；禁止滥用或高并发压测。',
})

const catalog = computed(() => modelCatalog.value ?? [])
const filteredCatalog = computed(() => catalog.value.filter(item => modelProviderCategory(item.provider) === form.providerCategory))
const catalogById = computed(() => new Map(catalog.value.map(item => [item.id, item])))
const selectedModels = computed(() => selectedCatalogItems(form, catalogById.value))
const incompatibleSelectedModels = computed(() => selectedModels.value.filter(item => modelProviderCategory(item.provider) !== form.providerCategory))
const missingSelectedModels = computed(() => form.selectedModels.filter(item => item.enabled && !catalogById.value.has(item.modelId)))
const hasImageCapableModel = computed(() => selectedModels.value.some(item => item.capabilities.includes('image_generation') || item.capabilities.includes('image_edit')))
const canConfigureImageCapability = computed(() => form.distributionSystem === 'sub2api' && form.providerCategory === 'gpt')
const pendingProviderCategoryLabel = computed(() => pendingProviderCategory.value ? providerCategoryLabels[pendingProviderCategory.value] : '')
const quotaForOneCny = computed(() => formatUsdQuotaForCny(form.cnyPerUsdCredit, 1))
const quotaForFiftyCny = computed(() => formatUsdQuotaForCny(form.cnyPerUsdCredit, 50))
const quotaForMinimumPurchase = computed(() => formatUsdQuotaForCny(form.cnyPerUsdCredit, form.minimumPurchaseCny ?? 0))
const allowedUsage = computed<UsageVisibility[]>(() => {
  if (form.billingMode === 'fixed_package') return ['fixed_package_only', 'not_available']
  if (form.distributionSystem !== 'sub2api') return ['merchant_confirmed', 'not_available']
  if (form.deliveryModes.includes('sub2api_panel_account')) return ['panel_realtime', 'panel_balance_only', 'merchant_confirmed', 'not_available']
  return ['merchant_confirmed', 'not_available']
})

watch(() => form.distributionSystem, value => {
  if (value === 'sub2api') {
    form.billingMode = 'metered_credit'
    form.defaultMultiplier = sub2ApiPricingPolicy.textModelMultiplier
    if (!form.deliveryModes.length) form.deliveryModes = ['api_key_endpoint', 'sub2api_panel_account']
  }
  if (value !== 'sub2api') {
    if (form.billingMode === 'metered_credit') form.billingMode = 'manual_credit'
    form.deliveryModes = ['api_key_endpoint']
    form.imageCapability.enabled = false
    form.imageCapability.supportsTextToImage = false
    form.imageCapability.supportsImageToImage = false
  }
}, { immediate: true })

watch(() => form.providerCategory, value => {
  if (value !== 'gpt') {
    form.imageCapability.enabled = false
    form.imageCapability.supportsTextToImage = false
    form.imageCapability.supportsImageToImage = false
  }
})

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

watch(() => form.billingMode, value => {
  if (form.distributionSystem === 'sub2api' && value !== 'metered_credit') {
    form.billingMode = 'metered_credit'
  }
  if (form.distributionSystem !== 'sub2api' && value === 'metered_credit') {
    form.billingMode = 'manual_credit'
  }
})

watch(allowedUsage, values => {
  if (!values.includes(form.usageVisibility)) {
    form.usageVisibility = values[0]
  }
}, { immediate: true })

watch(hasImageCapableModel, value => {
  if (!value) {
    form.imageCapability.enabled = false
    form.imageCapability.supportsTextToImage = false
    form.imageCapability.supportsImageToImage = false
  }
})

watch(canConfigureImageCapability, value => {
  if (!value) {
    form.imageCapability.enabled = false
    form.imageCapability.supportsTextToImage = false
    form.imageCapability.supportsImageToImage = false
  }
}, { immediate: true })

function setErrors(next: FieldErrors<Field>) {
  for (const key of Object.keys(errors) as Field[]) delete errors[key]
  Object.assign(errors, next)
}

function parseMultiplier(value: string) {
  const normalized = value.trim().replace(/x$/i, '')
  const parsed = Number(normalized)
  return Number.isFinite(parsed) ? parsed : Number.NaN
}

function isValidMultiplier(value: number) {
  return Number.isFinite(value) && value >= 0.01 && value <= 5
}

function hasContactLikeText(value: string) {
  return /@|微信|VX|vx|telegram|tg|邮箱|email|https?:\/\/|linux\.do|\.com|\.cn|[0-9]{6,}/i.test(value)
}

function hasMisleadingMerchantName(value: string) {
  return /官方|担保|兜底|认证|跑路|实名/i.test(value)
}

function validate(requireComplete: boolean) {
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
  if (form.distributionSystem === 'other' && !form.distributionSystemNote.trim()) next.distributionSystemNote = '请填写分发系统说明。'
  if (!form.deliveryModes.length) next.deliveryModes = '至少选择一种接入方式。'
  if (form.distributionSystem !== 'sub2api' && form.deliveryModes.includes('sub2api_panel_account')) next.deliveryModes = 'NewAPI Proxy 或其他系统只能使用 API 请求地址接入说明。'
  if (form.distributionSystem === 'sub2api') {
    if (!form.cnyPerUsdCredit || form.cnyPerUsdCredit < sub2ApiPricingPolicy.minimumCnyPerUsdCredit || form.cnyPerUsdCredit > sub2ApiPricingPolicy.maximumCnyPerUsdCredit) {
      next.cnyPerUsdCredit = '每 $1 美元额度售价必须大于 0。'
    }
    if (form.defaultMultiplier !== sub2ApiPricingPolicy.textModelMultiplier) next.defaultMultiplier = 'Sub2API 文本倍率必须保持 1.00x。'
    if (!form.availableCreditUsd || form.availableCreditUsd <= 0) next.availableCreditUsd = '最大可售美元额度必须大于 0。'
  } else if (!isValidMultiplier(form.defaultMultiplier)) {
    next.defaultMultiplier = '默认服务倍率必须在 0.01 到 5.00 之间。'
  }
  if (!form.selectedModels.some(item => item.enabled)) next.selectedModels = '至少选择一个模型。'
  if (missingSelectedModels.value.length) next.selectedModels = '已选模型不在当前后端模型目录中，请重新选择。'
  if (incompatibleSelectedModels.value.length) next.selectedModels = '已选模型必须全部属于当前模型大类。'
  if (form.distributionSystem !== 'sub2api' && form.billingMode === 'metered_credit') next.availableCreditUsd = 'NewAPI Proxy 或其他系统不允许使用精确额度计费。'
  if (form.billingMode !== 'fixed_package') {
    const minimumPurchaseCny = form.minimumPurchaseCny
    if (typeof minimumPurchaseCny !== 'number' || !Number.isInteger(minimumPurchaseCny) || minimumPurchaseCny < 1) next.minimumPurchaseCny = '最低意向金额必须为不小于 1 的整数元。'
  }
  if (requireComplete && requireManualBillingNote.value && !form.manualBillingNote.trim()) next.manualBillingNote = '请填写计费说明与用量核对方式。'
  if (form.billingMode === 'fixed_package') {
    const invalidPackage = form.packages.find(item => !item.name.trim() || !item.description.trim() || !Number.isFinite(item.priceCny) || item.priceCny <= 0 || (item.durationDays !== null && (!Number.isInteger(item.durationDays) || item.durationDays <= 0)))
    if (!form.packages.length || invalidPackage) next.packages = '固定套餐必须包含名称、价格、有效期和说明。'
  }
  if (form.validity.mode === 'days' && (!form.validity.days || form.validity.days <= 0)) next.validity = '请设置站外确认后可用天数。'
  if (form.imageCapability.enabled) {
    if (!canConfigureImageCapability.value) next.imageCapability = '仅 GPT + Sub2API 可以配置图像生成能力。'
    if (!hasImageCapableModel.value) next.imageCapability = '开启图像生成时必须选择支持图像能力的模型。'
    if (!form.imageCapability.supportsTextToImage && !form.imageCapability.supportsImageToImage) next.imageCapability = '至少选择文生图或图生图能力。'
  }
  if (!allowedUsage.value.includes(form.usageVisibility)) next.usageVisibility = '当前用量查看方式与分发系统或接入方式冲突。'
  if (requireComplete && form.warranty.mode === 'no_warranty') next.warranty = '发布前必须选择上游退款跟随或商户承诺。'
  if (form.warranty.mode === 'upstream_refund_only' && !form.warranty.refundNote?.trim()) next.warranty = '请填写上游退款后的处理说明。'
  if (form.warranty.mode === 'merchant_warranty' && (!form.warranty.warrantyDays || !form.warranty.coverage?.trim() || !form.warranty.compensation?.trim())) next.warranty = '商户承诺必须填写天数、适用范围和补偿方式。'
  if (!form.merchantNote.trim()) next.merchantNote = '请填写买家须知。'
  if (form.merchantNote.length > 800) next.merchantNote = '买家须知最多 800 字。'
  if (containsSensitiveContent([
    form.distributionSystemNote,
    form.merchantDisplayName,
    form.shortDescription,
    form.manualBillingNote,
    ...form.packages.flatMap(item => [item.name, item.description]),
    form.imageCapability.note ?? '',
    form.warranty.coverage ?? '',
    form.warranty.compensation ?? '',
    form.warranty.exclusions ?? '',
    form.warranty.refundNote ?? '',
    form.merchantNote,
  ])) next.sensitive = '请移除 API Key、Sub2API key、endpoint 密钥、token、密码或付款码内容。'

  if (!requireComplete) {
    delete next.merchantIdentity
    if (form.merchantIdentityMode === 'public_profile' || form.merchantDisplayName.trim()) delete next.merchantDisplayName
    delete next.providerCategory
    delete next.selectedModels
    delete next.availableCreditUsd
    delete next.cnyPerUsdCredit
    delete next.manualBillingNote
    delete next.minimumPurchaseCny
    delete next.packages
    delete next.validity
    delete next.usageVisibility
    delete next.warranty
    delete next.merchantNote
  }

  setErrors(next)
  return Object.keys(next).length === 0
}

const completeness = computed(() => {
  const conflict = (label: string) => ({ label, status: 'conflict' as const })
  const done = (label: string) => ({ label, status: 'done' as const })
  const pending = (label: string) => ({ label, status: 'pending' as const })
  const hasBillingRule = form.distributionSystem === 'sub2api'
    ? Boolean(form.cnyPerUsdCredit && form.cnyPerUsdCredit > 0)
    : form.billingMode === 'fixed_package'
      ? Boolean(form.packages.length && !errors.packages)
      : Boolean(form.manualBillingNote.trim() || !requireManualBillingNote.value)
  const hasInventory = form.billingMode === 'fixed_package'
    ? form.packages.some(item => item.inventory === null || item.inventory > 0)
    : form.distributionSystem === 'sub2api'
      ? Boolean(form.availableCreditUsd && form.availableCreditUsd > 0)
      : true
  return [
    done('分发系统'),
    form.merchantIdentityMode === 'public_profile' || form.merchantDisplayName.trim() ? done('展示身份') : pending('展示身份'),
    form.providerCategory ? done('模型大类') : pending('模型大类'),
    incompatibleSelectedModels.value.length ? conflict('具体模型') : form.selectedModels.some(item => item.enabled) ? done('具体模型') : pending('具体模型'),
    form.distributionSystem !== 'sub2api' && form.billingMode === 'metered_credit' ? conflict('计费规则') : hasBillingRule ? done('计费规则') : pending('计费规则'),
    form.deliveryModes.length ? done('接入方式') : pending('接入方式'),
    allowedUsage.value.includes(form.usageVisibility) ? done('用量查看') : conflict('用量查看'),
    hasInventory ? done('库存') : pending('库存'),
    form.billingMode === 'fixed_package' || (form.minimumPurchaseCny && form.minimumPurchaseCny >= 1) ? done('最低意向金额') : pending('最低意向金额'),
    form.validity.mode === 'permanent' || form.validity.days ? done('有效期') : pending('有效期'),
    form.warranty.mode === 'no_warranty' ? conflict('商户承诺') : form.warranty.mode === 'merchant_warranty' && (!form.warranty.warrantyDays || !form.warranty.coverage || !form.warranty.compensation) ? pending('商户承诺') : done('商户承诺'),
    form.merchantNote.trim() ? done('买家须知') : pending('买家须知'),
  ]
})

const requireManualBillingNote = computed(() => form.distributionSystem !== 'sub2api' && form.billingMode === 'manual_credit')
const risks = computed(() => {
  const rows: string[] = []
  if (form.distributionSystem !== 'sub2api') rows.push('该服务无法由平台核验精确余额，前台会标记“商户确认用量”。')
  if (form.warranty.mode === 'no_warranty') rows.push('当前未填写商户承诺，不能发布。')
  if (form.providerCategory === 'claude') rows.push('Claude 服务不显示 GPT 图像生成配置。')
  if (incompatibleSelectedModels.value.length) rows.push('当前存在不属于所选模型大类的模型，需清空后才能提交。')
  return rows
})

const canSubmit = computed(() => completeness.value.every(item => item.status === 'done') && !Object.keys(errors).length)

const publishMutation = useMutation({
  mutationFn: () => submitApiService({
    ...form,
    generatedTitle: generatedTitle(form, catalogById.value),
    status: 'reviewing',
  }),
  async onSuccess(result) {
    submittedId.value = String(result.id)
    await invalidateApiServicePublishQueries()
    toast.success('API 服务已发布。接单配置完整后会进入公开服务列表。')
  },
})

async function invalidateApiServicePublishQueries() {
  await queryClient.invalidateQueries({ queryKey: ['api-services'] })
  await queryClient.invalidateQueries({ queryKey: ['api-market'] })
  await queryClient.invalidateQueries({ queryKey: ['home-market'] })
  await queryClient.invalidateQueries({ queryKey: ['admin-section'] })
  await queryClient.invalidateQueries({ queryKey: ['notifications'] })
}

function setDistribution(value: DistributionSystem) {
  form.distributionSystem = value
}

function setBilling(value: BillingMode) {
  if (form.distributionSystem !== 'sub2api' && value === 'metered_credit') return
  if (form.distributionSystem === 'sub2api' && value !== 'metered_credit') return
  form.billingMode = value
}

function toggleDelivery(value: PublishDeliveryMode) {
  if (form.distributionSystem !== 'sub2api') {
    form.deliveryModes = ['api_key_endpoint']
    return
  }
  if (form.deliveryModes.includes(value)) {
    form.deliveryModes = form.deliveryModes.filter(item => item !== value)
  } else {
    form.deliveryModes = [...form.deliveryModes, value]
  }
}

function addModel(id: string) {
  if (form.selectedModels.some(item => item.modelId === id && item.enabled)) return
  const model = catalogById.value.get(id)
  if (!model || modelProviderCategory(model.provider) !== form.providerCategory) return
  form.selectedModels.push({ modelId: id, multiplierOverride: null, enabled: true })
}

function removeModel(id: string) {
  form.selectedModels = form.selectedModels.filter(item => item.modelId !== id)
}

function setModelMultiplier(id: string, value: string) {
  const target = form.selectedModels.find(item => item.modelId === id)
  if (!target) return
  const parsed = parseMultiplier(value)
  target.multiplierOverride = Number.isFinite(parsed) ? parsed : null
}

function publishService() {
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
        <h1 class="text-2xl font-semibold md:text-3xl">发布 API 服务</h1>
        <p class="mt-2 max-w-3xl text-sm text-muted-foreground">统一发布入口；选择分发系统和单一模型大类后，按对应规则配置计费、接入、库存与商户承诺。</p>
      </div>
      <div class="hidden gap-2 md:grid lg:flex">
        <Button variant="outline" @click="preview"><Eye class="h-4 w-4" />预览</Button>
      </div>
    </div>

    <div v-if="errors.sensitive" class="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
      {{ errors.sensitive }}
	</div>
	<div v-if="submittedId" class="rounded-lg border border-border bg-accent px-4 py-3 text-sm">
	  API 服务已发布：{{ submittedId }}。接单配置完整后会进入公开服务列表。
	</div>

    <div class="api-publish-layout grid min-w-0 gap-4 lg:items-start">
      <section class="min-w-0 space-y-3">
        <Card class="api-publish-card">
          <div class="api-publish-card-header">
            <h2>1. 对外展示身份</h2>
            <p>选择买家在 API 集市和意向记录中看到的商家身份。使用商家展示名时，不公开 linux.do 用户名和个人主页。</p>
          </div>
          <div class="api-publish-card-body">
            <div class="grid gap-3 md:grid-cols-2">
              <button
                type="button"
                class="rounded-lg border p-4 text-left transition"
                :class="form.merchantIdentityMode === 'public_profile' ? 'border-primary bg-primary/10 ring-1 ring-primary' : 'border-border bg-background hover:bg-muted'"
                @click="form.merchantIdentityMode = 'public_profile'"
              >
                <div class="font-semibold">公开个人身份</div>
                <p class="mt-1 text-sm leading-6 text-muted-foreground">展示站内用户名、公开主页和 linux.do 绑定信息。</p>
              </button>
              <button
                type="button"
                class="rounded-lg border p-4 text-left transition"
                :class="form.merchantIdentityMode === 'store_alias' ? 'border-primary bg-primary/10 ring-1 ring-primary' : 'border-border bg-background hover:bg-muted'"
                @click="form.merchantIdentityMode = 'store_alias'"
              >
                <div class="font-semibold">使用商家展示名</div>
                <p class="mt-1 text-sm leading-6 text-muted-foreground">隐藏 linux.do 用户名和个人主页，只展示商家名称与交易信用信息。</p>
              </button>
            </div>
            <label v-if="form.merchantIdentityMode === 'store_alias'" class="mt-4 block space-y-2">
              <span class="text-sm font-medium">商家展示名</span>
              <Input
                v-model="form.merchantDisplayName"
                maxlength="20"
                placeholder="例如：小葵 API"
              />
              <p v-if="errors.merchantDisplayName" class="text-xs text-destructive">{{ errors.merchantDisplayName }}</p>
              <p v-else class="text-xs text-muted-foreground">2-20 个字符；不能包含联系方式、链接、背书承诺或 linux.do 用户名。</p>
            </label>
            <p class="mt-3 rounded-md border border-border bg-muted/50 px-3 py-2 text-xs leading-5 text-muted-foreground">
              服务上线后，买家成功提交购买意向会立即看到你配置的站外联系方式。商家展示名只隐藏公开社区身份，不代表平台背书。
            </p>
          </div>
        </Card>

        <DistributionBillingSection
          :form="form"
          :errors="errors"
          @set-distribution="setDistribution"
          @set-billing="setBilling"
        />

        <ProviderCategorySelector
          :model-value="form.providerCategory"
          :selected-count="selectedModels.length"
          @update:model-value="requestProviderCategory"
        />

        <Card class="api-publish-card">
          <div class="api-publish-card-header">
            <h2>3. 模型与计费</h2>
            <p>{{ form.distributionSystem === 'sub2api' ? 'Sub2API：倍率固定 1.00x，商户只配置额度售价。' : '选择固定套餐或商户确认用量；不进入 Sub2API 标准额度榜单。' }}</p>
          </div>
          <div class="api-publish-card-body">
            <div class="mb-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              <div class="space-y-2">
                <span class="text-sm font-medium">自动生成标题</span>
                <div class="rounded-md border border-border bg-muted/50 px-3 py-2 text-sm font-semibold">{{ generatedTitle(form, catalogById) }}</div>
                <p class="text-xs text-muted-foreground">由模型大类、分发系统和 {{ billingLabels[form.billingMode] }} 自动组成。</p>
              </div>
              <label v-if="form.distributionSystem === 'sub2api'" class="space-y-2">
                <span class="text-sm font-medium">每 $1 美元额度售价</span>
                <div class="flex overflow-hidden rounded-md border border-input bg-background">
                  <Input
                    :model-value="form.cnyPerUsdCredit ?? ''"
                    class="border-0 shadow-none focus-visible:ring-0"
                    placeholder="0.80"
                    @update:model-value="value => form.cnyPerUsdCredit = Number(value)"
                  />
                  <span class="grid w-14 place-items-center border-l border-border text-sm text-muted-foreground">元</span>
                </div>
                <p v-if="errors.cnyPerUsdCredit" class="text-xs text-destructive">{{ errors.cnyPerUsdCredit }}</p>
                <p v-else class="text-xs text-muted-foreground">用于计算买家可向商户购买的美元额度上限，最终金额由双方站外确认。</p>
              </label>
              <label v-if="form.distributionSystem === 'sub2api'" class="space-y-2">
                <span class="text-sm font-medium">文本模型倍率</span>
                <div class="rounded-md border border-border bg-muted/50 px-3 py-2 text-sm font-semibold">1.00x（平台锁定）</div>
                <p class="text-xs text-muted-foreground">Sub2API 标准额度固定倍率，商户不能修改。</p>
              </label>
              <label v-if="form.distributionSystem !== 'sub2api'" class="space-y-2">
                <span class="text-sm font-medium">默认服务倍率</span>
                <Input
                  :model-value="form.defaultMultiplier"
                  placeholder="0.30"
                  @update:model-value="value => form.defaultMultiplier = Number(value)"
                />
                <p class="text-xs text-muted-foreground">按平台官方模型价格乘以该倍率计费。</p>
                <p v-if="errors.defaultMultiplier" class="text-xs text-destructive">{{ errors.defaultMultiplier }}</p>
              </label>
            </div>

            <div v-if="form.distributionSystem === 'sub2api'" class="api-publish-compute-grid mb-4">
              <div class="api-publish-compute-box">
                <b>{{ quotaForOneCny }}</b>
                <span>¥1 对应美元额度</span>
              </div>
              <div class="api-publish-compute-box">
                <b>{{ quotaForFiftyCny }}</b>
                <span>¥50 对应美元额度</span>
              </div>
              <div class="api-publish-compute-box">
                <b>1.00x</b>
                <span>文本模型固定倍率</span>
              </div>
            </div>

            <div v-if="incompatibleSelectedModels.length" class="mb-4 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
              当前存在不属于所选模型大类的模型，请切换模型大类并确认清空，或手动移除冲突模型。
            </div>
            <div v-if="catalogLoading" class="rounded-lg border border-border bg-background p-4 text-sm text-muted-foreground">正在加载平台模型目录...</div>
            <template v-else>
              <ModelMultiSelect :form="form" :provider-category="form.providerCategory" :catalog="filteredCatalog" :errors="errors" @add-model="addModel" />
              <div class="mt-4">
                <SelectedModelsPricingTable
                  :form="form"
                  :catalog-by-id="catalogById"
                  :sub2-api-locked="form.distributionSystem === 'sub2api'"
                  @remove-model="removeModel"
                  @set-multiplier="setModelMultiplier"
                />
              </div>
            </template>
          </div>
        </Card>

        <DeliveryModeSection :form="form" :errors="errors" @toggle-delivery="toggleDelivery" />

        <ImageCapabilitySection v-if="canConfigureImageCapability" :form="form" :has-image-capable-model="hasImageCapableModel" :errors="errors" />
        <PriceInventorySection :form="form" :allowed-usage="allowedUsage" :errors="errors" />
        <WarrantySection :form="form" :errors="errors" />

        <Card class="p-4 shadow-sm">
          <div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
            <Badge variant="model">流程</Badge>
            <span>补全服务信息</span>
            <span>→</span>
            <span>发布 API 服务</span>
            <span>→</span>
            <span>配置接单方式后公开接单</span>
          </div>
        </Card>
      </section>

      <ApiServicePublishPreview
        :form="form"
        :catalog-by-id="catalogById"
        :completeness="completeness"
        :risks="risks"
        :quota-for-minimum-purchase="quotaForMinimumPurchase"
      />
    </div>

    <div class="sticky bottom-0 z-30 border-t border-border bg-background/95 p-3 shadow-lg backdrop-blur md:static md:rounded-xl md:border md:bg-card md:p-4 md:shadow-sm">
      <div class="grid gap-3 md:flex md:items-center md:justify-between">
        <div class="hidden md:block">
          <div class="font-semibold">发布前检查</div>
          <p class="mt-1 text-sm text-muted-foreground">确认商家身份、美元额度售价、接入说明和商户承诺后发布；已绑定 linux.do 的 owner 会自动通过发布资格校验。</p>
        </div>
        <div class="grid gap-2 md:flex md:shrink-0">
          <Button :disabled="publishMutation.isPending.value || !canSubmit" @click="publishService"><Send class="h-4 w-4" />{{ publishMutation.isPending.value ? '发布中' : '发布 API 服务' }}</Button>
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
