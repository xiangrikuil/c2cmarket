import { ref } from 'vue'
import { describe, expect, it } from 'vitest'
import { useOwnerAPIHealthProbeForm } from '@/composables/useOwnerAPIHealthProbeForm'
import type { OwnerAPIHealthProbeConfig } from '@/types/apiHealth'

function probe(): OwnerAPIHealthProbeConfig {
  return {
    id: 'probe-1',
    apiServiceId: 'service-1',
    protocol: 'openai_chat_completions_v1',
    baseUrl: 'https://api.example.test/v1',
    normalizedOrigin: 'https://api.example.test:443',
    model: 'gpt-5-mini',
    credentialConfigured: true,
    enabled: true,
    authorizationStatus: 'approved',
    authorizationMethod: 'admin_approval',
    verifiedOrigin: 'https://api.example.test:443',
    verifiedAt: '2026-08-04T00:00:00Z',
    approvedAt: '2026-08-04T00:00:00Z',
    rejectionReason: null,
    challengeExpiresAt: null,
    measurementVersion: 1,
    lastConfigErrorCode: null,
    version: 4,
    createdAt: '2026-08-04T00:00:00Z',
    updatedAt: '2026-08-04T00:00:00Z',
  }
}

describe('Owner 探针表单', () => {
  it('从配置同步可编辑字段但不会回填 credential', () => {
    const config = ref<OwnerAPIHealthProbeConfig | null>(probe())
    const form = useOwnerAPIHealthProbeForm(config)
    expect(form.baseUrl.value).toBe('https://api.example.test/v1')
    expect(form.model.value).toBe('gpt-5-mini')
    expect(form.credential.value).toBe('')
    expect(form.payload('service-1')).not.toHaveProperty('credential')
  })

  it('只把本次输入的 credential 放入 payload，并可立即清空', () => {
    const config = ref<OwnerAPIHealthProbeConfig | null>(probe())
    const form = useOwnerAPIHealthProbeForm(config)
    form.credential.value = '  one-time-probe-key  '
    expect(form.payload('service-1')).toMatchObject({ credential: 'one-time-probe-key', version: 4 })
    form.clearCredential()
    expect(form.credential.value).toBe('')
  })

  it('首次启用要求凭据并拒绝非 HTTPS 或带查询参数的地址', () => {
    const config = ref<OwnerAPIHealthProbeConfig | null>(null)
    const form = useOwnerAPIHealthProbeForm(config)
    form.baseUrl.value = 'http://api.example.test/v1?token=x'
    form.model.value = 'gpt-5-mini'
    form.enabled.value = true
    expect(form.validation.value.baseUrl).toContain('HTTPS')
    expect(form.validation.value.credential).toContain('首次启用')
    expect(form.valid.value).toBe(false)
  })
})
