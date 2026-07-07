import assert from 'node:assert/strict'
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
    deliveryModes: ['sub2api_panel_account'],
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
    packages: [{ id: 'pkg', name: '旧套餐', priceCny: 50, durationDays: 30, description: '旧套餐', inventory: 1 }],
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

  assert.equal(form.distributionSystem, 'sub2api')
  assert.equal(form.billingMode, 'metered_credit')
  assert.deepEqual(form.deliveryModes, ['api_key_endpoint'])
  assert.equal(form.usageVisibility, 'merchant_confirmed')
  assert.equal(form.defaultMultiplier, 1)
  assert.equal(form.minimumPurchaseCny, 20)
  assert.equal(form.maximumPurchaseCny, 300)
  assert.equal(form.paymentWindowMinutes, 10)
  assert.deepEqual(form.paymentOptions.map(item => item.paymentMethod), ['wechat', 'alipay', 'usdt'])
  assert.equal(form.paymentOptions.some(item => item.enabled), false)
  assert.equal(form.paymentOptions.every(item => item.paymentQrCodeDataUrl === null), true)
  assert.equal(form.quotaExpiresAt, '2026-07-10T00:00')
  assert.equal(form.warranty.mode, 'no_warranty')
  assert.equal(form.warranty.warrantyDays, null)
  assert.equal(form.imageCapability.enabled, false)
  assert.deepEqual(form.packages, [])
  assert.equal(form.manualBillingNote, '')
  assert.equal(generatedTitle(form, new Map()), 'GPT · API 美元额度')

  assert.match(merchantNoteTemplate, /接入方式：/)
  assert.match(apiQuotaBoundaryNotice, /不托管支付/)
  assert.match(apiQuotaBoundaryNotice, /不保存 API Key/)
})

test('converts Beijing quota expiration input to a backend timestamp', () => {
  assert.equal(beijingDateTimeInputToISOString('2026-07-10T00:00'), '2026-07-09T16:00:00.000Z')
  assert.equal(beijingDateTimeInputToISOString('  '), '')
  assert.equal(beijingDateTimeInputToISOString('invalid'), '')
})
