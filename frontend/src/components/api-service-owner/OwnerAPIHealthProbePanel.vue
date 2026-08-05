<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import Activity from 'lucide-vue-next/dist/esm/icons/activity.js'
import TriangleAlert from 'lucide-vue-next/dist/esm/icons/triangle-alert.js'
import CheckCircle2 from 'lucide-vue-next/dist/esm/icons/circle-check-big.js'
import Clipboard from 'lucide-vue-next/dist/esm/icons/clipboard.js'
import KeyRound from 'lucide-vue-next/dist/esm/icons/key-round.js'
import RefreshCcw from 'lucide-vue-next/dist/esm/icons/refresh-ccw.js'
import Save from 'lucide-vue-next/dist/esm/icons/save.js'
import ShieldCheck from 'lucide-vue-next/dist/esm/icons/shield-check.js'
import Trash2 from 'lucide-vue-next/dist/esm/icons/trash-2.js'
import { toast } from 'vue-sonner'
import ErrorState from '@/components/market/ErrorState.vue'
import SkeletonBlock from '@/components/market/SkeletonBlock.vue'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useOwnerAPIHealthProbeForm } from '@/composables/useOwnerAPIHealthProbeForm'
import { backendErrorMessage } from '@/lib/backendClient'
import {
  useCreateAPIHealthChallengeMutation,
  useDeleteOwnerAPIHealthProbeMutation,
  useOwnerAPIHealthProbe,
  useSaveOwnerAPIHealthProbeMutation,
  useVerifyAPIHealthChallengeMutation,
} from '@/queries/useApiHealthQueries'
import type { ApiHealthAuthorizationMethod, ApiHealthAuthorizationStatus, ApiHealthSafeErrorCode, APIHealthProbeChallenge } from '@/types/apiHealth'

const props = defineProps<{ apiServiceId: string }>()

const probeQuery = useOwnerAPIHealthProbe(computed(() => props.apiServiceId))
const probe = computed(() => probeQuery.data.value)
const form = useOwnerAPIHealthProbeForm(probe)
const saveMutation = useSaveOwnerAPIHealthProbeMutation()
const deleteMutation = useDeleteOwnerAPIHealthProbeMutation()
const challengeMutation = useCreateAPIHealthChallengeMutation()
const verifyMutation = useVerifyAPIHealthChallengeMutation()
const challengeMethod = ref<Exclude<ApiHealthAuthorizationMethod, 'admin_approval'>>('dns_txt')
const challenge = ref<APIHealthProbeChallenge | null>(null)
const deleteDialogOpen = ref(false)

const statusLabels: Record<ApiHealthAuthorizationStatus, string> = {
  pending: '待授权',
  verified: '自动验证通过',
  approved: '管理员已批准',
  rejected: '已拒绝',
}

const methodLabels: Record<ApiHealthAuthorizationMethod, string> = {
  dns_txt: 'DNS TXT',
  http_challenge: 'HTTP Challenge',
  admin_approval: '管理员审批',
}

const errorLabels: Record<ApiHealthSafeErrorCode, string> = {
  blocked_target: '目标地址未通过平台网络校验',
  authorization_invalid: '目标授权已失效',
  dns_failed: '目标域名解析失败',
  connect_failed: '无法连接目标服务',
  tls_failed: '目标 TLS 连接失败',
  timeout: '目标响应超时',
  http_4xx: '目标返回 4xx 响应',
  http_5xx: '目标返回 5xx 响应',
  response_too_large: '目标响应超过平台读取上限',
  invalid_stream: '目标未返回兼容的流式响应',
  empty_response: '目标流式响应没有有效内容',
  decrypt_failed: '平台暂时无法读取探针专用 API Key',
  internal: '探针系统内部错误',
  internal_timeout: '探针任务超过平台硬超时',
  challenge_mismatch: '目标返回的验证内容不匹配',
  challenge_expired: '一次性验证信息已过期',
  dns_resolution_failed: '无法读取目标域名的 TXT 记录',
  invalid_origin: '当前规范化 Origin 无效',
  target_blocked: '当前 Origin 未通过平台目标校验',
  http_request_failed: '无法访问 HTTP 验证地址',
  http_status: 'HTTP 验证地址返回非成功状态',
  http_response_invalid: 'HTTP 验证内容超过限制或无法读取',
}

const busy = computed(() => saveMutation.isPending.value
  || deleteMutation.isPending.value
  || challengeMutation.isPending.value
  || verifyMutation.isPending.value)
const statusVariant = computed(() => probe.value?.authorizationStatus === 'verified' || probe.value?.authorizationStatus === 'approved'
  ? 'verified'
  : probe.value?.authorizationStatus === 'rejected'
    ? 'destructive'
    : 'secondary')
const runnerStatus = computed(() => {
  if (!probe.value) return '尚未配置'
  if (!probe.value.enabled) return '已停用，不会发起探测'
  if (probe.value.authorizationStatus === 'verified' || probe.value.authorizationStatus === 'approved') return '已进入平台探针调度'
  return '等待目标授权，不会发起探测'
})
const errorMessage = computed(() => backendErrorMessage(probeQuery.error.value, '探针配置暂时无法读取。'))
const currentVersion = computed(() => challenge.value?.configVersion ?? probe.value?.version ?? 0)

function save() {
  form.markTouched()
  if (!form.valid.value || busy.value) return
  saveMutation.mutate(form.payload(props.apiServiceId), {
    onSuccess() {
      form.clearCredential()
      challenge.value = null
      toast.success('探针配置已保存。')
    },
    onError(error) {
      toast.error(backendErrorMessage(error, '探针配置保存失败。'))
    },
  })
}

function createChallenge() {
  if (!probe.value || busy.value) return
  challenge.value = null
  challengeMutation.mutate({
    apiServiceId: props.apiServiceId,
    version: probe.value.version,
    method: challengeMethod.value,
  }, {
    onSuccess(result) {
      challenge.value = result
      toast.success('一次性验证信息已生成。')
    },
    onError(error) {
      toast.error(backendErrorMessage(error, '无法生成验证信息。'))
    },
  })
}

function verifyChallenge() {
  if (!probe.value || busy.value) return
  verifyMutation.mutate({ apiServiceId: props.apiServiceId, version: currentVersion.value }, {
    onSuccess(result) {
      challenge.value = null
      toast.success(result.authorizationStatus === 'verified' ? '目标控制权验证通过。' : '验证结果已更新。')
    },
    onError(error) {
      toast.error(backendErrorMessage(error, '目标控制权验证失败。'))
    },
  })
}

function requestDeleteConfig() {
  if (!probe.value || busy.value) return
  deleteDialogOpen.value = true
}

function deleteConfig() {
  if (!probe.value || busy.value) return
  deleteMutation.mutate({ apiServiceId: props.apiServiceId, version: probe.value.version }, {
    onSuccess() {
      form.clearCredential()
      challenge.value = null
      deleteDialogOpen.value = false
      toast.success('探针配置已删除。')
    },
    onError(error) {
      toast.error(backendErrorMessage(error, '探针配置删除失败。'))
    },
  })
}

async function copyChallenge(value: string) {
  await navigator.clipboard.writeText(value)
  toast.success('验证内容已复制。')
}

onBeforeUnmount(() => {
  form.clearCredential()
  challenge.value = null
})
</script>

<template>
  <Card id="health-probe" class="scroll-mt-20 overflow-hidden">
    <CardHeader class="border-b border-border bg-muted/20">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <CardTitle class="flex items-center gap-2 text-lg"><Activity class="h-5 w-5 text-primary" />平台健康探针</CardTitle>
          <CardDescription class="mt-1">配置专用低额度 API Key，由平台按五分钟槽测量当前模型链路。</CardDescription>
        </div>
        <Badge v-if="probe" :variant="statusVariant">{{ statusLabels[probe.authorizationStatus] }}</Badge>
        <Badge v-else variant="outline">尚未配置</Badge>
      </div>
    </CardHeader>

    <CardContent class="space-y-5 p-5">
      <SkeletonBlock v-if="probeQuery.isLoading.value" :lines="5" />
      <ErrorState v-else-if="probeQuery.error.value" title="无法读取探针配置" :description="errorMessage" @retry="probeQuery.refetch()" />

      <template v-else>
        <div class="grid gap-4 lg:grid-cols-2">
          <div class="space-y-2">
            <Label for="api-health-base-url">API 请求地址（Base URL）</Label>
            <Input
              id="api-health-base-url"
              v-model="form.baseUrl.value"
              type="url"
              inputmode="url"
              placeholder="https://api.example.com/v1"
              :aria-invalid="form.touched.value && Boolean(form.validation.value.baseUrl)"
              @input="form.markTouched"
            />
            <p v-if="form.touched.value && form.validation.value.baseUrl" class="text-xs text-destructive">{{ form.validation.value.baseUrl }}</p>
            <p class="text-xs text-muted-foreground">仅填写域名时自动补 /v1；已有路径保持不变。</p>
          </div>

          <div class="space-y-2">
            <Label for="api-health-model">探测模型</Label>
            <Input
              id="api-health-model"
              v-model="form.model.value"
              placeholder="gpt-5-mini"
              :aria-invalid="form.touched.value && Boolean(form.validation.value.model)"
              @input="form.markTouched"
            />
            <p v-if="form.touched.value && form.validation.value.model" class="text-xs text-destructive">{{ form.validation.value.model }}</p>
          </div>

          <div class="space-y-2 lg:col-span-2">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <Label for="api-health-credential">探针专用 API Key</Label>
              <span class="text-xs text-muted-foreground">{{ probe?.credentialConfigured ? '已配置，留空则保留' : '尚未配置' }}</span>
            </div>
            <Input
              id="api-health-credential"
              v-model="form.credential.value"
              type="password"
              autocomplete="new-password"
              placeholder="输入新的探针专用 API Key"
              :aria-invalid="form.touched.value && Boolean(form.validation.value.credential)"
              @input="form.markTouched"
            />
            <p v-if="form.touched.value && form.validation.value.credential" class="text-xs text-destructive">{{ form.validation.value.credential }}</p>
            <p class="text-xs text-muted-foreground">请使用专用、低额度、仅开放探测模型的 API Key，请勿填写主账号高权限 Key。</p>
          </div>

          <Alert v-if="form.isInsecureHttp.value" class="border-amber-300 bg-amber-50 text-amber-950 lg:col-span-2">
            <TriangleAlert class="h-4 w-4 text-warning" />
            <AlertTitle>HTTP 请求不会加密传输</AlertTitle>
            <AlertDescription class="space-y-3">
              <p>探针 API Key 和请求响应可能被链路中的第三方读取或篡改。请仅使用专用、低额度、低权限且仅允许探测模型的 API Key。</p>
              <label for="api-health-insecure-http-ack" class="flex cursor-pointer items-start gap-2 rounded border border-amber-300/70 bg-white/70 p-3">
                <Checkbox
                  id="api-health-insecure-http-ack"
                  :model-value="form.acknowledgeInsecureHttp.value"
                  class="mt-0.5"
                  @update:model-value="form.acknowledgeInsecureHttp.value = Boolean($event); form.markTouched()"
                />
                <span class="text-xs leading-5">我确认该 Key 不具备主账号权限，额度损失风险可接受，并了解 HTTP 探测结果的可信度低于 HTTPS。</span>
              </label>
              <p v-if="form.touched.value && form.validation.value.acknowledgeInsecureHttp" class="text-xs font-medium text-destructive">
                {{ form.validation.value.acknowledgeInsecureHttp }}
              </p>
            </AlertDescription>
          </Alert>
        </div>

        <div class="flex flex-col gap-3 rounded-md border border-border bg-muted/20 p-4 sm:flex-row sm:items-center sm:justify-between">
          <label for="api-health-enabled" class="flex min-w-0 cursor-pointer items-start gap-3">
            <Checkbox id="api-health-enabled" :model-value="form.enabled.value" class="mt-0.5" @update:model-value="form.enabled.value = Boolean($event); form.markTouched()" />
            <span class="min-w-0">
              <span class="block text-sm font-medium">启用平台主动探测</span>
              <span class="mt-1 block text-xs text-muted-foreground">{{ runnerStatus }}</span>
            </span>
          </label>
          <Button :disabled="busy" @click="save"><Save class="h-4 w-4" />保存配置</Button>
        </div>

        <dl v-if="probe" class="grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
          <div class="min-w-0">
            <dt class="text-xs text-muted-foreground">规范化 Origin</dt>
            <dd class="mt-1 break-all font-mono text-xs" :title="probe.normalizedOrigin">{{ probe.normalizedOrigin }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">授权方式</dt>
            <dd class="mt-1 font-medium">{{ probe.authorizationMethod ? methodLabels[probe.authorizationMethod] : '尚未选择' }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">授权状态</dt>
            <dd class="mt-1 font-medium">{{ statusLabels[probe.authorizationStatus] }}</dd>
          </div>
          <div>
            <dt class="text-xs text-muted-foreground">最近安全错误</dt>
            <dd class="mt-1 font-medium" :title="probe.lastConfigErrorCode ?? undefined">
              {{ probe.lastConfigErrorCode ? errorLabels[probe.lastConfigErrorCode] : '无' }}
            </dd>
          </div>
        </dl>

        <Alert v-if="probe?.rejectionReason" variant="destructive">
          <ShieldCheck class="h-4 w-4" />
          <AlertTitle>管理员未批准当前 Origin</AlertTitle>
          <AlertDescription>{{ probe.rejectionReason }}</AlertDescription>
        </Alert>

        <div v-if="probe" class="space-y-4 border-t border-border pt-5">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div class="w-full max-w-sm space-y-2">
              <Label for="api-health-challenge-method">控制权验证方式</Label>
              <Select v-model="challengeMethod">
                <SelectTrigger id="api-health-challenge-method"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="dns_txt">DNS TXT</SelectItem>
                  <SelectItem value="http_challenge">HTTP Challenge</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="flex flex-wrap gap-2">
              <Button variant="outline" :disabled="busy" @click="createChallenge"><KeyRound class="h-4 w-4" />生成验证信息</Button>
              <Button variant="outline" :disabled="busy || (!challenge && !probe.challengeExpiresAt)" @click="verifyChallenge"><RefreshCcw class="h-4 w-4" />立即验证</Button>
            </div>
          </div>

          <Alert v-if="challenge">
            <CheckCircle2 class="h-4 w-4" />
            <AlertTitle>一次性验证信息</AlertTitle>
            <AlertDescription class="space-y-3">
              <div v-if="challenge.dnsRecordName" class="grid gap-1">
                <span>TXT 记录名</span>
                <code class="break-all rounded bg-muted px-2 py-1 text-xs">{{ challenge.dnsRecordName }}</code>
              </div>
              <div v-if="challenge.httpUrl" class="grid gap-1">
                <span>验证地址</span>
                <code class="break-all rounded bg-muted px-2 py-1 text-xs">{{ challenge.httpUrl }}</code>
              </div>
              <div class="grid gap-1">
                <span>验证内容</span>
                <div class="flex min-w-0 items-center gap-2">
                  <code class="min-w-0 flex-1 break-all rounded bg-muted px-2 py-1 text-xs">{{ challenge.token }}</code>
                  <Button size="icon" variant="outline" title="复制验证内容" aria-label="复制验证内容" @click="copyChallenge(challenge.token)"><Clipboard class="h-4 w-4" /></Button>
                </div>
              </div>
            </AlertDescription>
          </Alert>

          <div class="flex justify-end">
            <Button variant="destructive" :disabled="busy" @click="requestDeleteConfig"><Trash2 class="h-4 w-4" />删除探针配置</Button>
          </div>
        </div>
      </template>
    </CardContent>
  </Card>

  <Dialog v-model:open="deleteDialogOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>删除探针配置</DialogTitle>
        <DialogDescription>平台将停止探测，并删除这项服务已经保留的探针样本。此操作无法撤销。</DialogDescription>
      </DialogHeader>
      <DialogFooter>
        <Button variant="outline" :disabled="deleteMutation.isPending.value" @click="deleteDialogOpen = false">取消</Button>
        <Button variant="destructive" :disabled="deleteMutation.isPending.value" @click="deleteConfig"><Trash2 class="h-4 w-4" />确认删除</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
