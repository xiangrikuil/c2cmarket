import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const healthPanel = source('../../components/api-market/ApiServiceHealthPanel.vue')
const ownerPanel = source('../../components/api-service-owner/OwnerAPIHealthProbePanel.vue')
const ownerPage = source('../../pages/MyApiServiceDetailPage.vue')
const adminPage = source('../../pages/AdminAPIHealthProbeReviewPage.vue')
const router = source('../../router.ts')
const adminShell = source('../../components/layout/AdminShell.vue')
const query = source('../../queries/useApiHealthQueries.ts')
const backend = source('../apiHealthBackend.ts')

describe('API 健康探针前端边界', () => {
  it('公共健康摘要固定渲染 12 槽并披露模型和单节点局限', () => {
    expect(healthPanel).toContain('Array.from({ length: 12 }')
    expect(healthPanel).toContain('平台探测')
    expect(healthPanel).toContain('仅代表当前模型与平台单节点')
    expect(healthPanel).toContain('aria-label="最近一小时五分钟探测槽"')
    expect(healthPanel).toContain('api-service-health-panel__metrics')
    expect(healthPanel).toContain('successRatePercent === null')
    expect(healthPanel).toContain("transportSecurity === 'insecure_http'")
    expect(healthPanel).toContain('HTTP 未加密')
    expect(healthPanel).toContain('可能在传输途中被读取或篡改')
    expect(healthPanel).not.toContain('min-h-[168px]')
  })

  it('Owner 详情接入独立探针面板且 credential 永不回填', () => {
    expect(ownerPage).toContain('<OwnerAPIHealthProbePanel')
    expect(ownerPanel).toContain('type="password"')
    expect(ownerPanel).toContain('autocomplete="new-password"')
    expect(ownerPanel).toContain('form.clearCredential()')
    expect(ownerPanel).toContain('probe?.credentialConfigured')
    expect(ownerPanel).toContain('API 请求地址（Base URL）')
    expect(ownerPanel).toContain('仅填写域名时自动补 /v1；已有路径保持不变。')
    expect(ownerPanel).toContain('探针专用 API Key')
    expect(ownerPanel).toContain('低额度、仅开放探测模型')
    expect(ownerPanel).toContain('HTTP 请求不会加密传输')
    expect(ownerPanel).toContain('可能被链路中的第三方读取或篡改')
    expect(ownerPanel).toContain('额度损失风险可接受')
    expect(ownerPanel).toContain('form.requiresInsecureHttpAcknowledgement.value')
    expect(ownerPanel).toContain('当前 HTTP 地址的未加密传输风险已确认')
    expect(ownerPanel).toContain('平台暂时无法读取探针专用 API Key')
    expect(ownerPanel).not.toContain('OpenAI 兼容 Base URL')
    expect(ownerPanel).not.toContain('探针专用凭据')
    expect(ownerPanel).not.toContain('credentialFingerprint')
    expect(ownerPanel).not.toContain('credentialCiphertext')
    expect(ownerPanel).not.toContain('v-model="probe')
  })

  it('Owner 删除探针配置使用统一确认对话框', () => {
    expect(ownerPanel).toContain('<Dialog v-model:open="deleteDialogOpen">')
    expect(ownerPanel).toContain('<DialogTitle>删除探针配置</DialogTitle>')
    expect(ownerPanel).toContain('@click="deleteConfig"')
    expect(ownerPanel).not.toContain('window.confirm')
  })

  it('Owner 与 Admin mutation 使用 If-Match facade 并失效相关查询', () => {
    expect(backend).toContain("{ method: 'PUT', ifMatch: input.version }")
    expect(backend).toContain("{ method: 'DELETE', ifMatch: input.version }")
    expect(backend).toContain('ifMatch: input.version')
    expect(query).toContain("queryClient.invalidateQueries({ queryKey: apiHealthQueryKeys.all })")
    expect(query).toContain("queryClient.invalidateQueries({ queryKey: ['api-services'] })")
  })

  it('Admin 使用独立受保护路由、管理导航和最小安全投影', () => {
    expect(router).toContain("path: '/admin/api-health-probes'")
    expect(router).toContain('component: AdminAPIHealthProbeReviewPage, meta: adminAuthMeta')
    expect(adminShell).toContain("{ label: 'API 探针授权', to: '/admin/api-health-probes'")
    expect(adminPage).toContain('精确 Origin')
    expect(adminPage).toContain('serviceLabel(row)')
    expect(adminPage).toContain('ownerLabel(row)')
    expect(adminPage).not.toContain('credential')
    expect(adminPage).not.toContain('fingerprint')
    expect(adminPage).not.toContain('row.baseUrl')
    expect(adminPage).not.toContain('row.model')
  })
})
