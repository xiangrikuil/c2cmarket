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

test('mock WeChat contact always covers every usage scope and cannot be disabled, converted, or deleted', async () => {
  const api = await loadMockAPI()
  const current = (await api.getMyContactMethods()).find(contact => contact.type === 'wechat')
  assert.ok(current)
  assert.deepEqual(current.usageScopes, ['carpool_owner', 'api_merchant', 'buyer', 'dispute'])

  const updated = await api.updateContactMethod(current.id, {
    type: 'wechat',
    label: '微信',
    displayValue: 'updated_wechat',
    usageScopes: ['buyer'],
    isDefault: current.isDefault,
    enabled: true,
  })
  assert.deepEqual(updated.usageScopes, ['carpool_owner', 'api_merchant', 'buyer', 'dispute'])

  await assert.rejects(api.updateContactMethod(current.id, {
    type: 'wechat',
    label: '微信',
    displayValue: updated.displayValue,
    usageScopes: updated.usageScopes,
    isDefault: updated.isDefault,
    enabled: false,
  }), /不能停用/)

  await assert.rejects(api.updateContactMethod(current.id, {
    type: 'email',
    label: '邮箱',
    displayValue: 'changed@example.com',
    usageScopes: ['buyer'],
    isDefault: false,
    enabled: true,
  }), /不能转换/)

  await assert.rejects(api.deleteContactMethod(current.id), /不能删除/)
})
