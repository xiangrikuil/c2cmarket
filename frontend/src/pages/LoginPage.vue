<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import AuthPageShell from '@/components/auth/AuthPageShell.vue'
import LoginPanel from '@/components/auth/LoginPanel.vue'
import StudentRegistrationPanel from '@/components/auth/StudentRegistrationPanel.vue'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  BackendProblemError,
  backendErrorMessage,
  getCurrentBackendSession,
  getEmailRegistrationConfig,
  type BackendSession,
  type EmailRegistrationConfig,
} from '@/lib/backendClient'
import { normalizeReturnTo } from '@/lib/authNavigation'
import { trackAnalytics } from '@/lib/analytics'
import { captureReferralCode } from '@/lib/referralCapture'

type AuthMode = 'login' | 'student-register'

const route = useRoute()
const router = useRouter()
const runtimeConfig = useRuntimeConfig()
const authMode = ref<AuthMode>('login')
const loadingSession = ref(true)
const loginRedirecting = ref(false)
const sessionLoadError = ref('')
const registrationConfig = ref<EmailRegistrationConfig | null>(null)
const registrationConfigLoading = ref(true)
const registrationConfigError = ref('')

const returnTo = computed(() => normalizeReturnTo(route.query.returnTo))
const turnstileSiteKey = computed(() => String(runtimeConfig.public.turnstileSiteKey ?? '').trim())

onMounted(async () => {
  const referral = Array.isArray(route.query.ref) ? route.query.ref[0] : route.query.ref
  captureReferralCode(referral)
  trackAnalytics('login_page_view', { source_route: '/login' })
  await Promise.all([refreshSession(), loadRegistrationConfig()])
})

const refreshSession = async () => {
  loadingSession.value = true
  sessionLoadError.value = ''
  try {
    const session = await getCurrentBackendSession()
    if (session?.audience === 'restricted_business') {
      await router.replace('/restricted-business')
    } else if (session) {
      await router.replace(returnTo.value)
    }
  } catch (error) {
    if (!(error instanceof BackendProblemError) || !['SESSION_EXPIRED', 'SESSION_REVOKED'].includes(error.code)) {
      sessionLoadError.value = backendErrorMessage(error, '暂时无法读取登录状态。')
    }
  } finally {
    loadingSession.value = false
  }
}

const loadRegistrationConfig = async () => {
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

const setAuthMode = (value: unknown) => {
  if (value === 'login' || value === 'student-register') authMode.value = value
}

const routeAuthenticatedSession = async (session: BackendSession) => {
  if (authMode.value === 'student-register') {
    if (session.audience === 'restricted_business') {
      await router.push('/restricted-business')
      return
    }
    toast.success('学生账号已创建。')
    await router.push(returnTo.value)
    return
  }

  loginRedirecting.value = true
  sessionLoadError.value = ''
  try {
    if (session.audience === 'restricted_business') {
      await router.push('/restricted-business')
      return
    }
    toast.success('已登录。')
    await router.push(returnTo.value)
  } catch {
    loginRedirecting.value = false
    sessionLoadError.value = '登录已完成，但页面跳转失败，请重试。'
  }
}

</script>

<template>
  <AuthPageShell>
    <div v-if="loadingSession" class="grid min-h-72 place-items-center text-sm text-muted-foreground" aria-live="polite">
      <span class="inline-flex items-center gap-2"><Loader2 class="h-4 w-4 animate-spin" />读取会话</span>
    </div>

    <div v-else-if="loginRedirecting" class="grid min-h-72 place-items-center text-center" aria-live="polite" aria-busy="true">
      <div>
        <Loader2 class="mx-auto h-7 w-7 animate-spin text-primary" />
        <p class="mt-3 font-medium">登录成功</p>
        <p class="mt-1 text-sm text-muted-foreground">正在进入 C2CMarket...</p>
      </div>
    </div>

    <Tabs v-else :model-value="authMode" class="gap-5" @update:model-value="setAuthMode">
      <TabsList class="grid h-10 w-full grid-cols-2">
        <TabsTrigger value="login">登录</TabsTrigger>
        <TabsTrigger value="student-register">学生注册</TabsTrigger>
      </TabsList>

      <TabsContent value="login" class="mt-0">
        <LoginPanel
          v-if="authMode === 'login'"
          :return-to="returnTo"
          :turnstile-site-key="turnstileSiteKey"
          :session-load-error="sessionLoadError"
          @authenticated="routeAuthenticatedSession"
        />
      </TabsContent>

      <TabsContent value="student-register" class="mt-0">
        <StudentRegistrationPanel
          v-if="authMode === 'student-register'"
          :config="registrationConfig"
          :config-loading="registrationConfigLoading"
          :config-error="registrationConfigError"
          :turnstile-site-key="turnstileSiteKey"
          @retry-config="loadRegistrationConfig"
          @authenticated="routeAuthenticatedSession"
        />
      </TabsContent>
    </Tabs>
  </AuthPageShell>
</template>
