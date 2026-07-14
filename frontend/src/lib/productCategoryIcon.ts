import { getProductCategory, type ProductCategoryKey } from '@/lib/productCategories'

export const productCategoryIconMaxBytes = 256 * 1024
export const productCategoryIconAccept = 'image/png,image/webp'

export type ProductCategoryIconMap = ReadonlyMap<string, string>

const fallbackIconByCategory: Partial<Record<ProductCategoryKey, string>> = {
  gpt: '/chatgpt-mark.svg',
  claude: '/claude-mark.svg',
  gemini: '/gemini-mark.svg',
}

/**
 * 套餐目录图标是各个展示页的唯一配置来源；目录尚未配置时使用同一套内置品牌图标。
 */
export function getProductCategoryIconSrc(category: ProductCategoryKey, iconByCategory: ProductCategoryIconMap) {
  return iconByCategory.get(category) || fallbackIconByCategory[category] || null
}

/**
 * 根据产品或模型名称归类后解析统一图标，避免首页、车源和 API 集市各自维护回退规则。
 */
export function getProductIconSrc(productName: string, iconByCategory: ProductCategoryIconMap) {
  return getProductCategoryIconSrc(getProductCategory(productName), iconByCategory)
}

type ApiServiceProductIdentity = {
  title: string
  models: readonly string[]
  modelPriceRows: ReadonlyArray<{ provider?: string }>
}

/**
 * API 服务发布时已限制模型属于同一提供商，因此优先使用模型价格快照中的提供商识别套餐目录分类。
 * 旧记录缺少提供商快照时依次使用首个模型和服务标题，无法识别时由调用方展示通用 API 图标。
 */
export function getApiServiceProductIconSrc(service: ApiServiceProductIdentity, iconByCategory: ProductCategoryIconMap) {
  const provider = service.modelPriceRows.find(row => row.provider?.trim())?.provider?.trim()
  const productIdentity = provider || service.models[0] || service.title
  return getProductIconSrc(productIdentity, iconByCategory)
}

export function validateProductCategoryIconFile(file: Pick<File, 'type' | 'size'>) {
  if (!['image/png', 'image/webp'].includes(file.type)) return '分类图标只支持 PNG 或 WebP。'
  if (file.size > productCategoryIconMaxBytes) return '分类图标不能超过 256 KB。'
  return null
}

export function readProductCategoryIcon(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => typeof reader.result === 'string' ? resolve(reader.result) : reject(new Error('分类图标读取失败。'))
    reader.onerror = () => reject(new Error('分类图标读取失败。'))
    reader.readAsDataURL(file)
  })
}
