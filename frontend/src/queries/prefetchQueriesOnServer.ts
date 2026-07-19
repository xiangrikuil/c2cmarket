import { setResponseStatus, useRequestEvent } from '#app'
import { onServerPrefetch } from 'vue'

type SuspendableQuery = {
  suspense: () => Promise<unknown>
}

function queryErrorStatus(error: unknown) {
  return typeof error === 'object' && error !== null && 'status' in error
    ? Number((error as { status?: unknown }).status)
    : null
}

export function prefetchQueriesOnServer(...queries: SuspendableQuery[]) {
  if (!import.meta.server) return

  onServerPrefetch(async () => {
    await Promise.all(queries.map(async (query) => {
      try {
        await query.suspense()
      } catch (error) {
        // 详情 404 由同页的状态标记处理；其他上游故障必须保留为服务端失败。
        if (queryErrorStatus(error) !== 404) throw error
      }
    }))
  })
}

export function markMissingQueryAsNotFoundOnServer(query: SuspendableQuery, exists: () => boolean) {
  if (!import.meta.server) return
  const event = useRequestEvent()

  onServerPrefetch(async () => {
    let error: unknown = null
    try {
      await query.suspense()
    } catch (queryError) {
      error = queryError
    }

    const status = queryErrorStatus(error)
    if (error && status !== 404) throw error
    if (event && !exists() && (!error || status === 404)) {
      setResponseStatus(event, 404, 'Page Not Found')
    }
  })
}
