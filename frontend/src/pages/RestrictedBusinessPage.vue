<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, FileWarning, Loader2, LogOut, ReceiptText } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { getRestrictedBusinessSession, logoutRestrictedBusinessSession, type BackendSession } from '@/lib/backendClient'

const router = useRouter()
const session = ref<BackendSession | null>(null)
const loading = ref(true)
const loadError = ref('')
const loggingOut = ref(false)

onMounted(async () => {
  try {
    session.value = await getRestrictedBusinessSession({ forceRefresh: true })
  } catch {
    loadError.value = '受限业务会话已失效，请重新验证身份。'
  } finally {
    loading.value = false
  }
})

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

      <section v-else-if="session" class="mt-8 space-y-8">
        <header class="border-b border-border pb-6">
          <p class="text-sm text-muted-foreground">{{ session.user.username }}</p>
          <h1 class="mt-2 text-2xl font-semibold">受限业务中心</h1>
        </header>

        <nav class="grid gap-px overflow-hidden border border-border bg-border sm:grid-cols-2" aria-label="受限业务入口">
          <RouterLink to="/my/api-orders" class="flex min-h-28 items-start gap-3 bg-background p-5 hover:bg-muted/50">
            <ReceiptText class="mt-0.5 h-5 w-5" />
            <span><strong class="block text-sm">API 订单</strong><span class="mt-1 block text-sm text-muted-foreground">查看已有订单与售后记录</span></span>
          </RouterLink>
          <RouterLink to="/my/reports" class="flex min-h-28 items-start gap-3 bg-background p-5 hover:bg-muted/50">
            <FileWarning class="mt-0.5 h-5 w-5" />
            <span><strong class="block text-sm">纠纷与申诉</strong><span class="mt-1 block text-sm text-muted-foreground">处理已有纠纷和补充材料</span></span>
          </RouterLink>
        </nav>

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
