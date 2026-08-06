<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowRight,
  Eye,
  EyeOff,
  LockKeyhole,
  LogIn,
  Loader2,
  UserRound,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  getCurrentBackendSession,
  loginWithPassword,
  logoutBackendSession,
  startOAuthLogin,
  type BackendSession,
} from '@/lib/backendClient'
import { normalizeReturnTo } from '@/lib/authNavigation'
import { trackAnalytics } from '@/lib/analytics'
import { captureReferralCode, getReferralCapture } from '@/lib/referralCapture'

const route = useRoute()
const router = useRouter()
const session = ref<BackendSession | null>(null)
const loadingSession = ref(true)
const oauthLoading = ref(false)
const passwordLoading = ref(false)
const passwordVisible = ref(false)
const username = ref('')
const password = ref('')
const showPasswordLogin = ref(false)

const loggedIn = computed(() => Boolean(session.value))
const displayName = computed(() => session.value?.user.displayName ?? session.value?.user.username ?? '未登录')
const linuxDo = computed(() => session.value?.user.linuxDoBinding)
const isAdmin = computed(() => session.value?.user.permissions.includes('admin') ?? false)
const returnTo = computed(() => normalizeReturnTo(route.query.returnTo))

onMounted(async () => {
  const referral = Array.isArray(route.query.ref) ? route.query.ref[0] : route.query.ref
  captureReferralCode(referral)
  trackAnalytics('login_page_view', { source_route: '/login' })
  await refreshSession()
})

async function refreshSession() {
  loadingSession.value = true
  try {
    session.value = await getCurrentBackendSession()
    if (session.value) await router.replace(returnTo.value)
  } catch {
    session.value = null
  } finally {
    loadingSession.value = false
  }
}

async function loginWithLinuxDo() {
  oauthLoading.value = true
  try {
    const { authorizationUrl } = await startOAuthLogin(returnTo.value, getReferralCapture())
    trackAnalytics('oauth_login_start', {
      method: 'oauth_linux_do',
      source_route: '/login',
    })
    window.location.assign(authorizationUrl)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '启动 linux.do 登录失败')
  } finally {
    oauthLoading.value = false
  }
}

async function submitPasswordLogin() {
  const trimmedUsername = username.value.trim()
  if (!trimmedUsername || !password.value) {
    toast.warning('请输入用户名和密码')
    return
  }
  passwordLoading.value = true
  try {
    session.value = await loginWithPassword({
      username: trimmedUsername,
      password: password.value,
    })
    trackAnalytics('login_success', {
      method: 'password',
      source_route: '/login',
    })
    password.value = ''
    toast.success('已登录')
    await router.push(returnTo.value)
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '用户名或密码不正确')
  } finally {
    passwordLoading.value = false
  }
}

async function logout() {
  passwordLoading.value = true
  try {
    await logoutBackendSession()
    session.value = null
    toast.success('已退出当前会话')
  } catch (error) {
    toast.error(error instanceof Error ? error.message : '退出登录失败')
  } finally {
    passwordLoading.value = false
  }
}
</script>

<template>
  <main class="login-page grid min-h-[100dvh] place-items-center px-5 py-8">
    <div class="flex w-full max-w-[440px] flex-col items-center">
      <section class="mb-5 flex flex-col items-center text-center">
        <div class="h-12 w-12 overflow-hidden rounded-lg">
          <img src="/c2cmarket-icon-512.png?v=20260806-deep-violet" alt="" class="h-full w-full object-cover" />
        </div>
        <h1 class="mt-3 text-2xl font-semibold text-primary">C2CMarket</h1>
        <p class="mt-1 text-sm text-muted-foreground">订阅拼车、API 服务与官网价格市场</p>
      </section>

      <Card class="w-full rounded-lg border-border bg-card p-5 shadow-sm md:p-6">
        <div class="mb-5 text-center">
          <h2 class="text-xl font-semibold tracking-tight text-foreground">欢迎回来</h2>
          <p class="mt-1 text-sm text-muted-foreground">
            使用 linux.do 登录 C2CMarket
          </p>
        </div>

        <div v-if="loadingSession" class="grid min-h-56 place-items-center text-sm text-muted-foreground">
          <span class="inline-flex items-center gap-2"><Loader2 class="h-4 w-4 animate-spin" />读取会话</span>
        </div>

        <div v-else class="space-y-4">
          <div v-if="loggedIn" class="rounded-xl border border-border bg-accent/45 p-4">
            <div class="flex items-start gap-3">
              <div class="grid h-11 w-11 place-items-center rounded-xl bg-primary text-primary-foreground">
                <UserRound class="h-5 w-5" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="truncate font-medium">{{ displayName }}</div>
                <div class="mt-1 text-xs text-muted-foreground">@{{ session?.user.username }}</div>
                <div class="mt-3 flex flex-wrap gap-2">
                  <Badge v-if="linuxDo?.bound" variant="trust">已绑定 linux.do</Badge>
                  <Badge v-if="linuxDo?.trustLevel" variant="secondary">信任等级{{ linuxDo.trustLevel }}</Badge>
                  <Badge v-if="isAdmin" variant="verified">管理员</Badge>
                </div>
              </div>
            </div>
            <div class="mt-5 grid gap-2 sm:grid-cols-2">
              <RouterLink to="/my">
                <Button class="h-11 w-full">进入工作台</Button>
              </RouterLink>
              <Button variant="outline" class="h-11 w-full bg-card/75" :disabled="passwordLoading" @click="logout">
                退出登录
              </Button>
            </div>
          </div>

          <template v-else>
            <div class="space-y-4">
              <Button
                class="h-11 w-full rounded-lg text-base"
                :disabled="oauthLoading"
                @click="loginWithLinuxDo"
              >
                <Loader2 v-if="oauthLoading" class="h-4 w-4 animate-spin" />
                <img v-else src="/linuxdo-mark.svg" alt="" class="h-5 w-5" />
                使用 linux.do 登录
                <ArrowRight class="ml-auto h-4 w-4" />
              </Button>

              <p class="text-center text-xs leading-5 text-muted-foreground">
                请使用 linux.do 登录；密码登录仅用于已绑定 linux.do 的账号恢复访问。
              </p>

              <p class="text-center text-xs leading-5 text-muted-foreground">
                账号已被暂停或封禁？
                <RouterLink class="font-medium text-primary hover:underline" to="/account-appeal">发起账号申诉</RouterLink>
              </p>

              <div class="relative">
                <div class="absolute inset-0 flex items-center"><span class="w-full border-t border-border"></span></div>
                <div class="relative flex justify-center text-xs"><span class="bg-card px-3 text-muted-foreground">账号恢复</span></div>
              </div>

              <Button type="button" variant="outline" class="h-11 w-full rounded-lg bg-card/80 text-sm" @click="showPasswordLogin = !showPasswordLogin">
                <LockKeyhole class="h-4 w-4" />
                {{ showPasswordLogin ? '收起密码登录' : '已绑定 linux.do 用户密码登录' }}
              </Button>
            </div>
          </template>

          <form v-if="!loggedIn && showPasswordLogin" class="space-y-3" @submit.prevent="submitPasswordLogin">
            <div class="space-y-2">
              <label for="login-username" class="text-sm font-medium text-foreground">用户名</label>
              <div class="relative">
                <UserRound class="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="login-username"
                  v-model="username"
                  autocomplete="username"
                  class="h-11 rounded-lg bg-card pl-11 text-sm shadow-sm"
                  placeholder="请输入用户名"
                />
              </div>
            </div>

            <div class="space-y-2">
              <label for="login-password" class="text-sm font-medium text-foreground">密码</label>
              <div class="relative">
                <LockKeyhole class="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="login-password"
                  v-model="password"
                  :type="passwordVisible ? 'text' : 'password'"
                  autocomplete="current-password"
                  class="h-11 rounded-lg bg-card px-11 text-sm shadow-sm"
                  placeholder="请输入密码"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  class="absolute right-0 top-1/2 grid h-11 w-11 -translate-y-1/2 place-items-center rounded-r-lg text-muted-foreground transition hover:bg-muted hover:text-foreground"
                  :aria-label="passwordVisible ? '隐藏密码' : '显示密码'"
                  @click="passwordVisible = !passwordVisible"
                >
                  <EyeOff v-if="passwordVisible" class="h-4 w-4" />
                  <Eye v-else class="h-4 w-4" />
                </Button>
              </div>
            </div>

            <Button class="h-11 w-full rounded-lg text-base" :disabled="passwordLoading" type="submit">
              <Loader2 v-if="passwordLoading" class="h-4 w-4 animate-spin" />
              <LogIn v-else class="h-4 w-4" />
              密码登录
            </Button>
          </form>
        </div>
      </Card>

      <p class="mt-4 text-xs text-muted-foreground">© 2026 C2CMarket. All rights reserved.</p>
    </div>
  </main>
</template>

<style scoped>
.login-page {
  background: var(--background);
}
</style>
