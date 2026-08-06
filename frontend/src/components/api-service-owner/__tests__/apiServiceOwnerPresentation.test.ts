import assert from 'node:assert/strict'
import { describe, test } from 'vitest'
import {
  apiServiceSalesViewOptions,
  getApiServiceSalesAvailabilitySummary,
  getApiServiceSalesChannelLabel,
  getApiServiceSalesStatus,
  getApiServiceSalesTimeSummary,
  getApiServiceProbeStatus,
  getInitialApiServiceSalesView,
} from '../apiServiceOwnerPresentation'

const healthSummary = (
  state: 'normal' | 'fluctuating' | 'abnormal' | 'no_sample',
  availabilityReason: 'unconfigured' | 'disabled' | 'unauthorized' | 'insufficient' | 'stale' | 'temporarily_unavailable' | null,
) => ({
  state,
  availabilityReason,
  successRatePercent: null,
  successfulSamples: 0,
  totalSamples: 0,
  medianTtftMs: null,
  probeModel: null,
  transportSecurity: null,
  lastSampledAt: null,
  samples: [],
})

describe('API 服务销售生命周期展示', () => {
  test('普通入口默认有效销售，限时包入口默认全部', () => {
    assert.equal(getInitialApiServiceSalesView(undefined), 'active')
    assert.equal(getInitialApiServiceSalesView('quota'), 'all')
    assert.equal(getInitialApiServiceSalesView(['quota']), 'active')
    assert.deepEqual(
      apiServiceSalesViewOptions.map(option => option.value),
      ['active', 'expired', 'paused', 'draft', 'all'],
    )
  })

  test('区分销售渠道与全部生命周期状态', () => {
    assert.equal(getApiServiceSalesChannelLabel('flexible_quota'), '自由额度')
    assert.equal(getApiServiceSalesChannelLabel('limited_quota'), '限时额度包')
    assert.deepEqual(
      ['selling', 'upcoming', 'paused', 'sold_out', 'expired', 'draft', 'offline', 'archived']
        .map(state => getApiServiceSalesStatus(state as Parameters<typeof getApiServiceSalesStatus>[0]).label),
      ['销售中', '待开始', '已暂停', '已售罄', '已过期', '草稿', '未上线', '已归档'],
    )
  })

  test('展示自由额度和限时包可售余量', () => {
    assert.equal(getApiServiceSalesAvailabilitySummary({
      kind: 'flexible_quota',
      state: 'selling',
      availableUsdAllowance: '420.000000',
    }), '可售 $420')
    assert.equal(getApiServiceSalesAvailabilitySummary({
      kind: 'limited_quota',
      state: 'selling',
      availableCopies: 18,
    }), '剩余 18 份')
  })

  test('展示待开始、销售中和已过期的权威时间', () => {
    assert.match(getApiServiceSalesTimeSummary({
      kind: 'limited_quota',
      state: 'upcoming',
      nextStartsAt: '2026-07-30T12:00:00Z',
    }), /^开售 /)
    assert.match(getApiServiceSalesTimeSummary({
      kind: 'limited_quota',
      state: 'selling',
      saleCutoffAt: '2026-07-31T14:00:00Z',
      expiresAt: '2026-07-31T15:00:00Z',
    }), /^停售 .+ · 失效 /)
    assert.match(getApiServiceSalesTimeSummary({
      kind: 'limited_quota',
      state: 'expired',
      expiresAt: '2026-07-31T15:00:00Z',
    }), /^已于 .+ 结束$/)
    assert.equal(getApiServiceSalesTimeSummary({
      kind: 'flexible_quota',
      state: 'selling',
    }), '长期服务')
  })

  test('只用服务列表健康摘要展示准确探针状态', () => {
    assert.deepEqual(
      [
        ['unconfigured', '未配置'],
        ['disabled', '已停用'],
        ['unauthorized', '待授权'],
        ['insufficient', '样本不足'],
        ['stale', '样本过期'],
        ['temporarily_unavailable', '暂不可用'],
      ].map(([reason]) => getApiServiceProbeStatus(healthSummary('no_sample', reason as Parameters<typeof healthSummary>[1])).label),
      ['未配置', '已停用', '待授权', '样本不足', '样本过期', '暂不可用'],
    )
    assert.deepEqual(
      ['normal', 'fluctuating', 'abnormal', 'no_sample']
        .map(state => getApiServiceProbeStatus(healthSummary(state as Parameters<typeof healthSummary>[0], null)).label),
      ['正常', '波动', '异常', '暂无数据'],
    )
  })
})
