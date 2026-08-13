<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQueryClient } from '@tanstack/vue-query'
import {
  ArrowRight,
  Eye,
  EyeOff,
  LockKeyhole,
  LogIn,
  Loader2,
  Mail,
  MailCheck,
  UserPlus,
  UserRound,
} from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import TurnstileWidget from '@/components/auth/TurnstileWidget.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  getCurrentBackendSession,
  getEmailRegistrationConfig,
  confirmEmailRegistration,
  loginWithPassword,
  startEmailRegistration,
  startOAuthLogin,
  BackendProblemError,
  backendErrorMessage,
  type BackendSession,
  type EmailRegistrationConfig,
} from '@/lib/backendClient'
import { normalizeReturnTo } from '@/lib/authNavigation'
import { trackAnalytics } from '@/lib/analytics'
import { captureReferralCode, getReferralCapture } from '@/lib/referralCapture'
import { CAPABILITY, hasCapability } from '@/lib/capabilities'
import { getBackupPasswordValidationMessage } from '@/lib/passwordPolicy'
import { logoutCurrentSession } from '@/lib/sessionActions'

const route = useRoute()
const router = useRouter()
const queryClient = useQueryClient()
const runtimeConfig = useRuntimeConfig()
const session = ref<BackendSession | null>(null)
const loadingSession = ref(true)
const oauthLoading = ref(false)
const passwordLoading = ref(false)
const passwordVisible = ref(false)
const username = ref('')
const password = ref('')
const showPasswordLogin = ref(false)
const turnstileToken = ref('')
const turnstileWidget = ref<{ reset: () => void } | null>(null)
const sessionLoadError = ref('')
const registrationConfig = ref<EmailRegistrationConfig | null>(null)
const registrationConfigLoading = ref(true)
const registrationConfigError = ref('')
const registrationOpen = ref(false)
const registrationStep = ref<'start' | 'confirm'>('start')
const registrationLoading = ref(false)
const registrationEmail = ref('')
const registrationCode = ref('')
const registrationUsername = ref('')
const registrationPassword = ref('')
const registrationPasswordConfirm = ref('')
const registrationPasswordVisible = ref(false)
const registrationTurnstileToken = ref('')
const registrationTurnstileWidget = ref<{ reset: () => void } | null>(null)
const registrationDevCode = ref('')

const loggedIn = computed(() => Boolean(session.value))
const displayName = computed(() => session.value?.user.displayName ?? session.value?.user.username ?? '未登录')
const linuxDo = computed(() => session.value?.user.linuxDoBinding)
const isAdmin = computed(() => hasCapability(session.value?.user, CAPABILITY.adminAccess))
const returnTo = computed(() => normalizeReturnTo(route.query.returnTo))
const turnstileSiteKey = computed(() => String(runtimeConfig.public.turnstileSiteKey ?? '').trim())

onMounted(async () => {
  const referral = Array.isArray(route.query.ref) ? route.query.ref[0] : route.query.ref
  captureReferralCode(referral)
  trackAnalytics('login_page_view', { source_route: '/login' })
  await Promise.all([refreshSession(), loadRegistrationConfig()])
})

async function refreshSession() {
  loadingSession.value = true
  sessionLoadError.value = ''
  try {
    session.value = await getCurrentBackendSession()
    if (session.value) await router.replace(returnTo.value)
  } catch (error) {
    session.value = null
    if (!(error instanceof BackendProblemError) || !['SESSION_EXPIRED', 'SESSION_REVOKED'].includes(error.code)) {
      sessionLoadError.value = backendErrorMessage(error, '暂时无法读取登录状态。')
      toast.error(sessionLoadError.value)
    }
  } finally {
    loadingSession.value = false
  }
}

async function loadRegistrationConfig() {
  registrationConfigLoading.value = true
  registrationConfigError.value = ''
  try {
    registrationConfig.value = await getEmailRegistrationConfig()
  } catch (error) {
    registrationConfig.value = null
    registrationConfigError.value = backendErrorMessage(error, '暂时无法读取学校邮箱注册配置。')
  } finally {
    registrationConfigLoading.value = false
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
  if (!turnstileToken.value) {
    toast.warning('请先完成人机验证')
    return
  }
  passwordLoading.value = true
  try {
    session.value = await loginWithPassword({
      username: trimmedUsername,
      password: password.value,
      turnstileToken: turnstileToken.value,
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
    turnstileWidget.value?.reset()
    passwordLoading.value = false
  }
}

async function requestRegistrationCode() {
  const email = registrationEmail.value.trim().toLowerCase()
  if (!email) {
    toast.warning('请输入学校邮箱。')
    return
  }
  if (!registrationTurnstileToken.value) {
    toast.warning('请先完成人机验证。')
    return
  }
  registrationLoading.value = true
  try {
    const challenge = await startEmailRegistration({
      email,
      turnstileToken: registrationTurnstileToken.value,
    })
    registrationEmail.value = challenge.email
    registrationDevCode.value = challenge.devCode ?? ''
    registrationStep.value = 'confirm'
    toast.success('验证码已发送，请查收学校邮箱。')
  } catch (error) {
    toast.error(backendErrorMessage(error, '验证码发送失败。'))
  } finally {
    registrationTurnstileToken.value = ''
    registrationTurnstileWidget.value?.reset()
    registrationLoading.value = false
  }
}

async function submitEmailRegistration() {
  const usernameValue = registrationUsername.value
  if (!/^[a-z0-9_-]{3,24}$/.test(usernameValue)) {
    toast.warning('用户名需为 3–24 位小写字母、数字、下划线或短横线。')
    return
  }
  if (!/^\d{6}$/.test(registrationCode.value.trim())) {
    toast.warning('请输入 6 位验证码。')
    return
  }
  const passwordError = getBackupPasswordValidationMessage(registrationPassword.value)
  if (passwordError) {
    toast.warning(passwordError)
    return
  }
  if (registrationPassword.value !== registrationPasswordConfirm.value) {
    toast.warning('两次输入的密码不一致。')
    return
  }
  registrationLoading.value = true
  try {
    session.value = await confirmEmailRegistration({
      email: registrationEmail.value,
      code: registrationCode.value.trim(),
      username: usernameValue,
      password: registrationPassword.value,
    })
    registrationPassword.value = ''
    registrationPasswordConfirm.value = ''
    toast.success('学生账号已创建并登录。')
    await router.push(returnTo.value)
  } catch (error) {
    toast.error(backendErrorMessage(error, '注册确认失败。'))
  } finally {
    registrationLoading.value = false
  }
}

function restartEmailRegistration() {
  registrationStep.value = 'start'
  registrationCode.value = ''
  registrationDevCode.value = ''
  registrationTurnstileToken.value = ''
}

async function logout() {
  passwordLoading.value = true
  try {
    await logoutCurrentSession(queryClient)
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

              <Button
                v-if="registrationConfig?.enabled"
                type="button"
                variant="outline"
                class="h-11 w-full rounded-lg bg-card/80 text-sm"
                @click="registrationOpen = !registrationOpen; showPasswordLogin = false"
              >
                <UserPlus class="h-4 w-4" />
                {{ registrationOpen ? '收起学校邮箱注册' : '使用学校邮箱注册' }}
              </Button>
              <p v-else-if="registrationConfigLoading" class="text-center text-xs text-muted-foreground">正在读取学校邮箱注册配置…</p>
              <div v-else-if="registrationConfigError" class="flex items-center justify-between gap-2 rounded-md border border-border p-3 text-xs text-muted-foreground">
                <span>{{ registrationConfigError }}</span>
                <Button size="sm" variant="ghost" @click="loadRegistrationConfig">重试</Button>
              </div>
              <p v-else class="text-center text-xs text-muted-foreground">学校邮箱注册暂未开放。</p>

              <p v-if="sessionLoadError" class="rounded-md border border-destructive/25 bg-destructive/5 p-3 text-xs leading-5 text-destructive">
                {{ sessionLoadError }}
              </p>

              <p class="text-center text-xs leading-5 text-muted-foreground">
                已有学生账号或已绑定 linux.do 的账号，也可以使用用户名或学校邮箱和密码登录。
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
                {{ showPasswordLogin ? '收起密码登录' : '用户名 / 学校邮箱密码登录' }}
              </Button>
            </div>
          </template>

          <div v-if="!loggedIn && registrationOpen" class="rounded-lg border border-border bg-muted/20 p-4">
            <form v-if="registrationStep === 'start'" class="space-y-4" @submit.prevent="requestRegistrationCode">
              <div>
                <h3 class="font-semibold">学校邮箱验证</h3>
                <p class="mt-1 text-xs leading-5 text-muted-foreground">
                  输入已开放学校的邮箱，验证码通过后再创建用户名和密码。
                </p>
              </div>
              <label class="block space-y-2">
                <span class="text-sm font-medium">学校邮箱</span>
                <span class="relative block">
                  <Mail class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input v-model="registrationEmail" type="email" autocomplete="email" class="h-11 pl-10" placeholder="name@school.edu" />
                </span>
              </label>
              <p v-if="registrationConfig?.institutions.length" class="text-xs leading-5 text-muted-foreground">
                当前开放：{{ registrationConfig.institutions.map(item => `${item.institutionName}（@${item.domain}）`).join('、') }}
              </p>
              <TurnstileWidget
                ref="registrationTurnstileWidget"
                :site-key="turnstileSiteKey"
                action="student_signup"
                @update:token="registrationTurnstileToken = $event"
              />
              <Button class="h-11 w-full" type="submit" :disabled="registrationLoading || !registrationTurnstileToken">
                <Loader2 v-if="registrationLoading" class="h-4 w-4 animate-spin" />
                <MailCheck v-else class="h-4 w-4" />
                发送验证码
              </Button>
            </form>

            <form v-else class="space-y-4" @submit.prevent="submitEmailRegistration">
              <div class="flex items-start justify-between gap-3">
                <div>
                  <h3 class="font-semibold">创建学生账号</h3>
                  <p class="mt-1 text-xs text-muted-foreground">验证码已发送至 {{ registrationEmail }}</p>
                </div>
                <Button type="button" size="sm" variant="ghost" @click="restartEmailRegistration">更换邮箱</Button>
              </div>
              <label class="block space-y-2">
                <span class="text-sm font-medium">6 位验证码</span>
                <span class="relative block">
                  <MailCheck class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input v-model="registrationCode" inputmode="numeric" autocomplete="one-time-code" maxlength="6" class="h-11 pl-10" placeholder="123456" />
                </span>
                <span v-if="registrationDevCode" class="block rounded-md border border-dashed border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900">开发验证码：{{ registrationDevCode }}</span>
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">用户名</span>
                <span class="relative block">
                  <UserRound class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                  <Input v-model="registrationUsername" autocomplete="username" class="h-11 pl-10" placeholder="3–24 位小写字母、数字、_ 或 -" />
                </span>
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">密码</span>
                <Input v-model="registrationPassword" :type="registrationPasswordVisible ? 'text' : 'password'" autocomplete="new-password" class="h-11" placeholder="8–32 位，包含字母、数字和特殊字符" />
              </label>
              <label class="block space-y-2">
                <span class="text-sm font-medium">确认密码</span>
                <Input v-model="registrationPasswordConfirm" :type="registrationPasswordVisible ? 'text' : 'password'" autocomplete="new-password" class="h-11" placeholder="再次输入密码" />
              </label>
              <Button type="button" size="sm" variant="ghost" class="px-0" @click="registrationPasswordVisible = !registrationPasswordVisible">
                <EyeOff v-if="registrationPasswordVisible" class="h-4 w-4" /><Eye v-else class="h-4 w-4" />
                {{ registrationPasswordVisible ? '隐藏密码' : '显示密码' }}
              </Button>
              <Button class="h-11 w-full" type="submit" :disabled="registrationLoading">
                <Loader2 v-if="registrationLoading" class="h-4 w-4 animate-spin" />
                <UserPlus v-else class="h-4 w-4" />
                创建账号并登录
              </Button>
            </form>
          </div>

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

            <TurnstileWidget
              ref="turnstileWidget"
              :site-key="turnstileSiteKey"
              action="password_login"
              @update:token="turnstileToken = $event"
            />

            <Button class="h-11 w-full rounded-lg text-base" :disabled="passwordLoading || !turnstileToken" type="submit">
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
