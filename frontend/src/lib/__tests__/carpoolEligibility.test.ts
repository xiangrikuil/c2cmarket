import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { carpools, type Carpool, type CarpoolApplicationEligibilityCode } from '@/data/mock'
import { evaluateCarpoolApplicationEligibility } from '@/lib/carpoolEligibility'

function baseCarpool(): Carpool {
  return {
    ...structuredClone(carpools[0]!),
    owner: 'other-owner',
    ownerUserId: 'owner-1',
    status: '可上车',
    accessArrangementMode: 'provider_member_invitation',
    accessArrangementNote: '通过官方成员邀请加入。',
    riskAcknowledged: true,
    hasUnresolvedDispute: false,
  }
}

describe('拼车申请资格投影', () => {
  const cases: Array<{
    code: CarpoolApplicationEligibilityCode
    mutate?: (carpool: Carpool) => void
    ongoing?: boolean
    member?: boolean
    userId?: string
    availableSeats?: number
  }> = [
    { code: 'eligible' },
    { code: 'sold_out', availableSeats: 0 },
    { code: 'paused', mutate: value => { value.status = '暂停' } },
    { code: 'credential_risk', mutate: value => { value.accessArrangementMode = 'not_allowed' } },
    { code: 'owner_action_required', mutate: value => { value.accessArrangementNote = '' } },
    { code: 'already_applied', ongoing: true },
    { code: 'already_member', ongoing: true, member: true },
    { code: 'self_owned', userId: 'owner-1' },
  ]

  for (const test of cases) {
    it(`返回唯一 ${test.code} 结论`, () => {
      const carpool = baseCarpool()
      test.mutate?.(carpool)
      const result = evaluateCarpoolApplicationEligibility(
        carpool,
        { availableSeats: test.availableSeats ?? 1 },
        test.ongoing,
        test.userId ?? 'buyer-1',
        test.member,
      )
      expect(result.code).toBe(test.code)
      expect(result.canApply).toBe(test.code === 'eligible')
      expect(result.reason).not.toBe('')
    })
  }

  it('产品风险优先于个人状态和售罄', () => {
    const carpool = baseCarpool()
    carpool.accessArrangementMode = 'not_allowed'
    carpool.status = '暂停'
    const result = evaluateCarpoolApplicationEligibility(carpool, { availableSeats: 0 }, true, 'owner-1', true)
    expect(result.code).toBe('credential_risk')
  })

  it('详情页只有一个打开申请弹窗的主入口', () => {
    const source = readFileSync(new URL('../../pages/CarpoolDetailPage.vue', import.meta.url), 'utf8')
    expect(source.match(/applyDialogOpen = true/g)).toHaveLength(1)
    expect(source).toContain('applicationEligibility.value?.code')
  })
})
