<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Mail, MessageCircle, Plus, Settings } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useCreateContactMethodMutation, useMyContactMethodsQuery, useMyProfileQuery } from '@/queries/useMarketQueries'
import { isTransactionContactEligible, transactionContactLabel } from '@/lib/transactionContacts'

const props = withDefaults(defineProps<{
  modelValue: string
  disabled?: boolean
  error?: string
  title?: string
  description?: string
}>(), {
  disabled: false,
  error: '',
  title: '交易联系方式',
  description: '仅在交易关系建立后按规则向对方展示。',
})

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()
const contactsQuery = useMyContactMethodsQuery()
const profileQuery = useMyProfileQuery()
const createContactMutation = useCreateContactMethodMutation()
const disclosureOpen = ref(false)
const eligibleContacts = computed(() => (contactsQuery.data.value ?? []).filter(isTransactionContactEligible))
const accountEmail = computed(() => profileQuery.data.value?.email?.trim().toLowerCase() ?? '')
const hasAccountEmailContact = computed(() => eligibleContacts.value.some(contact => (
  contact.type === 'email' && contact.displayValue.trim().toLowerCase() === accountEmail.value
)))
const canUseAccountEmail = computed(() => Boolean(
  profileQuery.data.value?.emailVerified && accountEmail.value && !hasAccountEmailContact.value,
))

watch(eligibleContacts, contacts => {
  if (contacts.some(contact => contact.id === props.modelValue)) return
  emit('update:modelValue', contacts.length === 1 ? contacts[0]!.id : '')
}, { immediate: true })

function selectContact(value: unknown) {
  if (typeof value === 'string') emit('update:modelValue', value)
}

function confirmAccountEmail() {
  if (!canUseAccountEmail.value) return
  createContactMutation.mutate({
    type: 'email',
    label: '账号邮箱',
    displayValue: accountEmail.value,
    isDefault: false,
    enabled: true,
  }, {
    onSuccess(contact) {
      disclosureOpen.value = false
      emit('update:modelValue', contact.id)
      toast.success('已将账号邮箱设为本次交易联系方式。')
    },
    onError(error) {
      toast.error(error instanceof Error ? error.message : '账号邮箱添加失败。')
    },
  })
}
</script>

<template>
  <section class="min-w-0 space-y-3" :aria-busy="contactsQuery.isPending.value">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h3 class="text-sm font-semibold">{{ title }}</h3>
        <p class="mt-1 text-xs leading-5 text-muted-foreground">{{ description }}</p>
      </div>
      <Button as-child size="sm" variant="outline">
        <RouterLink to="/my/contacts"><Settings class="h-3.5 w-3.5" />管理</RouterLink>
      </Button>
    </div>

    <div v-if="contactsQuery.isPending.value" class="rounded-md border border-border px-3 py-3 text-sm text-muted-foreground">
      正在读取联系方式...
    </div>
    <div v-else-if="contactsQuery.isError.value" class="rounded-md border border-destructive/30 px-3 py-3 text-sm text-destructive">
      联系方式暂时无法读取，请稍后重试。
    </div>
    <RadioGroup v-else-if="eligibleContacts.length" :model-value="modelValue" class="grid gap-2" :disabled="disabled" @update:model-value="selectContact">
      <label
        v-for="contact in eligibleContacts"
        :key="contact.id"
        class="flex min-w-0 cursor-pointer items-start gap-3 rounded-md border border-border px-3 py-3 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5"
      >
        <RadioGroupItem :value="contact.id" class="mt-0.5" />
        <component :is="contact.type === 'email' ? Mail : MessageCircle" class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
        <span class="min-w-0 flex-1">
          <span class="block text-sm font-medium">{{ transactionContactLabel(contact) }}</span>
          <span class="mt-0.5 block break-all text-xs text-muted-foreground">{{ contact.maskedValue }}</span>
        </span>
      </label>
    </RadioGroup>
    <div v-else class="rounded-md border border-dashed border-border px-3 py-3 text-sm text-muted-foreground">
      暂无可用联系方式。请添加微信，或使用已验证邮箱。
    </div>

    <div v-if="canUseAccountEmail" class="flex flex-wrap items-center gap-2">
      <Button type="button" size="sm" variant="ghost" :disabled="disabled" @click="disclosureOpen = true">
        <Plus class="h-3.5 w-3.5" />使用已验证账号邮箱
      </Button>
      <span class="text-xs text-muted-foreground">{{ accountEmail }}</span>
    </div>
    <p v-if="error" class="text-xs text-destructive" role="alert">{{ error }}</p>

    <Dialog v-model:open="disclosureOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>使用账号邮箱作为交易联系方式</DialogTitle>
          <DialogDescription>
            交易关系建立后，{{ accountEmail }} 可能会向交易对方展示。账号安全通知邮箱不会自动公开。
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" @click="disclosureOpen = false">取消</Button>
          <Button :disabled="createContactMutation.isPending.value" @click="confirmAccountEmail">确认使用</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </section>
</template>
