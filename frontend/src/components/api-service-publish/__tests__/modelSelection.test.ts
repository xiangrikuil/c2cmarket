import assert from 'node:assert/strict'
import { selectedModelIdSet, summarizeSelectedModelNames, toggleSelectedModel } from '../modelSelection'

type SelectedServiceModel = {
  modelId: string
  multiplierOverride: number | null
  enabled: boolean
}

const selectedModels: SelectedServiceModel[] = [
  { modelId: 'gpt-5-mini', multiplierOverride: null, enabled: true },
  { modelId: 'gpt-4-disabled', multiplierOverride: null, enabled: false },
]

assert.deepEqual([...selectedModelIdSet(selectedModels)], ['gpt-5-mini'])

assert.deepEqual(toggleSelectedModel(selectedModels, 'gpt-5.1'), [
  ...selectedModels,
  { modelId: 'gpt-5.1', multiplierOverride: null, enabled: true },
])

assert.deepEqual(toggleSelectedModel([{ modelId: 'gpt-5-mini', multiplierOverride: null, enabled: true }], 'gpt-5-mini'), [])

assert.equal(summarizeSelectedModelNames([]), '还没有选择模型')
assert.equal(summarizeSelectedModelNames(['GPT-5 mini', 'GPT-5.1']), '已选择 2 个模型：GPT-5 mini / GPT-5.1')
assert.equal(
  summarizeSelectedModelNames(['GPT-5 mini', 'GPT-5.1', 'GPT-4.1', 'o3', 'o4-mini']),
  '已选择 5 个模型：GPT-5 mini / GPT-5.1 / GPT-4.1 +2',
)
