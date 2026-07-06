import assert from 'node:assert/strict'
import { apiPublishAssistantSummary, apiServiceDetailPath } from '../publishAssistant'

const summary = apiPublishAssistantSummary([
  { label: '分发系统', status: 'done' },
  { label: '具体模型', status: 'pending' },
  { label: '商户承诺', status: 'conflict' },
  { label: '买家须知', status: 'pending' },
])

assert.equal(summary.doneCount, 1)
assert.equal(summary.pendingCount, 2)
assert.equal(summary.conflictCount, 1)
assert.equal(summary.badgeText, '3 项待处理')
assert.equal(summary.topPendingText, '还差：具体模型、买家须知')
assert.deepEqual(summary.pendingLabels, ['具体模型', '买家须知'])

const completeSummary = apiPublishAssistantSummary([
  { label: '分发系统', status: 'done' },
  { label: '具体模型', status: 'done' },
])

assert.equal(completeSummary.badgeText, '可发布')
assert.equal(completeSummary.topPendingText, '发布必填项已完成，可发布')
assert.equal(apiServiceDetailPath('api-123'), '/api-market/api-123')
assert.equal(apiServiceDetailPath(''), '')
