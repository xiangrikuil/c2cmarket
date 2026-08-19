<script setup lang="ts">
import { routes } from '@/router'

const route = useRoute()
const routeRecord = routes.find(item => item.path === route.path)
const target = typeof routeRecord?.redirect === 'function'
  ? routeRecord.redirect(route, route)
  : routeRecord?.redirect

if (!target) {
  throw createError({ statusCode: 404, statusMessage: 'Page not found' })
}

await navigateTo(target, { replace: true, redirectCode: 302 })
</script>

<template>
  <div />
</template>
