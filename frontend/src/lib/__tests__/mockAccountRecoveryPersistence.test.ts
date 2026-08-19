import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

type API = typeof import('../api')

async function loadMockAPI(): Promise<API> {
  vi.resetModules()
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'mock' })
  return import('../api')
}

afterEach(() => {
  vi.restoreAllMocks()
  vi.resetModules()
})

test('mock account recovery completion survives profile refetches for the same development identity', async () => {
  const api = await loadMockAPI()
  const mockAuth = await import('../mockAuth')
  const initial = await api.getMyProfile()
  assert.equal(initial.emailVerified, false)
  assert.equal(initial.passwordConfigured, false)

  const challenge = await api.startEmailVerification('orbit@example.com')
  await api.confirmEmailVerification({ email: challenge.email, code: challenge.devCode ?? '123456' })

  const afterEmailRefetch = await api.getMyProfile()
  assert.equal(afterEmailRefetch.email, 'orbit@example.com')
  assert.equal(afterEmailRefetch.emailVerified, true)
  assert.equal(afterEmailRefetch.passwordConfigured, false)

  mockAuth.setMockPersona('admin')
  assert.equal((await api.getMyProfile()).emailVerified, false)
  mockAuth.setMockPersona('linuxdo')
  assert.equal((await api.getMyProfile()).emailVerified, true)

  await api.setBackupPassword({ newPassword: 'Mock-password-2!' })

  const afterPasswordRefetch = await api.getMyProfile()
  assert.equal(afterPasswordRefetch.emailVerified, true)
  assert.equal(afterPasswordRefetch.passwordConfigured, true)
})
