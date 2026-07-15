<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from '@/components/layout/AppShell.vue'
import AdminShell from '@/components/layout/AdminShell.vue'
import { Toaster } from '@/components/ui/sonner'

const route = useRoute()
const standaloneLayout = computed(() => route.meta.standalone === true)
const adminLayout = computed(() => !standaloneLayout.value && route.path.startsWith('/admin'))
</script>

<template>
  <RouterView v-if="standaloneLayout" v-slot="{ Component }">
    <Transition name="fade" mode="out-in">
      <component :is="Component" class="c2c-fade-in" />
    </Transition>
  </RouterView>

  <AdminShell v-else-if="adminLayout">
    <RouterView v-slot="{ Component }">
      <Transition name="fade" mode="out-in">
        <component :is="Component" class="c2c-fade-in" />
      </Transition>
    </RouterView>
  </AdminShell>

  <AppShell v-else>
    <RouterView v-slot="{ Component }">
      <Transition name="fade" mode="out-in">
        <component :is="Component" class="c2c-fade-in" />
      </Transition>
    </RouterView>
  </AppShell>
  <Toaster position="top-right" rich-colors />
</template>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 120ms ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
