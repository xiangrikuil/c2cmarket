import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { afterEach, describe, it, vi } from 'vitest'

const pickerSource = readFileSync(new URL('../../components/api-order/DisputeEvidencePicker.vue', import.meta.url), 'utf8')
const reportsAppealsPageSource = readFileSync(new URL('../../pages/MyReportsAppealsPage.vue', import.meta.url), 'utf8')
const adapterSource = readFileSync(new URL('../disputeEvidenceBackend.ts', import.meta.url), 'utf8')

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

function adminSessionResponse() {
  return jsonResponse({
    audience: 'normal',
    csrfToken: 'csrf-evidence-admin',
    expiresAt: '2999-01-01T00:00:00Z',
    user: {
      id: 'admin-1',
      analyticsUserId: 'analytics-admin-1',
      username: 'moderator',
      displayName: 'Moderator',
      isAdmin: true,
      permissions: ['admin:reports'],
      capabilities: ['admin.access'],
      linuxDoBinding: { bound: true },
    },
  })
}

async function loadBackend(fetchMock: ReturnType<typeof vi.fn>) {
  vi.resetModules()
  vi.stubGlobal('fetch', fetchMock)
  const client = await import('../backendClient')
  client.setBackendRuntimeConfig({ apiMode: 'real' })
  const backend = await import('../disputeEvidenceBackend')
  return { backend }
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.resetModules()
})

describe('私有纠纷图片证据适配器', () => {
  it('为数组模型使用工厂默认值，保持 Vue 类型推断稳定', () => {
    assert.match(pickerSource, /defineModel<DisputeEvidenceAsset\[\]>\(\{ default: \(\) => \[\] \}\)/)
    assert.doesNotMatch(pickerSource, /defineModel<DisputeEvidenceAsset\[\]>\(\{ default: \[\] \}\)/)
  })

  it('上传前要求显式脱敏确认，并把确认值提交给后端', () => {
    assert.match(pickerSource, /<Checkbox v-model="redactionConfirmed"/)
    assert.match(pickerSource, /remaining === 0 \|\| !redactionConfirmed/)
    assert.match(pickerSource, /完整账号信息及所有二维码/)
    assert.match(adapterSource, /if \(!redactionConfirmed\) throw new Error/)
    assert.match(adapterSource, /form\.set\('redactionConfirmed', 'true'\)/)
  })

  it('按材料用途明确说明实际可见范围', () => {
    assert.match(pickerSource, /visibility: 'participants_admin'/)
    assert.match(pickerSource, /仅材料提交者和管理员可见，纠纷另一方不可见/)
    assert.match(pickerSource, /仅申诉人和管理员可见，纠纷另一方不可见/)
    assert.match(pickerSource, /纠纷双方和管理员可见/)
    assert.match(reportsAppealsPageSource, /visibility="appellant_admin"/)
    assert.match(reportsAppealsPageSource, /visibility="submitter_admin"/)
  })

  it('管理员隔离使用 CSRF、If-Match 和幂等键，正文只包含原因', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(adminSessionResponse())
      .mockResolvedValueOnce(jsonResponse({
        id: 'asset-1',
        status: 'quarantined',
        quarantinedExpiresAt: '2026-08-20T12:00:00Z',
        version: 2,
      }))
    const { backend } = await loadBackend(fetchMock)

    const result = await backend.quarantineDisputeEvidence('asset/1', 1, '图片包含完整账号信息。')

    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit]
    const headers = new Headers(init.headers)
    assert.equal(url, '/api/v1/admin/dispute-evidence/asset%2F1/quarantine')
    assert.equal(init.method, 'POST')
    assert.equal(headers.get('X-CSRF-Token'), 'csrf-evidence-admin')
    assert.equal(headers.get('If-Match'), '"1"')
    assert.match(headers.get('Idempotency-Key') ?? '', /^admin-dispute-evidence-quarantine-/)
    assert.deepEqual(JSON.parse(String(init.body)), { reason: '图片包含完整账号信息。' })
    assert.equal(String(init.body).includes('objectKey'), false)
    assert.equal(String(init.body).includes('contentPath'), false)
    assert.equal(result.status, 'quarantined')
  })
})
