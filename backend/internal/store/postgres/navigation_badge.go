package postgres

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/navigationbadge"
)

func (s *Store) NavigationBadgeSummary(ctx context.Context, userID string, isAdmin bool, now time.Time) (navigationbadge.Summary, *domain.AppError) {
	if s == nil || s.pool == nil {
		return navigationbadge.Summary{}, internalStoreError()
	}

	result := navigationbadge.Summary{GeneratedAt: now}
	admin := navigationbadge.AdminCounts{}
	err := s.pool.QueryRow(ctx, navigationBadgeSummarySQL, userID, now, isAdmin).Scan(
		&result.NotificationUnread,
		&result.ImportantAnnouncementUnread,
		&result.FeedbackUnread,
		&result.SupportActionCount,
		&result.Buyer.CarpoolActions,
		&result.Buyer.APIOrderActions,
		&result.Merchant.CarpoolActions,
		&result.Merchant.APIOrderActions,
		&admin.OfficialPrices,
		&admin.Carpools,
		&admin.APIServices,
		&admin.FeedbackTickets,
		&admin.Reports,
	)
	if err != nil {
		return navigationbadge.Summary{}, internalStoreError()
	}
	if isAdmin {
		admin.Total = admin.ActionableTotal()
		result.Admin = &admin
	}
	return result, nil
}

const navigationBadgeSummarySQL = `
WITH support_counts AS (
  SELECT
    (SELECT count(*)::int
     FROM feedback_tickets
     WHERE submitter_user_id = $1
       AND latest_admin_update_at IS NOT NULL
       AND (submitter_read_at IS NULL OR submitter_read_at < latest_admin_update_at)) AS feedback_unread,
    (SELECT count(*)::int
     FROM feedback_tickets
     WHERE submitter_user_id = $1
       AND (
         status = 'needs_user_info'
         OR (
           latest_admin_update_at IS NOT NULL
           AND (submitter_read_at IS NULL OR submitter_read_at < latest_admin_update_at)
         )
       )) AS feedback_actions,
    (SELECT count(*)::int
     FROM moderation_info_requests
     WHERE requested_from_user_id = $1
       AND status = 'open') AS moderation_actions
)
SELECT
  (SELECT count(*)::int
   FROM notifications
   WHERE user_id = $1 AND read_at IS NULL) AS notification_unread,
  (SELECT count(*)::int
   FROM announcements a
   LEFT JOIN announcement_receipts r
     ON r.announcement_id = a.id AND r.user_id = $1
   WHERE a.level IN ('important', 'critical')
     AND array_position(a.channels, 'message_center') IS NOT NULL
     AND a.status NOT IN ('draft', 'offline', 'archived')
     AND a.publish_at <= $2
     AND (a.expire_at IS NULL OR a.expire_at > $2)
     AND (
       a.audience_json->>'type' = 'all'
       OR EXISTS (
         SELECT 1
         FROM announcement_recipients recipient
         WHERE recipient.announcement_id = a.id
           AND recipient.user_id = $1
           AND recipient.announcement_version = a.version
       )
     )
     AND (
       r.announcement_id IS NULL
       OR r.announcement_version <> a.version
       OR r.read_at IS NULL
     )) AS important_announcement_unread,
  support_counts.feedback_unread,
  (support_counts.feedback_actions + support_counts.moderation_actions)::int AS support_action_count,
  0::int AS buyer_carpool_actions,
  (SELECT count(*)::int
   FROM api_orders
   WHERE buyer_user_id = $1
     AND (
       (status = 'pending_payment' AND payment_expires_at > $2)
       OR status = 'payment_issue'
       OR status = 'delivery_submitted'
     )) AS buyer_api_order_actions,
  (SELECT count(*)::int
   FROM carpool_applications application
   WHERE application.owner_user_id = $1
     AND application.status = 'pending_owner') AS merchant_carpool_actions,
  (SELECT count(*)::int
   FROM api_orders
   WHERE seller_user_id = $1
     AND status IN ('payment_submitted', 'paid_confirmed')) AS merchant_api_order_actions,
  CASE WHEN $3 THEN
    (SELECT count(*)::int FROM official_price_leads WHERE status = 'pending')
  ELSE 0 END AS admin_official_prices,
  CASE WHEN $3 THEN
    (SELECT count(*)::int FROM carpool_listings WHERE governance_status = 'removed')
  ELSE 0 END AS admin_carpools,
  CASE WHEN $3 THEN
    (SELECT count(*)::int
     FROM api_services
     WHERE review_status = 'pending_review'
       OR moderation_status = 'admin_suspended')
  ELSE 0 END AS admin_api_services,
  CASE WHEN $3 THEN
    (SELECT count(*)::int FROM feedback_tickets WHERE status IN ('submitted', 'following_up'))
  ELSE 0 END AS admin_feedback_tickets,
  CASE WHEN $3 THEN
    ((SELECT count(*)::int FROM reports WHERE status IN ('submitted', 'triaged'))
     + (SELECT count(*)::int FROM dispute_cases WHERE status = 'open')
     + (SELECT count(*)::int FROM appeals WHERE status = 'submitted'))::int
  ELSE 0 END AS admin_reports
FROM support_counts
`
