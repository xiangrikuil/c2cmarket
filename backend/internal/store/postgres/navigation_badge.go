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
SELECT
  (SELECT count(*)::int
   FROM notifications
   WHERE user_id = $1 AND read_at IS NULL) AS notification_unread,
  (SELECT count(*)::int
   FROM announcements a
   LEFT JOIN announcement_receipts r
     ON r.announcement_id = a.id AND r.user_id = $1
   WHERE a.level = 'important'
     AND array_position(a.channels, 'message_center') IS NOT NULL
     AND a.status NOT IN ('draft', 'offline', 'archived')
     AND a.publish_at <= $2
     AND (
       r.announcement_id IS NULL
       OR r.announcement_version <> a.version
       OR r.read_at IS NULL
     )) AS important_announcement_unread,
  (SELECT count(*)::int
   FROM feedback_tickets
   WHERE submitter_user_id = $1
     AND latest_admin_update_at IS NOT NULL
     AND (submitter_read_at IS NULL OR submitter_read_at < latest_admin_update_at)) AS feedback_unread,
  ((SELECT count(*)::int
    FROM carpool_applications application
    WHERE application.buyer_user_id = $1
      AND application.status = 'accepted_reserved'
      AND application.reservation_expires_at > $2
      AND application.join_confirmation_deadline > $2
      AND NOT EXISTS (
        SELECT 1
        FROM carpool_join_confirmations confirmation
        WHERE confirmation.carpool_application_id = application.id
          AND confirmation.actor_role = 'buyer'
      ))
   +
   (SELECT count(*)::int
    FROM carpool_memberships membership
    WHERE membership.buyer_user_id = $1
      AND membership.status = 'active'
      AND EXISTS (
        SELECT 1
        FROM carpool_completion_confirmations confirmation
        WHERE confirmation.carpool_membership_id = membership.id
          AND confirmation.actor_role = 'owner'
      )
      AND NOT EXISTS (
        SELECT 1
        FROM carpool_completion_confirmations confirmation
        WHERE confirmation.carpool_membership_id = membership.id
          AND confirmation.actor_role = 'buyer'
      )))::int AS buyer_carpool_actions,
  (SELECT count(*)::int
   FROM api_orders
   WHERE buyer_user_id = $1
     AND (
       (status = 'pending_payment' AND payment_expires_at > $2)
       OR status = 'payment_issue'
       OR status = 'delivery_submitted'
     )) AS buyer_api_order_actions,
  ((SELECT count(*)::int
    FROM carpool_applications application
    WHERE application.owner_user_id = $1
      AND application.status = 'pending_owner')
   +
   (SELECT count(*)::int
    FROM carpool_applications application
    WHERE application.owner_user_id = $1
      AND application.status = 'accepted_reserved'
      AND application.reservation_expires_at > $2
      AND application.join_confirmation_deadline > $2
      AND EXISTS (
        SELECT 1
        FROM carpool_join_confirmations confirmation
        WHERE confirmation.carpool_application_id = application.id
          AND confirmation.actor_role = 'buyer'
      )
      AND NOT EXISTS (
        SELECT 1
        FROM carpool_join_confirmations confirmation
        WHERE confirmation.carpool_application_id = application.id
          AND confirmation.actor_role = 'owner'
      ))
   +
   (SELECT count(*)::int
    FROM carpool_memberships membership
    WHERE membership.owner_user_id = $1
      AND membership.status = 'active'
      AND EXISTS (
        SELECT 1
        FROM carpool_completion_confirmations confirmation
        WHERE confirmation.carpool_membership_id = membership.id
          AND confirmation.actor_role = 'buyer'
      )
      AND NOT EXISTS (
        SELECT 1
        FROM carpool_completion_confirmations confirmation
        WHERE confirmation.carpool_membership_id = membership.id
          AND confirmation.actor_role = 'owner'
      )))::int AS merchant_carpool_actions,
  (SELECT count(*)::int
   FROM api_orders
   WHERE seller_user_id = $1
     AND status IN ('payment_submitted', 'paid_confirmed')) AS merchant_api_order_actions,
  CASE WHEN $3 THEN
    (SELECT count(*)::int FROM official_price_leads WHERE status = 'pending')
  ELSE 0 END AS admin_official_prices,
  CASE WHEN $3 THEN
    (SELECT count(*)::int FROM carpool_listings WHERE status = 'pending_review')
  ELSE 0 END AS admin_carpools,
  CASE WHEN $3 THEN
    (SELECT count(*)::int FROM api_services WHERE review_status = 'pending_review')
  ELSE 0 END AS admin_api_services,
  CASE WHEN $3 THEN
    (SELECT count(*)::int FROM feedback_tickets WHERE status IN ('submitted', 'following_up'))
  ELSE 0 END AS admin_feedback_tickets,
  CASE WHEN $3 THEN
    ((SELECT count(*)::int FROM reports WHERE status IN ('submitted', 'triaged'))
     + (SELECT count(*)::int FROM dispute_cases WHERE status = 'open')
     + (SELECT count(*)::int FROM appeals WHERE status = 'submitted'))::int
  ELSE 0 END AS admin_reports
`
