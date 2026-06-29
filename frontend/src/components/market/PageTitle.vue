<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { Button } from '@/components/ui/button'

defineProps<{
  title: string
  description?: string
  actionText?: string
  actionTo?: string
}>()
</script>

<template>
  <div class="mb-5 flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
    <div>
      <h1 class="text-2xl font-semibold md:text-3xl">{{ title }}</h1>
      <p v-if="description" class="mt-2 max-w-3xl text-sm text-muted-foreground">{{ description }}</p>
    </div>
    <RouterLink v-if="actionText && actionTo" :to="actionTo" class="w-full md:w-auto">
      <Button class="w-full md:w-auto">{{ actionText }}</Button>
    </RouterLink>
  </div>
</template>
