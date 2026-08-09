import { computed, ref, watch, type WatchSource } from 'vue'

export const DEFAULT_CURSOR_PAGE_SIZE = 20

export function useCursorPagination(resetSources: WatchSource[] = [], pageSize = DEFAULT_CURSOR_PAGE_SIZE) {
  const page = ref(1)
  const cursors = ref<Array<string | undefined>>([undefined])

  const cursor = computed(() => cursors.value[page.value - 1])

  function reset() {
    page.value = 1
    cursors.value = [undefined]
  }

  function next(nextCursor: string | undefined) {
    if (!nextCursor) return
    cursors.value = [...cursors.value.slice(0, page.value), nextCursor]
    page.value += 1
  }

  function previous() {
    if (page.value <= 1) return
    page.value -= 1
  }

  if (resetSources.length > 0) watch(resetSources, reset)

  return {
    page,
    pageSize,
    cursor,
    reset,
    next,
    previous,
  }
}
