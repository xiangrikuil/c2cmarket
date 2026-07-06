import assert from 'node:assert/strict'
import {
  apiPaymentSettingsMissingReason,
  apiPaymentSettingsSummary,
  createEmptyApiPaymentAccountSettings,
  defaultApiPaymentWindowMinutes,
  isApiPaymentAccountSettingsComplete,
  normalizeApiPaymentAccountSettings,
} from '../../../lib/apiPaymentSettings.ts'

const qrDataUrl = 'data:image/png;base64,aGVsbG8='

const empty = createEmptyApiPaymentAccountSettings()
assert.equal(empty.paymentWindowMinutes, 10)
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

const usdtWithoutInstructions = normalizeApiPaymentAccountSettings({
  paymentOptions: [
    { paymentMethod: 'usdt', enabled: true, paymentInstructions: '', paymentQrCodeDataUrl: qrDataUrl },
  ],
})
assert.equal(usdtWithoutInstructions.paymentOptions.find(option => option.paymentMethod === 'usdt')?.paymentQrCodeDataUrl, qrDataUrl)
assert.equal(isApiPaymentAccountSettingsComplete(usdtWithoutInstructions), false)
assert.match(apiPaymentSettingsMissingReason(usdtWithoutInstructions), /填写USDT收款说明/)

const invalidQr = normalizeApiPaymentAccountSettings({
  paymentOptions: [
    { paymentMethod: 'alipay', enabled: true, paymentInstructions: '', paymentQrCodeDataUrl: 'https://example.com/qr.png' },
  ],
})
assert.equal(invalidQr.paymentOptions.find(option => option.paymentMethod === 'alipay')?.paymentQrCodeDataUrl, null)
