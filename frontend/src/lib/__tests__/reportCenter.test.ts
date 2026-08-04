import { describe, expect, it } from 'vitest'
import type { MyAppeal, MyDispute, MyReport } from '@/lib/reportBackend'
import {
  buildModerationRecords,
  canCreateAppeal,
  createAppealPayload,
  filterModerationRecords,
  hasPendingAppeal,
} from '@/lib/reportCenter'

function report(overrides: Partial<MyReport> = {}): MyReport {
  return {
    id: 'report-1',
    reporterUsername: 'orbit',
    reporterName: 'Orbit',
    targetType: 'public_user',
    targetId: 'reported-user',
    canonicalTargetType: 'public_user',
    canonicalTargetId: 'reported-user',
    targetLabel: '公开主页 @reported-user',
    reportedUsername: 'reported-user',
    reasonCode: 'description_mismatch',
    title: '服务说明不一致',
    status: 'closed',
    createdAt: '2026-08-01T08:00:00Z',
    updatedAt: '2026-08-01T09:00:00Z',
    version: 2,
    ...overrides,
  }
}

function dispute(overrides: Partial<MyDispute> = {}): MyDispute {
  return {
    id: 'dispute-1',
    targetType: 'api_order',
    targetId: 'order-1',
    targetLabel: 'API 订单',
    primaryUsername: 'orbit',
    primaryDisplayName: 'Orbit',
    counterpartyUsername: 'seller',
    counterpartyName: 'Seller',
    status: 'resolved',
    publicSummary: 'API 接入说明争议',
    publicResultCode: 'api_delivery_issue',
    publicResult: '确认说明不一致，案件已处理。',
    openedAt: '2026-08-01T10:00:00Z',
    createdAt: '2026-08-01T10:00:00Z',
    updatedAt: '2026-08-01T11:00:00Z',
    version: 3,
    canAppeal: true,
    ...overrides,
  }
}

function appeal(overrides: Partial<MyAppeal> = {}): MyAppeal {
  return {
    id: 'appeal-1',
    appellantUsername: 'orbit',
    appellantName: 'Orbit',
    disputeId: 'dispute-1',
    targetType: 'api_order',
    targetId: 'order-1',
    title: '请求复核',
    status: 'submitted',
    createdAt: '2026-08-01T12:00:00Z',
    updatedAt: '2026-08-01T12:00:00Z',
    version: 1,
    ...overrides,
  }
}

describe('举报与申诉中心领域投影', () => {
  it('合并三类记录、按更新时间排序并支持类别筛选', () => {
    const records = buildModerationRecords([report()], [dispute()], [appeal()])
    expect(records.map(item => item.kind)).toEqual(['appeal', 'dispute', 'report'])
    expect(filterModerationRecords(records, 'dispute').map(item => item.id)).toEqual(['dispute-1'])
    expect(filterModerationRecords(records, 'all')).toHaveLength(3)
  })

  it('举报只允许最终且未转纠纷的记录申诉', () => {
    const closed = buildModerationRecords([report()], [], [])[0]!
    const processing = buildModerationRecords([report({ status: 'triaged' })], [], [])[0]!
    const transferred = buildModerationRecords([report({ disputeId: 'dispute-1' })], [], [])[0]!

    expect(canCreateAppeal(closed, [])).toBe(true)
    expect(canCreateAppeal(processing, [])).toBe(false)
    expect(canCreateAppeal(transferred, [])).toBe(false)
  })

  it('纠纷严格服从服务端 canAppeal 且待处理申诉阻止重复提交', () => {
    const allowed = buildModerationRecords([], [dispute({ canAppeal: true })], [])[0]!
    const denied = buildModerationRecords([], [dispute({ canAppeal: false })], [])[0]!
    const pending = [appeal()]

    expect(canCreateAppeal(allowed, [])).toBe(true)
    expect(canCreateAppeal(denied, [])).toBe(false)
    expect(hasPendingAppeal(allowed, pending)).toBe(true)
    expect(canCreateAppeal(allowed, pending)).toBe(false)
  })

  it('申诉 payload 只携带合法关联 ID，不复制目标类型或目标 ID', () => {
    const reportRecord = buildModerationRecords([report()], [], [])[0]!
    const disputeRecord = buildModerationRecords([], [dispute()], [])[0]!

    expect(createAppealPayload(reportRecord, '复核举报', '请复核该举报处理。')).toEqual({
      reportId: 'report-1',
      title: '复核举报',
      statement: '请复核该举报处理。',
    })
    expect(createAppealPayload(disputeRecord, '复核纠纷', '请复核该纠纷处理。')).toEqual({
      disputeId: 'dispute-1',
      title: '复核纠纷',
      statement: '请复核该纠纷处理。',
    })
  })
})
