import { computed, ref, watch, type ComputedRef, type Ref } from 'vue'

export const DEFAULT_TABLE_PAGE_SIZE = 20

type SourceRows<T> = Ref<T[]> | ComputedRef<T[]>

export function usePagination<T>(rows: SourceRows<T>, pageSize = DEFAULT_TABLE_PAGE_SIZE) {
  const page = ref(1)

  const total = computed(() => rows.value.length)
  const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
  const startItem = computed(() => total.value === 0 ? 0 : (page.value - 1) * pageSize + 1)
  const endItem = computed(() => Math.min(total.value, page.value * pageSize))
  const paginatedRows = computed(() => rows.value.slice((page.value - 1) * pageSize, page.value * pageSize))

  function goToPage(nextPage: number) {
    page.value = Math.min(Math.max(1, nextPage), pageCount.value)
  }

  function nextPage() {
    goToPage(page.value + 1)
  }

  function prevPage() {
    goToPage(page.value - 1)
  }

  watch(rows, () => {
    page.value = 1
  })

  watch(total, () => {
    goToPage(page.value)
  })

  return {
    page,
    pageSize,
    total,
    pageCount,
    startItem,
    endItem,
    paginatedRows,
    goToPage,
    nextPage,
    prevPage,
  }
}
