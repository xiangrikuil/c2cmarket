<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { GraduationCap, Pencil, Plus, RefreshCw, ShieldCheck } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import PageTitle from '@/components/market/PageTitle.vue'
import EmptyState from '@/components/market/EmptyState.vue'
import ErrorState from '@/components/market/ErrorState.vue'
import SkeletonTable from '@/components/market/SkeletonTable.vue'
import SoftTable from '@/components/market/SoftTable.vue'
import LocalTime from '@/components/market/LocalTime.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { backendErrorMessage } from '@/lib/backendClient'
import type { AdminStudentInstitutionDomain } from '@/lib/studentRegistrationAdminBackend'
import {
  useAdminStudentInstitutionDomains,
  useAdminStudentRegistrationSetting,
  useCreateAdminStudentInstitutionDomain,
  useUpdateAdminStudentInstitutionDomain,
  useUpdateAdminStudentRegistrationSetting,
} from '@/queries/useStudentRegistrationAdminQueries'

const settingQuery = useAdminStudentRegistrationSetting()
const domainsQuery = useAdminStudentInstitutionDomains()
const updateSettingMutation = useUpdateAdminStudentRegistrationSetting()
const createDomainMutation = useCreateAdminStudentInstitutionDomain()
const updateDomainMutation = useUpdateAdminStudentInstitutionDomain()

const settingDialogOpen = ref(false)
const nextSettingEnabled = ref(false)
const settingReason = ref('')
const domainDialogOpen = ref(false)
const editingDomain = ref<AdminStudentInstitutionDomain | null>(null)
const domainForm = reactive({ domain: '', institutionName: '', enabled: true, reason: '' })

const setting = settingQuery.data
const domains = computed(() => domainsQuery.data.value ?? [])
const enabledDomainCount = computed(() => domains.value.filter(item => item.enabled).length)
const domainMutationBusy = computed(() => createDomainMutation.isPending.value || updateDomainMutation.isPending.value)
const exactDomainPattern = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/

function openSettingDialog(enabled: boolean) {
  if (!setting.value || enabled === setting.value.enabled) return
  nextSettingEnabled.value = enabled
  settingReason.value = ''
  settingDialogOpen.value = true
}

async function saveSetting() {
  if (!setting.value || !settingReason.value.trim()) {
    toast.warning('请填写本次调整原因。')
    return
  }
  try {
    await updateSettingMutation.mutateAsync({
      enabled: nextSettingEnabled.value,
      expectedVersion: setting.value.version,
      reason: settingReason.value.trim(),
    })
    settingDialogOpen.value = false
    toast.success(nextSettingEnabled.value ? '学生邮箱注册已开放。' : '学生邮箱注册已关闭。')
  } catch (error) {
    await settingQuery.refetch()
    toast.error(backendErrorMessage(error, '更新注册开关失败，请刷新后重试。'))
  }
}

function openCreateDomain() {
  editingDomain.value = null
  Object.assign(domainForm, { domain: '', institutionName: '', enabled: true, reason: '' })
  domainDialogOpen.value = true
}

function openEditDomain(item: AdminStudentInstitutionDomain, enabled = item.enabled) {
  editingDomain.value = item
  Object.assign(domainForm, {
    domain: item.domain,
    institutionName: item.institutionName,
    enabled,
    reason: '',
  })
  domainDialogOpen.value = true
}

async function saveDomain() {
  const institutionName = domainForm.institutionName.trim()
  const reason = domainForm.reason.trim()
  const domain = domainForm.domain.trim().toLowerCase()
  if (!institutionName) {
    toast.warning('请填写院校名称。')
    return
  }
  if (!editingDomain.value && !exactDomainPattern.test(domain)) {
    toast.warning('请输入不含通配符的精确邮箱域名。')
    return
  }
  if (!reason) {
    toast.warning('请填写本次操作原因。')
    return
  }

  try {
    if (editingDomain.value) {
      await updateDomainMutation.mutateAsync({
        id: editingDomain.value.id,
        institutionName,
        enabled: domainForm.enabled,
        expectedVersion: editingDomain.value.version,
        reason,
      })
    } else {
      await createDomainMutation.mutateAsync({ domain, institutionName, enabled: domainForm.enabled, reason })
    }
    domainDialogOpen.value = false
    toast.success(editingDomain.value ? '院校域名已更新。' : '院校域名已新增。')
  } catch (error) {
    await domainsQuery.refetch()
    toast.error(backendErrorMessage(error, '保存院校域名失败，请刷新后重试。'))
  }
}

async function refreshAll() {
  await Promise.all([settingQuery.refetch(), domainsQuery.refetch()])
}
</script>

<template>
  <div class="space-y-5">
    <PageTitle title="学生注册管理" description="控制学校邮箱注册，并维护精确匹配的院校邮箱域名。" />

    <Card class="p-5">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex items-start gap-3">
          <span class="grid h-10 w-10 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary"><ShieldCheck class="h-5 w-5" /></span>
          <div>
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="font-semibold">学生邮箱注册总开关</h2>
              <Badge v-if="setting" :variant="setting.enabled ? 'verified' : 'secondary'">{{ setting.enabled ? '已开放' : '已关闭' }}</Badge>
            </div>
            <p class="mt-1 text-sm leading-6 text-muted-foreground">关闭后停止发送注册验证码和确认新注册，不影响已有账号、订单与售后。</p>
            <p v-if="setting" class="mt-1 text-xs text-muted-foreground">版本 {{ setting.version }}</p>
          </div>
        </div>
        <div v-if="settingQuery.isPending.value" class="text-sm text-muted-foreground">读取中…</div>
        <Switch v-else-if="setting" :model-value="setting.enabled" aria-label="学生邮箱注册总开关" @update:model-value="openSettingDialog(Boolean($event))" />
        <Button v-else size="sm" variant="outline" @click="settingQuery.refetch()"><RefreshCw class="h-4 w-4" />重试</Button>
      </div>
    </Card>

    <Card class="overflow-hidden p-0">
      <header class="flex flex-col gap-3 border-b border-border p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div class="flex items-center gap-2"><GraduationCap class="h-5 w-5 text-primary" /><h2 class="font-semibold">院校邮箱域名白名单</h2></div>
          <p class="mt-1 text-sm text-muted-foreground">{{ enabledDomainCount }} 个启用，共 {{ domains.length }} 条；只接受精确域名，不支持通配符或后缀自动放行。</p>
        </div>
        <div class="flex gap-2">
          <Button size="sm" variant="outline" :disabled="domainsQuery.isFetching.value" @click="refreshAll"><RefreshCw class="h-4 w-4" />刷新</Button>
          <Button size="sm" @click="openCreateDomain"><Plus class="h-4 w-4" />新增域名</Button>
        </div>
      </header>

      <div class="p-5">
        <SkeletonTable v-if="domainsQuery.isPending.value" :rows="4" :columns="6" />
        <ErrorState v-else-if="domainsQuery.isError.value" description="院校域名暂时无法读取。" @retry="domainsQuery.refetch()" />
        <EmptyState v-else-if="domains.length === 0" title="尚未配置院校域名" description="先添加精确邮箱域名；建议保持总开关关闭，完成验证后再开放注册。">
          <template #action><Button @click="openCreateDomain"><Plus class="h-4 w-4" />新增域名</Button></template>
        </EmptyState>
        <SoftTable v-else :columns="['院校', '精确域名', '状态', '版本', '更新时间', '操作']">
          <tr v-for="item in domains" :key="item.id">
            <td class="font-medium">{{ item.institutionName }}</td>
            <td class="font-mono text-sm">@{{ item.domain }}</td>
            <td><Badge :variant="item.enabled ? 'verified' : 'secondary'">{{ item.enabled ? '启用' : '停用' }}</Badge></td>
            <td>{{ item.version }}</td>
            <td><LocalTime :value="item.updatedAt" /></td>
            <td>
              <div class="flex flex-wrap gap-2">
                <Button size="sm" variant="outline" @click="openEditDomain(item)"><Pencil class="h-3.5 w-3.5" />编辑</Button>
                <Button size="sm" :variant="item.enabled ? 'outline' : 'default'" @click="openEditDomain(item, !item.enabled)">{{ item.enabled ? '停用' : '启用' }}</Button>
              </div>
            </td>
          </tr>
        </SoftTable>
      </div>
    </Card>

    <Dialog v-model:open="settingDialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader><DialogTitle>{{ nextSettingEnabled ? '开放学生邮箱注册' : '关闭学生邮箱注册' }}</DialogTitle><DialogDescription>提交时会校验当前版本并写入管理员操作日志。</DialogDescription></DialogHeader>
        <label class="space-y-2"><span class="text-sm font-medium">操作原因</span><Textarea v-model="settingReason" class="min-h-28" placeholder="说明本次开放或关闭的原因。" /></label>
        <DialogFooter><Button variant="outline" @click="settingDialogOpen = false">取消</Button><Button :disabled="updateSettingMutation.isPending.value || !settingReason.trim()" @click="saveSetting">{{ updateSettingMutation.isPending.value ? '保存中…' : '确认保存' }}</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="domainDialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader><DialogTitle>{{ editingDomain ? '编辑院校域名' : '新增院校域名' }}</DialogTitle><DialogDescription>{{ editingDomain ? '精确域名创建后不可修改；可更新名称与启用状态。' : '域名会按邮箱 @ 后的完整内容精确匹配。' }}</DialogDescription></DialogHeader>
        <div class="space-y-4">
          <label class="block space-y-2"><span class="text-sm font-medium">院校名称</span><Input v-model="domainForm.institutionName" placeholder="例如 示例大学" /></label>
          <label class="block space-y-2"><span class="text-sm font-medium">精确邮箱域名</span><Input v-model="domainForm.domain" :disabled="Boolean(editingDomain)" class="font-mono" placeholder="mail.example.edu" /><span class="text-xs text-muted-foreground">不含 @，不支持 * 通配符。</span></label>
          <div class="flex items-center justify-between rounded-md border border-border p-3"><div><div class="text-sm font-medium">允许新注册</div><div class="mt-1 text-xs text-muted-foreground">停用不会取消已有学生账号资格。</div></div><Switch v-model="domainForm.enabled" /></div>
          <label class="block space-y-2"><span class="text-sm font-medium">操作原因</span><Textarea v-model="domainForm.reason" class="min-h-24" placeholder="说明新增、改名、启用或停用原因。" /></label>
          <p v-if="editingDomain" class="text-xs text-muted-foreground">当前版本 {{ editingDomain.version }}；若已被其他管理员更新，本次提交会被拒绝并刷新列表。</p>
        </div>
        <DialogFooter><Button variant="outline" @click="domainDialogOpen = false">取消</Button><Button :disabled="domainMutationBusy" @click="saveDomain">{{ domainMutationBusy ? '保存中…' : '保存' }}</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
