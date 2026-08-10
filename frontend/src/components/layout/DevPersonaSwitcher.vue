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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  backendErrorMessage,
  createDevPersonaSession,
  shouldUseRealBackend,
  type DevPersona,
} from '@/lib/backendClient'

const props = defineProps<{
  currentUsername?: string
}>()

const router = useRouter()
const queryClient = useQueryClient()
const busyPersona = ref<DevPersona | null>(null)
const visible = import.meta.dev && shouldUseRealBackend()
const personaItems = [
  { persona: 'buyer' as const, username: 'dev-buyer', label: '买家' },
  { persona: 'seller' as const, username: 'dev-seller', label: '卖家' },
  { persona: 'admin' as const, username: 'dev-admin', label: '管理员' },
]
const activePersona = computed(() => personaItems.find(item => item.username === props.currentUsername)?.persona ?? null)

async function switchPersona(persona: DevPersona) {
  if (busyPersona.value || activePersona.value === persona) return
  busyPersona.value = persona
  try {
    const session = await createDevPersonaSession(persona)
    await queryClient.cancelQueries()
    queryClient.getMutationCache().clear()
    queryClient.removeQueries({ type: 'inactive' })
    await queryClient.resetQueries({ type: 'active' })
    await router.replace('/my')
    toast.success(`已切换为${session.user.displayName || session.user.username}。`)
  } catch (error) {
    toast.error(backendErrorMessage(error, '切换开发账号失败'))
  } finally {
    busyPersona.value = null
  }
}
</script>

<template>
  <DropdownMenu v-if="visible">
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger as-child>
          <DropdownMenuTrigger as-child>
            <Button
              variant="ghost"
              size="icon"
              :disabled="busyPersona !== null"
              aria-label="切换开发账号"
              data-testid="dev-persona-switcher"
            >
              <LoaderCircle v-if="busyPersona" class="h-4 w-4 animate-spin" />
              <UsersRound v-else class="h-4 w-4" />
            </Button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent side="bottom">切换开发账号</TooltipContent>
      </Tooltip>
    </TooltipProvider>
    <DropdownMenuContent align="end" class="w-48">
      <DropdownMenuLabel>开发身份</DropdownMenuLabel>
      <DropdownMenuSeparator />
      <DropdownMenuItem
        v-for="item in personaItems"
        :key="item.persona"
        :disabled="busyPersona !== null || activePersona === item.persona"
        class="justify-between"
        @select="switchPersona(item.persona)"
      >
        <span>
          <span class="font-medium">{{ item.label }}</span>
          <span class="ml-1 text-xs text-muted-foreground">{{ item.username }}</span>
        </span>
        <LoaderCircle v-if="busyPersona === item.persona" class="h-4 w-4 animate-spin" />
        <Check v-else-if="activePersona === item.persona" class="h-4 w-4 text-primary" />
      </DropdownMenuItem>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
