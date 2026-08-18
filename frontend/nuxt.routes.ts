import { resolve } from 'node:path'
import type { RouteLocationRaw, RouteMeta, RouteRecordRaw } from 'vue-router'

export type MaterializedNuxtPage = {
  path: string
  name?: string
  file?: string
  redirect?: RouteLocationRaw
  meta?: RouteMeta
}

export function materializeNuxtPages(routeRecords: RouteRecordRaw[], rootDirectory = process.cwd()): MaterializedNuxtPage[] {
  return routeRecords.map((route) => {
    const componentName = typeof route.component === 'function' ? route.component.name : ''
    const functionRedirect = typeof route.redirect === 'function'
    return {
      path: route.path,
      name: typeof route.name === 'string' ? route.name : undefined,
      ...(functionRedirect
        ? { file: resolve(rootDirectory, 'src/pages/RouteRedirectPage.vue') }
        : componentName ? { file: resolve(rootDirectory, 'src/pages', `${componentName}.vue`) } : {}),
      ...(route.redirect !== undefined && typeof route.redirect !== 'function' ? { redirect: route.redirect } : {}),
      ...(route.meta ? { meta: route.meta } : {}),
    }
  })
}
