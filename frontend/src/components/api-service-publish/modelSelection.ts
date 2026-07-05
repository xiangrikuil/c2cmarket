type SelectableServiceModel = {
  modelId: string
  multiplierOverride: number | null
  enabled: boolean
}

export const selectedModelIdSet = (selectedModels: SelectableServiceModel[]) => new Set(
  selectedModels
    .filter(item => item.enabled)
    .map(item => item.modelId),
)

export const toggleSelectedModel = (selectedModels: SelectableServiceModel[], modelId: string): SelectableServiceModel[] => {
  const selectedIds = selectedModelIdSet(selectedModels)
  if (selectedIds.has(modelId)) return selectedModels.filter(item => item.modelId !== modelId)
  return [
    ...selectedModels.filter(item => item.modelId !== modelId),
    { modelId, multiplierOverride: null, enabled: true },
  ]
}

export const summarizeSelectedModelNames = (names: string[]) => {
  if (!names.length) return '还没有选择模型'
  const visible = names.slice(0, 3).join(' / ')
  const remaining = names.length > 3 ? ` +${names.length - 3}` : ''
  return `已选择 ${names.length} 个模型：${visible}${remaining}`
}
