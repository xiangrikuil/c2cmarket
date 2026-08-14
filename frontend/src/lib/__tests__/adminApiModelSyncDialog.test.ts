import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const dialog = readFileSync(new URL('../../components/admin/AdminApiModelSyncDialog.vue', import.meta.url), 'utf8')

describe('models.dev 同步弹窗', () => {
  it('版本冲突后重新获取预览并要求管理员重新确认', () => {
    const conflictRecovery = dialog.match(/async function refreshPreviewAfterConflict\(\) \{.*?\n\}/s)?.[0] ?? ''

    expect(dialog).toContain("error instanceof BackendProblemError && error.code === 'VERSION_CONFLICT'")
    expect(dialog).toContain('await refreshPreviewAfterConflict()')
    expect(conflictRecovery).toContain('applyMutation.reset()')
    expect(conflictRecovery).toContain('previewMutation.mutateAsync([...selectedProviderIds.value])')
    expect(conflictRecovery).toContain('applyPreviewResult(result)')
    expect(conflictRecovery).not.toContain('applyMutation.mutateAsync')
    expect(dialog).toContain('模型目录已更新，旧选择已失效。请重新确认预览后再应用。')
  })

  it('刷新预览时重置旧选择和导入后启用状态', () => {
    expect(dialog).toContain("selectedCandidateKeys.value = result.items.filter(item => item.status === 'new')")
    expect(dialog).toContain('activeCandidateKeys.value = []')
    expect(dialog).toContain("result.counts.new > 0 ? 'new' : result.counts.priceChanged > 0 ? 'price_changed' : 'unchanged'")
  })
})
