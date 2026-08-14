import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import {
  merchantSupportNoteFromPublishPayload,
  projectAPIIntentPricingSnapshot,
} from '@/lib/apiMarketBackend'

const backendAdapterSource = readFileSync(new URL('../apiMarketBackend.ts', import.meta.url), 'utf8')

describe('API 订单意向快照投影', () => {
  it('读取冻结模型、倍率、用量规则与商户售后说明', () => {
    const snapshot = projectAPIIntentPricingSnapshot(JSON.stringify({
      models: [
        { modelKey: 'gpt-5.6', merchantMultiplier: '1.0000' },
        { modelKey: 'gpt-5-mini', merchantMultiplier: '0.2000' },
        { modelKey: 'gpt-5.6', merchantMultiplier: '1.0000' },
      ],
      usageVisibility: 'offsite_panel_readonly',
      merchantNote: '高峰期可能响应变慢。',
      merchantSupportNote: '商户承诺 7 天。',
      accountPoolType: 'custom',
      accountPoolLabel: 'Claude Max',
      declaredMaxConcurrency: 12,
      merchantRefundCommitment: true,
      merchantRefundPolicyVersion: 'api-merchant-refund-v1',
      serviceValidityExpiresAt: '2026-08-31T16:00:00Z',
    }))

    expect(snapshot).toMatchObject({
      models: ['gpt-5.6', 'gpt-5-mini'],
      multiplier: '按模型分别计算',
      defaultMultiplier: 1,
      usageVisibility: 'panel_realtime',
      usageVisibilitySnapshotMissing: false,
      merchantNote: '高峰期可能响应变慢。',
      merchantSupportNote: '商户承诺 7 天。',
      accountPoolType: 'custom',
      accountPoolLabel: 'Claude Max',
      declaredMaxConcurrency: 12,
      merchantRefundCommitment: true,
      merchantRefundPolicyVersion: 'api-merchant-refund-v1',
      serviceValidityExpiresAt: '2026-08-31T16:00:00Z',
    })
    expect(snapshot.issue).toBeUndefined()
    expect(snapshot.commercialFactsSnapshotIssue).toBeUndefined()
  })

  it('对旧快照明确标记未冻结的售后和用量字段', () => {
    const snapshot = projectAPIIntentPricingSnapshot('{"models":[{"modelKey":"gpt-5.6","merchantMultiplier":"1.0000"}],"recommendedConcurrency":7}')

    expect(snapshot.models).toEqual(['gpt-5.6'])
    expect(snapshot.multiplier).toBe('1.00x')
    expect(snapshot.merchantSupportNote).toBe('历史订单未冻结商户售后说明')
    expect(snapshot.usageVisibilitySnapshotMissing).toBe(true)
    expect(snapshot.commercialFactsSnapshotIssue).toBe('missing')
    expect(snapshot.accountPoolLabel).toBeUndefined()
    expect(snapshot.declaredMaxConcurrency).toBe(7)
  })

  it('损坏快照不会回退为服务标题', () => {
    const snapshot = projectAPIIntentPricingSnapshot('not-json')

    expect(snapshot.models).toEqual([])
    expect(snapshot.issue).toBe('invalid')
    expect(snapshot.merchantSupportNote).toContain('订单快照不可用')
    expect(snapshot.commercialFactsSnapshotIssue).toBe('invalid')
  })

  it('完整键存在但商业事实非法时不伪造订单信息', () => {
    const snapshot = projectAPIIntentPricingSnapshot(JSON.stringify({
      models: [],
      accountPoolType: 'unknown',
      accountPoolLabel: '',
      declaredMaxConcurrency: 0,
      merchantRefundCommitment: 'yes',
      merchantRefundPolicyVersion: '',
      serviceValidityExpiresAt: '',
    }))

    expect(snapshot.commercialFactsSnapshotIssue).toBe('invalid')
    expect(snapshot.accountPoolType).toBeUndefined()
    expect(snapshot.declaredMaxConcurrency).toBeUndefined()
    expect(snapshot.merchantRefundCommitment).toBeUndefined()
  })

  it('将历史服务明确冻结的空号池和空并发视为有效事实', () => {
    const snapshot = projectAPIIntentPricingSnapshot(JSON.stringify({
      models: [],
      accountPoolType: null,
      accountPoolLabel: null,
      declaredMaxConcurrency: null,
      merchantRefundCommitment: false,
      merchantRefundPolicyVersion: 'api-merchant-refund-v1',
      serviceValidityExpiresAt: null,
    }))

    expect(snapshot.commercialFactsSnapshotIssue).toBeUndefined()
    expect(snapshot.accountPoolType).toBeUndefined()
    expect(snapshot.accountPoolLabel).toBeUndefined()
    expect(snapshot.declaredMaxConcurrency).toBeUndefined()
    expect(snapshot.merchantRefundCommitment).toBe(false)
    expect(snapshot.serviceValidityExpiresAt).toBeNull()
  })

  it('订单映射以订单自身的定价快照为权威来源', () => {
    expect(backendAdapterSource).toContain("projectAPIIntentPricingSnapshot(order.pricingSnapshot ?? '')")
    expect(backendAdapterSource).toContain('mapAPIQuotaOrderSnapshot(order, pricingSnapshot)')
    expect(backendAdapterSource).toContain('accountPoolLabel: pricingSnapshot.accountPoolLabel')
    expect(backendAdapterSource).toContain('merchantRefundPolicyVersion: pricingSnapshot.merchantRefundPolicyVersion')
    expect(backendAdapterSource).not.toContain('intentSnapshot: intent.snapshot')
    expect(backendAdapterSource).toContain("hasOwnProperty.call(snapshot, 'declaredMaxConcurrency')")
  })
})

describe('API 服务售后配置映射', () => {
  it('保留历史售后模式并支持新的全额退款承诺口径', () => {
    expect(merchantSupportNoteFromPublishPayload({ mode: 'merchant_full_refund' })).toContain('连续不可用超过 1 小时')
    expect(merchantSupportNoteFromPublishPayload({ mode: 'no_warranty' })).toBe('无额外售后承诺，具体问题由双方站外协商。')
    expect(merchantSupportNoteFromPublishPayload({ mode: 'upstream_refund_only', refundNote: '上游到账后原路退回。' })).toBe('仅在上游退款后处理：上游到账后原路退回。')
    expect(merchantSupportNoteFromPublishPayload({
      mode: 'merchant_warranty',
      warrantyDays: 7,
      coverage: '接口不可用',
      compensation: '补偿额度',
      exclusions: '高并发压测',
    })).toBe('商户承诺 7 天；适用范围：接口不可用；补偿方式：补偿额度；不适用情形：高并发压测。')
  })
})
