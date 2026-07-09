export type ApiPublishCompletenessStatus = 'done' | 'pending' | 'conflict'

export type ApiPublishCompletenessItem = {
  label: string
  status: ApiPublishCompletenessStatus
}

export const apiPublishAssistantSummary = (items: ApiPublishCompletenessItem[]) => {
  const doneLabels = items.filter(item => item.status === 'done').map(item => item.label)
  const pendingLabels = items.filter(item => item.status === 'pending').map(item => item.label)
  const conflictLabels = items.filter(item => item.status === 'conflict').map(item => item.label)
  const totalCount = items.length
  const unresolvedCount = pendingLabels.length + conflictLabels.length
  const topPendingText = pendingLabels.length
    ? `还差：${pendingLabels.slice(0, 3).join('、')}`
    : conflictLabels.length
      ? `需处理：${conflictLabels.slice(0, 3).join('、')}`
      : '发布必填项已完成，可发布'

  return {
    totalCount,
    doneCount: doneLabels.length,
    pendingCount: pendingLabels.length,
    conflictCount: conflictLabels.length,
    progressPercent: totalCount ? Math.round((doneLabels.length / totalCount) * 100) : 0,
    pendingLabels,
    conflictLabels,
    badgeText: unresolvedCount ? `${unresolvedCount} 项待处理` : '可发布',
    topPendingText,
  }
}

export const apiServiceDetailPath = (id: string) => id ? `/api-market/${id}` : ''
