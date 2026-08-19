import type { UnifiedNotification } from '@/lib/api'

export type BusinessNotificationCategory = 'todo' | 'transactions' | 'system'

export function businessNotificationCategory(item: Pick<UnifiedNotification, 'type'>): BusinessNotificationCategory {
  if (item.type === '交易待办') return 'todo'
  if (['审核结果', '管理操作', '边界提醒'].includes(item.type)) return 'system'
  return 'transactions'
}

export function prioritizeTransactionTodos<T extends Pick<UnifiedNotification, 'type'>>(items: readonly T[]): T[] {
  return items
    .map((item, index) => ({ item, index }))
    .sort((left, right) => Number(right.item.type === '交易待办') - Number(left.item.type === '交易待办') || left.index - right.index)
    .map(({ item }) => item)
}
