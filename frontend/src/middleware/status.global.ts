import { defineNuxtRouteMiddleware, setResponseStatus } from '#app'

export default defineNuxtRouteMiddleware((to) => {
  if (import.meta.server && to.name === 'not-found') {
    setResponseStatus(404, 'Page Not Found')
  }
})
