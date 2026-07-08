export type DemandStatus = '匹配中' | '已匹配' | '已关闭' | '需处理'

export type DemandRecord = {
  id: string
  title: string
  maxPrice: number
  require: string
  poster: string
  trustLevel: number
  linuxdoPost: string
  status: DemandStatus
  region: string
  ownerPreference: 'personal' | 'only-personal' | 'only_personal' | 'any'
  sourceUrl: string
  note: string
  createdAt: string
  updatedAt: string
  backendKind?: 'demand'
  backendVersion?: number
}

export type SubmitDemandPayload = {
  sourceUrl: string
  title: string
  maxPrice: number
  region: string
  ownerPreference: 'personal' | 'only-personal' | 'any'
  note: string
}
