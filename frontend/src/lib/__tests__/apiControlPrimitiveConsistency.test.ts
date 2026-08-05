import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, test } from 'vitest'

function source(path: string) {
  return readFileSync(new URL(path, import.meta.url), 'utf8')
}

const apiSources = {
  accessSource: source('../../components/api-service-publish/ApiAccessSourceSection.vue'),
  distributionBilling: source('../../components/api-service-publish/DistributionBillingSection.vue'),
  fixedPackage: source('../../components/api-service-publish/FixedPackageSection.vue'),
  imageCapability: source('../../components/api-service-publish/ImageCapabilitySection.vue'),
  merchantNote: source('../../components/api-service-publish/MerchantNoteSection.vue'),
  modelMultiSelect: source('../../components/api-service-publish/ModelMultiSelect.vue'),
  providerCategory: source('../../components/api-service-publish/ProviderCategorySelector.vue'),
  selectedModels: source('../../components/api-service-publish/SelectedModelsPricingTable.vue'),
  quotaOwner: source('../../components/api-quota/ApiQuotaOwnerManager.vue'),
  orderDetail: source('../../pages/ApiPurchaseOrderDetailPage.vue'),
  adminModels: source('../../pages/AdminApiModelsPage.vue'),
  modelAudit: source('../../pages/AdminModelAuditPage.vue'),
}

describe('API 控件原语一致性', () => {
  test('业务控件不再直接维护原生按钮、下拉框或 checkbox/radio', () => {
    for (const [name, content] of Object.entries(apiSources)) {
      assert.doesNotMatch(content, /<button\b|<select\b|<option\b|type=["'](?:checkbox|radio)["']/, name)
      assert.doesNotMatch(content, /api-publish-toggle/, name)
    }
  })

  test('选择、开关和操作控件均来自 shadcn-vue primitive', () => {
    for (const content of [apiSources.accessSource, apiSources.distributionBilling, apiSources.providerCategory, apiSources.orderDetail]) {
      assert.match(content, /<RadioGroup/)
      assert.match(content, /<RadioGroupItem/)
    }
    assert.match(apiSources.fixedPackage, /from ['"]@\/components\/ui\/select['"]|<Select/)
    assert.match(apiSources.imageCapability, /from ['"]@\/components\/ui\/switch['"]|<Switch/)
    assert.match(apiSources.adminModels, /from ['"]@\/components\/ui\/checkbox['"]|<Checkbox/)
    assert.match(apiSources.modelAudit, /from ['"]@\/components\/ui\/checkbox['"]|<Checkbox/)
    for (const content of [apiSources.fixedPackage, apiSources.merchantNote, apiSources.modelMultiSelect, apiSources.selectedModels, apiSources.quotaOwner]) {
      assert.match(content, /from ['"]@\/components\/ui\/button['"]|<Button/)
    }
  })
})
