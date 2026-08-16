<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import type { CarpoolProductCatalogItem, CarpoolPublishForm, PublishFieldState, RegionOption } from './types'
import CarpoolProductCombobox from './CarpoolProductCombobox.vue'
import PublishSectionCard from './PublishSectionCard.vue'

const props = defineProps<{
  form: CarpoolPublishForm
  catalog: CarpoolProductCatalogItem[]
  regions: RegionOption[]
  errors: Partial<Record<string, string>>
  fieldStates?: Partial<Record<string, PublishFieldState>>
  highlightedKey?: string
}>()

function booleanSelectValue(value: boolean | null) {
  if (value === null) return ''
  return value ? 'true' : 'false'
}

function selectedBoolean(value: unknown) {
  if (value === 'true') return true
  if (value === 'false') return false
  return null
}

function fieldState(key: string): PublishFieldState {
  return props.fieldStates?.[key] ?? 'idle'
}

function fieldShellClass(key: string) {
  const state = fieldState(key)
  return [
    'rounded-lg border p-3 transition-colors',
    state === 'error' ? 'border-destructive/45 bg-destructive/5' : '',
    state === 'pendingRequired' ? 'border-warning/40 bg-warning/5' : '',
    state === 'defaulted' ? 'border-success/35 bg-success/5' : '',
    state === 'complete' ? 'border-border bg-background' : '',
    state === 'idle' ? 'border-transparent bg-transparent p-0' : '',
    props.highlightedKey === key ? 'ring-2 ring-primary/60 ring-offset-2 ring-offset-background' : '',
  ]
}

function stateLabel(key: string) {
  const state = fieldState(key)
  if (state === 'error') return '需要处理'
  if (state === 'pendingRequired') return '待填写'
  if (state === 'defaulted') return '系统默认'
  if (state === 'complete') return '已完成'
  return ''
}

function stateLabelClass(key: string) {
  const state = fieldState(key)
  if (state === 'error') return 'bg-destructive/10 text-destructive'
  if (state === 'pendingRequired') return 'bg-warning/10 text-warning'
  if (state === 'defaulted') return 'bg-success/10 text-success'
  if (state === 'complete') return 'bg-success/10 text-success'
  return 'bg-muted text-muted-foreground'
}
</script>

<template>
  <PublishSectionCard
    :index="1"
    title="基础信息"
    description="选择订阅产品，并补充额度、重置、网络和公开接入信号。"
  >
    <div class="grid gap-4 md:grid-cols-2">
      <div id="carpool-task-product" :class="fieldShellClass('product')">
        <div class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>产品 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('product')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('product')">{{ stateLabel('product') }}</span>
        </div>
        <div class="mt-2">
          <CarpoolProductCombobox
            v-model="form.productId"
            :custom-product-name="form.customProductName"
            :catalog="catalog"
            @update:custom-product-name="value => form.customProductName = value"
          />
        </div>
        <p v-if="errors.product" class="text-xs text-destructive">{{ errors.product }}</p>
        <p v-else-if="fieldState('product') === 'pendingRequired'" class="mt-2 text-xs text-warning">选择套餐目录后，系统会同步访问安排和风险提示。</p>
      </div>

      <label id="carpool-task-region" class="space-y-2" :class="fieldShellClass('region')">
        <span class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>开通区 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('region')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('region')">{{ stateLabel('region') }}</span>
        </span>
        <Select v-model="form.regionCode">
          <SelectTrigger class="w-full bg-background"><SelectValue placeholder="选择开通区" /></SelectTrigger>
          <SelectContent>
            <SelectItem v-for="region in regions" :key="region.code" :value="region.code">{{ region.displayName }}</SelectItem>
          </SelectContent>
        </Select>
        <Input
          v-if="form.regionCode === 'other'"
          :model-value="form.customRegionName ?? ''"
          placeholder="填写开通区，例如印度区、巴西区、欧洲区"
          @update:model-value="value => form.customRegionName = value === '' ? null : String(value)"
        />
        <p v-if="errors.region" class="text-xs text-destructive">{{ errors.region }}</p>
        <p v-else-if="fieldState('region') === 'pendingRequired'" class="text-xs text-warning">请选择或填写买家实际开通或使用的地区。</p>
      </label>

      <label id="carpool-task-monthlyPrice" class="space-y-2" :class="fieldShellClass('monthlyPrice')">
        <span class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>月费 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('monthlyPrice')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('monthlyPrice')">{{ stateLabel('monthlyPrice') }}</span>
        </span>
        <Input
          :model-value="form.monthlyPriceCny ?? ''"
          type="number"
          min="0"
          placeholder="68"
          @update:model-value="value => form.monthlyPriceCny = value === '' ? null : Number(value)"
        />
        <p v-if="errors.monthlyPriceCny" class="text-xs text-destructive">{{ errors.monthlyPriceCny }}</p>
        <p v-else class="text-xs" :class="fieldState('monthlyPrice') === 'pendingRequired' ? 'text-warning' : 'text-muted-foreground'">默认按人民币 / 月展示。</p>
      </label>

      <div id="carpool-task-dailyQuota" class="space-y-2" :class="fieldShellClass('dailyQuota')">
        <span class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>每日最大花费额度（美元） <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('dailyQuota')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('dailyQuota')">{{ stateLabel('dailyQuota') }}</span>
        </span>
        <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_80px]">
          <Input
            :model-value="form.dailyQuotaAmount ?? ''"
            type="number"
            min="0.01"
            step="1"
            placeholder="50"
            @update:model-value="value => form.dailyQuotaAmount = value === '' ? null : Number(value)"
          />
          <span class="flex h-9 items-center justify-center rounded-md border bg-muted text-sm font-medium">USD</span>
        </div>
        <p v-if="errors.dailyQuota" class="text-xs text-destructive">{{ errors.dailyQuota }}</p>
      </div>

      <div id="carpool-task-weeklyQuota" class="space-y-2" :class="fieldShellClass('weeklyQuota')">
        <span class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>每周最大花费额度（美元） <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('weeklyQuota')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('weeklyQuota')">{{ stateLabel('weeklyQuota') }}</span>
        </span>
        <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_80px]">
          <Input
            :model-value="form.weeklyQuotaAmount ?? ''"
            type="number"
            min="0.01"
            step="1"
            placeholder="200"
            @update:model-value="value => form.weeklyQuotaAmount = value === '' ? null : Number(value)"
          />
          <span class="flex h-9 items-center justify-center rounded-md border bg-muted text-sm font-medium">USD</span>
        </div>
        <p v-if="errors.weeklyQuota" class="text-xs text-destructive">{{ errors.weeklyQuota }}</p>
      </div>

      <label id="carpool-task-quotaReset" class="space-y-2" :class="fieldShellClass('quotaReset')">
        <span class="font-medium">是否跟随官方重置 <span class="text-xs text-primary">必填</span></span>
        <Select :model-value="booleanSelectValue(form.followsOfficialQuotaReset)" @update:model-value="value => form.followsOfficialQuotaReset = selectedBoolean(value)">
          <SelectTrigger class="w-full bg-background"><SelectValue placeholder="请选择" /></SelectTrigger>
          <SelectContent><SelectItem value="true">跟随官方重置</SelectItem><SelectItem value="false">不跟随官方重置</SelectItem></SelectContent>
        </Select>
        <p v-if="errors.quotaReset" class="text-xs text-destructive">{{ errors.quotaReset }}</p>
      </label>

      <label id="carpool-task-vpsRegion" class="space-y-2" :class="fieldShellClass('connection')">
        <span class="font-medium">VPS 区域 <span class="text-xs text-primary">必填</span></span>
        <Input v-model="form.vpsRegion" maxlength="64" placeholder="例如香港、新加坡、美国西部" />
        <p v-if="errors.connection" class="text-xs text-destructive">{{ errors.connection }}</p>
      </label>

      <label class="space-y-2" :class="fieldShellClass('connection')">
        <span class="font-medium">是否支持国内直连 <span class="text-xs text-primary">必填</span></span>
        <Select :model-value="booleanSelectValue(form.supportsMainlandChinaDirectConnection)" @update:model-value="value => form.supportsMainlandChinaDirectConnection = selectedBoolean(value)">
          <SelectTrigger class="w-full bg-background"><SelectValue placeholder="请选择" /></SelectTrigger>
          <SelectContent><SelectItem value="true">支持国内直连</SelectItem><SelectItem value="false">不支持国内直连</SelectItem></SelectContent>
        </Select>
      </label>

      <div id="carpool-task-distribution" class="space-y-3 md:col-span-2" :class="fieldShellClass('distribution')">
        <div class="flex items-center justify-between gap-2 text-sm font-medium">
          <span>分发方式与管理员账号 <span class="text-xs text-primary">必填</span></span>
          <span v-if="stateLabel('distribution')" class="rounded-full px-2 py-0.5 text-xs font-medium" :class="stateLabelClass('distribution')">{{ stateLabel('distribution') }}</span>
        </div>
        <div class="grid gap-3 md:grid-cols-2">
          <label class="space-y-2 text-sm">
            <span class="font-medium">分发方式</span>
            <Select v-model="form.distributionMethod"><SelectTrigger class="w-full bg-background"><SelectValue placeholder="选择分发方式" /></SelectTrigger><SelectContent><SelectItem value="sub2api">Sub2API</SelectItem><SelectItem value="other">其他</SelectItem></SelectContent></Select>
          </label>
          <label class="space-y-2 text-sm">
            <span class="font-medium">是否提供管理员账号</span>
            <Select :model-value="booleanSelectValue(form.providesAdminAccount)" @update:model-value="value => form.providesAdminAccount = selectedBoolean(value)"><SelectTrigger class="w-full bg-background"><SelectValue placeholder="请选择" /></SelectTrigger><SelectContent><SelectItem value="true">提供管理员账号</SelectItem><SelectItem value="false">不提供管理员账号</SelectItem></SelectContent></Select>
          </label>
        </div>
        <label v-if="form.distributionMethod === 'other'" class="block space-y-2 text-sm">
          <span class="font-medium">其他分发说明</span>
          <Textarea v-model="form.distributionMethodNote" class="min-h-20 bg-background" placeholder="说明站外分发方式，不填写任何账号凭据。" />
        </label>
        <p v-if="errors.distribution" class="text-xs text-destructive">{{ errors.distribution }}</p>
        <p v-else class="text-xs text-muted-foreground">具体权限与使用细节请站外确认，平台不保存管理员凭据。</p>
      </div>
    </div>
  </PublishSectionCard>
</template>
