import assert from 'node:assert/strict'
import { test } from 'vitest'
import { carpoolOpeningChannels, carpoolPaymentMethods, carpoolRegions } from '@/data/mock'
import { adminAccountLabel, canBuildCarpoolShareText, distributionFieldsComplete, distributionMethodLabel, openingChannelLabels, paymentMethodLabels, regionDisplayName } from '../utils'
import type { CarpoolPublishForm } from '../types'

test('uses the current opening channel and payment method options', () => {
  assert.deepEqual(carpoolOpeningChannels.map(item => item.displayName), [
    'Web 官网',
    'iOS App Store',
    'Google Play',
    'Team / Business 席位',
    '其他',
  ])
  assert.deepEqual(Object.values(openingChannelLabels), carpoolOpeningChannels.map(item => item.displayName))

  assert.deepEqual(carpoolPaymentMethods.map(item => item.displayName), [
    '信用卡',
    '虚拟卡',
    'Apple Pay',
    'Google Pay',
    'App Store 礼品卡',
    'Google Play 礼品卡',
    'PayPal',
    'U 卡',
    '其他',
  ])
  assert.deepEqual(Object.values(paymentMethodLabels), carpoolPaymentMethods.map(item => item.displayName))
})

test('requires exactly one carpool publish payment method', () => {
  const regionsByCode = new Map(carpoolRegions.map(item => [item.code, item]))
  const channelsByCode = new Map(carpoolOpeningChannels.map(item => [item.code, item]))
  const methodsByCode = new Map(carpoolPaymentMethods.map(item => [item.code, item]))
  const form: CarpoolPublishForm = {
    productId: 'chatgpt-pro-20x-web',
    customProductName: null,
    regionCode: 'other',
    customRegionName: '印度区',
    monthlyPriceCny: 68,
    serviceMultiplier: 1,
    dailyQuotaMode: 'amount',
    dailyQuotaAmount: 50,
    weeklyQuotaMode: 'amount',
    weeklyQuotaAmount: 200,
    followsOfficialQuotaReset: true,
    vpsRegion: '香港',
    supportsMainlandChinaDirectConnection: true,
    totalSeats: 5,
    occupiedSeats: 1,
    openingChannelCode: 'web',
    customOpeningChannel: '',
    paymentMethodCode: 'credit_card',
    customPaymentMethod: '',
    distributionMethod: 'sub2api',
    distributionMethodNote: '',
    providesAdminAccount: true,
    accessArrangementMode: 'personal_account_cost_share',
    accessArrangementNote: '个人订阅费用分摊，平台不保存、不交付任何密码、Session、Cookie 或 token。',
    riskAcknowledged: true,
    policyVersion: 1,
    riskNoticeCode: 'openai_subscription_carpool',
    warranty: {
      mode: 'remaining_days_compensation',
      fixedWarrantyDays: null,
      compensationMethod: '按剩余天数补偿',
      exclusions: '',
    },
    rulesNote: '买家按车主说明使用席位，站外确认细节。',
  }

  assert.equal(regionDisplayName(form, regionsByCode), '印度区')
  assert.equal(distributionMethodLabel(form.distributionMethod), 'Sub2API')
  assert.equal(adminAccountLabel(form.providesAdminAccount), '提供管理员账号')
  assert.equal(distributionFieldsComplete(form), true)
  assert.equal(canBuildCarpoolShareText(form, regionsByCode, channelsByCode, methodsByCode), true)

  form.paymentMethodCode = ''
  assert.equal(canBuildCarpoolShareText(form, regionsByCode, channelsByCode, methodsByCode), false)

  form.paymentMethodCode = 'other'
  form.customPaymentMethod = ''
  assert.equal(canBuildCarpoolShareText(form, regionsByCode, channelsByCode, methodsByCode), false)

  form.customPaymentMethod = '数字钱包'
  assert.equal(canBuildCarpoolShareText(form, regionsByCode, channelsByCode, methodsByCode), true)
})

test('requires other distribution note and admin account choice', () => {
  const form: Pick<CarpoolPublishForm, 'distributionMethod' | 'distributionMethodNote' | 'providesAdminAccount'> = {
    distributionMethod: 'other',
    distributionMethodNote: '',
    providesAdminAccount: false,
  }

  assert.equal(distributionFieldsComplete(form), false)
  form.distributionMethodNote = '家庭组成员安排，具体方式站外确认。'
  assert.equal(distributionFieldsComplete(form), true)
  form.providesAdminAccount = null
  assert.equal(distributionFieldsComplete(form), false)
})

test('account login does not require an administrator-account choice', () => {
  const form: Pick<CarpoolPublishForm, 'distributionMethod' | 'distributionMethodNote' | 'providesAdminAccount'> = {
    distributionMethod: 'account_login',
    distributionMethodNote: '',
    providesAdminAccount: null,
  }

  assert.equal(distributionMethodLabel(form.distributionMethod), '账号登录')
  assert.equal(distributionFieldsComplete(form), true)
})

test('unlimited spend limits and omitted network details still allow share text', () => {
  const regionsByCode = new Map(carpoolRegions.map(item => [item.code, item]))
  const channelsByCode = new Map(carpoolOpeningChannels.map(item => [item.code, item]))
  const methodsByCode = new Map(carpoolPaymentMethods.map(item => [item.code, item]))
  const form: CarpoolPublishForm = {
    productId: 'chatgpt-pro-20x-web', customProductName: null, regionCode: 'other', customRegionName: '印度区',
    monthlyPriceCny: 68, serviceMultiplier: 1, dailyQuotaMode: 'unlimited', dailyQuotaAmount: null,
    weeklyQuotaMode: 'unlimited', weeklyQuotaAmount: null, followsOfficialQuotaReset: true, vpsRegion: '',
    supportsMainlandChinaDirectConnection: null, totalSeats: 5, occupiedSeats: 1, openingChannelCode: 'web',
    customOpeningChannel: '', paymentMethodCode: 'credit_card', customPaymentMethod: '', distributionMethod: 'account_login',
    distributionMethodNote: '', providesAdminAccount: null, accessArrangementMode: 'personal_account_cost_share',
    accessArrangementNote: '个人订阅费用分摊，平台不保存、不交付任何密码、Session、Cookie 或 token。', riskAcknowledged: true,
    policyVersion: 1, riskNoticeCode: 'openai_subscription_carpool', warranty: { mode: 'no_warranty', fixedWarrantyDays: null, compensationMethod: null, exclusions: null },
    rulesNote: '买家按车主说明使用席位，站外确认细节。',
  }

  assert.equal(canBuildCarpoolShareText(form, regionsByCode, channelsByCode, methodsByCode), true)
})
