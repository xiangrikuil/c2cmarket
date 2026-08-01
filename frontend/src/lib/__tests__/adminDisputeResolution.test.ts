import { describe, expect, it } from 'vitest'
import {
  defaultOutcomeReasonCode,
  disputeParticipantOptions,
  findDisputeOutcome,
  normalizeOutcomeSeverity,
  parseDisputeEvidenceSnapshot,
  roleScopeFromSnapshot,
  validateOutcomeForm,
  validateResolutionForm,
  type DisputeDetailLike,
  type OutcomeForm,
} from '@/lib/adminDisputeResolution'
import type { DisputeReputationOutcome } from '@/types/reputation'

const transactionDispute: DisputeDetailLike = {
  id: 'dispute-1',
  targetType: 'api_order',
  targetId: 'order-1',
  primaryUserId: 'user-buyer',
  primaryUsername: 'buyer',
  primaryDisplayName: '买家',
  counterpartyUserId: 'user-seller',
  counterpartyUsername: 'seller',
  counterpartyName: '商户',
  subjectUserId: 'user-seller',
  subjectUsername: 'seller',
  subjectName: '商户',
}

describe('管理员纠纷裁决规则', () => {
  it('只解析后端允许展示的脱敏快照字段', () => {
    const parsed = parseDisputeEvidenceSnapshot(JSON.stringify({
      submittedTargetType: 'api_order',
      submittedTargetId: ' order-1 ',
      canonicalTargetType: 'api_order',
      canonicalTargetId: 'order-1',
      targetLabel: 'API 订单',
      reporterRole: 'buyer',
      participants: [
        { role: 'merchant', userId: 'user-seller', username: 'seller', ignored: 'not-rendered' },
        { role: 'buyer', userId: 'user-buyer', username: 'buyer' },
      ],
      businessStatus: 'payment_submitted',
      hasOrder: true,
      containsContactValue: false,
      privateEvidence: 'not-rendered',
    }))

    expect(parsed.status).toBe('valid')
    if (parsed.status !== 'valid') return
    expect(parsed.snapshot.submittedTargetId).toBe('order-1')
    expect(parsed.snapshot.participants).toEqual([
      { role: 'merchant', userId: 'user-seller', username: 'seller' },
      { role: 'buyer', userId: 'user-buyer', username: 'buyer' },
    ])
    expect(parsed.snapshot).not.toHaveProperty('privateEvidence')
  })

  it('显式报告快照解析失败和不应展示的联系方式标记', () => {
    expect(parseDisputeEvidenceSnapshot('{broken')).toMatchObject({ status: 'invalid' })
    expect(parseDisputeEvidenceSnapshot(JSON.stringify({ participants: 'invalid' }))).toMatchObject({ status: 'invalid' })
    expect(parseDisputeEvidenceSnapshot(JSON.stringify({ containsContactValue: true }))).toMatchObject({ status: 'invalid' })
    expect(parseDisputeEvidenceSnapshot('')).toEqual({ status: 'empty', snapshot: null })
  })

  it('从参与方快照派生买家和卖家角色，未知事实使用全部角色', () => {
    const parsed = parseDisputeEvidenceSnapshot(JSON.stringify({
      participants: [
        { role: 'buyer', userId: 'user-buyer', username: 'buyer' },
        { role: 'merchant', userId: 'user-seller', username: 'seller' },
      ],
    }))
    expect(parsed.status).toBe('valid')
    if (parsed.status !== 'valid') return
    expect(roleScopeFromSnapshot('user-buyer', parsed.snapshot)).toBe('buyer')
    expect(roleScopeFromSnapshot('user-seller', parsed.snapshot)).toBe('seller')
    expect(roleScopeFromSnapshot('unknown', parsed.snapshot)).toBe('all')
    expect(disputeParticipantOptions(transactionDispute, parsed.snapshot).map(item => [item.userId, item.roleScope])).toEqual([
      ['user-buyer', 'buyer'],
      ['user-seller', 'seller'],
    ])
  })

  it('公开主页纠纷只允许目标用户作为责任主体', () => {
    const options = disputeParticipantOptions({
      ...transactionDispute,
      targetType: 'public_user',
      targetId: 'user-seller',
    })
    expect(options).toEqual([expect.objectContaining({ userId: 'user-seller', roleScope: 'all' })])
  })

  it('不为后端不支持的目标或快照外用户提供责任主体', () => {
    expect(disputeParticipantOptions({
      ...transactionDispute,
      targetType: 'contact_snapshot',
    })).toEqual([])

    const parsed = parseDisputeEvidenceSnapshot(JSON.stringify({
      participants: [
        { role: 'merchant', userId: 'user-seller', username: 'seller' },
      ],
    }))
    expect(parsed.status).toBe('valid')
    if (parsed.status !== 'valid') return
    expect(disputeParticipantOptions({
      ...transactionDispute,
      primaryUserId: 'outside-reporter',
    }, parsed.snapshot).map(item => item.userId)).toEqual(['user-seller'])
  })

  it('未认定责任自动使用无严重度，认定责任至少使用低严重度', () => {
    expect(normalizeOutcomeSeverity('not_responsible', 'high')).toBe('none')
    expect(normalizeOutcomeSeverity('undetermined', 'medium')).toBe('none')
    expect(normalizeOutcomeSeverity('responsible', 'none')).toBe('low')
    expect(defaultOutcomeReasonCode('shared')).toBe('shared_responsibility')
  })

  it('第一步要求结构化公开结果、内部原因和明确确认', () => {
    expect(validateResolutionForm({
      publicResultCode: '',
      publicSummary: '短',
      publicResult: '',
      internalReason: '',
      confirmed: false,
    })).toEqual(expect.objectContaining({
      publicResultCode: expect.any(String),
      publicSummary: expect.any(String),
      publicResult: expect.any(String),
      internalReason: expect.any(String),
      confirmed: expect.any(String),
    }))
    expect(validateResolutionForm({
      publicResultCode: 'description_mismatch',
      publicSummary: '服务说明与实际记录不一致',
      publicResult: '已记录描述不一致并完成处理',
      internalReason: '根据关联举报和订单状态认定。',
      confirmed: true,
    })).toEqual({})
  })

  it('第二步校验主体、责任严重度组合和原因代码', () => {
    const invalid: OutcomeForm = {
      subjectUserId: 'outsider',
      responsibility: 'not_responsible',
      severity: 'high',
      roleScope: 'buyer',
      reasonCode: 'INVALID CODE',
      publicReason: '',
      internalReason: '',
      confirmed: false,
    }
    expect(validateOutcomeForm(invalid, ['user-buyer', 'user-seller'])).toEqual(expect.objectContaining({
      subjectUserId: expect.any(String),
      severity: expect.any(String),
      reasonCode: expect.any(String),
      publicReason: expect.any(String),
      internalReason: expect.any(String),
      confirmed: expect.any(String),
    }))
    expect(validateOutcomeForm({
      subjectUserId: 'user-seller',
      responsibility: 'responsible',
      severity: 'high',
      roleScope: 'seller',
      reasonCode: 'confirmed_responsibility',
      publicReason: '根据现有记录认定商户承担主要责任。',
      internalReason: '订单状态和举报说明相互印证。',
      confirmed: true,
    }, ['user-buyer', 'user-seller'])).toEqual({})
  })

  it('可跨参与方信誉审计定位同一纠纷的既有裁定', () => {
    const outcome = { disputeCaseId: 'dispute-1' } as DisputeReputationOutcome
    expect(findDisputeOutcome([{ outcomes: [] }, { outcomes: [outcome] }], 'dispute-1')).toBe(outcome)
    expect(findDisputeOutcome([{ outcomes: [] }], 'missing')).toBeNull()
  })
})
