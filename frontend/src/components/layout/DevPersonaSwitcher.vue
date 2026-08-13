<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import Check from 'lucide-vue-next/dist/esm/icons/check.js'
import LoaderCircle from 'lucide-vue-next/dist/esm/icons/loader-circle.js'
import UsersRound from 'lucide-vue-next/dist/esm/icons/users-round.js'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  backendErrorMessage,
  createDevPersonaSession,
  createMockPersonaSession,
  shouldUseRealBackend,
  type DevPersona,
} from '@/lib/backendClient'
import { getMockPersona, type MockPersona } from '@/lib/mockAuth'

const props = defineProps<{
  currentUsername?: string
}>()

const router = useRouter()
const queryClient = useQueryClient()
type SwitchPersona = DevPersona | MockPersona

const realBackend = shouldUseRealBackend()
const busyPersona = ref<SwitchPersona | null>(null)
const popoverOpen = ref(false)
const visible = import.meta.dev
const currentMockPersona = ref<MockPersona>(getMockPersona())
const realPersonaItems = [
  { persona: 'buyer' as const, username: 'dev-buyer', label: '买家' },
  { persona: 'seller' as const, username: 'dev-seller', label: '卖家' },
  { persona: 'admin' as const, username: 'dev-admin', label: '管理员' },
]
const mockPersonaItems = [
  { persona: 'anonymous' as const, username: '—', label: '匿名' },
  { persona: 'student' as const, username: 'student-buyer', label: '学生买家' },
  { persona: 'linuxdo' as const, username: 'orbit', label: 'Linux.do' },
  { persona: 'admin' as const, username: 'orbit', label: '管理员' },
]
const personaItems = computed(() => realBackend ? realPersonaItems : mockPersonaItems)
const collisionSuffixPattern = /^[a-f0-9]{8}(?:-\d+)?$/
const activePersona = computed(() => {
  if (!realBackend) return currentMockPersona.value
  const currentUsername = props.currentUsername?.trim() ?? ''
  const item = personaItems.value.find(({ username }) => {
    if (currentUsername === username) return true
    return currentUsername.startsWith(`${username}-`)
      && collisionSuffixPattern.test(currentUsername.slice(username.length + 1))
  })
  return item?.persona ?? null
})

function choosePersona(persona: SwitchPersona) {
  popoverOpen.value = false
  void switchPersona(persona)
}

async function switchPersona(persona: SwitchPersona) {
  if (busyPersona.value || activePersona.value === persona) return
  busyPersona.value = persona
  try {
    const session = realBackend
      ? await createDevPersonaSession(persona as DevPersona)
      : await createMockPersonaSession(persona as MockPersona)
    if (!realBackend) currentMockPersona.value = persona as MockPersona
    await queryClient.cancelQueries()
    queryClient.getMutationCache().clear()
    queryClient.removeQueries({ type: 'inactive' })
    await queryClient.resetQueries({ type: 'active' })
    await router.replace(session ? '/my' : '/login')
    toast.success(session ? `已切换为${session.user.displayName || session.user.username}。` : '已切换为匿名状态。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '切换开发账号失败'))
  } finally {
    busyPersona.value = null
  }
}
</script>

<template>
  <Popover v-if="visible" v-model:open="popoverOpen">
    <PopoverTrigger as-child>
      <Button
        variant="ghost"
        size="icon"
        :disabled="busyPersona !== null"
        aria-label="切换开发账号"
        title="切换开发账号"
        data-testid="dev-persona-switcher"
      >
        <LoaderCircle v-if="busyPersona" class="h-4 w-4 animate-spin" />
        <UsersRound v-else class="h-4 w-4" />
      </Button>
    </PopoverTrigger>
    <PopoverContent align="end" class="w-48 p-1">
      <div class="px-2 py-1.5 text-sm font-semibold">开发身份</div>
      <div class="-mx-1 my-1 h-px bg-border" />
      <div class="grid gap-1">
        <Button
          v-for="item in personaItems"
          :key="item.persona"
          type="button"
          role="menuitem"
          variant="ghost"
          size="sm"
          :disabled="busyPersona !== null || activePersona === item.persona"
          class="w-full justify-between px-2 font-normal"
          @click="choosePersona(item.persona)"
        >
          <span>
            <span class="font-medium">{{ item.label }}</span>
            <span class="ml-1 text-xs text-muted-foreground">{{ item.username }}</span>
          </span>
          <LoaderCircle v-if="busyPersona === item.persona" class="h-4 w-4 animate-spin" />
          <Check v-else-if="activePersona === item.persona" class="h-4 w-4 text-primary" />
        </Button>
      </div>
    </PopoverContent>
  </Popover>
</template>
