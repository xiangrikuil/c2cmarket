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
import { beijingDateTimeInputToISOString } from '@/lib/apiQuotaExpiration'

test('applies simplified API quota publish defaults', () => {
  const form: ApiServicePublishForm = {
    merchantIdentityMode: 'store_alias',
    merchantDisplayName: '小葵 API',
    distributionSystem: 'other',
    distributionSystemNote: 'NewAPI 自建中转',
    providerCategory: 'gpt',
    billingMode: 'fixed_package',
    deliveryModes: ['api_key_endpoint'],
    shortDescription: '旧短句',
    cnyPerUsdCredit: 0.8,
    manualBillingNote: '旧计费说明',
    defaultMultiplier: 2,
    selectedModels: [{ modelId: 'gpt-5-mini', multiplierOverride: 1.5, enabled: true }],
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
    warranty: {
      mode: 'merchant_warranty',
      warrantyDays: 7,
      coverage: '旧适用范围',
      compensation: '旧补偿方式',
      exclusions: '旧不适用情形',
      refundNote: '旧退款说明',
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
  assert.equal(form.warranty.mode, 'no_warranty')
  assert.equal(form.warranty.warrantyDays, null)
  assert.equal(form.imageCapability.enabled, false)
  assert.equal(form.packages[0].id, 'pkg')
  assert.deepEqual(form.packages[0].modelCatalogIds, ['gpt-5-mini'])
  assert.equal(form.manualBillingNote, '')
  assert.equal(generatedTitle(form, new Map()), 'GPT · API 限时套餐')

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

  assert.match(pageSource, /useMyProfileQuery/)
  assert.match(pageSource, /form\.merchantDisplayName = profileMerchantDisplayName\.value/)
  assert.match(pageSource, /发布必填 \{\{ publishAssistant\.doneCount \}\} \/ \{\{ publishAssistant\.totalCount \}\}/)
  assert.match(pageSource, /v-model:open="previewOpen"/)
  assert.match(pageSource, /preview-only/)
  assert.doesNotMatch(pageSource, /v-model="form\.merchantDisplayName"/)
  assert.doesNotMatch(pageSource, /placeholder="例如：小葵 API"/)
  assert.doesNotMatch(pageSource, /预览标题：/)
})
