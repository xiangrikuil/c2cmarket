import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const healthPanel = source('../../components/api-market/ApiServiceHealthPanel.vue')
const connectionPage = source('../../pages/MyApiProbeConnectionsPage.vue')
const ownerPage = source('../../pages/MyApiServiceDetailPage.vue')
const publishSection = source('../../components/api-service-publish/ProbeConnectionSection.vue')
const router = source('../../router.ts')
const appShell = source('../../components/layout/AppShell.vue')
const adminShell = source('../../components/layout/AdminShell.vue')
const query = source('../../queries/useApiHealthQueries.ts')
const backend = source('../apiHealthBackend.ts')

describe('共享探针连接前端边界', () => {
  it('公共健康只描述连接鉴权可用性，不展示模型或 TTFT', () => {
    expect(healthPanel).toContain('Array.from({ length: 12 }')
    expect(healthPanel).toContain('探针连接可用性')
    expect(healthPanel).toContain('不代表任一具体模型可调用')
    expect(healthPanel).toContain('aria-label="最近一小时五分钟探测槽"')
    expect(healthPanel).toContain("transportSecurity === 'insecure_http'")
    expect(healthPanel).toContain('HTTP 未加密')
    expect(healthPanel).not.toContain('probeModel')
    expect(healthPanel).not.toContain('medianTtftMs')
  })

  it('连接管理集中处理复用、写入专用 Key 和 HTTP 主动确认', () => {
    expect(connectionPage).toContain('PageTitle title="探针连接"')
    expect(connectionPage.match(/<template #action>/g)).toHaveLength(2)
    expect(connectionPage).toContain('type="password"')
    expect(connectionPage).toContain('autocomplete="new-password"')
    expect(connectionPage).toContain('留空则保持当前 Key')
    expect(connectionPage).toContain('已有相同 Base URL 的连接')
    expect(connectionPage).toContain('复用此连接')
    expect(connectionPage).toContain('仍创建独立连接')
    expect(connectionPage).toContain('确认使用未加密 HTTP')
    expect(connectionPage).toContain('不会验证服务器所有权或具体模型')
    expect(connectionPage).not.toContain('DNS TXT')
    expect(connectionPage).not.toContain('HTTP challenge')
  })

  it('服务发布选择 ready 连接，详情页支持改绑和解绑', () => {
    expect(publishSection).toContain("verificationStatus === 'verified'")
    expect(publishSection).toContain('管理连接')
    expect(publishSection).toContain('新建连接')
    expect(ownerPage).toContain('useUpdateApiServiceProbeConnectionMutation')
    expect(ownerPage).toContain('探针连接已改绑')
    expect(ownerPage).toContain("probeConnectionId: ''")
    expect(ownerPage).toContain('已有订单不受影响')
  })

  it('Owner mutation 使用 If-Match/幂等键并失效连接和服务查询', () => {
    expect(backend).toContain("{ method: 'PUT', ifMatch: input.version }")
    expect(backend).toContain("{ method: 'DELETE', ifMatch: input.version }")
    expect(backend).toContain("idempotencyPrefix: 'api-probe-connection-verify'")
    expect(query).toContain('apiHealthQueryKeys.all')
    expect(query).toContain("queryKey: ['api-services']")
  })

  it('独立 Owner 路由与导航存在，管理员审批入口已删除', () => {
    expect(router).toContain("path: '/my/api-probe-connections'")
    expect(router).not.toContain("path: '/admin/api-health-probes'")
    expect(appShell).toContain("{ label: '探针连接', to: '/my/api-probe-connections'")
    expect(adminShell).not.toContain('API 探针授权')
  })
})
