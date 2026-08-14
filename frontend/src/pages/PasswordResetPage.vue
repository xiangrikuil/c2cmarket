<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { CheckCircle2, Loader2, MailCheck, RefreshCw } from 'lucide-vue-next'
import AuthPageShell from '@/components/auth/AuthPageShell.vue'
import PasswordInput from '@/components/auth/PasswordInput.vue'
import TurnstileWidget from '@/components/auth/TurnstileWidget.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  BackendProblemError,
  backendErrorMessage,
  confirmPasswordReset,
  startPasswordReset,
} from '@/lib/backendClient'
import {
  backendFieldErrors,
  focusFirstInvalidField,
  validateEmail,
  validateNewPassword,
  validatePasswordConfirmation,
  validateVerificationCode,
} from '@/lib/authFormValidation'
import { loginRoute, normalizeReturnTo } from '@/lib/authNavigation'

type ResetStep = 'request' | 'confirm' | 'completed'
type PendingAction = null | 'reset-start' | 'reset-confirm'

const route = useRoute()
const runtimeConfig = useRuntimeConfig()
const step = ref<ResetStep>('request')
const pendingAction = ref<PendingAction>(null)
const email = ref('')
const code = ref('')
const newPassword = ref('')
const passwordConfirm = ref('')
const turnstileToken = ref('')
const turnstileWidget = ref<{ reset: () => void } | null>(null)
const panelError = ref('')
const serverErrors = reactive<Record<string, string>>({})
const touched = reactive({ email: false, code: false, newPassword: false, passwordConfirm: false, turnstile: false })
const resendRequested = ref(false)
const resendCompleted = ref(false)
let requestGeneration = 0

const returnTo = computed(() => normalizeReturnTo(route.query.returnTo))
const loginDestination = computed(() => loginRoute(returnTo.value))
const turnstileSiteKey = computed(() => String(runtimeConfig.public.turnstileSiteKey ?? '').trim())
const canonicalEmail = computed(() => email.value.trim().toLowerCase())
const emailError = computed(() => serverErrors.email || (touched.email ? validateEmail(email.value) : ''))
const codeError = computed(() => serverErrors.code || (touched.code ? validateVerificationCode(code.value) : ''))
const newPasswordError = computed(() => serverErrors.newPassword || (touched.newPassword ? validateNewPassword(newPassword.value) : ''))
const passwordConfirmError = computed(() => touched.passwordConfirm ? validatePasswordConfirmation(newPassword.value, passwordConfirm.value) : '')
const turnstileError = computed(() => touched.turnstile && !turnstileToken.value ? '请先完成人机验证。' : '')

watch(email, () => {
  requestGeneration += 1
  if (pendingAction.value === 'reset-start') pendingAction.value = null
  turnstileToken.value = ''
  touched.turnstile = false
  turnstileWidget.value?.reset()
  delete serverErrors.email
  panelError.value = ''
})
watch(code, () => delete serverErrors.code)
watch(newPassword, () => delete serverErrors.newPassword)

onUnmounted(() => {
  requestGeneration += 1
})

const clearServerErrors = () => {
  Object.keys(serverErrors).forEach(key => delete serverErrors[key])
  panelError.value = ''
}

const requestResetCode = async () => {
  if (pendingAction.value || step.value !== 'request') return
  touched.email = true
  touched.turnstile = true
  const validationErrors = {
    email: validateEmail(email.value),
    turnstile: turnstileToken.value ? '' : '请先完成人机验证。',
  }
  if (Object.values(validationErrors).some(Boolean)) {
    await focusFirstInvalidField(validationErrors, [
      ['email', 'reset-email'],
      ['turnstile', 'reset-turnstile'],
    ])
    return
  }

  pendingAction.value = 'reset-start'
  clearServerErrors()
  const submittedEmail = canonicalEmail.value
  const generation = ++requestGeneration
  try {
    await startPasswordReset({ email: submittedEmail, turnstileToken: turnstileToken.value })
    if (generation !== requestGeneration || submittedEmail !== canonicalEmail.value) return
    code.value = ''
    step.value = 'confirm'
    resendCompleted.value = resendRequested.value
    resendRequested.value = false
  } catch (error) {
    if (generation !== requestGeneration) return
    const { fields: mapped, hasUnmapped } = backendFieldErrors(error, {}, ['email'])
    Object.assign(serverErrors, mapped)
    if (hasUnmapped || !Object.keys(mapped).length) panelError.value = backendErrorMessage(error, '暂时无法提交请求，请稍后重试。')
    await focusFirstInvalidField(serverErrors, [['email', 'reset-email']])
  } finally {
    if (generation === requestGeneration) {
      turnstileToken.value = ''
      turnstileWidget.value?.reset()
      pendingAction.value = null
    }
  }
}

const confirmReset = async () => {
  if (pendingAction.value || step.value !== 'confirm') return
  touched.code = true
  touched.newPassword = true
  touched.passwordConfirm = true
  const validationErrors = {
    code: validateVerificationCode(code.value),
    newPassword: validateNewPassword(newPassword.value),
    passwordConfirm: validatePasswordConfirmation(newPassword.value, passwordConfirm.value),
  }
  if (Object.values(validationErrors).some(Boolean)) {
    await focusFirstInvalidField(validationErrors, [
      ['code', 'reset-code'],
      ['newPassword', 'reset-new-password'],
      ['passwordConfirm', 'reset-password-confirm'],
    ])
    return
  }

  pendingAction.value = 'reset-confirm'
  clearServerErrors()
  const generation = ++requestGeneration
  try {
    await confirmPasswordReset({
      email: canonicalEmail.value,
      code: code.value.trim(),
      newPassword: newPassword.value,
    })
    if (generation !== requestGeneration || step.value !== 'confirm') return
    step.value = 'completed'
    newPassword.value = ''
    passwordConfirm.value = ''
  } catch (error) {
    if (generation !== requestGeneration || step.value === 'completed') return
    const { fields: mapped, hasUnmapped } = backendFieldErrors(error, {}, ['code', 'newPassword'])
    Object.assign(serverErrors, mapped)
    if (error instanceof BackendProblemError && error.code === 'VERIFICATION_CODE_INVALID') {
      serverErrors.code = '验证码无效或已过期，请重新获取。'
    } else if (hasUnmapped || !Object.keys(mapped).length) {
      panelError.value = backendErrorMessage(error, '密码重置失败，请稍后重试。')
    }
    await focusFirstInvalidField(serverErrors, [
      ['code', 'reset-code'],
      ['newPassword', 'reset-new-password'],
    ])
  } finally {
    if (generation === requestGeneration && step.value !== 'completed') pendingAction.value = null
  }
}

const restartRequest = () => {
  if (pendingAction.value || step.value === 'completed') return
  requestGeneration += 1
  step.value = 'request'
  code.value = ''
  turnstileToken.value = ''
  touched.turnstile = false
  resendRequested.value = true
  resendCompleted.value = false
  clearServerErrors()
  turnstileWidget.value?.reset()
}
</script>

<template>
  <AuthPageShell
    title="重置密码"
    description="使用学生账号已验证的学校邮箱重置密码"
    :back-to="loginDestination"
  >
    <div class="space-y-5">
      <div v-if="step !== 'completed'" class="flex items-center justify-between gap-3 text-xs text-muted-foreground" aria-label="密码重置进度">
        <span :class="step === 'request' ? 'font-medium text-primary' : ''">1. 验证邮箱</span>
        <span class="h-px flex-1 bg-border" />
        <span :class="step === 'confirm' ? 'font-medium text-primary' : ''">2. 设置新密码</span>
      </div>

      <Alert v-if="panelError" variant="destructive" aria-live="polite">
        <AlertDescription>{{ panelError }}</AlertDescription>
      </Alert>

      <form v-if="step === 'request'" class="space-y-4" novalidate @submit.prevent="requestResetCode">
        <Alert v-if="resendRequested">
          <RefreshCw class="h-4 w-4" />
          <AlertDescription>重新发送需要先完成一次新的人机验证。</AlertDescription>
        </Alert>

        <div class="space-y-2">
          <label for="reset-email" class="text-sm font-medium">学校邮箱</label>
          <Input
            id="reset-email"
            v-model="email"
            type="email"
            autocomplete="email"
            class="h-11"
            :class="emailError ? 'border-destructive' : ''"
            :aria-invalid="emailError ? 'true' : undefined"
            :aria-describedby="emailError ? 'reset-email-error' : undefined"
            placeholder="student@school.edu"
            @blur="touched.email = true"
          />
          <p v-if="emailError" id="reset-email-error" class="text-xs text-destructive">{{ emailError }}</p>
        </div>

        <div id="reset-turnstile" tabindex="-1">
          <TurnstileWidget
            ref="turnstileWidget"
            :site-key="turnstileSiteKey"
            action="password_reset"
            @update:token="turnstileToken = $event; touched.turnstile = true"
          />
          <p v-if="turnstileError" class="mt-2 text-xs text-destructive">{{ turnstileError }}</p>
        </div>

        <Button class="h-11 w-full" type="submit" :disabled="pendingAction !== null">
          <Loader2 v-if="pendingAction === 'reset-start'" class="h-4 w-4 animate-spin" />
          <MailCheck v-else class="h-4 w-4" />
          {{ resendRequested ? '重新发送验证码' : '发送验证码' }}
        </Button>
      </form>

      <form v-else-if="step === 'confirm'" class="space-y-4" novalidate @submit.prevent="confirmReset">
        <Alert>
          <MailCheck class="h-4 w-4" />
          <AlertTitle>请检查邮箱</AlertTitle>
          <AlertDescription>如果该邮箱可用于密码重置，你会收到一封包含 6 位验证码的邮件。验证码 15 分钟内有效。</AlertDescription>
        </Alert>

        <Alert v-if="resendCompleted">
          <CheckCircle2 class="h-4 w-4" />
          <AlertDescription>新验证码已发送，之前的验证码已失效。</AlertDescription>
        </Alert>

        <div class="space-y-2">
          <label for="reset-code" class="text-sm font-medium">6 位验证码</label>
          <Input
            id="reset-code"
            v-model="code"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="6"
            class="h-11"
            :class="codeError ? 'border-destructive' : ''"
            :aria-invalid="codeError ? 'true' : undefined"
            :aria-describedby="codeError ? 'reset-code-error' : undefined"
            placeholder="123456"
            @blur="touched.code = true"
          />
          <p v-if="codeError" id="reset-code-error" class="text-xs text-destructive">{{ codeError }}</p>
        </div>

        <div class="space-y-2">
          <label for="reset-new-password" class="text-sm font-medium">新密码</label>
          <PasswordInput
            id="reset-new-password"
            v-model="newPassword"
            label="新密码"
            autocomplete="new-password"
            placeholder="8-32 位，包含字母、数字和特殊字符"
            :invalid="Boolean(newPasswordError)"
            :described-by="newPasswordError ? 'reset-new-password-error' : undefined"
            @blur="touched.newPassword = true"
          />
          <p v-if="newPasswordError" id="reset-new-password-error" class="text-xs text-destructive">{{ newPasswordError }}</p>
        </div>

        <div class="space-y-2">
          <label for="reset-password-confirm" class="text-sm font-medium">确认新密码</label>
          <PasswordInput
            id="reset-password-confirm"
            v-model="passwordConfirm"
            label="确认新密码"
            autocomplete="new-password"
            placeholder="再次输入新密码"
            :invalid="Boolean(passwordConfirmError)"
            :described-by="passwordConfirmError ? 'reset-password-confirm-error' : undefined"
            @blur="touched.passwordConfirm = true"
          />
          <p v-if="passwordConfirmError" id="reset-password-confirm-error" class="text-xs text-destructive">{{ passwordConfirmError }}</p>
        </div>

        <Button class="h-11 w-full" type="submit" :disabled="pendingAction !== null">
          <Loader2 v-if="pendingAction === 'reset-confirm'" class="h-4 w-4 animate-spin" />
          <CheckCircle2 v-else class="h-4 w-4" />
          重置密码
        </Button>
        <Button type="button" variant="ghost" class="w-full text-xs" :disabled="pendingAction !== null" @click="restartRequest">重新发送验证码</Button>
      </form>

      <div v-else class="py-7 text-center" aria-live="polite">
        <span class="mx-auto grid h-12 w-12 place-items-center rounded-full bg-emerald-500/10 text-emerald-600">
          <CheckCircle2 class="h-6 w-6" />
        </span>
        <h3 class="mt-4 text-lg font-semibold">密码已重置</h3>
        <p class="mt-2 text-sm leading-6 text-muted-foreground">现有登录会话已退出，请使用新密码重新登录。</p>
        <Button class="mt-5 h-11 w-full" as-child>
          <RouterLink :to="loginDestination">返回登录</RouterLink>
        </Button>
      </div>
    </div>
  </AuthPageShell>
</template>
