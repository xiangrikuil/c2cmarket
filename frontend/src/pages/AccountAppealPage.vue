<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  CheckCircle2,
  ExternalLink,
  Loader2,
  RotateCcw,
  ShieldAlert,
} from 'lucide-vue-next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import {
  BackendProblemError,
  backendErrorMessage,
  getAccountAppealSession,
  startAccountAppealVerification,
  submitAccountGovernanceAppeal,
  type AccountAppealSession,
  type AccountGovernanceAppeal,
} from '@/lib/backendClient'

type PageState = 'loading' | 'verification' | 'verified' | 'submitted' | 'error'
type CallbackOutcome = 'verified' | 'ineligible' | null

const route = useRoute()
const router = useRouter()
const pageState = ref<PageState>('loading')
const session = ref<AccountAppealSession | null>(null)
const submittedAppeal = ref<AccountGovernanceAppeal | null>(null)
const statement = ref('')
const verifyLoading = ref(false)
const submitLoading = ref(false)
const errorMessage = ref('')
const formError = ref('')

const accountStatusLabel = computed(() => session.value?.accountStatus === 'banned' ? '账号已封禁' : '账号已暂停')
const expiresAtLabel = computed(() => formatDateTime(session.value?.expiresAt))
const submittedAtLabel = computed(() => formatDateTime(submittedAppeal.value?.createdAt))

onMounted(async () => {
  const outcome = readCallbackOutcome(route.query.accountAppealOutcome)
  if ('accountAppealOutcome' in route.query) {
    const query = { ...route.query }
    delete query.accountAppealOutcome
    await router.replace({ path: route.path, query, hash: route.hash })
  }
  await loadAppealSession(outcome)
})

function readCallbackOutcome(value: unknown): CallbackOutcome {
  const scalar = Array.isArray(value) ? value[0] : value
  return scalar === 'verified' || scalar === 'ineligible' ? scalar : null
}

function formatDateTime(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(date)
}

async function loadAppealSession(outcome: CallbackOutcome = null) {
  pageState.value = 'loading'
  errorMessage.value = ''
  try {
    session.value = await getAccountAppealSession()
    pageState.value = 'verified'
  } catch (error) {
    session.value = null
    if (outcome === 'ineligible') {
      errorMessage.value = '当前 linux.do 身份不符合受限账号申诉条件，或未绑定到可申诉账号。'
      pageState.value = 'error'
      return
    }
    if (error instanceof BackendProblemError && error.status === 401) {
      pageState.value = 'verification'
      return
    }
    errorMessage.value = backendErrorMessage(error, '账号申诉会话暂时无法读取，请稍后重试。')
    pageState.value = 'error'
  }
}

async function startVerification() {
  verifyLoading.value = true
  errorMessage.value = ''
  try {
    const { authorizationUrl } = await startAccountAppealVerification()
    window.location.assign(authorizationUrl)
  } catch (error) {
    errorMessage.value = backendErrorMessage(error, 'linux.do 验证暂时无法启动，请稍后重试。')
    pageState.value = 'error'
  } finally {
    verifyLoading.value = false
  }
}

async function submitAppeal() {
  const value = statement.value.trim()
  formError.value = ''
  if (value.length < 4) {
    formError.value = '请至少填写 4 个字符。'
    return
  }

  submitLoading.value = true
  try {
    submittedAppeal.value = await submitAccountGovernanceAppeal(value)
    statement.value = ''
    pageState.value = 'submitted'
  } catch (error) {
    if (error instanceof BackendProblemError && error.status === 401) {
      session.value = null
      errorMessage.value = '验证会话已失效，请重新通过 linux.do 验证。'
      pageState.value = 'verification'
      return
    }
    formError.value = backendErrorMessage(error, '申诉提交失败，请检查内容后重试。')
  } finally {
    submitLoading.value = false
  }
}
</script>

<template>
  <main class="min-h-screen bg-slate-50 px-4 py-8 sm:px-6 sm:py-12">
    <div class="mx-auto w-full max-w-xl">
      <RouterLink class="mb-5 inline-flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground" to="/login">
        <ArrowLeft class="h-4 w-4" />返回登录
      </RouterLink>

      <div class="mb-6 flex items-center gap-3">
        <div class="grid h-11 w-11 shrink-0 place-items-center rounded-lg bg-primary text-primary-foreground">
          <ShieldAlert class="h-5 w-5" />
        </div>
        <div>
          <h1 class="text-2xl font-semibold text-foreground">账号申诉</h1>
          <p class="mt-1 text-sm text-muted-foreground">C2CMarket 受限账号复核</p>
        </div>
      </div>

      <Card class="rounded-lg border-border bg-card p-5 shadow-sm sm:p-6">
        <div v-if="pageState === 'loading'" class="grid min-h-64 place-items-center" aria-live="polite">
          <span class="inline-flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 class="h-4 w-4 animate-spin" />读取验证状态
          </span>
        </div>

        <section v-else-if="pageState === 'verification'" class="space-y-5">
          <div>
            <h2 class="text-lg font-semibold text-foreground">验证受限账号</h2>
            <p class="mt-2 text-sm leading-6 text-muted-foreground">使用该账号绑定的 linux.do 身份验证后，可提交一次账号治理申诉。</p>
          </div>
          <Alert v-if="errorMessage" variant="destructive">
            <AlertTitle>验证会话已失效</AlertTitle>
            <AlertDescription>{{ errorMessage }}</AlertDescription>
          </Alert>
          <Button class="w-full" :disabled="verifyLoading" @click="startVerification">
            <Loader2 v-if="verifyLoading" class="h-4 w-4 animate-spin" />
            <ExternalLink v-else class="h-4 w-4" />
            {{ verifyLoading ? '正在跳转' : '使用 linux.do 验证' }}
          </Button>
        </section>

        <form v-else-if="pageState === 'verified' && session" class="space-y-5" @submit.prevent="submitAppeal">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 class="text-lg font-semibold text-foreground">提交复核说明</h2>
              <p class="mt-1 text-sm text-muted-foreground">验证有效至 {{ expiresAtLabel }}</p>
            </div>
            <Badge variant="destructive">{{ accountStatusLabel }}</Badge>
          </div>

          <div class="space-y-2">
            <label for="account-appeal-statement" class="text-sm font-medium text-foreground">申诉说明</label>
            <Textarea
              id="account-appeal-statement"
              v-model="statement"
              class="min-h-40 resize-y"
              maxlength="1000"
              placeholder="说明需要复核的事实和理由。"
              aria-describedby="account-appeal-boundary"
            />
            <div class="flex items-start justify-between gap-4 text-xs">
              <p id="account-appeal-boundary" class="leading-5 text-muted-foreground">不要提交联系方式、密码、API Key、token、session、cookie 或恢复码。</p>
              <span class="shrink-0 text-muted-foreground">{{ statement.length }}/1000</span>
            </div>
          </div>

          <Alert v-if="formError" variant="destructive" aria-live="polite">
            <AlertTitle>无法提交申诉</AlertTitle>
            <AlertDescription>{{ formError }}</AlertDescription>
          </Alert>

          <Button class="w-full" type="submit" :disabled="submitLoading">
            <Loader2 v-if="submitLoading" class="h-4 w-4 animate-spin" />
            <ShieldAlert v-else class="h-4 w-4" />
            {{ submitLoading ? '提交中' : '提交申诉' }}
          </Button>
        </form>

        <section v-else-if="pageState === 'submitted' && submittedAppeal" class="space-y-5 text-center" aria-live="polite">
          <CheckCircle2 class="mx-auto h-11 w-11 text-emerald-600" />
          <div>
            <h2 class="text-lg font-semibold text-foreground">申诉已提交</h2>
            <p class="mt-2 text-sm leading-6 text-muted-foreground">管理员会独立审核申诉；审核通过不会自动恢复账号状态。</p>
          </div>
          <dl class="grid gap-3 border-y border-border py-4 text-left text-sm sm:grid-cols-2">
            <div><dt class="text-xs text-muted-foreground">申诉编号</dt><dd class="mt-1 break-all font-medium">{{ submittedAppeal.id }}</dd></div>
            <div><dt class="text-xs text-muted-foreground">提交时间</dt><dd class="mt-1 font-medium">{{ submittedAtLabel }}</dd></div>
          </dl>
          <RouterLink to="/login"><Button variant="outline" class="w-full"><ArrowLeft class="h-4 w-4" />返回登录</Button></RouterLink>
        </section>

        <section v-else class="space-y-5">
          <Alert variant="destructive" aria-live="polite">
            <AlertTitle>无法继续账号申诉</AlertTitle>
            <AlertDescription>{{ errorMessage || '账号申诉状态暂时无法确认。' }}</AlertDescription>
          </Alert>
          <div class="grid gap-2 sm:grid-cols-2">
            <Button variant="outline" @click="loadAppealSession()"><RotateCcw class="h-4 w-4" />重试</Button>
            <Button :disabled="verifyLoading" @click="startVerification">
              <Loader2 v-if="verifyLoading" class="h-4 w-4 animate-spin" />
              <ExternalLink v-else class="h-4 w-4" />重新验证
            </Button>
          </div>
        </section>
      </Card>
    </div>
  </main>
</template>
