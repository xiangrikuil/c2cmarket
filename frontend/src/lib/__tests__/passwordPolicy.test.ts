import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  getBackupPasswordStrength,
  getBackupPasswordValidationMessage,
  getPasswordChecks,
} from '../passwordPolicy'

test('backup password requires length, letter, digit, and special character', () => {
  assert.equal(getBackupPasswordValidationMessage('short1!'), '密码需为 8–32 位字符。')
  assert.equal(getBackupPasswordValidationMessage('Password1!Password1!Password1!Long'), '密码需为 8–32 位字符。')
  assert.equal(getBackupPasswordValidationMessage('password-only'), '密码需同时包含字母、数字和特殊字符。')
  assert.equal(getBackupPasswordValidationMessage('password1'), '密码需同时包含字母、数字和特殊字符。')
  assert.equal(getBackupPasswordValidationMessage('Password1 '), '密码需同时包含字母、数字和特殊字符。')
  assert.equal(getBackupPasswordValidationMessage('密码123456!'), '密码需同时包含字母、数字和特殊字符。')
  assert.equal(getBackupPasswordValidationMessage('Password1!'), null)
  assert.deepEqual(
    getPasswordChecks('Password1!').map(item => [item.id, item.label, item.completed]),
    [
      ['length', '8–32 位', true],
      ['letter', '包含字母', true],
      ['number', '包含数字', true],
      ['special', '包含特殊字符', true],
    ],
  )
})

test('backup password strength gives visible progress states', () => {
  assert.deepEqual(getBackupPasswordStrength(''), { label: '弱', tone: 'muted', passedCount: 0 })
  assert.equal(getBackupPasswordStrength('password').label, '中')
  assert.equal(getBackupPasswordStrength('Password1').label, '中')
  assert.equal(getBackupPasswordStrength('LongerPassword1!').label, '强')
})
