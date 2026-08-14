import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { initialSidebarCollapsed } from '@/composables/usePersistentSidebar'

const appSource = readFileSync(new URL('../../App.vue', import.meta.url), 'utf8')
const appShellSource = readFileSync(new URL('../../components/layout/AppShell.vue', import.meta.url), 'utf8')
const adminShellSource = readFileSync(new URL('../../components/layout/AdminShell.vue', import.meta.url), 'utf8')
const persistentSidebarSource = readFileSync(new URL('../../composables/usePersistentSidebar.ts', import.meta.url), 'utf8')

describe('独立管理端与渐进导航', () => {
  it('根据路由选择独立管理壳', () => {
    expect(appSource).toContain("route.path.startsWith('/admin')")
    expect(appSource).toContain('<AdminShell v-else-if="adminLayout">')
    expect(adminShellSource).toContain("title: '待办与治理'")
    expect(adminShellSource).toContain("title: '市场目录'")
    expect(adminShellSource).toContain("title: '交易与用户'")
    expect(adminShellSource).toContain("title: '内容与系统'")
    expect(adminShellSource).toContain("{ label: '增长推广', to: '/admin/growth-promotions'")
    expect(adminShellSource).toContain("{ label: '车源管理', to: '/admin/carpools'")
    expect(adminShellSource).toContain("{ label: 'API 服务管理', to: '/admin/api-services'")
    expect(adminShellSource).toContain('后台全局搜索')
    expect(adminShellSource).toContain('返回用户端')
  })

  it('普通侧栏不混排后台目录', () => {
    expect(appShellSource).toContain("{ label: '进入管理台', to: '/admin'")
    for (const label of ['套餐目录', 'API 模型目录', '官网价格维护', '车源管理', '举报纠纷']) {
      expect(appShellSource).not.toContain(`{ label: '${label}', to: '/admin`)
    }
  })

  it('折叠状态持久化且窄屏默认折叠', () => {
    expect(initialSidebarCollapsed(null, 1023)).toBe(true)
    expect(initialSidebarCollapsed(null, 1440)).toBe(false)
    expect(initialSidebarCollapsed('false', 390)).toBe(false)
    expect(initialSidebarCollapsed('true', 1440)).toBe(true)
    expect(appShellSource).toContain("usePersistentSidebar('c2c-user-sidebar-collapsed')")
    expect(adminShellSource).toContain("usePersistentSidebar('c2c-admin-sidebar-collapsed')")
  })

  it('hydration 前使用服务端稳定的展开状态', () => {
    expect(persistentSidebarSource).toContain('const sidebarCollapsed = ref(false)')
    expect(persistentSidebarSource).toMatch(/onMounted\(\(\) => \{[\s\S]*?window\.localStorage\.getItem\(storageKey\)/)
  })

  it('移动抽屉支持语义和 Escape 关闭', () => {
    for (const source of [appShellSource, adminShellSource]) {
      expect(source).toContain('role="dialog"')
      expect(source).toContain('aria-modal="true"')
      expect(source).toContain("event.key === 'Escape'")
    }
  })
})
