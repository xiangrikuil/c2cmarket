import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { formatLocalDateTime, shortId, statusTone } from '@/lib/presentation'

describe('共享展示格式', () => {
  it('生成稳定短编号并保留业务前缀', () => {
    expect(shortId('12049d7e-7088-4c99-80c6-e6cc0e8eeed1', 'API')).toBe('API-8EEED1')
  })

  it('把 ISO 时间转为用户本地时间而不是原样输出', () => {
    const formatted = formatLocalDateTime('2026-07-11T12:30:00Z')
    expect(formatted).toMatch(/^2026-07-11 \d{2}:30$/)
    expect(formatted).not.toContain('T')
    expect(formatLocalDateTime('not-a-date')).toBe('—')
  })

  it('统一状态语义色', () => {
    expect(statusTone('pending_payment')).toBe('waiting')
    expect(statusTone('credential_risk')).toBe('risk')
    expect(statusTone('completed')).toBe('success')
    expect(statusTone('cancelled')).toBe('complete')
  })

  it('短编号复制动作具有可访问名称，异步状态带语义属性', () => {
    const shortIdSource = readFileSync(new URL('../../components/market/ShortId.vue', import.meta.url), 'utf8')
    const skeletonSource = readFileSync(new URL('../../components/market/SkeletonBlock.vue', import.meta.url), 'utf8')
    const errorSource = readFileSync(new URL('../../components/market/ErrorState.vue', import.meta.url), 'utf8')
    expect(shortIdSource).toContain('aria-label')
    expect(skeletonSource).toContain('aria-busy="true"')
    expect(errorSource).toContain('role="alert"')
  })
})
