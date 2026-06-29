<script setup lang="ts">
import { FileText } from 'lucide-vue-next'
import { Textarea } from '@/components/ui/textarea'
import SubmitSectionHeader from './SubmitSectionHeader.vue'
import type { OfficialPriceSubmitErrors, OfficialPriceSubmitForm } from './types'

defineProps<{
  form: OfficialPriceSubmitForm
  errors: OfficialPriceSubmitErrors
}>()
</script>

<template>
  <section class="border-t border-border pt-5">
    <SubmitSectionHeader title="补充说明" hint="选填，但有助于加快审核" :icon="FileText" />
    <label class="space-y-2">
      <Textarea
        v-model="form.note"
        class="min-h-20"
        placeholder="说明是否含税、是否可复现、付款条件；不要填写密码、密钥、token 或付款码内容。"
      />
      <p v-if="errors.note" class="text-xs text-destructive">{{ errors.note }}</p>
    </label>
  </section>
</template>
