type IdRecord = { id: string }

export function cloneMock<T>(value: T): T {
  return structuredClone(value)
}

function isIdRecordArray(value: unknown): value is IdRecord[] {
  return Array.isArray(value)
    && value.every(item => item !== null && typeof item === 'object' && typeof (item as { id?: unknown }).id === 'string')
}

function mergeSeedRecords<T extends IdRecord>(seed: T[], stored: T[]) {
  const storedIds = new Set(stored.map(item => item.id))
  return [
    ...stored,
    ...cloneMock(seed.filter(item => !storedIds.has(item.id))),
  ]
}

export function readMockSessionStore<T>(key: string, seed: T): T {
  if (typeof window === 'undefined') return cloneMock(seed)
  const stored = window.sessionStorage.getItem(key)
  if (!stored) return cloneMock(seed)
  const parsed = JSON.parse(stored) as T
  if (isIdRecordArray(seed) && isIdRecordArray(parsed)) {
    return mergeSeedRecords(seed, parsed) as T
  }
  return parsed
}

export function writeMockSessionStore<T>(key: string, value: T) {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(key, JSON.stringify(value))
}
