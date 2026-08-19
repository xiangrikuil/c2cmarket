import { onBeforeUnmount, onMounted, type Ref } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'

export function shouldAllowUnsavedNavigation(
  dirty: boolean,
  message: string,
  confirmLeave: (message: string) => boolean,
) {
  return !dirty || confirmLeave(message)
}

export function useUnsavedChangesGuard(dirty: Ref<boolean>, message = '当前内容尚未保存，确认离开此页面？') {
  function beforeUnload(event: BeforeUnloadEvent) {
    if (!dirty.value) return
    event.preventDefault()
    event.returnValue = ''
  }

  function confirmNavigation() {
    return shouldAllowUnsavedNavigation(dirty.value, message, prompt => window.confirm(prompt))
  }

  onMounted(() => window.addEventListener('beforeunload', beforeUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
  onBeforeRouteLeave(confirmNavigation)
  onBeforeRouteUpdate(confirmNavigation)
}
