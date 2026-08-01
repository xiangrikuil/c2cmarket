import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

describe('管理员用户目录页面', () => {
  const page = source('../../pages/AdminUsersPage.vue')
  const queries = source('../../queries/useAdminUserQueries.ts')

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

  it('保留公开主页与信誉审计并要求治理原因和二次确认', () => {
    expect(page).toContain('AdminReputationAuditPanel')
    expect(page).toContain('打开公开主页')
    expect(page).toContain('请填写操作原因')
    expect(page).toContain('请完成二次确认')
    expect(page).toContain('不能修改自己的账号状态或管理员权限')
    expect(page).toContain('最后一个有效管理员')
  })
})
