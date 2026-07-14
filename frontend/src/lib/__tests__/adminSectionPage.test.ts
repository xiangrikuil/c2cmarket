import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const adminSectionSource = readFileSync(new URL('../../pages/AdminSectionPage.vue', import.meta.url), 'utf8')
const reportBackendSource = readFileSync(new URL('../reportBackend.ts', import.meta.url), 'utf8')

describe('管理列表数据状态', () => {
  it('在请求失败时显示错误而不是空记录，并移除固定演示统计', () => {
    expect(adminSectionSource).toContain('const { data, error, isFetching, isLoading, refetch } = useAdminSectionRows(section)')
    expect(adminSectionSource).toContain('v-else-if="error"')
    expect(adminSectionSource).toContain('管理数据读取失败')
    expect(adminSectionSource).toContain('@click="refetch()"')
    expect(adminSectionSource).toContain('本页记录')
    expect(adminSectionSource).toContain('当前筛选')
    expect(adminSectionSource).toContain('function requiresAdminAction')
    expect(adminSectionSource).toContain("if (row.targetType === 'appeal') return row.status === '申诉复核中'")
    expect(adminSectionSource).not.toContain('>3</div></Card>')
    expect(adminSectionSource).not.toContain('>12</div></Card>')
  })

  it('举报纠纷队列包含角标统计覆盖的待处理申诉', () => {
    expect(reportBackendSource).toContain('const [reports, disputes, appeals] = await Promise.all([')
    expect(reportBackendSource).toContain('...appeals.items.map(mapAppealRow)')
  })
})
