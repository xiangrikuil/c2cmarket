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
  ShieldCheck,
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
const returnTo = computed(() => {
  const value = typeof route.query.returnTo === 'string' ? route.query.returnTo : '/my'
  return value.startsWith('/') && !value.startsWith('//') ? value : '/my'
})

const linuxDoIconPaths = [
  {
    d: 'm7.44,0s.09,0,.13,0c.09,0,.19,0,.28,0,.14,0,.29,0,.43,0,.09,0,.18,0,.27,0q.12,0,.25,0t.26.08c.15.03.29.06.44.08,1.97.38,3.78,1.47,4.95,3.11.04.06.09.12.13.18.67.96,1.15,2.11,1.3,3.28q0,.19.09.26c0,.15,0,.29,0,.44,0,.04,0,.09,0,.13,0,.09,0,.19,0,.28,0,.14,0,.29,0,.43,0,.09,0,.18,0,.27,0,.08,0,.17,0,.25q0,.19-.08.26c-.03.15-.06.29-.08.44-.38,1.97-1.47,3.78-3.11,4.95-.06.04-.12.09-.18.13-.96.67-2.11,1.15-3.28,1.3q-.19,0-.26.09c-.15,0-.29,0-.44,0-.04,0-.09,0-.13,0-.09,0-.19,0-.28,0-.14,0-.29,0-.43,0-.09,0-.18,0-.27,0-.08,0-.17,0-.25,0q-.19,0-.26-.08c-.15-.03-.29-.06-.44-.08-1.97-.38-3.78-1.47-4.95-3.11q-.07-.09-.13-.18c-.67-.96-1.15-2.11-1.3-3.28q0-.19-.09-.26c0-.15,0-.29,0-.44,0-.04,0-.09,0-.13,0-.09,0-.19,0-.28,0-.14,0-.29,0-.43,0-.09,0-.18,0-.27,0-.08,0-.17,0-.25q0-.19.08-.26c.03-.15.06-.29.08-.44.38-1.97,1.47-3.78,3.11-4.95.06-.04.12-.09.18-.13C4.42.73,5.57.26,6.74.1,7,.07,7.15,0,7.44,0Z',
    fill: '#EFEFEF',
  },
  {
    d: 'm1.27,11.33h13.45c-.94,1.89-2.51,3.21-4.51,3.88-1.99.59-3.96.37-5.8-.57-1.25-.7-2.67-1.9-3.14-3.3Z',
    fill: '#FEB005',
  },
  {
    d: 'm12.54,1.99c.87.7,1.82,1.59,2.18,2.68H1.27c.87-1.74,2.33-3.13,4.2-3.78,2.44-.79,5-.47,7.07,1.1Z',
    fill: '#1D1D1F',
  },
] as const

onMounted(async () => {
  await refreshSession()
})

async function refreshSession() {
  loadingSession.value = true
  try {
    session.value = await getCurrentBackendSession()
  } catch {
    session.value = null
  } finally {
    loadingSession.value = false
  }
}

async function loginWithLinuxDo() {
  oauthLoading.value = true
  try {
    const { authorizationUrl } = await startOAuthLogin(returnTo.value)
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
  <main class="login-page relative grid min-h-screen justify-items-center overflow-hidden px-5 pb-6 pt-8">
    <div class="relative z-10 flex w-full max-w-[450px] flex-col items-center">
      <section class="mb-5 flex flex-col items-center text-center">
        <div class="grid h-12 w-12 place-items-center rounded-xl bg-foreground text-background shadow-xl shadow-primary/20">
          <ShieldCheck class="h-6 w-6 text-primary" />
        </div>
        <h1 class="mt-3 text-2xl font-semibold text-primary">C2CMarket</h1>
        <p class="mt-1 text-sm text-muted-foreground">AI 低价情报与社区撮合平台</p>
      </section>

      <Card class="w-full rounded-2xl border-white bg-card p-5 shadow-2xl shadow-slate-900/12 backdrop-blur md:p-6">
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
                class="h-11 w-full rounded-lg text-base shadow-lg shadow-primary/25"
                :disabled="oauthLoading"
                @click="loginWithLinuxDo"
              >
                <Loader2 v-if="oauthLoading" class="h-4 w-4 animate-spin" />
                <svg v-else class="h-5 w-5" viewBox="0 0 16 16" aria-hidden="true" focusable="false">
                  <g data-name="linuxdo_icon">
                    <path v-for="path in linuxDoIconPaths" :key="path.fill" :d="path.d" :fill="path.fill" />
                  </g>
                </svg>
                使用 linux.do 登录
                <ArrowRight class="ml-auto h-4 w-4" />
              </Button>

              <p class="text-center text-xs leading-5 text-muted-foreground">
                请使用 linux.do 登录；密码登录仅用于已绑定 linux.do 的账号恢复访问。
              </p>

              <div class="relative">
                <div class="absolute inset-0 flex items-center"><span class="w-full border-t border-border"></span></div>
                <div class="relative flex justify-center text-xs"><span class="bg-card px-3 text-muted-foreground">账号恢复</span></div>
              </div>

              <Button type="button" variant="outline" class="h-10 w-full rounded-lg bg-card/80 text-sm" @click="showPasswordLogin = !showPasswordLogin">
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
                  class="h-10 rounded-lg bg-card pl-11 text-sm shadow-sm"
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
                  class="h-10 rounded-lg bg-card px-11 text-sm shadow-sm"
                  placeholder="请输入密码"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 grid h-8 w-8 -translate-y-1/2 place-items-center rounded-md text-muted-foreground transition hover:bg-muted hover:text-foreground"
                  :aria-label="passwordVisible ? '隐藏密码' : '显示密码'"
                  @click="passwordVisible = !passwordVisible"
                >
                  <EyeOff v-if="passwordVisible" class="h-4 w-4" />
                  <Eye v-else class="h-4 w-4" />
                </button>
              </div>
            </div>

            <Button class="h-10 w-full rounded-lg text-base shadow-lg shadow-primary/25" :disabled="passwordLoading" type="submit">
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
  background:
    radial-gradient(circle at 8% 88%, color-mix(in oklab, var(--primary) 18%, transparent) 0 0, transparent 260px),
    radial-gradient(circle at 92% 4%, color-mix(in oklab, var(--primary) 14%, transparent) 0 0, transparent 320px),
    linear-gradient(color-mix(in oklab, var(--primary) 7%, transparent) 1px, transparent 1px),
    linear-gradient(90deg, color-mix(in oklab, var(--primary) 7%, transparent) 1px, transparent 1px),
    color-mix(in oklab, var(--background) 96%, white);
  background-size: auto, auto, 42px 42px, 42px 42px, auto;
}
</style>
