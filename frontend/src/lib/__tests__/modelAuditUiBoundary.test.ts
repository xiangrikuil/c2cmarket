import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const pageSource = readFileSync(new URL('../../pages/AdminModelAuditPage.vue', import.meta.url), 'utf8')
const queriesSource = readFileSync(new URL('../../queries/useModelAuditQueries.ts', import.meta.url), 'utf8')

describe('model audit V1 UI boundary', () => {
  it('does not expose scheduled monitor state or controls', () => {
    for (const unsupportedSource of [
      'useModelAuditMonitors',
      'useCreateModelAuditMonitor',
      'ModelAuditMonitorInput',
      'monitorForm',
      'monitorsQuery',
      'createMonitorMutation',
      '定时巡检',
      '巡检配置',
      'cronSpec',
      "value: 'scheduled'",
    ]) {
      expect(pageSource).not.toContain(unsupportedSource)
    }
  })

  it('keeps the real manual audit run path while backend monitor hooks stay available', () => {
    expect(pageSource).toContain('useCreateModelAuditRun')
    expect(pageSource).toContain('const createRunMutation = useCreateModelAuditRun()')
    expect(pageSource).toContain('async function createRun()')
    expect(pageSource).toContain('await createRunMutation.mutateAsync')
    expect(pageSource).toContain('@click="createRun"')
    expect(pageSource).toContain('启动审计')

    expect(queriesSource).toContain('export function useModelAuditMonitors')
    expect(queriesSource).toContain('export function useCreateModelAuditMonitor')
  })
})
