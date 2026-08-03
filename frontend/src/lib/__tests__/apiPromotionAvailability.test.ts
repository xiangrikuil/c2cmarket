import assert from 'node:assert/strict'
import { test } from 'vitest'
import type { ApiServicePromotionAvailability } from '@/api/generated/openapi'
import { apiPromotionAvailabilityBlockReasons } from '../apiPromotionAvailability'

function availability(overrides: Partial<ApiServicePromotionAvailability> = {}): ApiServicePromotionAvailability {
  return {
    eligibility: {
      configurable: true,
      displayable: true,
      hardBlockReasons: [],
      warningReasons: [],
      suppressionReasons: [],
    },
    overlappingCampaigns: 1,
    capacity: 3,
    remainingCapacity: 2,
    sameServiceOverlap: false,
    ...overrides,
  }
}

test('temporary display suppression does not block promotion scheduling', () => {
  const result = apiPromotionAvailabilityBlockReasons(availability({
    eligibility: {
      configurable: true,
      displayable: false,
      hardBlockReasons: [],
      warningReasons: [],
      suppressionReasons: ['服务当前暂停接单。'],
    },
  }))

  assert.deepEqual(result, [])
})

test('hard eligibility, same-service overlap, and full peak capacity block scheduling', () => {
  const result = apiPromotionAvailabilityBlockReasons(availability({
    eligibility: {
      configurable: false,
      displayable: false,
      hardBlockReasons: ['商户账号当前不可用。'],
      warningReasons: [],
      suppressionReasons: ['商户账号当前不可用。'],
    },
    remainingCapacity: 0,
    sameServiceOverlap: true,
  }))

  assert.deepEqual(result, [
    '商户账号当前不可用。',
    '该服务在所选时间范围内已有推广排期。',
    '所选时间范围内的推广池峰值容量已满。',
  ])
})
