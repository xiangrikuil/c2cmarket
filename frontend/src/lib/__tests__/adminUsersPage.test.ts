import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

describe('管理员用户目录页面', () => {
  const page = source('../../pages/AdminUsersPage.vue')
  const queries = source('../../queries/useAdminUserQueries.ts')
  const shell = source('../../components/layout/AdminShell.vue')

  it('列表由 URL 和后端分页元数据驱动', () => {
    expect(page).toContain('normalizeAdminUserDirectoryQuery(route.query')
    expect(page).toContain('pagination?.totalItems')
    expect(page).toContain('@update:page="replaceDirectoryQuery')
    expect(page).not.toContain('usePagination')
    expect(page).not.toContain('.slice(')
    expect(page).not.toContain('visibleRows')
    expect(queries).toContain('placeholderData: keepPreviousData')
  })

  it('筛选变更回到第一页并修正越界页码', () => {
    expect(page).toContain('set: value => replaceDirectoryQuery({ status:')
    expect(page).toContain('resetPage ? 1')
    expect(page).toContain('directoryQuery.value.page > lastValidPage')
    expect(page).toContain('replaceDirectoryQuery({ page: lastValidPage })')
  })

  it('详情抽屉按治理职责分区并要求原因和二次确认', () => {
    expect(page).toContain('AdminReputationAuditPanel')
    expect(page).toContain('<TabsTrigger value="overview">概览</TabsTrigger>')
    expect(page).toContain('<TabsTrigger value="governance">账号治理</TabsTrigger>')
    expect(page).toContain('<TabsTrigger value="capabilities">能力限制</TabsTrigger>')
    expect(page).toContain('<TabsTrigger value="audit">审计记录</TabsTrigger>')
    expect(page).toContain('打开公开主页')
    expect(page).toContain('请填写操作原因')
    expect(page).toContain('请完成二次确认')
    expect(page).toContain('impactPreview.activeSessions')
    expect(page).toContain('auditTransition(entry)')
  })

  it('治理按钮完全消费服务端动作，不复制状态机或管理员总数判断', () => {
    expect(page).toContain('selectedDetail.value?.availableActions')
    expect(page).toContain('action.blockedReason')
    expect(page).not.toContain('summary.value?.adminUsers === 1')
    expect(page).not.toContain('const allowed: Record<AdminUserStatus')
  })

  it('用户目录只保留一处搜索入口并压缩表格行操作', () => {
    expect(shell).toContain("route.path !== '/admin/users'")
    expect(page).toContain('@click="openManagement(user)"')
    expect(page).toContain('查看\n              </Button>')
    expect(page).not.toContain('>信誉审计</Button>')
  })
})
