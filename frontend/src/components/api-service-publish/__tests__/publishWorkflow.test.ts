import assert from 'node:assert/strict'
import { describe, test } from 'vitest'
import {
  canVisitPublishStep,
  completePublishStep,
  firstErrorStep,
  publishStepStatus,
} from '../publishWorkflow'

describe('progressive API publish workflow', () => {
  test('distinguishes active, completed, and pending steps', () => {
    assert.equal(publishStepStatus(1, 2, [1]), 'completed')
    assert.equal(publishStepStatus(2, 2, [1]), 'active')
    assert.equal(publishStepStatus(3, 2, [1]), 'pending')
  })

  test('completes a step without duplicating navigation state', () => {
    assert.deepEqual(completePublishStep([1, 2], 2), [1, 2])
    assert.deepEqual(completePublishStep([3, 1], 2), [1, 2, 3])
  })

  test('only allows active or completed steps to be revisited', () => {
    assert.equal(canVisitPublishStep(1, 2, [1]), true)
    assert.equal(canVisitPublishStep(2, 2, [1]), true)
    assert.equal(canVisitPublishStep(3, 2, [1]), false)
  })

  test('maps the first validation error to its owning step', () => {
    const fieldSteps = {
      price: 1,
      model: 2,
      payment: 3,
    } as const

    assert.equal(firstErrorStep({ model: '请选择模型', payment: '请配置收款' }, fieldSteps), 2)
    assert.equal(firstErrorStep({}, fieldSteps), undefined)
  })
})
