import { describe, expect, it } from 'vitest'
import type { UnifiedNotification } from '@/lib/api'
import { businessNotificationCategory, prioritizeTransactionTodos } from '@/lib/notificationUi'

function notification(id: string, type: UnifiedNotification['type'], unread = true): UnifiedNotification {
  return { id, type, unread, title: id, detail: id, time: '2026-08-18T00:00:00Z', to: '/' }
}

describe('notification UI classification', () => {
  it('uses explicit backend types instead of unread state', () => {
    expect(businessNotificationCategory(notification('todo-read', '交易待办', false))).toBe('todo')
    expect(businessNotificationCategory(notification('notice-unread', '交易通知', true))).toBe('transactions')
    expect(businessNotificationCategory(notification('legacy-unread', 'API 订单', true))).toBe('transactions')
    expect(businessNotificationCategory(notification('system', '管理操作', true))).toBe('system')
  })

  it('keeps transaction todos ahead of ordinary notifications without reordering peers', () => {
    const rows = [
      notification('notice-1', '交易通知'),
      notification('todo-1', '交易待办'),
      notification('notice-2', 'API 订单'),
      notification('todo-2', '交易待办'),
    ]
    expect(prioritizeTransactionTodos(rows).map(item => item.id)).toEqual(['todo-1', 'todo-2', 'notice-1', 'notice-2'])
  })
})
