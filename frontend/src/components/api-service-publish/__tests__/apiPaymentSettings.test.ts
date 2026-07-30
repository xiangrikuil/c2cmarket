import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  apiPaymentMethodIconSrc,
  apiPaymentMethods,
  apiPaymentSettingsMissingReason,
  apiPaymentSettingsSummary,
  cloneApiPaymentAccountSettings,
  createEmptyApiPaymentAccountSettings,
  defaultApiPaymentWindowMinutes,
  isApiPaymentAccountSettingsComplete,
  normalizeApiPaymentAccountSettings,
} from '../../../lib/apiPaymentSettings.ts'

test('normalizes and validates API payment account settings', () => {
  const qrDataUrl = 'data:image/png;base64,aGVsbG8='

  const empty = createEmptyApiPaymentAccountSettings()
  assert.equal(empty.paymentWindowMinutes, 10)
  assert.deepEqual(apiPaymentMethods.map(option => option.value), ['wechat', 'alipay'])
  assert.deepEqual(apiPaymentMethodIconSrc, {
    wechat: '/wechat-mark.svg',
    alipay: '/alipay-mark.svg',
  })
  assert.equal(empty.paymentOptions.every(option => option.paymentQrCodeDataUrl === null), true)
  assert.equal(isApiPaymentAccountSettingsComplete(empty), false)
  assert.match(apiPaymentSettingsMissingReason(empty), /启用至少一种/)
  assert.doesNotMatch(apiPaymentSettingsMissingReason(empty), /个人中心/)

  const wechatWithoutQr = normalizeApiPaymentAccountSettings({
    paymentWindowMinutes: 15,
    paymentOptions: [
      { paymentMethod: 'wechat', enabled: true, paymentInstructions: '扫码备注 API 意向', paymentQrCodeDataUrl: null },
    ],
  })
  assert.equal(wechatWithoutQr.paymentWindowMinutes, defaultApiPaymentWindowMinutes)
  assert.equal(isApiPaymentAccountSettingsComplete(wechatWithoutQr), false)
  assert.match(apiPaymentSettingsMissingReason(wechatWithoutQr), /上传微信收款码/)
  assert.doesNotMatch(apiPaymentSettingsMissingReason(wechatWithoutQr), /个人中心/)

  const wechatWithQr = normalizeApiPaymentAccountSettings({
    paymentOptions: [
      { paymentMethod: 'wechat', enabled: true, paymentInstructions: '', paymentQrCodeDataUrl: qrDataUrl },
    ],
  })
  assert.equal(isApiPaymentAccountSettingsComplete(wechatWithQr), true)
  assert.match(apiPaymentSettingsSummary(wechatWithQr), /固定 10 分钟确认/)

  const legacyBothEnabled = normalizeApiPaymentAccountSettings({
    paymentOptions: [
      { paymentMethod: 'alipay', enabled: true, paymentInstructions: '', paymentQrCodeDataUrl: qrDataUrl },
      { paymentMethod: 'wechat', enabled: true, paymentInstructions: '', paymentQrCodeDataUrl: qrDataUrl },
    ],
  })
  assert.deepEqual(legacyBothEnabled.paymentOptions.map(option => option.enabled), [false, true])
  assert.equal(isApiPaymentAccountSettingsComplete(legacyBothEnabled), true)

  const legacyUSDT = normalizeApiPaymentAccountSettings({
    paymentOptions: [
      { paymentMethod: 'usdt', enabled: true, paymentInstructions: '', paymentQrCodeDataUrl: qrDataUrl },
    ],
  })
  assert.deepEqual(legacyUSDT.paymentOptions.map(option => option.paymentMethod), ['wechat', 'alipay'])
  assert.equal(isApiPaymentAccountSettingsComplete(legacyUSDT), false)

  const invalidQr = normalizeApiPaymentAccountSettings({
    paymentOptions: [
      { paymentMethod: 'alipay', enabled: true, paymentInstructions: '', paymentQrCodeDataUrl: 'https://example.com/qr.png' },
    ],
  })
  assert.equal(invalidQr.paymentOptions.find(option => option.paymentMethod === 'alipay')?.paymentQrCodeDataUrl, null)

  const isolatedDraft = cloneApiPaymentAccountSettings(wechatWithQr)
  isolatedDraft.paymentOptions[0]!.enabled = false
  isolatedDraft.paymentOptions[0]!.paymentQrCodeDataUrl = null
  assert.equal(wechatWithQr.paymentOptions[0]!.enabled, true)
  assert.equal(wechatWithQr.paymentOptions[0]!.paymentQrCodeDataUrl, qrDataUrl)
})
