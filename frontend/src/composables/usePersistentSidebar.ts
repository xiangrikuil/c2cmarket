import { ref, watch } from 'vue'

export function initialSidebarCollapsed(storageValue: string | null, viewportWidth: number) {
  if (storageValue === 'true') return true
  if (storageValue === 'false') return false
  return viewportWidth < 1024
}

export function usePersistentSidebar(storageKey: string) {
  const stored = typeof window === 'undefined' ? null : window.localStorage.getItem(storageKey)
  const viewportWidth = typeof window === 'undefined' ? 1440 : window.innerWidth
  const sidebarCollapsed = ref(initialSidebarCollapsed(stored, viewportWidth))

  watch(sidebarCollapsed, value => {
    if (typeof window !== 'undefined') window.localStorage.setItem(storageKey, String(value))
  })

  return { sidebarCollapsed }
}
