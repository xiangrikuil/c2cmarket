import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'vitest'
import {
  getApiServiceProductIconSrc,
  getProductCategoryIconSrc,
  getProductIconSrc,
  productCategoryIconMaxBytes,
  validateProductCategoryIconFile,
} from '../productCategoryIcon.ts'

test('accepts PNG and WebP category icons within the size limit', () => {
  assert.equal(validateProductCategoryIconFile({ type: 'image/png', size: 1024 }), null)
  assert.equal(validateProductCategoryIconFile({ type: 'image/webp', size: productCategoryIconMaxBytes }), null)
})

test('rejects unsupported or oversized category icons', () => {
  assert.match(validateProductCategoryIconFile({ type: 'image/svg+xml', size: 1024 }) ?? '', /PNG 或 WebP/)
  assert.match(validateProductCategoryIconFile({ type: 'image/png', size: productCategoryIconMaxBytes + 1 }) ?? '', /256 KB/)
})

test('resolves uploaded and fallback category icons consistently', () => {
  const categoryIcons = new Map([['gpt', 'data:image/png;base64,目录图标']])

  assert.equal(getProductCategoryIconSrc('gpt', categoryIcons), 'data:image/png;base64,目录图标')
  assert.equal(getProductCategoryIconSrc('claude', categoryIcons), '/claude-mark.svg')
  assert.equal(getProductIconSrc('OpenAI GPT-4.1', categoryIcons), 'data:image/png;base64,目录图标')
  assert.equal(getProductIconSrc('未知产品', categoryIcons), null)
})

test('resolves API service icons from the selected model provider instead of the generic title', () => {
  const categoryIcons = new Map([['gpt', 'data:image/png;base64,套餐目录GPT图标']])

  assert.equal(getApiServiceProductIconSrc({
    title: 'Sub2API 美元额度服务',
    models: ['GPT-4.1'],
    modelPriceRows: [{ provider: 'OpenAI' }],
  }, categoryIcons), 'data:image/png;base64,套餐目录GPT图标')
  assert.equal(getApiServiceProductIconSrc({
    title: '美元额度服务',
    models: ['Claude Sonnet'],
    modelPriceRows: [{ provider: 'Anthropic' }],
  }, new Map()), '/claude-mark.svg')
})

test('wires category icons through admin upload and public category rendering', () => {
  const adminSource = readFileSync(new URL('../../pages/AdminProductPlansPage.vue', import.meta.url), 'utf8')
  const carpoolSource = readFileSync(new URL('../../pages/CarpoolsPage.vue', import.meta.url), 'utf8')
  const homeSource = readFileSync(new URL('../../pages/HomePage.vue', import.meta.url), 'utf8')
  const backendSource = readFileSync(new URL('../productCatalogBackend.ts', import.meta.url), 'utf8')

  assert.match(adminSource, /readProductCategoryIcon/)
  assert.match(adminSource, /categoryForm\.iconDataUrl/)
  assert.match(adminSource, /替换图标/)
  assert.match(carpoolSource, /categoryIconByCode/)
  assert.match(carpoolSource, /getProductCategoryIconSrc/)
  assert.match(carpoolSource, /getCatalogProductIconSrc/)
  assert.match(homeSource, /useProductCategories/)
  assert.match(homeSource, /getApiServiceProductIconSrc/)
  assert.match(backendSource, /iconDataUrl: input\.iconDataUrl\.trim\(\)/)
})
