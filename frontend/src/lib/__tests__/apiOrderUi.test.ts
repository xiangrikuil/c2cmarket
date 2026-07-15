import { describe, expect, it } from 'vitest'
import {
  buildApiOrderCancelReason,
  formatApiOrderCancelReason,
  formatOrderDateTime,
  merchantHandlingDeadline,
  orderCountdown,
} from '@/lib/apiOrderUi'

describe('API order UI helpers', () => {
  it('starts the merchant handling window from payment submission', () => {
    expect(merchantHandlingDeadline('2026-07-11T10:00:00.000Z')).toBe('2026-07-11T10:10:00.000Z')
    expect(merchantHandlingDeadline(undefined)).toBeNull()
  })

  it('formats active, urgent and expired countdowns', () => {
    expect(orderCountdown('2026-07-11T10:10:00.000Z', Date.parse('2026-07-11T10:00:15.000Z'))).toMatchObject({
      label: '09:45',
      expired: false,
      urgent: false,
    })
    expect(orderCountdown('2026-07-11T10:01:00.000Z', Date.parse('2026-07-11T10:00:00.000Z'))).toMatchObject({ urgent: true })
    expect(orderCountdown('2026-07-11T10:00:00.000Z', Date.parse('2026-07-11T10:00:01.000Z'))).toMatchObject({
      label: '00:00',
      expired: true,
    })
  })

  it('builds a readable cancellation reason with responsibility', () => {
    expect(buildApiOrderCancelReason('merchant_unresponsive', '已等待多次回复')).toBe('商家原因｜商家长时间未响应｜补充说明：已等待多次回复')
    expect(() => buildApiOrderCancelReason('buyer_other', '')).toThrow('请填写其他取消原因。')
  })

  it('formats server timestamps for the current locale', () => {
    expect(formatOrderDateTime('invalid')).toBe('invalid')
    expect(formatOrderDateTime()).toBe('—')
  })

  it('maps system cancellation codes to user-facing copy', () => {
    expect(formatApiOrderCancelReason('payment_timeout')).toBe('未在付款时间内完成付款，系统已自动取消订单。')
    expect(formatApiOrderCancelReason('个人原因｜我不再需要该服务')).toBe('个人原因｜我不再需要该服务')
  })
})
