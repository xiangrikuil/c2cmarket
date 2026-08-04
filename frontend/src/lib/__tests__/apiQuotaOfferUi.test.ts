import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'vitest'
import { apiQuotaOffers } from '../../data/mock'
import { BackendProblemError } from '../backendClient'
import {
  apiMarketViewFromQuery,
  apiQuotaDurationLabel,
  apiQuotaOfferCountdown,
  apiQuotaOfferErrorMessage,
  withApiMarketViewQuery,
} from '../apiQuotaOfferUi'

const marketPageSource = readFileSync(new URL('../../pages/ApiMarketPage.vue', import.meta.url), 'utf8')
const quotaOfferCardSource = readFileSync(new URL('../../components/api-market/ApiQuotaOfferCard.vue', import.meta.url), 'utf8')
const quotaPolicyStripSource = readFileSync(new URL('../../components/api-market/ApiQuotaPolicyStrip.vue', import.meta.url), 'utf8')
const freeServiceCardSource = readFileSync(new URL('../../components/api-market/ApiFreeServiceCard.vue', import.meta.url), 'utf8')
const myApiServicesPageSource = readFileSync(new URL('../../pages/MyApiServicesPage.vue', import.meta.url), 'utf8')
const myCenterPageSource = readFileSync(new URL('../../pages/MyCenterPage.vue', import.meta.url), 'utf8')
const quotaOwnerManagerSource = readFileSync(new URL('../../components/api-quota/ApiQuotaOwnerManager.vue', import.meta.url), 'utf8')
const apiServicePublishPageSource = readFileSync(new URL('../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')
const quotaRushPublishPageSource = readFileSync(new URL('../../pages/ApiQuotaRushPublishPage.vue', import.meta.url), 'utf8')
const sellingModeSelectorSource = readFileSync(new URL('../../components/api-service-publish/SellingModeSelector.vue', import.meta.url), 'utf8')
const fixedPackageSectionSource = readFileSync(new URL('../../components/api-service-publish/FixedPackageSection.vue', import.meta.url), 'utf8')
const publishPreviewSource = readFileSync(new URL('../../components/api-service-publish/ApiServicePublishPreview.vue', import.meta.url), 'utf8')
const selectedModelsPricingTableSource = readFileSync(new URL('../../components/api-service-publish/SelectedModelsPricingTable.vue', import.meta.url), 'utf8')
const quotaRushPublishPreviewSource = readFileSync(new URL('../../components/api-quota/ApiQuotaRushPublishPreview.vue', import.meta.url), 'utf8')
const publishWorkflowStepperSource = readFileSync(new URL('../../components/api-service-publish/PublishWorkflowStepper.vue', import.meta.url), 'utf8')
const publishStepSectionSource = readFileSync(new URL('../../components/api-service-publish/PublishStepSection.vue', import.meta.url), 'utf8')
const responsivePublishPreviewSource = readFileSync(new URL('../../components/api-service-publish/ResponsivePublishPreview.vue', import.meta.url), 'utf8')
const apiAccessSourceSectionSource = readFileSync(new URL('../../components/api-service-publish/ApiAccessSourceSection.vue', import.meta.url), 'utf8')
const accountPaymentSummarySource = readFileSync(new URL('../../components/api-service-publish/AccountPaymentSummarySection.vue', import.meta.url), 'utf8')
const paymentSettingsEditorSource = readFileSync(new URL('../../components/contact-payment/ApiPaymentSettingsEditor.vue', import.meta.url), 'utf8')
const paymentSettingsDialogSource = readFileSync(new URL('../../components/contact-payment/ApiPaymentSettingsDialog.vue', import.meta.url), 'utf8')
const paymentMethodCardSource = readFileSync(new URL('../../components/contact-payment/PaymentMethodCard.vue', import.meta.url), 'utf8')
const providerCategorySelectorSource = readFileSync(new URL('../../components/api-service-publish/ProviderCategorySelector.vue', import.meta.url), 'utf8')
const apiPaymentMethodIconSource = readFileSync(new URL('../../components/api-payment/ApiPaymentMethodIcon.vue', import.meta.url), 'utf8')
const apiPurchasePanelSource = readFileSync(new URL('../../components/api-service-detail/ApiPurchasePanel.vue', import.meta.url), 'utf8')
const apiServiceHeaderSource = readFileSync(new URL('../../components/api-service-detail/ApiServiceHeader.vue', import.meta.url), 'utf8')
const apiServiceOwnerOverviewSource = readFileSync(new URL('../../components/api-service-owner/ApiServiceOwnerOverview.vue', import.meta.url), 'utf8')
const apiOrderDetailSource = readFileSync(new URL('../../pages/ApiPurchaseOrderDetailPage.vue', import.meta.url), 'utf8')
const myApiServiceDetailSource = readFileSync(new URL('../../pages/MyApiServiceDetailPage.vue', import.meta.url), 'utf8')
const myApiOrdersPageSource = readFileSync(new URL('../../pages/MyApiOrdersPage.vue', import.meta.url), 'utf8')
const merchantApiOrdersPageSource = readFileSync(new URL('../../pages/MerchantApiOrdersPage.vue', import.meta.url), 'utf8')
const apiFacadeSource = readFileSync(new URL('../api.ts', import.meta.url), 'utf8')
const apiMarketBackendSource = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')
const wechatMarkSource = readFileSync(new URL('../../../public/wechat-mark.svg', import.meta.url), 'utf8')
const alipayMarkSource = readFileSync(new URL('../../../public/alipay-mark.svg', import.meta.url), 'utf8')

describe('API 额度包市场视图', () => {
  test('默认进入限时额度包并把同级视图写回 URL', () => {
    assert.equal(apiMarketViewFromQuery(undefined), 'limited')
    assert.equal(apiMarketViewFromQuery('unknown'), 'limited')
    assert.equal(apiMarketViewFromQuery('packages'), 'packages')
    assert.equal(apiMarketViewFromQuery('free'), 'free')
    assert.deepEqual(withApiMarketViewQuery({ category: 'api' }, 'free'), { category: 'api', view: 'free' })
    assert.match(marketPageSource, /<TabsTrigger value="limited">限时额度包<\/TabsTrigger>/)
    assert.match(marketPageSource, /<TabsTrigger value="packages">限时流量包<\/TabsTrigger>/)
    assert.match(marketPageSource, /<TabsTrigger value="free">自由额度<\/TabsTrigger>/)
    assert.match(marketPageSource, /router\.replace\(\{ query: withApiMarketViewQuery\(route\.query, value\) \}\)/)
  })

  test('市场入口进入可复用已有服务的固定场次发布向导', () => {
    assert.match(marketPageSource, /<RouterLink to="\/api-market\/quota\/new">[\s\S]*?发布限时额度包/)
    assert.match(marketPageSource, /卖家也可以发布自己的限时额度包/)
    assert.match(myApiServicesPageSource, /quotaPublishIntent \? '选择 API 服务'/)
    assert.match(myApiServicesPageSource, /\/api-market\/quota\/new/)
    assert.match(myApiServicesPageSource, /选择并发布额度包/)
    assert.match(myApiServicesPageSource, /`\/api-market\/quota\/new\?serviceId=\$\{item\.id\}`/)
    assert.match(quotaRushPublishPageSource, /useMyApiServices[\s\S]*?ApiAccessSourceSection[\s\S]*?AccountPaymentSummarySection[\s\S]*?ModelMultiSelect/)
    assert.match(quotaOwnerManagerSource, /<Card id="quota-offers"/)
    assert.match(quotaOwnerManagerSource, /销售管理/)
    assert.match(quotaOwnerManagerSource, /<TabsTrigger value="batches"[\s\S]*?额度批次/)
    assert.match(quotaOwnerManagerSource, /<TabsTrigger value="offers"[\s\S]*?销售规格/)
    assert.match(quotaOwnerManagerSource, /<TabsTrigger value="rounds"[\s\S]*?放量计划/)
    assert.match(quotaOwnerManagerSource, /<TabsTrigger value="credentials"[\s\S]*?交付凭据/)
    assert.match(quotaOwnerManagerSource, /copyOfferRoute\(offer\)[\s\S]*?复制再发/)
    assert.match(quotaRushPublishPageSource, /route\.query\.copy === '1'[\s\S]*?copiedQueryValue\('deliveryEtaMinutes'\)/)
    assert.doesNotMatch(quotaRushPublishPageSource, /copiedQueryValue\('(copies|slotKey|expiresAt)'\)/)
  })

  test('限时包先显示紧凑服务列表并把新建服务降为次要动作', () => {
    assert.match(quotaRushPublishPageSource, /选择要发布额度的 API 服务/)
    assert.match(quotaRushPublishPageSource, /我的 API 服务/)
    assert.match(quotaRushPublishPageSource, /新建 API 服务/)
    assert.match(quotaRushPublishPageSource, /返回选择服务/)
    assert.match(quotaRushPublishPageSource, /当前服务/)
    assert.match(quotaRushPublishPageSource, /服务编号 \{\{ serviceShortId\(service\.id\) \}\}/)
    assert.match(quotaRushPublishPageSource, /max-h-72/)
    assert.doesNotMatch(quotaRushPublishPageSource, /<Tabs v-model="serviceMode"|选择已有服务|创建基础服务/)
  })

  test('发布页先选择三种同级销售模式', () => {
    assert.match(apiServicePublishPageSource, /apiPublishModeFromQuery\(route\.query\.mode, route\.query\.after\)/)
    assert.match(sellingModeSelectorSource, /value: 'free'[\s\S]*?title: '自由额度'/)
    assert.match(sellingModeSelectorSource, /value: 'package'[\s\S]*?title: '固定额度包'/)
    assert.match(sellingModeSelectorSource, /value: 'limited'[\s\S]*?title: '限时额度包'/)
    assert.match(apiServicePublishPageSource, /<template v-if="!sellingMode">[\s\S]*?<SellingModeSelector @select="chooseSellingMode"[\s\S]*?<template v-else>/)
    assert.match(apiServicePublishPageSource, /<FixedPackageSection v-else-if="isFixedPackageMode"[\s\S]*?<PriceInventorySection v-else/)
    assert.doesNotMatch(apiServicePublishPageSource, /BillingModeSection/)
    assert.doesNotMatch(fixedPackageSectionSource, /RadioGroup|计费方式|限时流量包/)
    assert.match(apiServicePublishPageSource, /formDirty\.value && !window\.confirm\('API 服务配置尚未发布，确认返回选择销售模式？'\)/)
    assert.match(apiServicePublishPageSource, /const \{ mode: _mode, after: _after, \.\.\.query \} = route\.query/)
    assert.match(apiServicePublishPageSource, /保存基础服务，下一步设置额度包/)
    assert.match(apiServicePublishPageSource, /发布固定额度包/)
    assert.match(apiServicePublishPageSource, /发布自由额度服务/)
    assert.match(apiServicePublishPageSource, /isLimitedQuotaMode[\s\S]*?`\/api-market\/quota\/new\?serviceId=\$\{service\.id\}`[\s\S]*?: '\/my\/api-services'/)
  })

  test('固定额度包第一步不重复选择模型并明确美元面板额度', () => {
    assert.doesNotMatch(fixedPackageSectionSource, /支持模型|api-publish-model-chip|catalogById/)
    assert.match(fixedPackageSectionSource, /面板额度（USD）[\s\S]*?>\$<\/span>[\s\S]*?selectedPackage\.panelAllowance/)
    assert.match(apiServicePublishPageSource, /item\.modelCatalogIds = \[\.\.\.enabledModelIds\]/)
  })

  test('固定额度包使用有界摘要列表并只编辑当前套餐', () => {
    assert.match(fixedPackageSectionSource, /max-h-\[284px\][\s\S]*?overflow-y-auto/)
    assert.match(fixedPackageSectionSource, /const selectedPackageId = ref[\s\S]*?const selectedPackage = computed/)
    assert.match(fixedPackageSectionSource, /props\.form\.packages\.push\(item\)[\s\S]*?selectedPackageId\.value = item\.id/)
    assert.match(fixedPackageSectionSource, /v-if="selectedPackage"[\s\S]*?v-model="selectedPackage\.name"/)
    assert.match(fixedPackageSectionSource, /draggable="true"[\s\S]*?<GripVertical/)
    assert.match(fixedPackageSectionSource, /class="sm:hidden"[\s\S]*?<ArrowUp/)
    assert.match(fixedPackageSectionSource, /<Package[\s\S]*?<Clock3[\s\S]*?<Boxes[\s\S]*?<Pencil[\s\S]*?<Trash2/)
    assert.equal(fixedPackageSectionSource.match(/v-for="\(item, index\) in form\.packages"/g)?.length, 1)
  })

  test('所有模型与额度包统一继承服务默认倍率', () => {
    assert.match(publishPreviewSource, /服务倍率[\s\S]*?formatMultiplier\(props\.form\.defaultMultiplier\)/)
    assert.match(apiAccessSourceSectionSource, /模型、固定额度包和限时额度包统一继承该倍率/)
    assert.doesNotMatch(selectedModelsPricingTableSource, /multiplierOverride|setMultiplier|服务倍率/)
    assert.doesNotMatch(quotaRushPublishPageSource, /rush\.modelMultiplier|copiedQueryValue\('modelMultiplier'\)/)
    assert.match(quotaRushPublishPageSource, /modelMultiplier: serviceDefaultMultiplierDecimal\.value/)
    assert.match(quotaRushPublishPreviewSource, /服务倍率[\s\S]*?defaultMultiplier\.toFixed\(2\)/)
    assert.doesNotMatch(quotaOwnerManagerSource, /offerForm\.modelMultiplier|modelMultiplier: offer\.modelMultiplier/)
    assert.match(quotaOwnerManagerSource, /modelMultiplier: serviceDefaultMultiplierDecimal\.value/)
    assert.match(myApiServiceDetailSource, /:default-multiplier="service\.defaultMultiplier"/)
    assert.doesNotMatch(apiFacadeSource, /multiplierOverride/)
    assert.doesNotMatch(apiMarketBackendSource, /multiplierOverride/)
  })

  test('限时额度包预览披露体验与真实买家流程', () => {
    assert.match(publishPreviewSource, /<ApiQuotaPolicyStrip[\s\S]*?:policy="previewPackage\.quotaUsagePolicy"/)
    assert.match(apiServicePublishPageSource, /选择额度包 → 创建订单 → 站外付款 → 卖家确认收款 → 获取交付凭证/)
    assert.match(publishPreviewSource, /卖家确认收款后交付；平台记录订单，不代收款/)

    const touchedCopy = [apiServicePublishPageSource, sellingModeSelectorSource, publishPreviewSource].join('\n')
    assert.doesNotMatch(touchedCopy, /自动发货|平台担保|资金安全|安全可靠|获取 API Key/)
  })

  test('两类发布页共享渐进步骤、响应式预览与常驻主操作', () => {
    assert.match(apiServicePublishPageSource, /<h1 class="text-xl font-semibold">发布 API 额度<\/h1>/)
    assert.match(apiServicePublishPageSource, /发布必填 \{\{ publishAssistant\.doneCount \}\} \/ \{\{ publishAssistant\.totalCount \}\}/)
    assert.match(sellingModeSelectorSource, /max-w-5xl/)
    assert.match(sellingModeSelectorSource, /md:grid-cols-3/)
    assert.match(apiServicePublishPageSource, /<div class="api-publish-layout[\s\S]*?<section class="api-publish-editor[\s\S]*?<ApiServicePublishPreview/)
    assert.match(apiServicePublishPageSource, /const currentStep = ref<ApiServicePublishStep>[\s\S]*?const completedSteps = ref<ApiServicePublishStep\[\]>/)
    assert.match(apiServicePublishPageSource, /<PublishWorkflowStepper[\s\S]*?<PublishStepSection[\s\S]*?<ResponsivePublishPreview/)
    assert.match(quotaRushPublishPageSource, /const step = ref\(1\)[\s\S]*?const completedSteps = ref<number\[\]>\(\[\]\)/)
    assert.match(quotaRushPublishPageSource, /<PublishWorkflowStepper[\s\S]*?<PublishStepSection[\s\S]*?<ResponsivePublishPreview/)
    assert.match(publishWorkflowStepperSource, /<StepperSeparator[\s\S]*?h-0\.5[\s\S]*?group-data-\[state=completed\]:bg-primary\/60/)
    assert.match(publishStepSectionSource, /v-if="status === 'completed'"[\s\S]*?修改[\s\S]*?v-if="status === 'active'"/)
    assert.match(responsivePublishPreviewSource, /v-if="desktopPreview"[\s\S]*?<Dialog v-else v-model:open="open"/)
    assert.match(apiServicePublishPageSource, /class="sticky bottom-0/)
    assert.match(quotaRushPublishPageSource, /class="sticky bottom-0/)
    assert.doesNotMatch(apiServicePublishPageSource, /md:static/)
    assert.match(apiServicePublishPageSource, /PackageCheck[\s\S]*?Bot[\s\S]*?<MerchantIdentitySection/)
  })

  test('API 额度链路使用支付与模型品牌图标', () => {
    assert.match(wechatMarkSource, /fill="#07C160"[\s\S]*?<title>WeChat<\/title>/)
    assert.match(alipayMarkSource, /fill="#1677FF"[\s\S]*?<title>Alipay<\/title>/)
    assert.match(apiPaymentMethodIconSource, /apiPaymentMethodIconSrc\[method\]/)
    assert.match(accountPaymentSummarySource, /<ApiPaymentMethodIcon :method="enabledOption\.paymentMethod"/)
    assert.match(myCenterPageSource, /<ApiPaymentSettingsEditor/)
    assert.match(paymentSettingsEditorSource, /<PaymentMethodCard/)
    assert.match(paymentMethodCardSource, /<ApiPaymentMethodIcon :method="option\.paymentMethod"/)
    assert.match(publishPreviewSource, /<ApiPaymentMethodIcon v-for="method in paymentMethods"/)
    assert.match(apiPurchasePanelSource, /支持付款[\s\S]*?<ApiPaymentMethodIcon :method="method"/)
    assert.match(apiOrderDetailSource, /<ApiPaymentMethodIcon :method="order\.selectedPaymentMethod"/)
    assert.match(myApiServiceDetailSource, /<ApiServiceOwnerOverview :service="service"/)
    assert.match(apiServiceOwnerOverviewSource, /service\.acceptedPaymentMethods[\s\S]*?<ApiPaymentMethodIcon :method="method"/)
    assert.match(myApiOrdersPageSource, /<ApiPaymentMethodIcon :method="item\.selectedPaymentMethod"/)
    assert.match(merchantApiOrdersPageSource, /<ApiPaymentMethodIcon :method="item\.selectedPaymentMethod"/)

    assert.match(providerCategorySelectorSource, /getProductCategoryIconSrc/)
    assert.match(apiServiceHeaderSource, /<img v-if="iconSrc"/)
    assert.match(marketPageSource, /getApiServiceProductIconSrc[\s\S]*?getProductIconSrc/)
    assert.match(myApiServicesPageSource, /getApiServiceProductIconSrc[\s\S]*?<img v-if="serviceIconSrc\(item\)"/)
    assert.match(myApiOrdersPageSource, /getApiServiceProductIconSrc[\s\S]*?<img v-if="orderProductIconSrc\(item\)"/)
  })

  test('API 发布页内复用账户收款编辑器并保留当前发布流程', () => {
    assert.doesNotMatch(accountPaymentSummarySource, /RouterLink|\/my\/contacts/)
    assert.match(accountPaymentSummarySource, /emit\('edit'\)/)
    assert.match(apiServicePublishPageSource, /<ApiPaymentSettingsDialog[\s\S]*?v-model:open="paymentSettingsDialogOpen"/)
    assert.equal(apiServicePublishPageSource.match(/<ApiPaymentSettingsDialog/g)?.length, 1)
    assert.match(quotaRushPublishPageSource, /<AccountPaymentSummarySection[\s\S]*?@edit="paymentSettingsDialogOpen = true"/)
    assert.match(quotaRushPublishPageSource, /<ApiPaymentSettingsDialog[\s\S]*?v-model:open="paymentSettingsDialogOpen"/)
    assert.match(paymentSettingsDialogSource, /discardConfirmationOpen[\s\S]*?放弃未保存的修改/)
    assert.match(paymentSettingsDialogSource, /handleSaved[\s\S]*?emit\('update:open', false\)/)
    assert.match(paymentSettingsEditorSource, /useUpdateApiPaymentAccountSettingsMutation/)
    assert.match(paymentSettingsEditorSource, /<RadioGroup v-model="selectedPaymentMethod"/)
    assert.doesNotMatch(paymentMethodCardSource, /<Checkbox/)
    assert.match(paymentSettingsEditorSource, /containsSensitiveContent/)
    assert.match(apiServicePublishPageSource, /watch\(accountPaymentSettingsValue,[\s\S]*?form\.paymentOptions = settings\.paymentOptions\.map/)
    assert.match(quotaRushPublishPageSource, /accountSettingsComplete[\s\S]*?paymentSettingsDialogOpen\.value = true/)
    assert.match(quotaRushPublishPageSource, /baseErrors\.paymentOptions = '请先设置收款方式。'[\s\S]*?paymentSettingsDialogOpen\.value = true/)
    assert.match(quotaRushPublishPageSource, /@saved="handlePaymentSettingsSaved"/)
  })

  test('限时与自由额度卡按产品分类复用弱主题并保留统一购买操作', () => {
    assert.match(marketPageSource, /function quotaOfferCategory\(item: PublicApiQuotaOffer\)[\s\S]*?getProductCategory/)
    assert.match(marketPageSource, /function freeServiceCategory\(service: ApiService\)[\s\S]*?getApiServiceProductCategory/)
    assert.equal(marketPageSource.match(/<ApiQuotaOfferCard/g)?.length, 2)
    assert.equal(marketPageSource.match(/:category="quotaOfferCategory\(item\)"/g)?.length, 2)
    assert.equal(quotaOfferCardSource.match(/class="quota-offer-card/g)?.length, 1)
    assert.match(quotaOfferCardSource, /<ApiQuotaPolicyStrip[\s\S]*?:policy="offer\.quotaUsagePolicy"/)
    assert.match(quotaPolicyStripSource, /\.api-quota-policy-strip dd \{[\s\S]*?overflow-wrap: anywhere;[\s\S]*?white-space: normal;/)
    assert.doesNotMatch(quotaPolicyStripSource, /text-overflow: ellipsis/)
    assert.match(
      marketPageSource,
      /<ApiFreeServiceCard[\s\S]*?:card="freeServiceCard\(entry\.service\)"[\s\S]*?:promoted="Boolean\(entry\.promotion\)"/,
    )
    assert.match(marketPageSource, /sellerReputation: service\.sellerReputation/)
    assert.match(freeServiceCardSource, /:data-category="card\.category"/)
    assert.match(freeServiceCardSource, /api-free-service-card__watermark/)
    assert.match(freeServiceCardSource, /api-free-service-card__icon/)
    assert.match(freeServiceCardSource, /api-free-service-card__price/)
    assert.match(freeServiceCardSource, /可售 \$\{\{ formatDecimal\(card\.availableUsdAllowance/)
    assert.doesNotMatch(marketPageSource, /价格、额度与性能由商户声明，平台未测速/)
    assert.match(marketPageSource, /平台测量只代表当前探测模型与平台节点/)
    assert.match(marketPageSource, /class="quota-free-grid"/)
    assert.match(marketPageSource, /grid-template-columns: repeat\(auto-fit, minmax\(min\(100%, 330px\), 1fr\)\)/)
    assert.match(marketPageSource, /max-width: 1640px/)
    assert.match(marketPageSource, /margin-inline: auto/)
    assert.match(freeServiceCardSource, /\.api-free-service-card \{[\s\S]*?min-height: 640px/)
    assert.match(freeServiceCardSource, /Megaphone[\s\S]*?推广/)
    assert.doesNotMatch(freeServiceCardSource, /商业推广，不代表平台质量认证或信誉背书/)
    assert.match(freeServiceCardSource, /card\.accountPoolLabel/)
    assert.match(freeServiceCardSource, /商户全额退款承诺/)
    assert.match(freeServiceCardSource, /:compact="card\.sellerReputation\?\.state === 'active'"/)
    assert.match(freeServiceCardSource, /card\.delivery \}\} · \{\{ modelCountLabel/)
    assert.match(freeServiceCardSource, /compactModels\.visibleModels/)
    assert.match(freeServiceCardSource, /\+\{\{ compactModels\.hiddenModelCount \}\}/)
    assert.match(freeServiceCardSource, /预览状态，不可操作/)
    assert.match(publishPreviewSource, /<ApiFreeServiceCard v-if="isFreeQuotaMode" :card="freeServiceCard" preview/)
    assert.doesNotMatch(marketPageSource, /SourceAuthorVerificationBadge/)
    assert.doesNotMatch(marketPageSource, /可创建订单/)
    for (const category of ['gpt', 'claude', 'gemini', 'cursor', 'perplexity', 'other']) {
      assert.match(freeServiceCardSource, new RegExp(`data-category='${category}'`))
    }
    assert.match(quotaOfferCardSource, /class="h-10 w-full" @click="emit\('purchase', offer\)"/)
    assert.match(freeServiceCardSource, /<RouterLink v-else-if="card\.actionHref"/)
    assert.match(freeServiceCardSource, /aria-disabled="true"/)
    assert.doesNotMatch(marketPageSource, /自动交付|安全可靠|平台担保|虚构原价/)
  })

  test('账号完善后保留限时额度包发布意图', () => {
    assert.match(myCenterPageSource, /accountRecoveryReturnTo\.value === '\/api-market\/quota\/new'/)
    assert.match(myCenterPageSource, /发布限时额度包前先完成账号设置/)
    assert.match(myCenterPageSource, /继续发布限时额度包/)
  })

  test('固定场次使用服务端时钟并直接创建额度包订单', () => {
    assert.match(marketPageSource, /今日限时抢/)
    assert.match(marketPageSource, /serverClockOffset/)
    assert.match(marketPageSource, /slotQuery\.data\.value\?\.items/)
    assert.match(marketPageSource, /refreshAtSlotBoundary/)
    assert.match(marketPageSource, /useApiQuotaOffers\(rushFilters\)/)
    assert.match(marketPageSource, /`明日 \$\{slotTime\(selectedSlot\)\} 场预告`/)
    assert.match(quotaOfferCardSource, /立即抢购 ¥\$\{formatDecimal\(props\.offer\.priceCny/)
    assert.doesNotMatch(marketPageSource, /selectedOffer|confirmPurchase|<Dialog/)
  })

  test('订单详情只展示购买时冻结的额度规则与到期语义', () => {
    assert.match(apiOrderDetailSource, /购买时额度规则/)
    assert.match(apiOrderDetailSource, /:policy="order\.quotaUsagePolicySnapshot"/)
    assert.match(apiOrderDetailSource, /order\.value\.quotaSnapshot[\s\S]*?quotaSnapshot\.expiresAt/)
    assert.match(apiOrderDetailSource, /order\.value\.packageExpiresAt[\s\S]*?order\.value\.packageSnapshot[\s\S]*?`交付后 \$\{order\.value\.packageSnapshot\.durationDays\} 天`/)
    assert.match(apiOrderDetailSource, /serviceValiditySnapshotLabel\(order\.value\.intentSnapshot\)/)
  })

  test('三步向导只接受开放场次并支持条件凭据 CSV', () => {
    assert.match(quotaRushPublishPageSource, /slot\.state === 'registration_open'/)
    assert.match(quotaRushPublishPageSource, /useCreateApiQuotaRushOfferMutation/)
    assert.match(quotaRushPublishPageSource, /deliveryMode === 'preimported'/)
    assert.match(quotaRushPublishPageSource, /凭据数量至少需要/)
    assert.match(quotaRushPublishPageSource, /24 小时后[\s\S]*?3 天后[\s\S]*?7 天后/)
    assert.match(quotaRushPublishPageSource, /watch\(selectedSlot,[\s\S]*?\}, \{ immediate: true \}\)/)
  })

  test('限时额度发布页区分资料失败并严格匹配显式服务 ID', () => {
    assert.match(quotaRushPublishPageSource, /isLoading: profileLoading[\s\S]*?isError: profileIsError[\s\S]*?isSuccess: profileIsSuccess[\s\S]*?error: profileError[\s\S]*?refetch: refetchProfile/)
    assert.match(quotaRushPublishPageSource, /profileIsError\.value \|\| !profileIsSuccess\.value/)
    assert.match(quotaRushPublishPageSource, /个人资料加载失败/)
    assert.match(quotaRushPublishPageSource, /requestedServiceUnavailable/)
    assert.match(quotaRushPublishPageSource, /if \(requestedId\) \{[\s\S]*?selectedServiceId\.value = rows\.some[\s\S]*?serviceMode\.value = 'existing'[\s\S]*?return/)
    assert.match(quotaRushPublishPageSource, /不会自动改用其他服务/)
  })

  test('倒计时覆盖未开始、当前轮、持续销售和结束边界', () => {
    const base = structuredClone(apiQuotaOffers[0]!)
    const now = Date.parse('2026-07-19T01:00:00.000Z')
    assert.equal(apiQuotaDurationLabel('2026-07-19T01:00:00.000Z', now), '已结束')
    assert.equal(apiQuotaDurationLabel('2026-07-19T01:00:30.000Z', now), '1 分钟')
    assert.equal(apiQuotaDurationLabel('2026-07-19T03:05:00.000Z', now), '2 小时 5 分')

    assert.equal(apiQuotaOfferCountdown({
      ...base,
      isOrderable: false,
      nextRound: { ...base.currentRound!, startsAt: '2026-07-19T02:00:00.000Z' },
    }, now), '1 小时 0 分 后开售')
    assert.equal(apiQuotaOfferCountdown({
      ...base,
      currentRound: { ...base.nextRound!, endsAt: '2026-07-19T01:20:00.000Z' },
      isOrderable: true,
    }, now), '本轮 20 分钟 后结束')
    assert.match(apiQuotaOfferCountdown({ ...base, currentRound: undefined, nextRound: undefined }, now), /后停售$/)
  })

  test('按 Problem Details code 映射抢购失败原因', () => {
    assert.equal(
      apiQuotaOfferErrorMessage(new BackendProblemError({ code: 'API_QUOTA_SOLD_OUT', detail: 'raw detail' }, 409)),
      '本轮额度包已经售罄。',
    )
    assert.equal(
      apiQuotaOfferErrorMessage(new BackendProblemError({ code: 'API_QUOTA_BUYER_ROUND_LIMIT' }, 409)),
      '你本轮已经抢到过 1 份，取消后也不能再次抢购。',
    )
    assert.equal(
      apiQuotaOfferErrorMessage(new BackendProblemError({ code: 'UNEXPECTED', detail: '服务端详情' }, 409)),
      '服务端详情',
    )
  })
})
