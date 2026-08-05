import assert from 'node:assert/strict'
import { test } from 'vitest'
import {
  accountRecoveryRequirements,
  isAccountRecoveryAllowedPath,
  isAccountRecoveryComplete,
  outstandingAccountRecoveryRequirements,
  sanitizeAccountRecoveryReturnTo,
  shouldRedirectToAccountRecovery,
} from '../accountRecovery'

const completeProfile = {
  emailVerified: true,
  passwordConfigured: true,
  linuxDoBinding: { bound: true },
}

const incompleteProfile = {
  emailVerified: false,
  passwordConfigured: false,
  linuxDoBinding: { bound: true },
}

test('linux.do account recovery requires both verified email and backup password', () => {
  assert.equal(isAccountRecoveryComplete(completeProfile), true)
  assert.equal(isAccountRecoveryComplete(incompleteProfile), false)
  assert.deepEqual(
    outstandingAccountRecoveryRequirements(incompleteProfile).map(item => item.id),
    ['email', 'password'],
  )
  assert.deepEqual(
    accountRecoveryRequirements({ emailVerified: true, passwordConfigured: false, linuxDoBinding: { bound: true } }).map(item => [item.id, item.completed]),
    [['email', true], ['password', false]],
  )
})

test('unbound account recovery does not require or expose a backup password', () => {
  const unboundProfile = {
    emailVerified: true,
    passwordConfigured: false,
    linuxDoBinding: { bound: false },
  }

  assert.equal(isAccountRecoveryComplete(unboundProfile), true)
  assert.deepEqual(accountRecoveryRequirements(unboundProfile).map(item => item.id), ['email'])
  assert.equal(isAccountRecoveryComplete({ ...unboundProfile, emailVerified: false }), false)
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

test('account recovery redirects only authenticated workspace and transaction routes', () => {
  assert.equal(shouldRedirectToAccountRecovery('/carpools', undefined), false)
  assert.equal(shouldRedirectToAccountRecovery('/api-market/service-1', undefined), false)
  assert.equal(shouldRedirectToAccountRecovery('/official-prices/p1', undefined), false)
  assert.equal(shouldRedirectToAccountRecovery('/carpools/new', 'user'), true)
  assert.equal(shouldRedirectToAccountRecovery('/my/api-orders', 'user'), true)
  assert.equal(shouldRedirectToAccountRecovery('/admin', 'admin'), true)
  assert.equal(shouldRedirectToAccountRecovery('/my/account', 'user'), false)
})

test('account recovery return target stays internal and skips allowed setup pages', () => {
  assert.equal(sanitizeAccountRecoveryReturnTo('/carpools/new?source=nav'), '/carpools/new?source=nav')
  assert.equal(sanitizeAccountRecoveryReturnTo('/my/api-services?intent=quota'), '/my/api-services?intent=quota')
  assert.equal(sanitizeAccountRecoveryReturnTo('/my/account'), null)
  assert.equal(sanitizeAccountRecoveryReturnTo('/u/orbit'), null)
  assert.equal(sanitizeAccountRecoveryReturnTo('https://example.test/carpools'), null)
  assert.equal(sanitizeAccountRecoveryReturnTo('//example.test/carpools'), null)
})
