import type { ApiServicePromotionAvailability } from '@/api/generated/openapi'

export function apiPromotionAvailabilityBlockReasons(availability: ApiServicePromotionAvailability) {
  const reasons = [...availability.eligibility.hardBlockReasons]
  if (!availability.eligibility.configurable && reasons.length === 0) {
    reasons.push('服务当前不符合推广配置条件。')
  }
  if (availability.sameServiceOverlap) {
    reasons.push('该服务在所选时间范围内已有推广排期。')
  }
  if (availability.remainingCapacity <= 0) {
    reasons.push('所选时间范围内的推广池峰值容量已满。')
  }
  return reasons
}
