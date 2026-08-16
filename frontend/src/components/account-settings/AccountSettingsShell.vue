<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'

type SettingsTab = {
  label: string
  to: string
}

const props = withDefaults(defineProps<{
  contactLabel?: string
  locked?: boolean
}>(), {
  contactLabel: '联系与收款',
  locked: false,
})

const emit = defineEmits<{
  'blocked-navigation': []
}>()

const route = useRoute()
const activeLink = ref<HTMLElement | null>(null)

const tabs = computed<SettingsTab[]>(() => [
  { label: '个人资料', to: '/my/profile' },
  { label: props.contactLabel, to: '/my/contacts' },
  { label: '登录与认证', to: '/my/account' },
  { label: '隐私', to: '/my/privacy' },
])

function isLocked(to: string) {
  return props.locked && to !== '/my/account'
}

function handleClick(to: string, event: MouseEvent) {
  if (!isLocked(to)) return
  event.preventDefault()
  emit('blocked-navigation')
}

function setLinkRef(element: unknown, to: string) {
  if (to !== route.path) return
  if (typeof HTMLElement === 'undefined') return
  const linkElement = element instanceof HTMLElement
    ? element
    : (element as { $el?: unknown } | null)?.$el
  activeLink.value = linkElement instanceof HTMLElement ? linkElement : null
}

watch(
  () => route.path,
  async () => {
    await nextTick()
    activeLink.value?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
  },
  { immediate: true, flush: 'post' },
)
</script>

<template>
  <section class="account-settings-shell">
    <header class="account-settings-shell__heading">
      <div class="min-w-0">
        <p>账户 / 账户设置</p>
        <h1>账户设置</h1>
        <slot name="description" />
      </div>
      <div class="account-settings-shell__actions">
        <slot name="actions" />
      </div>
    </header>

    <div class="account-settings-shell__nav-wrap">
      <nav class="account-settings-shell__nav" aria-label="账户设置">
        <RouterLink
          v-for="tab in tabs"
          :key="tab.to"
          :ref="element => setLinkRef(element, tab.to)"
          :to="tab.to"
          :aria-current="route.path === tab.to ? 'page' : undefined"
          :aria-disabled="isLocked(tab.to) || undefined"
          :class="{ 'is-active': route.path === tab.to, 'is-locked': isLocked(tab.to) }"
          @click.capture="handleClick(tab.to, $event)"
        >
          {{ tab.label }}
        </RouterLink>
      </nav>
    </div>

    <div class="account-settings-shell__content">
      <slot />
    </div>
  </section>
</template>
