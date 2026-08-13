import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const switcher = readFileSync(new URL('../DevPersonaSwitcher.vue', import.meta.url), 'utf8')
const appShell = readFileSync(new URL('../AppShell.vue', import.meta.url), 'utf8')
const adminShell = readFileSync(new URL('../AdminShell.vue', import.meta.url), 'utf8')

describe('development persona switcher', () => {
  it('is development-only and exposes explicit real and mock personas', () => {
    expect(switcher).toContain('const visible = import.meta.dev')
    expect(switcher).toContain('const realBackend = shouldUseRealBackend()')
    expect(switcher).toContain('createDevPersonaSession(persona as DevPersona)')
    expect(switcher).toContain('createMockPersonaSession(persona as MockPersona)')
    for (const username of ['dev-buyer', 'dev-seller', 'dev-admin']) {
      expect(switcher).toContain(username)
    }
    for (const persona of ["persona: 'anonymous'", "persona: 'student'", "persona: 'linuxdo'", "persona: 'admin'"]) {
      expect(switcher).toContain(persona)
    }
    expect(switcher).not.toContain('ensureBackendSession')
    expect(switcher).not.toContain('window.location.reload')
  })

  it('clears and refetches stale user state before navigating home', () => {
    expect(switcher).toContain('v-model:open="popoverOpen"')
    expect(switcher).toContain('<PopoverContent align="end"')
    expect(switcher).toContain('@click="choosePersona(item.persona)"')
    expect(switcher).toContain('popoverOpen.value = false')
    expect(switcher).toContain('collisionSuffixPattern.test')
    expect(switcher).toContain('queryClient.getMutationCache().clear()')
    expect(switcher).toContain("queryClient.removeQueries({ type: 'inactive' })")
    expect(switcher).toContain("queryClient.resetQueries({ type: 'active' })")
    expect(switcher).toContain("router.replace(session ? '/my' : '/login')")
    expect(switcher.indexOf("queryClient.resetQueries({ type: 'active' })"))
      .toBeLessThan(switcher.indexOf("router.replace(session ? '/my' : '/login')"))
    expect(switcher).toContain('busyPersona')
    expect(switcher).toContain('activePersona')
  })

  it('is shared by user and admin shells with the current identity', () => {
    for (const shell of [appShell, adminShell]) {
      expect(shell).toContain("import DevPersonaSwitcher from '@/components/layout/DevPersonaSwitcher.vue'")
      expect(shell).toContain('<DevPersonaSwitcher :current-username="currentUsername" />')
    }
  })
})
