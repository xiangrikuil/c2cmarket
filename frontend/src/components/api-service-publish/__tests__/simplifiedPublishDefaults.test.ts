import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import {
  apiQuotaBoundaryNotice,
  applySimplifiedApiQuotaDefaults,
  createDefaultPaymentOptions,
  defaultPaymentWindowMinutes,
  generatedTitle,
  merchantNoteTemplate,
} from '../utils.ts'
import type { ApiServicePublishForm } from '../types.ts'
import { compactApiServiceModels } from '../../api-market/apiFreeServiceCard'
import { beijingDateTimeInputToISOString } from '@/lib/apiQuotaExpiration'

test('applies simplified API quota publish defaults', () => {
  const form: ApiServicePublishForm = {
    probeConnectionId: 'probe-connection-1',
    merchantIdentityMode: 'store_alias',
    merchantDisplayName: '小葵 API',
    distributionSystem: 'other',
    distributionSystemNote: 'NewAPI 自建中转',
    providerCategory: 'gpt',
    billingMode: 'fixed_package',
    deliveryModes: ['api_key_endpoint'],
    shortDescription: '旧短句',
    cnyPerUsdCredit: 0.8,
    quotaUsagePolicy: {
      fiveHour: { mode: 'unlimited' },
      daily: { mode: 'unlimited' },
    },
    manualBillingNote: '旧计费说明',
    defaultMultiplier: 2,
    selectedModels: [{ modelId: 'gpt-5-mini', enabled: true }],
    imageCapability: {
      enabled: true,
      supportsTextToImage: true,
      supportsImageToImage: true,
      pricingMode: 'custom_multiplier',
      customMultiplier: 2,
      note: '旧图像能力',
    },
    availableCreditUsd: 500,
    quotaExpiresAt: '2026-07-10T00:00',
    minimumPurchaseCny: null,
    maximumPurchaseCny: null,
    paymentWindowMinutes: defaultPaymentWindowMinutes,
    paymentOptions: createDefaultPaymentOptions(),
    packages: [{
      id: 'pkg',
      name: '旧套餐',
      priceCny: 50,
      panelAllowance: 20,
      quotaUsagePolicy: {
        fiveHour: { mode: 'limited', amountUsd: '5' },
        daily: { mode: 'limited', amountUsd: '20' },
      },
      durationDays: 30,
      stockTotal: 1,
      description: '旧套餐',
      enabled: true,
      modelCatalogIds: ['gpt-5-mini'],
    }],
    validity: {
      mode: 'permanent',
      days: null,
      startsAt: 'delivered_at',
    },
    usageVisibility: 'fixed_package_only',
	accountPoolType: 'gpt_pro_5x',
	accountPoolCustomName: '',
    warranty: {
	  mode: 'merchant_full_refund',
    },
    merchantNote: merchantNoteTemplate,
  }

  applySimplifiedApiQuotaDefaults(form)

  assert.equal(form.distributionSystem, 'other')
  assert.equal(form.distributionSystemNote, 'NewAPI 自建中转')
  assert.equal(form.billingMode, 'fixed_package')
  assert.deepEqual(form.deliveryModes, ['api_key_endpoint'])
  assert.equal(form.usageVisibility, 'fixed_package_only')
  assert.equal(form.defaultMultiplier, 2)
  assert.equal(form.minimumPurchaseCny, null)
  assert.equal(form.maximumPurchaseCny, null)
  assert.equal(form.paymentWindowMinutes, 10)
  assert.deepEqual(form.paymentOptions.map(item => item.paymentMethod), ['wechat', 'alipay'])
  assert.equal(form.paymentOptions.some(item => item.enabled), false)
  assert.equal(form.paymentOptions.every(item => item.paymentQrCodeDataUrl === null), true)
  assert.equal(form.quotaExpiresAt, '2026-07-10T00:00')
	assert.equal(form.warranty.mode, 'merchant_full_refund')
	assert.equal(form.accountPoolType, 'gpt_pro_5x')
  assert.equal(form.imageCapability.enabled, false)
  assert.equal(form.packages[0].id, 'pkg')
  assert.deepEqual(form.packages[0].modelCatalogIds, ['gpt-5-mini'])
  assert.equal(form.manualBillingNote, '')
  assert.equal(generatedTitle(form, new Map()), 'GPT · 短期流量包')

  form.billingMode = 'metered_credit'
  assert.equal(generatedTitle(form, new Map()), 'GPT · 其他 API 接入 自选额度')

  assert.doesNotMatch(merchantNoteTemplate, new RegExp('接入' + '方式：'))
  assert.match(apiQuotaBoundaryNotice, /不托管支付/)
  assert.match(apiQuotaBoundaryNotice, /不保存 API Key/)
})

test('converts Beijing quota expiration input to a backend timestamp', () => {
  assert.equal(beijingDateTimeInputToISOString('2026-07-10T00:00'), '2026-07-09T16:00:00.000Z')
  assert.equal(beijingDateTimeInputToISOString('  '), '')
  assert.equal(beijingDateTimeInputToISOString('invalid'), '')
})

test('locks API publish merchant display name to profile data', () => {
  const pageSource = readFileSync(new URL('../../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')
  const rushPageSource = readFileSync(new URL('../../../pages/ApiQuotaRushPublishPage.vue', import.meta.url), 'utf8')
  const identitySectionSource = readFileSync(new URL('../MerchantIdentitySection.vue', import.meta.url), 'utf8')

  assert.match(pageSource, /useMyProfileQuery/)
  assert.match(pageSource, /merchantIdentityMode: 'public_profile'/)
  assert.match(rushPageSource, /merchantIdentityMode: 'public_profile'/)
  assert.match(pageSource, /form\.merchantDisplayName = profileMerchantDisplayName\.value/)
  assert.match(pageSource, /发布必填 \{\{ publishAssistant\.doneCount \}\} \/ \{\{ publishAssistant\.totalCount \}\}/)
  assert.match(pageSource, /v-model:open="previewOpen"/)
  assert.match(pageSource, /preview-only/)
  assert.match(identitySectionSource, /默认公开个人身份/)
  assert.match(identitySectionSource, /隐藏社区身份，仅展示商家展示名/)
  assert.doesNotMatch(pageSource, /v-model="form\.merchantDisplayName"/)
  assert.doesNotMatch(pageSource, /placeholder="例如：小葵 API"/)
  assert.doesNotMatch(pageSource, /预览标题：/)
})

test('publishes from the third configuration step without a duplicate confirmation step', () => {
  const pageSource = readFileSync(new URL('../../../pages/ApiServicePublishPage.vue', import.meta.url), 'utf8')

  assert.match(pageSource, /type ApiServicePublishStep = 1 \| 2 \| 3/)
  assert.match(pageSource, /currentStep\.value === 3[\s\S]*?publishService\(\)/)
  assert.match(pageSource, /currentStep\.value === 2\) return '继续：交易与服务'[\s\S]*?sellingModeLabels\.package[\s\S]*?sellingModeLabels\.free/)
  assert.match(pageSource, /currentStep\.value < 3/)
  assert.match(pageSource, /发布必填 \{\{ publishAssistant\.doneCount \}\} \/ \{\{ publishAssistant\.totalCount \}\}/)
  assert.match(pageSource, /<ApiServicePublishPreview[\s\S]*?preview-only/)
  assert.match(pageSource, /currentStep === 3[\s\S]*?min-\[1241px\]:hidden[\s\S]*?@click="preview"/)
  assert.match(pageSource, /class="hidden min-\[1241px\]:inline-flex"[\s\S]*?@click="preview"/)
  assert.doesNotMatch(pageSource, /title: '确认发布'|title="确认发布"|:step="4"|publishStepStatus\(4|publicationReviewRows|发布信息确认清单/)
})

test('compacts one, two, and many model labels for the market card', () => {
  assert.deepEqual(compactApiServiceModels(['gpt-5-mini']), {
    visibleModels: ['gpt-5-mini'],
    hiddenModelCount: 0,
  })
  assert.deepEqual(compactApiServiceModels(['gpt-5-mini', 'gpt-5']), {
    visibleModels: ['gpt-5-mini', 'gpt-5'],
    hiddenModelCount: 0,
  })
  assert.deepEqual(compactApiServiceModels(['gpt-5-mini', 'gpt-5', 'gpt-4.1', 'o3']), {
    visibleModels: ['gpt-5-mini', 'gpt-5'],
    hiddenModelCount: 2,
  })
})
