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

test('mock WeChat contact can be disabled and deleted', async () => {
  const api = await loadMockAPI()
  const current = (await api.getMyContactMethods()).find(contact => contact.type === 'wechat')
  assert.ok(current)

  const disabled = await api.updateContactMethod(current.id, {
    type: 'wechat',
    label: '微信',
    displayValue: 'updated_wechat',
    isDefault: current.isDefault,
    enabled: false,
  })
  assert.equal(disabled.enabled, false)

  await api.deleteContactMethod(current.id)
  assert.equal((await api.getMyContactMethods()).some(contact => contact.id === current.id), false)
})

test('mock accepts verified email and rejects unverified email for a transaction', async () => {
  const api = await loadMockAPI()
  const verified = (await api.getMyContactMethods()).find(contact => contact.type === 'email' && contact.verified)
  assert.ok(verified)

  const unverified = await api.createContactMethod({
    type: 'email',
    label: '备用邮箱',
    displayValue: 'unverified@example.com',
    isDefault: false,
    enabled: true,
  })
  assert.equal(unverified.verified, false)

  await assert.rejects(
    () => api.createApiPurchaseIntent({
      serviceId: 'a1',
      buyerContactMethodId: unverified.id,
      purchaseAmountCny: 10,
      deliveryMode: 'api_key_endpoint',
      targetModel: 'GPT-5 mini',
    }),
    /请选择有效的交易联系方式/,
  )
})
