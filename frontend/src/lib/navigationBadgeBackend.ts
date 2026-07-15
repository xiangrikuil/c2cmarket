import { backendRequest, requireBackendSession } from '@/lib/backendClient'

export type NavigationBadgeRoleSummary = {
  carpoolActions: number
  apiOrderActions: number
}

export type NavigationBadgeAdminSummary = {
  total: number
  officialPrices: number
  carpools: number
  apiServices: number
  feedbackTickets: number
  reports: number
}

export type NavigationBadgeSummary = {
  generatedAt: string
  notificationUnread: number
  importantAnnouncementUnread: number
  feedbackUnread: number
  buyer: NavigationBadgeRoleSummary
  merchant: NavigationBadgeRoleSummary
  admin: NavigationBadgeAdminSummary | null
}

export async function backendNavigationBadges(): Promise<NavigationBadgeSummary> {
  await requireBackendSession()
  return backendRequest<NavigationBadgeSummary>('/api/v1/me/navigation-badges')
}
