import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  accountRecoveryRequirements,
  isAccountRecoveryAllowedPath,
  isAccountRecoveryComplete,
  outstandingAccountRecoveryRequirements,
  sanitizeAccountRecoveryReturnTo,
} from '../accountRecovery'

const completeProfile = {
  emailVerified: true,
  passwordConfigured: true,
}

const incompleteProfile = {
  emailVerified: false,
  passwordConfigured: false,
}

test('account recovery requires both verified email and password', () => {
  assert.equal(isAccountRecoveryComplete(completeProfile), true)
  assert.equal(isAccountRecoveryComplete(incompleteProfile), false)
  assert.deepEqual(
    outstandingAccountRecoveryRequirements(incompleteProfile).map(item => item.id),
    ['email', 'password'],
  )
  assert.deepEqual(
    accountRecoveryRequirements({ emailVerified: true, passwordConfigured: false }).map(item => [item.id, item.completed]),
    [['email', true], ['password', false]],
  )
})

test('account recovery allows only setup and public explanation paths before completion', () => {
  assert.equal(isAccountRecoveryAllowedPath('/'), true)
  assert.equal(isAccountRecoveryAllowedPath('/my/account'), true)
  assert.equal(isAccountRecoveryAllowedPath('/announcements/platform-rules'), true)
  assert.equal(isAccountRecoveryAllowedPath('/u/orbit'), true)
  assert.equal(isAccountRecoveryAllowedPath('/my'), false)
  assert.equal(isAccountRecoveryAllowedPath('/carpools'), false)
  assert.equal(isAccountRecoveryAllowedPath('/api-market/new'), false)
})

test('account recovery return target stays internal and skips allowed setup pages', () => {
  assert.equal(sanitizeAccountRecoveryReturnTo('/carpools/new?source=nav'), '/carpools/new?source=nav')
  assert.equal(sanitizeAccountRecoveryReturnTo('/my/account'), null)
  assert.equal(sanitizeAccountRecoveryReturnTo('/u/orbit'), null)
  assert.equal(sanitizeAccountRecoveryReturnTo('https://example.test/carpools'), null)
  assert.equal(sanitizeAccountRecoveryReturnTo('//example.test/carpools'), null)
})
