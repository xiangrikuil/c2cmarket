import { onMounted, ref, watch } from 'vue'

export function initialSidebarCollapsed(storageValue: string | null, viewportWidth: number) {
  if (storageValue === 'true') return true
  if (storageValue === 'false') return false
  return viewportWidth < 1024
}

export function usePersistentSidebar(storageKey: string) {
  const sidebarCollapsed = ref(false)

  onMounted(() => {
    const stored = window.localStorage.getItem(storageKey)
    sidebarCollapsed.value = initialSidebarCollapsed(stored, window.innerWidth)
  })

  watch(sidebarCollapsed, value => {
    if (typeof window !== 'undefined') window.localStorage.setItem(storageKey, String(value))
  })

  return { sidebarCollapsed }
}
