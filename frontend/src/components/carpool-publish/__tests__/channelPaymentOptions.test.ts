import assert from 'node:assert/strict'
import { test } from 'vitest'
import { carpoolOpeningChannels, carpoolPaymentMethods } from '@/data/mock'
import { openingChannelLabels, paymentMethodLabels } from '../utils'

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
    '其他',
  ])
  assert.deepEqual(Object.values(paymentMethodLabels), carpoolPaymentMethods.map(item => item.displayName))
})
