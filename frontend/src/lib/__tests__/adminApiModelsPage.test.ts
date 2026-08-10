import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const page = readFileSync(new URL('../../pages/AdminApiModelsPage.vue', import.meta.url), 'utf8')

describe('API 模型目录管理页', () => {
  it('默认以模型和提供商页签组织目录，并按页签显示主操作', () => {
    expect(page).toContain("const activeCatalogTab = ref<CatalogTab>('models')")
    expect(page).toContain('<Tabs v-model="activeCatalogTab"')
    expect(page).toContain('<TabsTrigger value="models"')
    expect(page).toContain('<TabsTrigger value="providers"')
    expect(page).toContain('同步价格')
    expect(page).toContain('新建模型')
    expect(page).toContain('新建提供商')
    expect(page).not.toContain('<CompactStats')
    expect(page).not.toContain('<StatusTabs')
  })

  it('组合模型搜索、模型自身状态和提供商筛选', () => {
    expect(page).toContain('v-model="modelSearch"')
    expect(page).toContain('v-model="providerFilter"')
    expect(page).toContain("statusFilter.value === 'active' ? item.active : !item.active")
    expect(page).toContain('[item.modelKey, item.provider, item.providerCode]')
    expect(page).toContain('item.providerId !== providerFilter.value')
    expect(page).toContain('选择当前筛选的全部模型')
  })

  it('使用开关和明确文字表达状态，并保留提供商停用原因', () => {
    expect(page).toContain(':model-value="model.active"')
    expect(page).toContain(':model-value="provider.active"')
    expect(page).toContain("model.active ? '已启用' : '已停用'")
    expect(page).toContain("provider.active ? '已启用' : '已停用'")
    expect(page).toContain('提供商已停用')
    expect(page).not.toContain('ToggleLeft')
    expect(page).not.toContain('ToggleRight')
    expect(page).not.toContain("{{ model.active ? '停用' : '启用' }}")
  })

  it('精简模型表格并提供完整批量操作', () => {
    expect(page).toContain('<th class="px-3 py-2.5 font-medium">官网价格</th>')
    expect(page).not.toContain('<th class="px-3 py-2 font-medium">能力</th>')
    expect(page).not.toContain('<th class="px-3 py-2 font-medium">来源版本</th>')
    expect(page).not.toContain('<th class="px-3 py-2 font-medium">排序</th>')
    expect(page).toContain('批量启用')
    expect(page).toContain('批量停用')
    expect(page).toContain('aria-label="清除选择"')
    expect(page).toContain('@click="selectedModelIds = []"')
  })
})
