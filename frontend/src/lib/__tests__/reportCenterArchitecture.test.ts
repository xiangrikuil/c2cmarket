import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const source = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')
const pageSource = source('../../pages/MyReportsAppealsPage.vue')
const routerSource = source('../../router.ts')
const appShellSource = source('../../components/layout/AppShell.vue')
const myCenterSource = source('../../pages/MyCenterPage.vue')
const querySource = source('../../queries/useReportQueries.ts')

describe('举报与申诉中心前端闭环', () => {
  it('注册受保护列表和详情路由并提供统一账户导航', () => {
    expect(routerSource).toContain("path: '/my/reports', name: 'my-reports', component: MyReportsAppealsPage, meta: userAuthMeta")
    expect(routerSource).toContain("path: '/my/reports/:kind/:id', name: 'my-report-detail', component: MyReportsAppealsPage, meta: userAuthMeta")
    expect(appShellSource).toContain("{ label: '举报与申诉', to: '/my/reports', count: null, icon: Siren }")
    expect(myCenterSource).toContain("router.push('/my/reports')")
    expect(myCenterSource).not.toContain("toast('申诉请求已记录。')")
  })

  it('三类真实查询具有明确加载、失败和空状态', () => {
    expect(pageSource).toContain('useMyReportsQuery()')
    expect(pageSource).toContain('useMyDisputesQuery()')
    expect(pageSource).toContain('useMyAppealsQuery()')
    expect(pageSource).toContain('<SkeletonTable v-if="isLoading"')
    expect(pageSource).toContain('<ErrorState v-else-if="hasError"')
    expect(pageSource).toContain('v-else-if="records.length === 0"')
    expect(pageSource).toContain("{ label: '我的举报', value: 'report' }")
    expect(pageSource).toContain("{ label: '相关纠纷', value: 'dispute' }")
    expect(pageSource).toContain("{ label: '我的申诉', value: 'appeal' }")
  })

  it('只展示现有公开结果与申诉状态，不虚构未实现能力', () => {
    expect(pageSource).toContain('selectedRecord.source.publicSummary')
    expect(pageSource).toContain('selectedRecord.source.publicResult')
    expect(pageSource).toContain('canCreateAppeal(selectedRecord')
    expect(pageSource).toContain('createAppealMutation.mutate(payload')
    expect(pageSource).not.toContain('处理时间线')
    expect(pageSource).not.toContain('追加材料')
    expect(pageSource).not.toContain('封禁账号申诉')
  })

  it('无效详情深链不回退到其他记录', () => {
    expect(pageSource).toContain('if (hasDetailRequest.value) return requestedRecord.value')
    expect(pageSource).toContain('该记录不存在或你无权查看。')
  })

  it('申诉成功只刷新权威查询和相关通知缓存', () => {
    expect(querySource).toContain('queryClient.setQueryData(myAppealsQueryKey')
    expect(querySource).toContain("queryKey: ['navigation-badges']")
    expect(querySource).toContain("queryKey: ['notifications']")
    expect(querySource).not.toContain('sessionStorage')
    expect(querySource).not.toContain("from '@/lib/api'")
  })
})
