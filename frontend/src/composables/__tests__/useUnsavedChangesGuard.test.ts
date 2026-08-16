import { readFileSync } from 'node:fs'
import { describe, expect, it, vi } from 'vitest'
import { shouldAllowUnsavedNavigation } from '../useUnsavedChangesGuard'

const composableSource = readFileSync(new URL('../useUnsavedChangesGuard.ts', import.meta.url), 'utf8')

describe('shouldAllowUnsavedNavigation', () => {
  it('leaves without prompting after a successful save resets dirty state', () => {
    const confirmLeave = vi.fn(() => false)

    expect(shouldAllowUnsavedNavigation(false, '未保存', confirmLeave)).toBe(true)
    expect(confirmLeave).not.toHaveBeenCalled()
  })

  it('keeps a dirty form on the current route when the user cancels', () => {
    const confirmLeave = vi.fn(() => false)

    expect(shouldAllowUnsavedNavigation(true, '未保存', confirmLeave)).toBe(false)
    expect(confirmLeave).toHaveBeenCalledWith('未保存')
  })

  it('allows tabs, sidebar links, browser back and other router navigation after confirmation', () => {
    expect(shouldAllowUnsavedNavigation(true, '未保存', () => true)).toBe(true)
  })

  it('guards both component leave and same-component route updates', () => {
    expect(composableSource).toContain('onBeforeRouteLeave(confirmNavigation)')
    expect(composableSource).toContain('onBeforeRouteUpdate(confirmNavigation)')
  })
})
