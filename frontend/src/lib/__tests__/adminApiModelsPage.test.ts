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

  it('组合模型搜索、三态有效状态和提供商筛选', () => {
    expect(page).toContain('v-model="modelSearch"')
    expect(page).toContain('v-model="providerFilter"')
    expect(page).toContain('item.effectiveStatus === statusFilter.value')
    expect(page).toContain('[item.modelKey, item.provider, item.providerCode]')
    expect(page).toContain('item.providerId !== providerFilter.value')
    expect(page).toContain("statusFilter = 'blocked'")
  })

  it('使用状态徽标和专用动作菜单治理生命周期', () => {
    expect(page).toContain('statusLabel(model.status)')
    expect(page).toContain('lifecycleActions(model.status)')
    expect(page).toContain('lifecycleActions(provider.status)')
    expect(page).toContain('因提供商')
    expect(page).toContain('目录状态操作')
  })

  it('精简模型表格并锁定已被引用的身份字段', () => {
    expect(page).toContain('<th class="px-3 py-2.5 font-medium">官网价格</th>')
    expect(page).not.toContain('<th class="px-3 py-2 font-medium">能力</th>')
    expect(page).not.toContain('<th class="px-3 py-2 font-medium">来源版本</th>')
    expect(page).not.toContain('<th class="px-3 py-2 font-medium">排序</th>')
    expect(page).toContain(':disabled="Boolean(editingModel?.identityLocked)"')
    expect(page).toContain(':disabled="Boolean(editingProvider?.identityLocked)"')
    expect(page).not.toContain('selectedModelIds')
  })
})
