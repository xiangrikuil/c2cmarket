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
        { modelNameSnapshot: 'GPT-5.6', merchantMultiplier: '1.0000' },
        { modelNameSnapshot: 'GPT-5 mini', merchantMultiplier: '0.2000' },
        { modelNameSnapshot: 'GPT-5.6', merchantMultiplier: '1.0000' },
      ],
      usageVisibility: 'offsite_panel_readonly',
      merchantNote: '高峰期可能响应变慢。',
      merchantSupportNote: '商户承诺 7 天。',
    }))

    expect(snapshot).toMatchObject({
      models: ['GPT-5.6', 'GPT-5 mini'],
      multiplier: '按模型分别计算',
      defaultMultiplier: 1,
      usageVisibility: 'panel_realtime',
      usageVisibilitySnapshotMissing: false,
      merchantNote: '高峰期可能响应变慢。',
      merchantSupportNote: '商户承诺 7 天。',
    })
    expect(snapshot.issue).toBeUndefined()
  })

  it('对旧快照明确标记未冻结的售后和用量字段', () => {
    const snapshot = projectAPIIntentPricingSnapshot('{"models":[{"modelNameSnapshot":"GPT-5.6","merchantMultiplier":"1.0000"}]}')

    expect(snapshot.models).toEqual(['GPT-5.6'])
    expect(snapshot.multiplier).toBe('1.00x')
    expect(snapshot.merchantSupportNote).toBe('历史订单未冻结商户售后说明')
    expect(snapshot.usageVisibilitySnapshotMissing).toBe(true)
  })

  it('损坏快照不会回退为服务标题', () => {
    const snapshot = projectAPIIntentPricingSnapshot('not-json')

    expect(snapshot.models).toEqual([])
    expect(snapshot.issue).toBe('invalid')
    expect(snapshot.merchantSupportNote).toContain('订单快照不可用')
  })

  it('订单映射以订单自身的定价快照为权威来源', () => {
    expect(backendAdapterSource).toContain("projectAPIIntentPricingSnapshot(order.pricingSnapshot ?? '')")
    expect(backendAdapterSource).not.toContain('intentSnapshot: intent.snapshot')
  })
})

describe('API 服务售后配置映射', () => {
  it('生成三类卖家售后口径', () => {
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
