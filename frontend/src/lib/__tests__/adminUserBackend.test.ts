import { describe, expect, it } from 'vitest'
import { mapBackendAdminUser } from '@/lib/adminUserBackend'

describe('管理员用户目录适配', () => {
  it('只映射账号目录字段，不把举报案件混入用户行', () => {
    const row = mapBackendAdminUser({
      id: 'user-1',
      username: 'alice',
      displayName: 'Alice',
      accountStatus: 'active',
      isAdmin: false,
      linuxDoBound: true,
      trustLevel: 3,
      createdAt: '2026-07-11T08:00:00Z',
      lastActiveAt: '2026-07-11T09:00:00Z',
    })

    expect(row).toMatchObject({
      id: 'user-1',
      primary: 'alice',
      owner: '普通账号',
      status: '正常',
      targetType: 'user',
      targetTo: '/u/alice',
    })
    expect(row.secondary).toContain('已绑定 linux.do')
    expect(row.risk).toContain('注册')
    expect(row.risk).not.toContain('举报')
    expect(row.detailItems?.map(item => item.label)).not.toContain('举报')
  })
})
