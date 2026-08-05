<script setup lang="ts">
import { FileText, Gauge, Layers3, ShieldAlert, ShieldCheck } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'
import type { ApiServicePublishForm } from './types'
import { accountPoolLabels, merchantNoteQuickInserts } from './utils'

const accountPoolOptions = [
  'gpt_pro_20x',
  'gpt_pro_5x',
  'gpt_plus',
  'custom',
] as const

defineProps<{
  form: ApiServicePublishForm
  errors: Partial<Record<string, string>>
}>()

const insertSnippet = (form: ApiServicePublishForm, value: string) => {
  if (form.merchantNote.includes(value)) return
  const separator = form.merchantNote.trim().endsWith('。') || form.merchantNote.trim().endsWith('；') ? '\n' : '；'
  form.merchantNote = [form.merchantNote.trim(), value].filter(Boolean).join(separator)
}
</script>

<template>
  <Card class="api-publish-card">
    <div class="api-publish-card-header">
      <div class="flex items-start gap-2">
        <span class="grid h-7 w-7 shrink-0 place-items-center rounded-md bg-amber-50 text-amber-600">
          <FileText class="h-4 w-4" />
        </span>
        <div>
          <h2>服务规则与说明</h2>
          <p>声明并发、提示词审计、用量核对与售后口径。</p>
        </div>
      </div>
    </div>

    <div class="api-publish-card-body space-y-3">
		<div class="grid gap-3 border-b border-border pb-3 md:grid-cols-2">
			<label class="space-y-2">
				<span class="flex items-center gap-1.5 text-sm font-medium"><Layers3 class="h-4 w-4 text-primary" />号池</span>
				<Select v-model="form.accountPoolType">
					<SelectTrigger
						:aria-invalid="Boolean(errors.accountPool)"
						:aria-describedby="errors.accountPool ? 'api-publish-account-pool-error' : undefined"
					>
						<SelectValue placeholder="请选择一个号池" />
					</SelectTrigger>
					<SelectContent>
						<SelectItem v-for="option in accountPoolOptions" :key="option" :value="option">{{ accountPoolLabels[option] }}</SelectItem>
					</SelectContent>
				</Select>
			</label>
			<label v-if="form.accountPoolType === 'custom'" class="space-y-2">
				<span class="text-sm font-medium">其他号池名称</span>
				<Input
					v-model="form.accountPoolCustomName"
					maxlength="40"
					placeholder="例如 Claude Max、Gemini Advanced"
					:aria-invalid="Boolean(errors.accountPool)"
					:aria-describedby="errors.accountPool ? 'api-publish-account-pool-error' : undefined"
				/>
			</label>
			<p v-if="errors.accountPool" id="api-publish-account-pool-error" class="text-xs text-destructive md:col-span-2">{{ errors.accountPool }}</p>
		</div>

      <div class="grid gap-3 md:grid-cols-2">
        <label class="space-y-2 rounded-md border border-border p-3">
          <span class="flex items-center gap-1.5 text-sm font-medium"><Gauge class="h-4 w-4 text-primary" />商户声明最大并发</span>
          <Input
            v-model.number="form.declaredMaxConcurrency"
            :aria-invalid="Boolean(errors.performance)"
            :aria-describedby="errors.performance ? 'api-publish-performance-error' : undefined"
            type="number"
            min="1"
            max="100000"
          />
          <span class="block text-xs leading-5 text-muted-foreground">这是卖家声明的并发上限，不是平台探测结果。</span>
        </label>
        <fieldset class="space-y-2 rounded-md border border-border p-3">
          <legend class="flex items-center gap-1.5 px-1 text-sm font-medium"><ShieldAlert class="h-4 w-4 text-orange-600" />提示词审计</legend>
          <RadioGroup
            :model-value="form.promptAuditEnabled === null ? undefined : String(form.promptAuditEnabled)"
            class="grid grid-cols-2 gap-2"
            :aria-invalid="Boolean(errors.performance)"
            :aria-describedby="errors.performance ? 'api-publish-performance-error' : 'api-publish-prompt-audit-description'"
            @update:model-value="value => form.promptAuditEnabled = value === 'true'"
          >
            <label class="flex cursor-pointer items-center gap-2 rounded-md border border-border px-3 py-2 text-sm has-[[data-state=checked]]:border-orange-400 has-[[data-state=checked]]:bg-orange-50/70">
              <RadioGroupItem value="true" />
              开启
            </label>
            <label class="flex cursor-pointer items-center gap-2 rounded-md border border-border px-3 py-2 text-sm has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
              <RadioGroupItem value="false" />
              关闭
            </label>
          </RadioGroup>
          <p id="api-publish-prompt-audit-description" class="text-xs leading-5 text-muted-foreground">
            开启表示卖家声明可能查看或记录提示词；关闭也只是卖家声明，不代表平台已验证。
          </p>
        </fieldset>
      </div>
      <p v-if="errors.performance" id="api-publish-performance-error" class="text-xs text-destructive">{{ errors.performance }}</p>

		<div class="space-y-2 border-y border-border py-3">
			<div class="flex items-center gap-1.5 text-sm font-medium"><ShieldCheck class="h-4 w-4 text-primary" />退款承诺</div>
			<RadioGroup v-model="form.warranty.mode" class="grid gap-2 md:grid-cols-2">
				<label class="flex cursor-pointer gap-3 rounded-md border border-border p-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5">
					<RadioGroupItem value="no_warranty" class="mt-0.5" />
					<span><strong class="text-sm">无额外退款承诺</strong><span class="mt-1 block text-xs leading-5 text-muted-foreground">具体问题由双方站外协商，平台不托管、不垫付、不代赔。</span></span>
				</label>
				<label class="flex cursor-pointer gap-3 rounded-md border border-border p-3 has-[[data-state=checked]]:border-orange-400 has-[[data-state=checked]]:bg-orange-50/70">
					<RadioGroupItem value="merchant_full_refund" class="mt-0.5" />
					<span><strong class="text-sm">商户全额退款承诺</strong><span class="mt-1 block text-xs leading-5 text-muted-foreground">服务有效期内未交付、订单事实不符，或交付后连续不可用超过 1 小时，商户退还全部实付金额。</span></span>
				</label>
			</RadioGroup>
			<p v-if="form.warranty.mode === 'merchant_full_refund'" class="text-xs leading-5 text-muted-foreground">买家违规、超出商户声明最大并发、额度正常耗尽、正常上游限流或买家网络问题不适用。规则版本：api-merchant-refund-v1。</p>
			<p v-if="errors.refundCommitment" class="text-xs text-destructive">{{ errors.refundCommitment }}</p>
		</div>

      <div class="rounded-md border border-border bg-muted/45 px-3 py-2 text-xs leading-5 text-muted-foreground">
        不要填写 API Key、token、密码、Session、Cookie、付款码或面板凭据；买家创建订单后，双方站外确认 API 细节。
      </div>

      <div class="space-y-2">
        <Textarea
          v-model="form.merchantNote"
          :aria-invalid="Boolean(errors.merchantNote)"
          :aria-describedby="errors.merchantNote ? 'api-publish-merchant-note-error' : undefined"
          class="min-h-28"
          maxlength="800"
          placeholder="请说明用量核对、限速规则、可用时间和售后口径。"
        />
        <div class="flex items-center justify-between gap-3">
          <p v-if="errors.merchantNote" id="api-publish-merchant-note-error" class="text-xs text-destructive">{{ errors.merchantNote }}</p>
          <p class="ml-auto text-xs text-muted-foreground">已输入 {{ form.merchantNote.length }} / 800 字</p>
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <Button
          v-for="snippet in merchantNoteQuickInserts"
          :key="snippet"
          size="sm"
          variant="outline"
          class="h-auto rounded-full px-3 py-1 text-xs"
          @click="insertSnippet(form, snippet)"
        >
          + {{ snippet }}
        </Button>
      </div>
    </div>
  </Card>
</template>
