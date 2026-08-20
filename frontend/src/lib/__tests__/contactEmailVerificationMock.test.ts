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

test('mock contact email challenge survives metadata changes but rejects version drift', async () => {
  const api = await loadMockAPI()
  const first = await api.createContactMethod({
    type: 'email',
    label: '交易邮箱',
    displayValue: 'metadata@example.com',
    isDefault: false,
    enabled: true,
  })
  const metadataChallenge = await api.startContactEmailVerification(first.id)
  await api.updateContactMethod(first.id, {
    type: 'email',
    label: '订单邮箱',
    displayValue: 'metadata@example.com',
    isDefault: true,
    enabled: true,
  })
  const metadataVerified = await api.confirmContactEmailVerification(first.id, metadataChallenge.devCode ?? '')
  assert.equal(metadataVerified.verified, true)

  const second = await api.createContactMethod({
    type: 'email',
    label: '备用邮箱',
    displayValue: 'version-a@example.com',
    isDefault: false,
    enabled: true,
  })
  const staleChallenge = await api.startContactEmailVerification(second.id)
  await api.updateContactMethod(second.id, {
    type: 'email',
    label: '备用邮箱',
    displayValue: 'version-b@example.com',
    isDefault: false,
    enabled: true,
  })
  await api.updateContactMethod(second.id, {
    type: 'email',
    label: '备用邮箱',
    displayValue: 'version-a@example.com',
    isDefault: false,
    enabled: true,
  })

  await assert.rejects(
    api.confirmContactEmailVerification(second.id, staleChallenge.devCode ?? ''),
    /验证码无效或已过期/,
  )
})

test('mock contact email challenge locks after five wrong attempts', async () => {
  const api = await loadMockAPI()
  const contact = await api.createContactMethod({
    type: 'email',
    label: '交易邮箱',
    displayValue: 'attempts@example.com',
    isDefault: false,
    enabled: true,
  })
  const challenge = await api.startContactEmailVerification(contact.id)

  for (let attempt = 0; attempt < 5; attempt += 1) {
    await assert.rejects(api.confirmContactEmailVerification(contact.id, '000000'), /验证码无效或已过期/)
  }
  await assert.rejects(
    api.confirmContactEmailVerification(contact.id, challenge.devCode ?? ''),
    /验证码无效或已过期/,
  )
})
