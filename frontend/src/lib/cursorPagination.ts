export type CursorPageRequest = {
  limit?: number
  cursor?: string
}

export type CursorPage<T> = {
  items: T[]
  nextCursor?: string
}

export function normalizeNextCursor(value: string | null | undefined) {
  const cursor = value?.trim()
  return cursor || undefined
}

export function nextUnseenCursor(value: string | null | undefined, pageParams: readonly unknown[]) {
  const cursor = normalizeNextCursor(value)
  if (!cursor || pageParams.includes(cursor)) return undefined
  return cursor
}

export function paginateCursorItems<T>(items: readonly T[], request: CursorPageRequest = {}): CursorPage<T> {
  const limit = Math.min(100, Math.max(1, request.limit ?? 20))
  const match = request.cursor?.match(/^mock:(\d+)$/)
  if (request.cursor && !match) throw new Error('分页 cursor 无效。')
  const offset = match ? Number(match[1]) : 0
  if (!Number.isSafeInteger(offset)) throw new Error('分页 cursor 无效。')
  const end = Math.min(items.length, offset + limit)
  return {
    items: items.slice(offset, end),
    nextCursor: end < items.length ? `mock:${end}` : undefined,
  }
}

export function flattenUniqueCursorPages<T extends { id: string }>(pages: readonly CursorPage<T>[] | undefined) {
  const rows = new Map<string, T>()
  for (const page of pages ?? []) {
    for (const item of page.items) rows.set(item.id, item)
  }
  return [...rows.values()]
}

export async function collectCursorPages<T>(loadPage: (request: CursorPageRequest) => Promise<CursorPage<T>>) {
  const rows: T[] = []
  const seenCursors = new Set<string>()
  let cursor: string | undefined
  do {
    const page = await loadPage({ limit: 100, cursor })
    rows.push(...page.items)
    cursor = normalizeNextCursor(page.nextCursor)
    if (cursor && seenCursors.has(cursor)) throw new Error('分页 cursor 重复，无法继续加载。')
    if (cursor) seenCursors.add(cursor)
  } while (cursor)
  return rows
}
