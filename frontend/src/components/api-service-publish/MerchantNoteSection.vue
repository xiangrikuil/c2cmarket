<script setup lang="ts">
import { Gauge, Layers3, ShieldCheck } from 'lucide-vue-next'
import { Card } from '@/components/ui/card'
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
          <Gauge class="h-4 w-4" />
        </span>
        <div>
          <h2>服务体验与说明</h2>
          <p>填写商户自报体验、用量核对与售后口径。</p>
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

      <div class="grid gap-3 md:grid-cols-3">
        <label class="space-y-2">
          <span class="text-sm font-medium">首字响应区间</span>
          <Select v-model="form.declaredTtftBand">
            <SelectTrigger
              :aria-invalid="Boolean(errors.performance)"
              :aria-describedby="errors.performance ? 'api-publish-performance-error' : undefined"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="under_1s">&lt;1 秒</SelectItem>
              <SelectItem value="1_to_3s">1-3 秒</SelectItem>
              <SelectItem value="3_to_5s">3-5 秒</SelectItem>
              <SelectItem value="5_to_10s">5-10 秒</SelectItem>
              <SelectItem value="over_10s">&gt;10 秒</SelectItem>
            </SelectContent>
          </Select>
        </label>
        <label class="space-y-2">
			<span class="text-sm font-medium">商户声明最大并发</span>
          <Input
            v-model.number="form.declaredMaxConcurrency"
            :aria-invalid="Boolean(errors.performance)"
            :aria-describedby="errors.performance ? 'api-publish-performance-error' : undefined"
            type="number"
            min="1"
            max="100000"
          />
        </label>
        <label class="space-y-2">
          <span class="text-sm font-medium">最近确认时间</span>
          <Input
            v-model="form.performanceConfirmedAt"
            :aria-invalid="Boolean(errors.performance)"
            :aria-describedby="errors.performance ? 'api-publish-performance-error' : undefined"
            type="datetime-local"
          />
        </label>
      </div>
      <p v-if="errors.performance" id="api-publish-performance-error" class="text-xs text-destructive">{{ errors.performance }}</p>
      <p class="text-xs text-muted-foreground">商户自报，平台未测速。额度包将冻结购买时的声明。</p>

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
        <button
          v-for="snippet in merchantNoteQuickInserts"
          :key="snippet"
          type="button"
          class="rounded-full border border-border bg-background px-3 py-1 text-xs hover:bg-muted"
          @click="insertSnippet(form, snippet)"
        >
          + {{ snippet }}
        </button>
      </div>
    </div>
  </Card>
</template>
