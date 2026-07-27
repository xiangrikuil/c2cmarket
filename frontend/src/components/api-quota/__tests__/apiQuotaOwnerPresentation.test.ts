import assert from 'node:assert/strict'
import { describe, test } from 'vitest'
import { getApiServiceOwnerStatus } from '../../api-service-owner/apiServiceOwnerPresentation'
import {
  findNextApiQuotaRound,
  getApiQuotaBatchStatus,
  getApiQuotaOfferStatus,
  getApiQuotaRoundStatus,
} from '../apiQuotaOwnerPresentation'

describe('API 服务卖家管理展示', () => {
  test('服务状态只映射为四个中文经营状态', () => {
    assert.deepEqual(getApiServiceOwnerStatus({ online: true, state: 'online' }), { label: '接单中', tone: 'success' })
    assert.deepEqual(getApiServiceOwnerStatus({ online: false, state: 'reviewing' }), { label: '审核中', tone: 'waiting' })
    assert.deepEqual(getApiServiceOwnerStatus({ online: false, state: 'paused' }), { label: '已暂停', tone: 'warning' })
    assert.deepEqual(getApiServiceOwnerStatus({ online: false, state: 'offline' }), { label: '未上线', tone: 'neutral' })
  })

  test('批次和规格状态不向中文界面泄露英文枚举', () => {
    assert.equal(getApiQuotaBatchStatus('published').label, '销售中')
    assert.equal(getApiQuotaBatchStatus('archived').label, '已归档')
    assert.equal(getApiQuotaOfferStatus('draft').label, '草稿')
    assert.equal(getApiQuotaOfferStatus('paused').label, '已暂停')
  })

  test('放量计划按时间边界展示待开始、进行中和已完成', () => {
    const round = {
      status: 'scheduled' as const,
      startsAt: '2026-07-26T05:00:00.000Z',
      endsAt: '2026-07-26T06:00:00.000Z',
    }

    assert.equal(getApiQuotaRoundStatus(round, Date.parse('2026-07-26T04:59:59.000Z')).label, '待开始')
    assert.equal(getApiQuotaRoundStatus(round, Date.parse('2026-07-26T05:00:00.000Z')).label, '进行中')
    assert.equal(getApiQuotaRoundStatus(round, Date.parse('2026-07-26T06:00:00.000Z')).label, '已完成')
    assert.equal(getApiQuotaRoundStatus({ ...round, status: 'cancelled' }).label, '已取消')
  })

  test('摘要选择未来最近的一次放量', () => {
    const now = Date.parse('2026-07-26T04:00:00.000Z')
    const next = findNextApiQuotaRound([
      { status: 'closed', startsAt: '2026-07-26T03:00:00.000Z' },
      { status: 'scheduled', startsAt: '2026-07-26T06:00:00.000Z' },
      { status: 'scheduled', startsAt: '2026-07-26T05:00:00.000Z' },
    ], now)

    assert.equal(next?.startsAt, '2026-07-26T05:00:00.000Z')
  })
})
