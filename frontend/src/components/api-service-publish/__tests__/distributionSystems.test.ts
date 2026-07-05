import assert from 'node:assert/strict'
import { distributionLabels, publishDistributionOptions } from '../utils.ts'

assert.deepEqual(
  publishDistributionOptions.map(option => option.value),
  ['sub2api', 'other'],
)

assert.equal(distributionLabels.new_api_proxy, '其他 API 接入')
assert.equal(distributionLabels.other, '其他 API 接入')
