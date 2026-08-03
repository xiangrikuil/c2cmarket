import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const adminSectionSource = readFileSync(new URL('../../pages/AdminSectionPage.vue', import.meta.url), 'utf8')
const disputeDialogSource = readFileSync(new URL('../../components/admin/AdminDisputeResolutionDialog.vue', import.meta.url), 'utf8')
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
    expect(reportBackendSource).toContain("item.status !== 'dispute_opened' && !disputeReportIds.has(item.id)")
  })

  it('待补充记录保留合法动作，已裁决纠纷收敛为责任认定', () => {
    expect(adminSectionSource).toContain("if (row.targetType === 'report') return ['待处理', '已分诊', '需要补充信息'].includes(row.status)")
    expect(adminSectionSource).toContain("if (row.targetType === 'dispute') return ['处理中', '需要补充信息'].includes(row.status)")
    expect(reportBackendSource).toContain("publicResult: pathAction === 'request-info' ? '等待补充信息' : '案件已关闭，未作责任认定'")
    expect(reportBackendSource).not.toContain("reason || detail.publicResult || '已处理'")
    expect(adminSectionSource).toContain("openModerationDrawer(row, 'request_info')")
    expect(adminSectionSource).toContain('v-model="requestedFromUserId"')
    expect(adminSectionSource).toContain("runAdminModerationAction(row, backendAction, reason.value.trim(), requestedFromUserId.value)")
    expect(reportBackendSource).toContain("throw new Error('举报补充信息只能指定当前举报人。')")
    expect(reportBackendSource).toContain("throw new Error('请选择当前纠纷中的有效参与者补充信息。')")
  })

  it('管理员详情展示用户补充正文但不复用公开列表字段', () => {
    expect(adminSectionSource).toContain('drawerRow.moderationSupplements?.length')
    expect(adminSectionSource).toContain('用户补充材料')
    expect(disputeDialogSource).toContain('dispute.supplements?.length || report?.supplements?.length')
    expect(reportBackendSource).toContain('backendAdminModerationDetailRow')
  })

  it('纠纷详情和主操作进入专用两步裁决面板', () => {
    expect(adminSectionSource).toContain('AdminDisputeResolutionDialog')
    expect(adminSectionSource).toContain("if (row.targetType === 'dispute')")
    expect(adminSectionSource).toContain('@click="openDisputeResolution(row)"')
    expect(adminSectionSource).toContain("if (row.status === '已处理') return '责任认定'")
    expect(adminSectionSource).not.toContain("if (row?.targetType === 'dispute') return '标记处理'")
    expect(disputeDialogSource).toContain('backendResolveAdminDispute')
    expect(disputeDialogSource).toContain('useCreateDisputeReputationOutcomeMutation')
    expect(disputeDialogSource).toContain('基础裁决已保存，请继续责任认定。')
    expect(disputeDialogSource).toContain("['VERSION_CONFLICT', 'INVALID_STATE_TRANSITION'].includes(error.code)")
    expect(disputeDialogSource).toContain('resolutionSubmitting')
    expect(disputeDialogSource).toContain('根据脱敏参与方快照派生；无法可靠判断时使用全部角色。')
    expect(disputeDialogSource).toContain('账号限制仍在独立的用户信誉治理入口处理。')
    expect(disputeDialogSource).not.toContain('createReputationRestriction')
  })

  it('通用纠纷动作不能再用默认结果静默结案', () => {
    expect(reportBackendSource).toContain("throw new Error('请使用纠纷裁决面板填写公开结果和内部原因。')")
    expect(reportBackendSource).not.toContain("pathAction === 'resolve' ? 'other_resolved' : 'no_action'")
    expect(reportBackendSource).toContain('backendResolveAdminDispute(input: ResolveAdminDisputeInput)')
    expect(reportBackendSource).toContain('publicResultCode: input.publicResultCode')
    expect(reportBackendSource).toContain("type BackendDispute = DisputeCase")
  })

  it('举报的打开纠纷按钮调用真实 open-dispute 动作', () => {
    expect(adminSectionSource).toContain("runAdminModerationAction(row, 'restore', '管理台打开纠纷并进入人工裁决。')")
    expect(adminSectionSource).toContain('已打开纠纷。')
  })
})
