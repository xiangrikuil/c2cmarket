<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { ArrowRight, Loader2, LogIn } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import PasswordInput from '@/components/auth/PasswordInput.vue'
import TurnstileWidget from '@/components/auth/TurnstileWidget.vue'
import {
  BackendProblemError,
  backendErrorMessage,
  loginWithPassword,
  startOAuthLogin,
  type BackendSession,
} from '@/lib/backendClient'
import {
  backendFieldErrors,
  focusFirstInvalidField,
  validateLoginIdentifier,
  validateRequiredPassword,
} from '@/lib/authFormValidation'
import { passwordResetRoute } from '@/lib/authNavigation'
import { trackAnalytics } from '@/lib/analytics'
import { getReferralCapture } from '@/lib/referralCapture'

const props = defineProps<{
  returnTo: string
  turnstileSiteKey: string
  sessionLoadError?: string
}>()

const emit = defineEmits<{
  (event: 'authenticated', session: BackendSession): void
}>()

type PendingAction = null | 'oauth-login' | 'password-login'

const identifier = ref('')
const password = ref('')
const turnstileToken = ref('')
const pendingAction = ref<PendingAction>(null)
const panelError = ref('')
const serverErrors = reactive<Record<string, string>>({})
const touched = reactive({ identifier: false, password: false, turnstile: false })
const turnstileWidget = ref<{ reset: () => void } | null>(null)
let requestGeneration = 0

const identifierError = computed(() => serverErrors.identifier || (touched.identifier ? validateLoginIdentifier(identifier.value) : ''))
const passwordError = computed(() => serverErrors.password || (touched.password ? validateRequiredPassword(password.value) : ''))
const turnstileError = computed(() => touched.turnstile && !turnstileToken.value ? '请先完成人机验证。' : '')
const resetRoute = computed(() => passwordResetRoute(props.returnTo))

watch(identifier, () => {
  delete serverErrors.identifier
  panelError.value = ''
})
watch(password, () => {
  delete serverErrors.password
  panelError.value = ''
})

onUnmounted(() => {
  requestGeneration += 1
})

const loginWithLinuxDo = async () => {
  if (pendingAction.value) return
  pendingAction.value = 'oauth-login'
  panelError.value = ''
  const generation = ++requestGeneration
  try {
    const { authorizationUrl } = await startOAuthLogin(props.returnTo, getReferralCapture())
    if (generation !== requestGeneration) return
    trackAnalytics('oauth_login_start', { method: 'oauth_linux_do', source_route: '/login' })
    window.location.assign(authorizationUrl)
  } catch (error) {
    if (generation === requestGeneration) panelError.value = backendErrorMessage(error, '启动 linux.do 登录失败。')
  } finally {
    if (generation === requestGeneration) pendingAction.value = null
  }
}

const submitPasswordLogin = async () => {
  if (pendingAction.value) return
  touched.identifier = true
  touched.password = true
  touched.turnstile = true
  const validationErrors = {
    identifier: validateLoginIdentifier(identifier.value),
    password: validateRequiredPassword(password.value),
    turnstile: turnstileToken.value ? '' : '请先完成人机验证。',
  }
  if (Object.values(validationErrors).some(Boolean)) {
    await focusFirstInvalidField(validationErrors, [
      ['identifier', 'login-identifier'],
      ['password', 'login-password'],
      ['turnstile', 'login-turnstile'],
    ])
    return
  }

  pendingAction.value = 'password-login'
  panelError.value = ''
  Object.keys(serverErrors).forEach(key => delete serverErrors[key])
  const generation = ++requestGeneration
  try {
    const session = await loginWithPassword({
      username: identifier.value.trim(),
      password: password.value,
      turnstileToken: turnstileToken.value,
    })
    if (generation !== requestGeneration) return
    trackAnalytics('login_success', { method: 'password', source_route: '/login' })
    emit('authenticated', session)
  } catch (error) {
    if (generation !== requestGeneration) return
    const { fields: mapped, hasUnmapped } = backendFieldErrors(
      error,
      { username: 'identifier' },
      ['identifier', 'password'],
    )
    Object.assign(serverErrors, mapped)
    if (error instanceof BackendProblemError && error.code === 'INVALID_CREDENTIALS') {
      panelError.value = '用户名、学校邮箱或密码不正确。'
    } else if (hasUnmapped || !Object.keys(mapped).length) {
      panelError.value = backendErrorMessage(error, '登录失败，请稍后重试。')
    }
    await focusFirstInvalidField(serverErrors, [
      ['identifier', 'login-identifier'],
      ['password', 'login-password'],
    ])
  } finally {
    turnstileToken.value = ''
    turnstileWidget.value?.reset()
    if (generation === requestGeneration) pendingAction.value = null
  }
}
</script>

<template>
  <div class="space-y-5">
    <Alert v-if="sessionLoadError || panelError" variant="destructive" aria-live="polite">
      <AlertDescription>{{ panelError || sessionLoadError }}</AlertDescription>
    </Alert>

    <div>
      <Button
        class="h-11 w-full rounded-lg text-sm"
        :disabled="pendingAction !== null"
        @click="loginWithLinuxDo"
      >
        <Loader2 v-if="pendingAction === 'oauth-login'" class="h-4 w-4 animate-spin" />
        <img v-else src="/linuxdo-mark.svg" alt="" class="h-5 w-5" />
        使用 linux.do 继续
        <ArrowRight class="ml-auto h-4 w-4" />
      </Button>
      <p class="mt-2 text-center text-xs text-muted-foreground">首次使用将自动创建账号</p>
    </div>

    <div class="relative" aria-hidden="true">
      <div class="absolute inset-0 flex items-center"><span class="w-full border-t border-border" /></div>
      <div class="relative flex justify-center text-xs"><span class="bg-card px-3 text-muted-foreground">或使用用户名 / 学校邮箱</span></div>
    </div>

    <form class="space-y-4" novalidate @submit.prevent="submitPasswordLogin">
      <div class="space-y-2">
        <label for="login-identifier" class="text-sm font-medium text-foreground">用户名或学校邮箱</label>
        <Input
          id="login-identifier"
          v-model="identifier"
          autocomplete="username"
          class="h-11"
          :class="identifierError ? 'border-destructive' : ''"
          :aria-invalid="identifierError ? 'true' : undefined"
          :aria-describedby="identifierError ? 'login-identifier-error' : undefined"
          placeholder="用户名或 student@school.edu"
          @blur="touched.identifier = true"
        />
        <p v-if="identifierError" id="login-identifier-error" class="text-xs text-destructive">{{ identifierError }}</p>
      </div>

      <div class="space-y-2">
        <div class="flex items-center justify-between gap-3">
          <label for="login-password" class="text-sm font-medium text-foreground">密码</label>
          <RouterLink :to="resetRoute" class="text-xs font-medium text-primary hover:underline">忘记密码？</RouterLink>
        </div>
        <PasswordInput
          id="login-password"
          v-model="password"
          label="密码"
          autocomplete="current-password"
          placeholder="请输入密码"
          :invalid="Boolean(passwordError)"
          :described-by="passwordError ? 'login-password-error' : undefined"
          @blur="touched.password = true"
        />
        <p v-if="passwordError" id="login-password-error" class="text-xs text-destructive">{{ passwordError }}</p>
      </div>

      <div id="login-turnstile" tabindex="-1">
        <TurnstileWidget
          ref="turnstileWidget"
          :site-key="turnstileSiteKey"
          action="password_login"
          @update:token="turnstileToken = $event; touched.turnstile = true"
        />
        <p v-if="turnstileError" class="mt-2 text-xs text-destructive">{{ turnstileError }}</p>
      </div>

      <Button class="h-11 w-full" type="submit" :disabled="pendingAction !== null">
        <Loader2 v-if="pendingAction === 'password-login'" class="h-4 w-4 animate-spin" />
        <LogIn v-else class="h-4 w-4" />
        登录
      </Button>
    </form>

    <p class="text-center text-xs leading-5 text-muted-foreground">
      账号被限制？
      <RouterLink class="font-medium text-primary hover:underline" to="/account-appeal">发起申诉</RouterLink>
    </p>
  </div>
</template>
