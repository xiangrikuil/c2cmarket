import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  apiPaymentMethods,
  apiPaymentSettingsMissingReason,
  apiPaymentSettingsSummary,
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
  assert.equal(empty.paymentOptions.every(option => option.paymentQrCodeDataUrl === null), true)
  assert.equal(isApiPaymentAccountSettingsComplete(empty), false)
  assert.match(apiPaymentSettingsMissingReason(empty), /启用至少一种/)

  const wechatWithoutQr = normalizeApiPaymentAccountSettings({
    paymentWindowMinutes: 15,
    paymentOptions: [
      { paymentMethod: 'wechat', enabled: true, paymentInstructions: '扫码备注 API 意向', paymentQrCodeDataUrl: null },
    ],
  })
  assert.equal(wechatWithoutQr.paymentWindowMinutes, defaultApiPaymentWindowMinutes)
  assert.equal(isApiPaymentAccountSettingsComplete(wechatWithoutQr), false)
  assert.match(apiPaymentSettingsMissingReason(wechatWithoutQr), /上传微信收款码/)

  const wechatWithQr = normalizeApiPaymentAccountSettings({
    paymentOptions: [
      { paymentMethod: 'wechat', enabled: true, paymentInstructions: '', paymentQrCodeDataUrl: qrDataUrl },
    ],
  })
  assert.equal(isApiPaymentAccountSettingsComplete(wechatWithQr), true)
  assert.match(apiPaymentSettingsSummary(wechatWithQr), /固定 10 分钟确认/)

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
})
