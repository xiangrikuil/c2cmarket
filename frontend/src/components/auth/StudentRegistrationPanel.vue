<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { CheckCircle2, Info, Loader2, MailCheck, RefreshCw, UserPlus } from 'lucide-vue-next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import PasswordInput from '@/components/auth/PasswordInput.vue'
import TurnstileWidget from '@/components/auth/TurnstileWidget.vue'
import InstitutionDirectoryDialog from '@/components/auth/InstitutionDirectoryDialog.vue'
import {
  BackendProblemError,
  backendErrorMessage,
  checkUsernameAvailability,
  confirmEmailRegistration,
  startEmailRegistration,
  type BackendSession,
  type EmailRegistrationConfig,
} from '@/lib/backendClient'
import {
  backendFieldErrors,
  focusFirstInvalidField,
  validateEmail,
  validateNewPassword,
  validatePasswordConfirmation,
  validateUsername,
  validateVerificationCode,
} from '@/lib/authFormValidation'

const props = defineProps<{
  config: EmailRegistrationConfig | null
  configLoading: boolean
  configError: string
  turnstileSiteKey: string
}>()

const emit = defineEmits<{
  (event: 'retry-config'): void
  (event: 'authenticated', session: BackendSession): void
}>()

type RegistrationStep = 'email' | 'account' | 'completed'
type PendingAction = null | 'registration-start' | 'registration-confirm'
type UsernameAvailabilityState = 'idle' | 'checking' | 'available' | 'unavailable' | 'error'

const step = ref<RegistrationStep>('email')
const pendingAction = ref<PendingAction>(null)
const email = ref('')
const code = ref('')
const username = ref('')
const password = ref('')
const passwordConfirm = ref('')
const turnstileToken = ref('')
const turnstileWidget = ref<{ reset: () => void } | null>(null)
const panelError = ref('')
const serverErrors = reactive<Record<string, string>>({})
const touched = reactive({ email: false, code: false, username: false, password: false, passwordConfirm: false, turnstile: false })
const directoryOpen = ref(false)
const developmentCode = ref('')
const requestingResend = ref(false)
const resendCompleted = ref(false)
const usernameAvailability = ref<UsernameAvailabilityState>('idle')
let requestGeneration = 0
let usernameAvailabilityGeneration = 0

const institutions = computed(() => props.config?.institutions ?? [])
const canonicalEmail = computed(() => email.value.trim().toLowerCase())
const emailDomain = computed(() => canonicalEmail.value.split('@')[1] ?? '')
const matchedInstitution = computed(() => institutions.value.find(item => item.domain === emailDomain.value) ?? null)
const emailError = computed(() => serverErrors.email || (touched.email ? validateEmail(email.value) : ''))
const codeError = computed(() => serverErrors.code || (touched.code ? validateVerificationCode(code.value) : ''))
const usernameError = computed(() => (
  serverErrors.username
  || (touched.username ? validateUsername(username.value) : '')
  || (usernameAvailability.value === 'unavailable' ? '该用户名已被占用，请换一个。' : '')
))
const usernameDescriptionId = computed(() => (
  usernameError.value
    ? 'registration-username-error'
    : usernameAvailability.value !== 'idle'
      ? 'registration-username-status'
      : undefined
))
const passwordError = computed(() => serverErrors.password || (touched.password ? validateNewPassword(password.value) : ''))
const passwordConfirmError = computed(() => touched.passwordConfirm ? validatePasswordConfirmation(password.value, passwordConfirm.value) : '')
const turnstileError = computed(() => touched.turnstile && !turnstileToken.value ? '请先完成人机验证。' : '')
const maskedEmail = computed(() => {
  const [localPart = '', domain = ''] = canonicalEmail.value.split('@')
  if (!localPart || !domain) return canonicalEmail.value
  const visible = localPart.slice(0, Math.min(2, localPart.length))
  return `${visible}${'*'.repeat(Math.max(3, localPart.length - visible.length))}@${domain}`
})

watch(email, () => {
  requestGeneration += 1
  if (pendingAction.value === 'registration-start') pendingAction.value = null
  turnstileToken.value = ''
  touched.turnstile = false
  turnstileWidget.value?.reset()
  delete serverErrors.email
  panelError.value = ''
})
watch(code, () => delete serverErrors.code)
watch(username, () => {
  usernameAvailabilityGeneration += 1
  usernameAvailability.value = 'idle'
  delete serverErrors.username
})
watch(password, () => delete serverErrors.password)

onUnmounted(() => {
  requestGeneration += 1
  usernameAvailabilityGeneration += 1
})

const clearServerErrors = () => {
  Object.keys(serverErrors).forEach(key => delete serverErrors[key])
  panelError.value = ''
}

const checkUsernameOnBlur = async () => {
  touched.username = true
  delete serverErrors.username
  if (validateUsername(username.value)) {
    usernameAvailability.value = 'idle'
    return
  }

  const checkedUsername = username.value
  const generation = ++usernameAvailabilityGeneration
  usernameAvailability.value = 'checking'
  try {
    const result = await checkUsernameAvailability(checkedUsername)
    if (generation !== usernameAvailabilityGeneration || checkedUsername !== username.value) return
    usernameAvailability.value = result.available ? 'available' : 'unavailable'
  } catch {
    if (generation !== usernameAvailabilityGeneration || checkedUsername !== username.value) return
    usernameAvailability.value = 'error'
  }
}

const requestRegistrationCode = async () => {
  if (pendingAction.value || step.value !== 'email') return
  touched.email = true
  touched.turnstile = true
  const validationErrors = {
    email: validateEmail(email.value),
    turnstile: turnstileToken.value ? '' : '请先完成人机验证。',
  }
  if (Object.values(validationErrors).some(Boolean)) {
    await focusFirstInvalidField(validationErrors, [
      ['email', 'registration-email'],
      ['turnstile', 'registration-turnstile'],
    ])
    return
  }

  pendingAction.value = 'registration-start'
  clearServerErrors()
  const submittedEmail = canonicalEmail.value
  const generation = ++requestGeneration
  try {
    const challenge = await startEmailRegistration({
      email: submittedEmail,
      turnstileToken: turnstileToken.value,
    })
    if (generation !== requestGeneration || submittedEmail !== canonicalEmail.value) return
    developmentCode.value = challenge.devCode ?? ''
    code.value = ''
    step.value = 'account'
    resendCompleted.value = requestingResend.value
    requestingResend.value = false
  } catch (error) {
    if (generation !== requestGeneration) return
    const { fields: mapped, hasUnmapped } = backendFieldErrors(error, {}, ['email'])
    Object.assign(serverErrors, mapped)
    if (hasUnmapped || !Object.keys(mapped).length) panelError.value = backendErrorMessage(error, '验证码发送失败，请稍后重试。')
    await focusFirstInvalidField(serverErrors, [['email', 'registration-email']])
  } finally {
    if (generation === requestGeneration) {
      turnstileToken.value = ''
      turnstileWidget.value?.reset()
      pendingAction.value = null
    }
  }
}

const submitRegistration = async () => {
  if (pendingAction.value || step.value !== 'account') return
  touched.code = true
  touched.username = true
  touched.password = true
  touched.passwordConfirm = true
  const validationErrors = {
    code: validateVerificationCode(code.value),
    username: validateUsername(username.value) || (usernameAvailability.value === 'unavailable' ? '该用户名已被占用，请换一个。' : ''),
    password: validateNewPassword(password.value),
    passwordConfirm: validatePasswordConfirmation(password.value, passwordConfirm.value),
  }
  if (Object.values(validationErrors).some(Boolean)) {
    await focusFirstInvalidField(validationErrors, [
      ['code', 'registration-code'],
      ['username', 'registration-username'],
      ['password', 'registration-password'],
      ['passwordConfirm', 'registration-password-confirm'],
    ])
    return
  }

  pendingAction.value = 'registration-confirm'
  clearServerErrors()
  const generation = ++requestGeneration
  try {
    const session = await confirmEmailRegistration({
      email: canonicalEmail.value,
      code: code.value.trim(),
      username: username.value,
      password: password.value,
    })
    if (generation !== requestGeneration || step.value !== 'account') return
    step.value = 'completed'
    password.value = ''
    passwordConfirm.value = ''
    emit('authenticated', session)
  } catch (error) {
    if (generation !== requestGeneration || step.value === 'completed') return
    const { fields: mapped, hasUnmapped } = backendFieldErrors(
      error,
      { newPassword: 'password' },
      ['code', 'username', 'password'],
    )
    Object.assign(serverErrors, mapped)
    if (error instanceof BackendProblemError && error.code === 'VERIFICATION_CODE_INVALID') {
      serverErrors.code = '验证码无效或已过期，请重新获取。'
    } else if (error instanceof BackendProblemError && error.code === 'USERNAME_UNAVAILABLE') {
      usernameAvailability.value = 'unavailable'
    } else if (hasUnmapped || !Object.keys(mapped).length) {
      panelError.value = backendErrorMessage(error, '注册确认失败，请稍后重试。')
    }
    await focusFirstInvalidField(serverErrors, [
      ['code', 'registration-code'],
      ['username', 'registration-username'],
      ['password', 'registration-password'],
    ])
  } finally {
    if (generation === requestGeneration && step.value !== 'completed') pendingAction.value = null
  }
}

const changeEmail = (resend = false) => {
  if (pendingAction.value || step.value === 'completed') return
  requestGeneration += 1
  step.value = 'email'
  code.value = ''
  developmentCode.value = ''
  turnstileToken.value = ''
  touched.turnstile = false
  requestingResend.value = resend
  resendCompleted.value = false
  clearServerErrors()
  turnstileWidget.value?.reset()
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="configLoading" class="grid min-h-64 place-items-center text-sm text-muted-foreground" aria-live="polite">
      <span class="inline-flex items-center gap-2"><Loader2 class="h-4 w-4 animate-spin" />读取注册配置</span>
    </div>

    <Alert v-else-if="configError" variant="destructive">
      <AlertTitle>暂时无法读取注册配置</AlertTitle>
      <AlertDescription class="mt-1">{{ configError }}</AlertDescription>
      <Button size="sm" variant="outline" class="mt-3" @click="emit('retry-config')">重试</Button>
    </Alert>

    <Alert v-else-if="!config?.enabled">
      <Info class="h-4 w-4" />
      <AlertTitle>学生注册暂未开放</AlertTitle>
      <AlertDescription>已创建的学生账号仍可在登录面板使用。</AlertDescription>
    </Alert>

    <Alert v-else-if="institutions.length === 0">
      <Info class="h-4 w-4" />
      <AlertTitle>暂无开放域名</AlertTitle>
      <AlertDescription>学生注册已开启，但当前没有可用的学校邮箱域名。</AlertDescription>
    </Alert>

    <template v-else>
      <div class="flex items-center justify-between gap-3 text-xs text-muted-foreground" aria-label="注册进度">
        <span :class="step === 'email' ? 'font-medium text-primary' : ''">1. 验证学校邮箱</span>
        <span class="h-px flex-1 bg-border" />
        <span :class="step === 'account' || step === 'completed' ? 'font-medium text-primary' : ''">2. 创建账号</span>
      </div>

      <Alert v-if="panelError" variant="destructive" aria-live="polite">
        <AlertDescription>{{ panelError }}</AlertDescription>
      </Alert>

      <form v-if="step === 'email'" class="space-y-4" novalidate @submit.prevent="requestRegistrationCode">
        <div>
          <h3 class="text-base font-semibold">验证学校邮箱</h3>
          <p class="mt-1 text-xs leading-5 text-muted-foreground">仅支持当前已经开放的学校邮箱。</p>
        </div>

        <Alert v-if="requestingResend">
          <RefreshCw class="h-4 w-4" />
          <AlertDescription>重新发送需要先完成一次新的人机验证。</AlertDescription>
        </Alert>

        <div class="space-y-2">
          <label for="registration-email" class="text-sm font-medium">学校邮箱</label>
          <Input
            id="registration-email"
            v-model="email"
            type="email"
            autocomplete="email"
            class="h-11"
            :class="emailError ? 'border-destructive' : ''"
            :aria-invalid="emailError ? 'true' : undefined"
            :aria-describedby="emailError ? 'registration-email-error' : undefined"
            placeholder="student@school.edu"
            @blur="touched.email = true"
          />
          <p v-if="emailError" id="registration-email-error" class="text-xs text-destructive">{{ emailError }}</p>
          <p v-else-if="matchedInstitution" class="flex items-start gap-1.5 text-xs leading-5 text-emerald-700 dark:text-emerald-400">
            <CheckCircle2 class="mt-0.5 h-3.5 w-3.5 shrink-0" />
            已支持：{{ matchedInstitution.institutionName }} · @{{ matchedInstitution.domain }}
          </p>
          <p v-else-if="emailDomain" class="text-xs leading-5 text-muted-foreground">该域名不在当前开放列表中。</p>
        </div>

        <div id="registration-turnstile" tabindex="-1">
          <TurnstileWidget
            ref="turnstileWidget"
            :site-key="turnstileSiteKey"
            action="student_signup"
            @update:token="turnstileToken = $event; touched.turnstile = true"
          />
          <p v-if="turnstileError" class="mt-2 text-xs text-destructive">{{ turnstileError }}</p>
        </div>

        <Button class="h-11 w-full" type="submit" :disabled="pendingAction !== null">
          <Loader2 v-if="pendingAction === 'registration-start'" class="h-4 w-4 animate-spin" />
          <MailCheck v-else class="h-4 w-4" />
          {{ requestingResend ? '重新发送验证码' : '发送验证码' }}
        </Button>

        <Button type="button" variant="ghost" class="h-auto w-full whitespace-normal py-2 text-xs text-muted-foreground" @click="directoryOpen = true">
          已开放 {{ institutions.length }} 个学校邮箱域名 · 查看全部
        </Button>
      </form>

      <form v-else-if="step === 'account'" class="space-y-4" novalidate @submit.prevent="submitRegistration">
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0">
            <h3 class="text-base font-semibold">创建学生账号</h3>
            <p class="mt-1 break-all text-xs text-muted-foreground">验证码已发送至 {{ maskedEmail }}，15 分钟内有效。</p>
          </div>
          <Button type="button" size="sm" variant="ghost" class="shrink-0" :disabled="pendingAction !== null" @click="changeEmail(false)">更换邮箱</Button>
        </div>

        <Alert v-if="resendCompleted">
          <CheckCircle2 class="h-4 w-4" />
          <AlertDescription>新验证码已发送，之前的验证码已失效。</AlertDescription>
        </Alert>

        <div class="space-y-2">
          <label for="registration-code" class="text-sm font-medium">6 位验证码</label>
          <Input
            id="registration-code"
            v-model="code"
            inputmode="numeric"
            autocomplete="one-time-code"
            maxlength="6"
            class="h-11"
            :class="codeError ? 'border-destructive' : ''"
            :aria-invalid="codeError ? 'true' : undefined"
            :aria-describedby="codeError ? 'registration-code-error' : undefined"
            placeholder="123456"
            @blur="touched.code = true"
          />
          <p v-if="codeError" id="registration-code-error" class="text-xs text-destructive">{{ codeError }}</p>
          <p v-if="developmentCode" class="text-xs text-amber-700 dark:text-amber-300">开发验证码：{{ developmentCode }}</p>
        </div>

        <div class="space-y-2">
          <label for="registration-username" class="text-sm font-medium">用户名</label>
          <Input
            id="registration-username"
            v-model="username"
            autocomplete="username"
            class="h-11"
            :class="usernameError ? 'border-destructive' : usernameAvailability === 'available' ? 'border-emerald-600' : ''"
            :aria-invalid="usernameError ? 'true' : undefined"
            :aria-describedby="usernameDescriptionId"
            placeholder="3-24 位小写字母、数字、_ 或 -"
            @blur="checkUsernameOnBlur"
          />
          <p v-if="usernameError" id="registration-username-error" class="text-xs text-destructive">{{ usernameError }}</p>
          <p v-else-if="usernameAvailability === 'checking'" id="registration-username-status" class="flex items-center gap-1.5 text-xs text-muted-foreground" aria-live="polite">
            <Loader2 class="h-3.5 w-3.5 animate-spin" />正在检查用户名
          </p>
          <p v-else-if="usernameAvailability === 'available'" id="registration-username-status" class="flex items-center gap-1.5 text-xs text-emerald-700 dark:text-emerald-400" aria-live="polite">
            <CheckCircle2 class="h-3.5 w-3.5" />用户名可用
          </p>
          <p v-else-if="usernameAvailability === 'error'" id="registration-username-status" class="text-xs text-amber-700 dark:text-amber-300" aria-live="polite">
            暂时无法检查用户名，提交时会再次确认。
          </p>
        </div>

        <div class="space-y-2">
          <label for="registration-password" class="text-sm font-medium">密码</label>
          <PasswordInput
            id="registration-password"
            v-model="password"
            label="密码"
            autocomplete="new-password"
            placeholder="8-32 位，包含字母、数字和特殊字符"
            :invalid="Boolean(passwordError)"
            :described-by="passwordError ? 'registration-password-error' : undefined"
            @blur="touched.password = true"
          />
          <p v-if="passwordError" id="registration-password-error" class="text-xs text-destructive">{{ passwordError }}</p>
        </div>

        <div class="space-y-2">
          <label for="registration-password-confirm" class="text-sm font-medium">确认密码</label>
          <PasswordInput
            id="registration-password-confirm"
            v-model="passwordConfirm"
            label="确认密码"
            autocomplete="new-password"
            placeholder="再次输入密码"
            :invalid="Boolean(passwordConfirmError)"
            :described-by="passwordConfirmError ? 'registration-password-confirm-error' : undefined"
            @blur="touched.passwordConfirm = true"
          />
          <p v-if="passwordConfirmError" id="registration-password-confirm-error" class="text-xs text-destructive">{{ passwordConfirmError }}</p>
        </div>

        <Button class="h-11 w-full" type="submit" :disabled="pendingAction !== null">
          <Loader2 v-if="pendingAction === 'registration-confirm'" class="h-4 w-4 animate-spin" />
          <UserPlus v-else class="h-4 w-4" />
          创建账号并登录
        </Button>
        <Button type="button" variant="ghost" class="w-full text-xs" :disabled="pendingAction !== null" @click="changeEmail(true)">
          重新发送验证码
        </Button>
      </form>

      <div v-else class="grid min-h-56 place-items-center text-center" aria-live="polite">
        <div>
          <CheckCircle2 class="mx-auto h-8 w-8 text-emerald-600" />
          <p class="mt-3 font-medium">学生账号已创建</p>
          <p class="mt-1 text-sm text-muted-foreground">正在进入 C2CMarket...</p>
        </div>
      </div>
    </template>

    <InstitutionDirectoryDialog v-model:open="directoryOpen" :institutions="institutions" />
  </div>
</template>
