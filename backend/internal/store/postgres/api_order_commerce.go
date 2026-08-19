package postgres

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
)

func (s *Store) GetSellerCommerceStatus(ctx context.Context, sellerUserID string, now time.Time) (apiorder.SellerCommerceStatus, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiorder.SellerCommerceStatus{}, internalStoreError()
	}
	return sellerCommerceStatus(ctx, s.pool, sellerUserID, now)
}

func sellerCommerceStatus(ctx context.Context, q queryer, sellerUserID string, now time.Time) (apiorder.SellerCommerceStatus, *domain.AppError) {
	rows, err := queryRows(ctx, q, `
		SELECT api_order.dispute_case_id::text, api_order.id::text, api_order.order_no,
		       api_order.api_service_id::text, api_order.buyer_user_id::text,
		       api_order.dispute_status, COALESCE(dispute.next_actor, 'none'), dispute.due_at,
		       api_order.service_title_snapshot,
		       COALESCE((
		         SELECT remedy.source
		         FROM api_order_dispute_remedies remedy
		         WHERE remedy.dispute_case_id = api_order.dispute_case_id
		           AND remedy.status IN ('pending', 'claimed_fulfilled')
		         ORDER BY remedy.created_at DESC
		         LIMIT 1
		       ), '')
		FROM api_orders api_order
		JOIN dispute_cases dispute ON dispute.id = api_order.dispute_case_id
		WHERE api_order.seller_user_id = $1
		  AND api_order.dispute_case_id IS NOT NULL
		ORDER BY dispute.due_at NULLS LAST, api_order.updated_at DESC
	`, sellerUserID)
	if err != nil {
		return apiorder.SellerCommerceStatus{}, internalStoreError()
	}
	defer rows.Close()

	facts := make([]apiorder.DisputeCommerceFact, 0)
	for rows.Next() {
		var fact apiorder.DisputeCommerceFact
		if err := rows.Scan(
			&fact.DisputeID, &fact.OrderID, &fact.OrderNo, &fact.APIServiceID,
			&fact.BuyerUserID, &fact.DisputeStatus, &fact.NextActor, &fact.DueAt,
			&fact.ServiceTitle, &fact.RemedySource,
		); err != nil {
			return apiorder.SellerCommerceStatus{}, internalStoreError()
		}
		facts = append(facts, fact)
	}
	if err := rows.Err(); err != nil {
		return apiorder.SellerCommerceStatus{}, internalStoreError()
	}
	return apiorder.EvaluateSellerCommerce(facts, now), nil
}
