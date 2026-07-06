import { demands } from '@/data/mock'
import type { DemandRecord, SubmitDemandPayload } from '@/features/demand/types'
import { cloneMock, readMockSessionStore, writeMockSessionStore } from '@/mocks/storage'

const demandStorageKey = 'c2cmarket.demands.v1'
const currentBuyerName = 'demo_user'

const seedDemands: DemandRecord[] = demands.map(item => ({
  ...item,
  status: item.status as DemandRecord['status'],
  region: item.title.includes('菲律宾') ? '菲律宾区' : item.title.includes('香港') ? '香港区' : '不限',
  ownerPreference: item.require.includes('个人车主') ? 'personal' : 'any',
  sourceUrl: `https://linux.do/t/demand-${item.id}`,
  note: item.require,
  createdAt: '2026-06-19 12:00',
  updatedAt: '2026-06-19 12:00',
}))

let demandStore = readMockSessionStore<DemandRecord[]>(demandStorageKey, seedDemands)

type DemandCreatedListener = (demand: DemandRecord) => void

let demandCreatedListener: DemandCreatedListener | null = null

export function setMockDemandCreatedListener(listener: DemandCreatedListener | null) {
  demandCreatedListener = listener
}

export function nowMockText() {
  return new Intl.DateTimeFormat('sv-SE', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date())
}

function persistDemandStore() {
  writeMockSessionStore(demandStorageKey, demandStore)
}

export function listMockDemands() {
  return cloneMock(demandStore)
}

export function getMockDemandById(id: string) {
  return cloneMock(demandStore.find(item => item.id === id) ?? null)
}

export function createMockDemand(payload: SubmitDemandPayload) {
  const id = `demand-${Date.now()}`
  const demand: DemandRecord = {
    id,
    title: payload.title.trim(),
    maxPrice: payload.maxPrice,
    require: `${payload.region} · ${payload.ownerPreference === 'only-personal' ? '只看个人车主' : payload.ownerPreference === 'personal' ? '个人车主优先' : '不限车主'} · ${payload.note.trim() || '等待车主匹配'}`,
    poster: currentBuyerName,
    trustLevel: 3,
    linuxdoPost: '已绑定求车帖',
    status: '匹配中',
    region: payload.region,
    ownerPreference: payload.ownerPreference,
    sourceUrl: payload.sourceUrl,
    note: payload.note,
    createdAt: nowMockText(),
    updatedAt: nowMockText(),
  }
  demandStore.unshift(demand)
  persistDemandStore()
  demandCreatedListener?.(demand)
  return cloneMock(demand)
}

export function toggleMockDemandClosed(id: string) {
  const target = demandStore.find(item => item.id === id)
  if (!target) throw new Error('未找到求车需求')
  target.status = target.status === '已关闭' ? '匹配中' : '已关闭'
  target.updatedAt = nowMockText()
  persistDemandStore()
  return cloneMock(target)
}

export function updateMockDemandAdminStatus(id: string, status: string) {
  const target = demandStore.find(item => item.id === id)
  if (!target) return null
  target.status = status === '已关闭' || status === '已下架' ? '已关闭' : status === '待复核' ? '待审核' : status === '已通过' || status === '已恢复' ? '匹配中' : target.status
  target.updatedAt = nowMockText()
  persistDemandStore()
  return cloneMock(target)
}

export function resetMockDemandsForTest(next: DemandRecord[] = seedDemands) {
  demandStore = cloneMock(next)
  persistDemandStore()
}
