<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, ArrowUpRight, Clock3, Loader2, LogOut, ReceiptText, ShieldAlert } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import {
  getAccountGovernanceBusinessCenter,
  getRestrictedBusinessSession,
  logoutRestrictedBusinessSession,
  type AccountGovernanceBusinessCenterResponse,
  type BackendSession,
} from '@/lib/backendClient'

const router = useRouter()
const session = ref<BackendSession | null>(null)
const center = ref<AccountGovernanceBusinessCenterResponse | null>(null)
const loading = ref(true)
const loadError = ref('')
const loggingOut = ref(false)

onMounted(async () => {
  try {
    session.value = await getRestrictedBusinessSession({ forceRefresh: true })
    center.value = await getAccountGovernanceBusinessCenter('restricted_business')
  } catch {
    loadError.value = '受限业务会话已失效，请重新验证身份。'
  } finally {
    loading.value = false
  }
})

const preservedItems = computed(() => center.value?.items.filter(item => item.result === 'preserved') ?? [])
const closedItems = computed(() => center.value?.items.filter(item => item.result !== 'preserved') ?? [])

const resultLabels: Record<string, string> = {
  preserved: '保留处理入口',
  cancelled: '已治理关闭',
  sales_stopped: '已停止销售',
  already_terminal: '历史关系',
}

function formatTime(value: string | null | undefined) {
  if (!value) return '长期'
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

async function logout() {
  loggingOut.value = true
  try {
    await logoutRestrictedBusinessSession()
    await router.replace('/login')
  } finally {
    loggingOut.value = false
  }
}
</script>

<template>
  <main class="min-h-[100dvh] bg-background px-5 py-8 text-foreground">
    <div class="mx-auto w-full max-w-3xl">
      <RouterLink class="inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground" to="/login">
        <ArrowLeft class="h-4 w-4" />返回登录
      </RouterLink>

      <div v-if="loading" class="grid min-h-72 place-items-center">
        <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
      </div>

      <section v-else-if="session && center" class="mt-8 space-y-8">
        <header class="border-b border-border pb-6">
          <p class="text-sm text-muted-foreground">{{ session.user.username }}</p>
          <h1 class="mt-2 text-2xl font-semibold">受限业务中心</h1>
          <div v-if="center.currentAction" class="mt-5 grid gap-3 border-l-2 border-destructive pl-4 text-sm sm:grid-cols-[1fr_auto]">
            <div>
              <p class="font-medium">{{ center.currentAction.publicReason || center.currentAction.reasonCode }}</p>
              <p class="mt-1 text-muted-foreground">生效于 {{ formatTime(center.currentAction.effectiveAt) }}</p>
            </div>
            <div class="flex items-center gap-2 text-muted-foreground">
              <Clock3 class="h-4 w-4" />{{ formatTime(center.currentAction.expiresAt) }}
            </div>
          </div>
        </header>

        <div v-if="center.processingStatus === 'processing'" class="flex items-center gap-3 border-y border-border py-4 text-sm text-muted-foreground">
          <Loader2 class="h-4 w-4 animate-spin" />业务关系正在核对，列表会持续更新。
        </div>

        <section class="space-y-3">
          <div class="flex items-center justify-between">
            <h2 class="text-base font-semibold">继续处理</h2>
            <span class="text-sm text-muted-foreground">{{ preservedItems.length }}</span>
          </div>
          <div v-if="preservedItems.length" class="divide-y divide-border border-y border-border">
            <RouterLink v-for="item in preservedItems" :key="item.id" :to="item.targetUrl" class="flex items-start justify-between gap-4 py-4 hover:bg-muted/30">
              <span class="flex min-w-0 items-start gap-3">
                <ReceiptText class="mt-0.5 h-5 w-5 shrink-0" />
                <span class="min-w-0"><strong class="block truncate text-sm">{{ item.resourceLabel }}</strong><span class="mt-1 block text-sm text-muted-foreground">{{ item.beforeStatus }} · {{ resultLabels[item.result] }}</span></span>
              </span>
              <ArrowUpRight class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
            </RouterLink>
          </div>
          <p v-else class="border-y border-border py-5 text-sm text-muted-foreground">暂无需要继续处理的关系。</p>
        </section>

        <section class="space-y-3">
          <div class="flex items-center justify-between">
            <h2 class="text-base font-semibold">处置记录</h2>
            <span class="text-sm text-muted-foreground">{{ closedItems.length }}</span>
          </div>
          <div v-if="closedItems.length" class="divide-y divide-border border-y border-border">
            <div v-for="item in closedItems" :key="item.id" class="py-4">
              <div class="flex items-start justify-between gap-4">
                <span class="flex min-w-0 items-start gap-3">
                  <ShieldAlert class="mt-0.5 h-5 w-5 shrink-0" />
                  <span class="min-w-0"><strong class="block truncate text-sm">{{ item.resourceLabel }}</strong><span class="mt-1 block text-sm text-muted-foreground">{{ item.beforeStatus }} → {{ item.afterStatus }} · {{ resultLabels[item.result] }}</span></span>
                </span>
                <RouterLink v-if="item.targetUrl" :to="item.targetUrl" class="shrink-0 text-sm text-primary hover:underline">查看</RouterLink>
              </div>
              <p v-if="item.paymentClaimEligible" class="mt-3 border-l-2 border-amber-500 pl-3 text-sm text-muted-foreground">付款申报截止 {{ formatTime(item.paymentClaimDeadlineAt) }}</p>
            </div>
          </div>
          <p v-else class="border-y border-border py-5 text-sm text-muted-foreground">暂无治理处置记录。</p>
        </section>

        <Button variant="outline" :disabled="loggingOut" @click="logout">
          <Loader2 v-if="loggingOut" class="h-4 w-4 animate-spin" />
          <LogOut v-else class="h-4 w-4" />
          退出受限业务
        </Button>
      </section>

      <section v-else class="mt-8 border-y border-border py-8">
        <h1 class="text-xl font-semibold">无法进入受限业务中心</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ loadError }}</p>
        <RouterLink to="/login"><Button class="mt-5">重新验证</Button></RouterLink>
      </section>
    </div>
  </main>
</template>
