import assert from 'node:assert/strict'
import { afterEach, test, vi } from 'vitest'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

function backendSession() {
  return {
    csrfToken: 'csrf-payment',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'user-payment',
      username: 'seller',
      displayName: 'Seller',
      isAdmin: false,
      permissions: [],
      linuxDoBinding: { bound: true },
    },
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

test('real account payment adapter reads normalized backend settings', async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(backendSession()))
    .mockResolvedValueOnce(jsonResponse({
      paymentWindowMinutes: 10,
      paymentOptions: [
        {
          paymentMethod: 'wechat',
          enabled: true,
          paymentInstructions: '',
          paymentQrCodeDataUrl: 'data:image/png;base64,d2VjaGF0',
        },
        {
          paymentMethod: 'alipay',
          enabled: false,
          paymentInstructions: '备用资料',
        },
      ],
      updatedAt: '2026-07-30T09:00:00Z',
    }))
  vi.stubGlobal('fetch', fetchMock)

  vi.resetModules()
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendGetApiPaymentAccountSettings } = await import('../apiPaymentSettingsBackend')
  const settings = await backendGetApiPaymentAccountSettings()

  assert.equal(fetchMock.mock.calls[0]?.[0], '/api/v1/auth/session')
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/me/api-payment-settings')
  assert.deepEqual(settings.paymentOptions.map(option => option.paymentMethod), ['wechat', 'alipay'])
  assert.deepEqual(settings.paymentOptions.map(option => option.enabled), [true, false])
})

test('real account payment adapter persists one method with PUT and CSRF', async () => {
  const response = {
    paymentWindowMinutes: 10,
    paymentOptions: [
      {
        paymentMethod: 'wechat',
        enabled: false,
        paymentInstructions: '备用资料',
        paymentQrCodeDataUrl: 'data:image/png;base64,d2VjaGF0',
      },
      {
        paymentMethod: 'alipay',
        enabled: true,
        paymentInstructions: '',
        paymentQrCodeDataUrl: 'data:image/png;base64,YWxpcGF5',
      },
    ],
    updatedAt: '2026-07-30T09:05:00Z',
  }
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(jsonResponse(backendSession()))
    .mockResolvedValueOnce(jsonResponse(response))
  vi.stubGlobal('fetch', fetchMock)

  vi.resetModules()
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const { backendUpdateApiPaymentAccountSettings } = await import('../apiPaymentSettingsBackend')
  const settings = await backendUpdateApiPaymentAccountSettings({
    paymentWindowMinutes: 10,
    paymentOptions: response.paymentOptions.map(option => ({
      ...option,
      paymentMethod: option.paymentMethod as 'wechat' | 'alipay',
    })),
  })

  const [, request] = fetchMock.mock.calls[1] ?? []
  assert.equal(fetchMock.mock.calls[1]?.[0], '/api/v1/me/api-payment-settings')
  assert.equal(request?.method, 'PUT')
  assert.equal(new Headers(request?.headers).get('X-CSRF-Token'), 'csrf-payment')
  assert.deepEqual(JSON.parse(String(request?.body)), {
    paymentWindowMinutes: 10,
    paymentOptions: response.paymentOptions,
  })
  assert.deepEqual(settings.paymentOptions.map(option => option.enabled), [false, true])
})
